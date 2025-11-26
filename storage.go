// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/logical"
)

// InMemoryStorage implements logical.Storage
type InMemoryStorage struct {
	data map[string]*logical.StorageEntry
	mu   sync.RWMutex
}

// NewInMemoryStorage creates a new in-memory storage instance
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string]*logical.StorageEntry),
	}
}

func (s *InMemoryStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *InMemoryStorage) Get(ctx context.Context, key string) (*logical.StorageEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok {
		return nil, nil
	}
	return entry, nil
}

func (s *InMemoryStorage) Put(ctx context.Context, entry *logical.StorageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[entry.Key] = entry
	return nil
}

func (s *InMemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}
