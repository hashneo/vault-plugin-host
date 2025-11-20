// Copyright (c) 2025 Steven Taylor
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
)

func TestNewInMemoryStorage(t *testing.T) {
	storage := NewInMemoryStorage()
	if storage == nil {
		t.Fatal("NewInMemoryStorage returned nil")
	}
	if storage.data == nil {
		t.Fatal("storage.data map is nil")
	}
}

func TestStoragePutAndGet(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	// Test Put
	entry := &logical.StorageEntry{
		Key:   "test-key",
		Value: []byte("test-value"),
	}

	err := storage.Put(ctx, entry)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test Get
	retrieved, err := storage.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Get returned nil for existing key")
	}

	if retrieved.Key != entry.Key {
		t.Errorf("Key mismatch: got %s, want %s", retrieved.Key, entry.Key)
	}

	if string(retrieved.Value) != string(entry.Value) {
		t.Errorf("Value mismatch: got %s, want %s", retrieved.Value, entry.Value)
	}
}

func TestStorageGetNonExistent(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	retrieved, err := storage.Get(ctx, "non-existent-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved != nil {
		t.Errorf("Get returned non-nil for non-existent key: %v", retrieved)
	}
}

func TestStorageDelete(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	// Put an entry
	entry := &logical.StorageEntry{
		Key:   "test-key",
		Value: []byte("test-value"),
	}
	err := storage.Put(ctx, entry)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Delete it
	err = storage.Delete(ctx, "test-key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	retrieved, err := storage.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}

	if retrieved != nil {
		t.Errorf("Get returned non-nil after delete: %v", retrieved)
	}
}

func TestStorageList(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	// Put multiple entries with same prefix
	entries := []*logical.StorageEntry{
		{Key: "prefix/key1", Value: []byte("value1")},
		{Key: "prefix/key2", Value: []byte("value2")},
		{Key: "prefix/subdir/key3", Value: []byte("value3")},
		{Key: "other/key4", Value: []byte("value4")},
	}

	for _, entry := range entries {
		err := storage.Put(ctx, entry)
		if err != nil {
			t.Fatalf("Put failed for %s: %v", entry.Key, err)
		}
	}

	// List with prefix
	keys, err := storage.List(ctx, "prefix/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("List returned %d keys, want 3", len(keys))
	}

	// Verify all keys start with prefix
	for _, key := range keys {
		if len(key) < 7 || key[:7] != "prefix/" {
			t.Errorf("Key %s does not start with 'prefix/'", key)
		}
	}
}

func TestStorageListEmpty(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	keys, err := storage.List(ctx, "nonexistent/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("List returned %d keys for empty storage, want 0", len(keys))
	}
}

func TestStorageConcurrency(t *testing.T) {
	storage := NewInMemoryStorage()
	ctx := context.Background()

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			entry := &logical.StorageEntry{
				Key:   "concurrent-key",
				Value: []byte("value"),
			}
			storage.Put(ctx, entry)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func(n int) {
			storage.Get(ctx, "concurrent-key")
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data integrity
	retrieved, err := storage.Get(ctx, "concurrent-key")
	if err != nil {
		t.Fatalf("Get after concurrent access failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Get returned nil after concurrent writes")
	}
}
