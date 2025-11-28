// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/framework"
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

// LeaseInfo stores lease information
type LeaseInfo struct {
	LeaseID    string                 `json:"lease_id"`
	Path       string                 `json:"path"`
	Data       map[string]interface{} `json:"data"`
	Secret     *logical.Secret        `json:"secret"`
	IssueTime  time.Time              `json:"issue_time"`
	ExpireTime time.Time              `json:"expire_time"`
	Duration   time.Duration          `json:"duration"`
	Renewable  bool                   `json:"renewable"`
}

// Handler manages HTTP requests and forwards them to the plugin
type Handler struct {
	backend   PluginBackend
	storage   StorageView
	logger    hclog.Logger
	mountPath string
	mu        sync.RWMutex
	leases    map[string]*LeaseInfo // lease storage
	leaseMu   sync.RWMutex          // separate mutex for lease operations
}

// NewHandler creates a new HTTP handler
func NewHandler(backend PluginBackend, storage StorageView, logger hclog.Logger, mountPath string) *Handler {
	return &Handler{
		backend:   backend,
		storage:   storage,
		logger:    logger,
		mountPath: mountPath,
		leases:    make(map[string]*LeaseInfo),
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
		h.writeVaultError(w, http.StatusServiceUnavailable, "plugin not started")
		return
	}
	backend := h.backend
	h.mu.RUnlock()

	// Parse the request
	var requestData map[string]interface{}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to read body: %v", err))
			return
		}

		if len(body) > 0 {
			if err := json.Unmarshal(body, &requestData); err != nil {
				h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse JSON: %v", err))
				return
			}
		}
	}

	// Extract path (remove leading /v1/ and mount prefix)
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	path = strings.TrimPrefix(path, h.mountPath+"/")

	// Determine operation type
	var operation logical.Operation

	// Check for special lifecycle operations based on path or query parameters
	if strings.HasSuffix(path, "/revoke") || r.URL.Query().Get("operation") == "revoke" {
		operation = logical.RevokeOperation
		// Remove /revoke suffix from path if present
		path = strings.TrimSuffix(path, "/revoke")
	} else if strings.HasSuffix(path, "/renew") || r.URL.Query().Get("operation") == "renew" {
		operation = logical.RenewOperation
		// Remove /renew suffix from path if present
		path = strings.TrimSuffix(path, "/renew")
	} else if strings.HasSuffix(path, "/rollback") || r.URL.Query().Get("operation") == "rollback" {
		operation = logical.RollbackOperation
		// Remove /rollback suffix from path if present
		path = strings.TrimSuffix(path, "/rollback")
	} else if strings.HasSuffix(path, "/rotate") || r.URL.Query().Get("operation") == "rotate" {
		operation = logical.RotationOperation
		// Remove /rotate suffix from path if present
		path = strings.TrimSuffix(path, "/rotate")
	} else {
		// Standard HTTP method-based operations
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
			h.writeVaultError(w, http.StatusMethodNotAllowed, "unsupported method")
			return
		}
	}

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
			h.writeVaultError(w, http.StatusForbidden, "permission denied")
			return
		}

		h.writeVaultError(w, http.StatusInternalServerError, err.Error())
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

			// Generate lease for read operations that return data
			if operation == logical.ReadOperation {
				leaseID := h.generateLeaseID(path)
				leaseDuration := 24 * time.Hour // Default 24 hour lease
				if resp.Secret != nil && resp.Secret.TTL > 0 {
					leaseDuration = resp.Secret.TTL
				}

				// Store lease information
				leaseInfo := &LeaseInfo{
					LeaseID:    leaseID,
					Path:       path,
					Data:       resp.Data,
					Secret:     resp.Secret, // Store the secret for renewal/revocation
					IssueTime:  time.Now(),
					ExpireTime: time.Now().Add(leaseDuration),
					Duration:   leaseDuration,
					Renewable:  true,
				}

				h.leaseMu.Lock()
				h.leases[leaseID] = leaseInfo
				h.leaseMu.Unlock()

				// Add lease information to response (matching Vault format)
				response["lease_id"] = leaseID
				response["lease_duration"] = int(leaseDuration.Seconds())
				response["renewable"] = true
			}
		}

		// Only include warnings if they exist
		if len(resp.Warnings) > 0 {
			response["warnings"] = resp.Warnings
		}

		// Add standard Vault response fields
		response["request_id"] = h.generateRequestID()
		response["wrap_info"] = nil
		response["auth"] = nil
		// Add mount type from the mount path (the part before the first slash)
		response["mount_type"] = strings.TrimPrefix(h.mountPath, "/")
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
		h.writeVaultError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list storage: %v", err))
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

// HandleOpenAPI returns the OpenAPI document from the plugin with corrected paths
func (h *Handler) HandleOpenAPI(w http.ResponseWriter, r *http.Request, oasDoc interface{}) {
	if oasDoc == nil {
		h.writeVaultError(w, http.StatusNotFound, "OpenAPI document not available")
		return
	}

	// Convert to map so we can modify the paths
	var docMap map[string]interface{}

	switch v := oasDoc.(type) {
	case map[string]interface{}:
		docMap = v
	case *framework.OASDocument:
		// Convert OASDocument to map
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			h.logger.Error("Failed to marshal OpenAPI document", "error", err)
			h.writeVaultError(w, http.StatusInternalServerError, "Failed to process OpenAPI document")
			return
		}
		if err := json.Unmarshal(jsonBytes, &docMap); err != nil {
			h.logger.Error("Failed to unmarshal OpenAPI document", "error", err)
			h.writeVaultError(w, http.StatusInternalServerError, "Failed to process OpenAPI document")
			return
		}
	default:
		// Try to marshal and unmarshal as fallback
		jsonBytes, err := json.Marshal(oasDoc)
		if err != nil {
			h.logger.Error("Failed to marshal OpenAPI document", "error", err)
			h.writeVaultError(w, http.StatusInternalServerError, "Failed to process OpenAPI document")
			return
		}
		if err := json.Unmarshal(jsonBytes, &docMap); err != nil {
			h.logger.Error("Failed to unmarshal OpenAPI document", "error", err)
			h.writeVaultError(w, http.StatusInternalServerError, "Failed to process OpenAPI document")
			return
		}
	}

	// Fix the paths to include /v1/{mount} prefix
	if paths, ok := docMap["paths"].(map[string]interface{}); ok {
		newPaths := make(map[string]interface{})
		for path, pathItem := range paths {
			// Add /v1/{mount} prefix to each path
			newPath := fmt.Sprintf("/v1/%s%s", h.mountPath, path)
			newPaths[newPath] = pathItem
		}
		docMap["paths"] = newPaths
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docMap)
}

// generateLeaseID generates a unique lease ID in Vault format
func (h *Handler) generateLeaseID(path string) string {
	bytes := make([]byte, 12) // Generate 12 random bytes for the suffix
	rand.Read(bytes)
	// Create lease ID in format: mountPath/path/randomString
	// Remove leading slash from mountPath if present
	mountPath := strings.TrimPrefix(h.mountPath, "/")
	leaseID := fmt.Sprintf("%s/%s/%s", mountPath, path, hex.EncodeToString(bytes))
	return leaseID
}

// generateRequestID generates a unique request ID in UUID format
func (h *Handler) generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	// Format as UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// writeVaultError writes an error response in Vault's JSON format
func (h *Handler) writeVaultError(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := map[string]interface{}{
		"errors": []string{message},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}

// HandleLeaseRenew handles lease renewal requests at /v1/sys/leases/renew
func (h *Handler) HandleLeaseRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		h.writeVaultError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to read body: %v", err))
		return
	}

	var requestData map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &requestData); err != nil {
			h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse JSON: %v", err))
			return
		}
	}

	// Get lease_id from request
	leaseID, ok := requestData["lease_id"].(string)
	if !ok || leaseID == "" {
		h.writeVaultError(w, http.StatusBadRequest, "lease_id is required")
		return
	}

	// Get optional increment
	incrementVal := requestData["increment"]
	var increment time.Duration
	if incrementVal != nil {
		switch v := incrementVal.(type) {
		case float64:
			increment = time.Duration(v) * time.Second
		case string:
			if parsed, err := time.ParseDuration(v); err == nil {
				increment = parsed
			} else if seconds, err := strconv.Atoi(v); err == nil {
				increment = time.Duration(seconds) * time.Second
			}
		}
	}

	// Default increment is the original lease duration
	if increment == 0 {
		increment = 24 * time.Hour
	}

	h.leaseMu.Lock()
	leaseInfo, exists := h.leases[leaseID]
	h.leaseMu.Unlock()

	if !exists {
		h.writeVaultError(w, http.StatusNotFound, "lease not found")
		return
	}

	// Check if lease is renewable
	if !leaseInfo.Renewable {
		h.writeVaultError(w, http.StatusBadRequest, "lease is not renewable")
		return
	}

	// Check if lease has already expired
	if time.Now().After(leaseInfo.ExpireTime) {
		h.writeVaultError(w, http.StatusBadRequest, "lease has expired")
		return
	}

	// Calculate new expiration time but don't update yet
	newExpireTime := time.Now().Add(increment)

	// Notify plugin backend about lease renewal
	h.mu.RLock()
	backend := h.backend
	h.mu.RUnlock()

	if backend != nil {
		// Create renewal request for the plugin
		renewReq := &logical.Request{
			Operation: logical.RenewOperation,
			Path:      leaseInfo.Path,
			Storage:   h.storage,
			Secret:    leaseInfo.Secret, // Include the secret
			Data: map[string]interface{}{
				"lease_id":   leaseID,
				"increment":  int(increment.Seconds()),
				"issue_time": leaseInfo.IssueTime,
			},
		}

		ctx := context.Background()
		if _, err := backend.HandleRequest(ctx, renewReq); err != nil {
			h.logger.Error("plugin renewal notification failed", "error", err, "lease_id", leaseID)
			h.writeVaultError(w, http.StatusInternalServerError, fmt.Sprintf("failed to renew lease: %v", err))
			return
		}
	}

	// Plugin succeeded, now update the lease
	h.leaseMu.Lock()
	h.leases[leaseID].ExpireTime = newExpireTime
	h.leases[leaseID].Duration = increment
	h.leaseMu.Unlock()

	h.logger.Info("lease renewed", "lease_id", leaseID, "increment", increment, "new_expire_time", newExpireTime)

	// Build response
	response := map[string]interface{}{
		"lease_id":       leaseID,
		"lease_duration": int(increment.Seconds()),
		"renewable":      true,
		"data":           leaseInfo.Data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleLeaseRevoke handles lease revocation requests at /v1/sys/leases/revoke
func (h *Handler) HandleLeaseRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		h.writeVaultError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to read body: %v", err))
		return
	}

	var requestData map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &requestData); err != nil {
			h.writeVaultError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse JSON: %v", err))
			return
		}
	}

	// Get lease_id from request
	leaseID, ok := requestData["lease_id"].(string)
	if !ok || leaseID == "" {
		h.writeVaultError(w, http.StatusBadRequest, "lease_id is required")
		return
	}

	h.leaseMu.Lock()
	leaseInfo, exists := h.leases[leaseID]
	if exists {
		delete(h.leases, leaseID)
	}
	h.leaseMu.Unlock()

	if !exists {
		h.writeVaultError(w, http.StatusNotFound, "lease not found")
		return
	}

	// Notify plugin backend about lease revocation
	h.mu.RLock()
	backend := h.backend
	h.mu.RUnlock()

	if backend != nil {
		// Create revocation request for the plugin
		revokeReq := &logical.Request{
			Operation: logical.RevokeOperation,
			Path:      leaseInfo.Path,
			Storage:   h.storage,
			Secret:    leaseInfo.Secret, // Include the secret
			Data: map[string]interface{}{
				"lease_id":   leaseID,
				"issue_time": leaseInfo.IssueTime,
				"data":       leaseInfo.Data,
			},
		}

		ctx := context.Background()
		if _, err := backend.HandleRequest(ctx, revokeReq); err != nil {
			h.logger.Error("plugin revocation notification failed", "error", err, "lease_id", leaseID)
			// Continue with the response even if plugin notification fails
		}
	}

	h.logger.Info("lease revoked", "lease_id", leaseID)

	w.WriteHeader(http.StatusNoContent)
}

// HandleLeaseRevokeByPath handles lease revocation requests at /v1/sys/leases/revoke/<lease_id>
func (h *Handler) HandleLeaseRevokeByPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		h.writeVaultError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract lease_id from URL path
	// URL format: /v1/sys/leases/revoke/plugin/creds/test/92b21cb389fd74c4bf558d44
	path := strings.TrimPrefix(r.URL.Path, "/v1/sys/leases/revoke/")
	if path == "" {
		h.writeVaultError(w, http.StatusBadRequest, "lease_id is required in path")
		return
	}

	leaseID := path

	h.leaseMu.Lock()
	leaseInfo, exists := h.leases[leaseID]
	if exists {
		delete(h.leases, leaseID)
	}
	h.leaseMu.Unlock()

	if !exists {
		h.writeVaultError(w, http.StatusNotFound, "lease not found")
		return
	}

	// Notify plugin backend about lease revocation
	h.mu.RLock()
	backend := h.backend
	h.mu.RUnlock()

	if backend != nil {
		// Create revocation request for the plugin
		revokeReq := &logical.Request{
			Operation: logical.RevokeOperation,
			Path:      leaseInfo.Path,
			Storage:   h.storage,
			Secret:    leaseInfo.Secret, // Include the secret
			Data: map[string]interface{}{
				"lease_id":   leaseID,
				"issue_time": leaseInfo.IssueTime,
				"data":       leaseInfo.Data,
			},
		}

		ctx := context.Background()
		if _, err := backend.HandleRequest(ctx, revokeReq); err != nil {
			h.logger.Error("plugin revocation notification failed", "error", err, "lease_id", leaseID)
			// Continue with the response even if plugin notification fails
		}
	}

	h.logger.Info("lease revoked", "lease_id", leaseID)

	w.WriteHeader(http.StatusNoContent)
}
