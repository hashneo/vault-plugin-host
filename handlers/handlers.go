// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
)

// PluginBackend interface defines the methods needed from the plugin
type PluginBackend interface {
	HandleRequest(ctx context.Context, req *logical.Request) (*logical.Response, error)
}

// StorageView interface defines the methods needed from storage
type StorageView interface {
	List(ctx context.Context, prefix string) ([]string, error)
	Get(ctx context.Context, key string) (*logical.StorageEntry, error)
	Put(ctx context.Context, entry *logical.StorageEntry) error
	Delete(ctx context.Context, key string) error
}

// Handler manages HTTP requests and forwards them to the plugin
type Handler struct {
	backend   PluginBackend
	storage   StorageView
	logger    hclog.Logger
	mountPath string
	mu        sync.RWMutex
}

// NewHandler creates a new HTTP handler
func NewHandler(backend PluginBackend, storage StorageView, logger hclog.Logger, mountPath string) *Handler {
	return &Handler{
		backend:   backend,
		storage:   storage,
		logger:    logger,
		mountPath: mountPath,
	}
}

// SetBackend updates the backend (used after plugin starts)
func (h *Handler) SetBackend(backend PluginBackend) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.backend = backend
}

// HandleRequest handles an HTTP request and forwards it to the plugin
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	if h.backend == nil {
		h.mu.RUnlock()
		http.Error(w, "plugin not started", http.StatusServiceUnavailable)
		return
	}
	backend := h.backend
	h.mu.RUnlock()

	// Parse the request
	var requestData map[string]interface{}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
			return
		}

		if len(body) > 0 {
			if err := json.Unmarshal(body, &requestData); err != nil {
				http.Error(w, fmt.Sprintf("failed to parse JSON: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	// Determine operation type
	var operation logical.Operation
	switch r.Method {
	case http.MethodGet:
		operation = logical.ReadOperation
	case http.MethodPost, http.MethodPut:
		operation = logical.UpdateOperation
	case http.MethodDelete:
		operation = logical.DeleteOperation
	case "LIST":
		operation = logical.ListOperation
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract path (remove leading /v1/ and mount prefix)
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	path = strings.TrimPrefix(path, h.mountPath+"/")

	h.logger.Debug("handling request", "method", r.Method, "path", path, "operation", operation)

	// Create logical request
	req := &logical.Request{
		Operation: operation,
		Path:      path,
		Storage:   h.storage,
		Data:      requestData,
	}

	// Handle the request
	ctx := context.Background()
	resp, err := backend.HandleRequest(ctx, req)

	if err != nil {
		h.logger.Error("request failed", "error", err)

		// Check if it's a permission denied error
		if err == logical.ErrPermissionDenied {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response
	response := make(map[string]interface{})

	if resp != nil {
		if resp.Auth != nil {
			authData := map[string]interface{}{
				"client_token":   "mock-token-" + time.Now().Format("20060102150405"),
				"accessor":       "mock-accessor",
				"policies":       resp.Auth.Policies,
				"metadata":       resp.Auth.Metadata,
				"lease_duration": int(resp.Auth.TTL.Seconds()),
				"renewable":      resp.Auth.Renewable,
			}
			response["auth"] = authData
		}

		if resp.Data != nil {
			response["data"] = resp.Data
		}

		if resp.Secret != nil {
			response["secret"] = resp.Secret
		}

		if len(resp.Warnings) > 0 {
			response["warnings"] = resp.Warnings
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleHealth handles health check requests
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	running := h.backend != nil
	h.mu.RUnlock()

	// Get storage entry count
	ctx := context.Background()
	keys, err := h.storage.List(ctx, "")
	entryCount := 0
	if err == nil {
		entryCount = len(keys)
	}

	status := map[string]interface{}{
		"plugin_running":  running,
		"storage_entries": entryCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleStorage shows storage contents
func (h *Handler) HandleStorage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	keys, err := h.storage.List(ctx, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list storage: %v", err), http.StatusInternalServerError)
		return
	}

	// Return array of key-value objects for frontend
	data := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		entry, err := h.storage.Get(ctx, key)
		if err != nil {
			h.logger.Warn("failed to get storage entry", "key", key, "error", err)
			continue
		}
		if entry != nil {
			data = append(data, map[string]string{
				"key":   key,
				"value": string(entry.Value),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// HandleOpenAPI returns the OpenAPI document from the plugin
func (h *Handler) HandleOpenAPI(w http.ResponseWriter, r *http.Request, oasDoc interface{}) {
	if oasDoc == nil {
		http.Error(w, "OpenAPI document not available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oasDoc)
}
