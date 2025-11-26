// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/hashicorp/go-plugin"
	backendplugin "github.com/hashicorp/vault/sdk/plugin"
)

func TestNewPluginHost(t *testing.T) {
	host, err := NewPluginHost("/fake/path", false, nil, "test")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	if host == nil {
		t.Fatal("NewPluginHost returned nil")
	}

	if host.pluginPath != "/fake/path" {
		t.Errorf("pluginPath = %s, want /fake/path", host.pluginPath)
	}

	if host.mountPath != "test" {
		t.Errorf("mountPath = %s, want test", host.mountPath)
	}

	if host.storage == nil {
		t.Error("storage is nil")
	}

	if host.logger == nil {
		t.Error("logger is nil")
	}

	if host.config == nil {
		t.Error("config is nil")
	}
}

func TestNewPluginHostWithConfig(t *testing.T) {
	config := map[string]string{
		"version": "2",
		"max_ttl": "3600",
	}

	host, err := NewPluginHost("/fake/path", true, config, "plugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	if len(host.config) != 2 {
		t.Errorf("config length = %d, want 2", len(host.config))
	}

	if host.config["version"] != "2" {
		t.Errorf("config[version] = %s, want 2", host.config["version"])
	}

	if host.config["max_ttl"] != "3600" {
		t.Errorf("config[max_ttl] = %s, want 3600", host.config["max_ttl"])
	}
}

func TestNewPluginHostWithNilConfig(t *testing.T) {
	host, err := NewPluginHost("/fake/path", false, nil, "plugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	if host.config == nil {
		t.Fatal("config should be initialized to empty map, not nil")
	}

	if len(host.config) != 0 {
		t.Errorf("config should be empty, got %d entries", len(host.config))
	}
}

func TestNewPluginHostVerboseLogging(t *testing.T) {
	host, err := NewPluginHost("/fake/path", true, nil, "plugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	if host.logger == nil {
		t.Fatal("logger is nil")
	}

	// Can't directly test log level without reflection, but ensure no panic
}

func TestVersionedPluginSet(t *testing.T) {
	if versionedPluginSet == nil {
		t.Fatal("versionedPluginSet is nil")
	}

	expectedVersions := []int{3, 4, 5}
	for _, version := range expectedVersions {
		pluginSet, ok := versionedPluginSet[version]
		if !ok {
			t.Errorf("versionedPluginSet missing version %d", version)
			continue
		}

		backend, ok := pluginSet["backend"]
		if !ok {
			t.Errorf("versionedPluginSet[%d] missing 'backend' plugin", version)
			continue
		}

		_, ok = backend.(*backendplugin.GRPCBackendPlugin)
		if !ok {
			t.Errorf("versionedPluginSet[%d]['backend'] is not *GRPCBackendPlugin", version)
		}
	}
}

func TestPluginHostStop(t *testing.T) {
	host, err := NewPluginHost("/fake/path", false, nil, "plugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	// Stop should not panic even when nothing is running
	host.Stop()

	// Verify fields are reset
	if host.backend != nil {
		t.Error("backend should be nil after Stop")
	}

	if host.client != nil {
		t.Error("client should be nil after Stop")
	}

	if host.pluginCmd != nil {
		t.Error("pluginCmd should be nil after Stop")
	}
}

func TestPluginHostStopMultipleTimes(t *testing.T) {
	host, err := NewPluginHost("/fake/path", false, nil, "plugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	// Multiple stops should not panic
	host.Stop()
	host.Stop()
	host.Stop()
}

func TestGetUsageInfo(t *testing.T) {
	host, err := NewPluginHost("/fake/path", false, nil, "myplugin")
	if err != nil {
		t.Fatalf("NewPluginHost failed: %v", err)
	}

	info := host.GetUsageInfo("8300")

	if info == "" {
		t.Error("GetUsageInfo returned empty string")
	}

	// Should contain system endpoints
	if !contains(info, "/v1/sys/health") {
		t.Error("Usage info missing /v1/sys/health endpoint")
	}

	if !contains(info, "/v1/sys/storage") {
		t.Error("Usage info missing /v1/sys/storage endpoint")
	}
}

func TestProtocolVersions(t *testing.T) {
	// Verify we support the expected protocol versions
	expectedVersions := []int{3, 4, 5}

	for _, version := range expectedVersions {
		if _, ok := versionedPluginSet[version]; !ok {
			t.Errorf("Missing support for protocol version %d", version)
		}
	}

	// Ensure each version has the backend plugin
	for version, pluginSet := range versionedPluginSet {
		if _, ok := pluginSet["backend"]; !ok {
			t.Errorf("Protocol version %d missing 'backend' plugin", version)
		}
	}
}

func TestPluginSetImplementation(t *testing.T) {
	for version, pluginSet := range versionedPluginSet {
		backend, ok := pluginSet["backend"]
		if !ok {
			t.Errorf("Version %d missing backend", version)
			continue
		}

		// Verify it implements plugin.Plugin interface
		_, ok = backend.(plugin.Plugin)
		if !ok {
			t.Errorf("Version %d backend does not implement plugin.Plugin", version)
		}

		// Verify it's specifically a GRPCBackendPlugin
		_, ok = backend.(*backendplugin.GRPCBackendPlugin)
		if !ok {
			t.Errorf("Version %d backend is not *GRPCBackendPlugin", version)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
