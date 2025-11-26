// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
)

func TestHandleHealthWithoutBackend(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	req := httptest.NewRequest("GET", "/v1/sys/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if running, ok := response["plugin_running"].(bool); !ok || running {
		t.Error("plugin_running should be false when backend is nil")
	}

	if entries, ok := response["storage_entries"].(float64); !ok || entries != 0 {
		t.Errorf("storage_entries = %v, want 0", entries)
	}
}

func TestHandleStorageEmpty(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	req := httptest.NewRequest("GET", "/v1/sys/storage", nil)
	w := httptest.NewRecorder()

	handler.HandleStorage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	storageData, ok := response["storage"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'storage' field")
	}

	if len(storageData) != 0 {
		t.Errorf("storage should be empty, got %d entries", len(storageData))
	}
}

func TestHandleStorageWithData(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	// Put some data in storage
	ctx := context.Background()
	entry := &logical.StorageEntry{
		Key:   "test-key",
		Value: []byte("test-value"),
	}
	if err := storage.Put(ctx, entry); err != nil {
		t.Fatalf("Failed to put test data: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/sys/storage", nil)
	w := httptest.NewRecorder()

	handler.HandleStorage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	storageData, ok := response["storage"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'storage' field")
	}

	if len(storageData) != 1 {
		t.Errorf("storage should have 1 entry, got %d", len(storageData))
	}

	if val, ok := storageData["test-key"].(string); !ok || val != "test-value" {
		t.Errorf("storage[test-key] = %v, want test-value", val)
	}
}

func TestHandleRequestWithoutBackend(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	req := httptest.NewRequest("GET", "/v1/plugin/test", nil)
	w := httptest.NewRecorder()

	handler.HandleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	if !bytes.Contains(w.Body.Bytes(), []byte("plugin not started")) {
		t.Error("Response should contain 'plugin not started'")
	}
}

func TestHandleRequestInvalidJSON(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	backend := &mockBackend{}
	handler := NewHandler(backend, storage, logger, "plugin")

	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest("POST", "/v1/plugin/test", invalidJSON)
	w := httptest.NewRecorder()

	handler.HandleRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	if !bytes.Contains(w.Body.Bytes(), []byte("failed to parse JSON")) {
		t.Error("Response should contain 'failed to parse JSON'")
	}
}

func TestHandleRequestUnsupportedMethod(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	backend := &mockBackend{}
	handler := NewHandler(backend, storage, logger, "plugin")

	req := httptest.NewRequest("PATCH", "/v1/plugin/test", nil)
	w := httptest.NewRecorder()

	handler.HandleRequest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRequestMethodMapping(t *testing.T) {
	tests := []struct {
		method            string
		expectedOperation logical.Operation
	}{
		{"GET", logical.ReadOperation},
		{"POST", logical.UpdateOperation},
		{"PUT", logical.UpdateOperation},
		{"DELETE", logical.DeleteOperation},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			storage := newMockStorage()
			logger := hclog.NewNullLogger()
			mock := &mockBackend{expectedOp: tt.expectedOperation}
			handler := NewHandler(mock, storage, logger, "plugin")

			var body *bytes.Buffer
			if tt.method == "POST" || tt.method == "PUT" {
				body = bytes.NewBufferString(`{"key":"value"}`)
			} else {
				body = bytes.NewBufferString("")
			}

			req := httptest.NewRequest(tt.method, "/v1/plugin/test", body)
			w := httptest.NewRecorder()

			handler.HandleRequest(w, req)

			if !mock.called {
				t.Error("Backend HandleRequest was not called")
			}

			if mock.receivedOp != tt.expectedOperation {
				t.Errorf("Operation = %v, want %v", mock.receivedOp, tt.expectedOperation)
			}
		})
	}
}

func TestContentTypeHeaders(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	t.Run("HandleHealth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/sys/health", nil)
		w := httptest.NewRecorder()
		handler.HandleHealth(w, req)

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", contentType)
		}
	})

	t.Run("HandleStorage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/sys/storage", nil)
		w := httptest.NewRecorder()
		handler.HandleStorage(w, req)

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", contentType)
		}
	})
}

func TestSetBackend(t *testing.T) {
	storage := newMockStorage()
	logger := hclog.NewNullLogger()
	handler := NewHandler(nil, storage, logger, "plugin")

	if handler.backend != nil {
		t.Error("Backend should be nil initially")
	}

	backend := &mockBackend{}
	handler.SetBackend(backend)

	if handler.backend == nil {
		t.Error("Backend should be set after SetBackend")
	}

	// Verify it's usable
	req := httptest.NewRequest("GET", "/v1/plugin/test", nil)
	w := httptest.NewRecorder()
	handler.HandleRequest(w, req)

	if w.Code == http.StatusServiceUnavailable {
		t.Error("Should not return ServiceUnavailable after backend is set")
	}
}

// mockBackend implements PluginBackend for testing
type mockBackend struct {
	called     bool
	expectedOp logical.Operation
	receivedOp logical.Operation
}

func (m *mockBackend) HandleRequest(ctx context.Context, req *logical.Request) (*logical.Response, error) {
	m.called = true
	m.receivedOp = req.Operation
	return &logical.Response{
		Data: map[string]interface{}{
			"test": "response",
		},
	}, nil
}

// mockStorage implements StorageView for testing
type mockStorage struct {
	data map[string]*logical.StorageEntry
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		data: make(map[string]*logical.StorageEntry),
	}
}

func (s *mockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range s.data {
		if prefix == "" || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *mockStorage) Get(ctx context.Context, key string) (*logical.StorageEntry, error) {
	entry, ok := s.data[key]
	if !ok {
		return nil, nil
	}
	return entry, nil
}

func (s *mockStorage) Put(ctx context.Context, entry *logical.StorageEntry) error {
	s.data[entry.Key] = entry
	return nil
}

func (s *mockStorage) Delete(ctx context.Context, key string) error {
	delete(s.data, key)
	return nil
}
