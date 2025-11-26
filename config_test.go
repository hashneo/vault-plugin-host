// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestParsePluginConfigEmpty(t *testing.T) {
	config, err := parsePluginConfig("")
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if config == nil {
		t.Fatal("parsePluginConfig returned nil")
	}

	if len(config) != 0 {
		t.Errorf("Expected empty config, got %d entries", len(config))
	}
}

func TestParsePluginConfigJSON(t *testing.T) {
	jsonStr := `{"version":"2","max_ttl":"3600"}`
	config, err := parsePluginConfig(jsonStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 2 {
		t.Errorf("Expected 2 config entries, got %d", len(config))
	}

	if config["version"] != "2" {
		t.Errorf("version = %s, want 2", config["version"])
	}

	if config["max_ttl"] != "3600" {
		t.Errorf("max_ttl = %s, want 3600", config["max_ttl"])
	}
}

func TestParsePluginConfigJSONWithSpaces(t *testing.T) {
	jsonStr := `  {"key": "value"}  `
	config, err := parsePluginConfig(jsonStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 1 {
		t.Errorf("Expected 1 config entry, got %d", len(config))
	}

	if config["key"] != "value" {
		t.Errorf("key = %s, want value", config["key"])
	}
}

func TestParsePluginConfigKeyValue(t *testing.T) {
	kvStr := "version=2,max_ttl=3600"
	config, err := parsePluginConfig(kvStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 2 {
		t.Errorf("Expected 2 config entries, got %d", len(config))
	}

	if config["version"] != "2" {
		t.Errorf("version = %s, want 2", config["version"])
	}

	if config["max_ttl"] != "3600" {
		t.Errorf("max_ttl = %s, want 3600", config["max_ttl"])
	}
}

func TestParsePluginConfigKeyValueWithSpaces(t *testing.T) {
	kvStr := "key1 = value1 , key2 = value2"
	config, err := parsePluginConfig(kvStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 2 {
		t.Errorf("Expected 2 config entries, got %d", len(config))
	}

	if config["key1"] != "value1" {
		t.Errorf("key1 = %s, want value1", config["key1"])
	}

	if config["key2"] != "value2" {
		t.Errorf("key2 = %s, want value2", config["key2"])
	}
}

func TestParsePluginConfigSingleKeyValue(t *testing.T) {
	kvStr := "version=2"
	config, err := parsePluginConfig(kvStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 1 {
		t.Errorf("Expected 1 config entry, got %d", len(config))
	}

	if config["version"] != "2" {
		t.Errorf("version = %s, want 2", config["version"])
	}
}

func TestParsePluginConfigInvalidJSON(t *testing.T) {
	jsonStr := `{"invalid json`
	_, err := parsePluginConfig(jsonStr)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestParsePluginConfigInvalidKeyValue(t *testing.T) {
	kvStr := "invalid-no-equals"
	_, err := parsePluginConfig(kvStr)
	if err == nil {
		t.Fatal("Expected error for invalid key=value format, got nil")
	}
}

func TestParsePluginConfigKeyValueWithEmptyEntries(t *testing.T) {
	kvStr := "key1=value1,,key2=value2"
	config, err := parsePluginConfig(kvStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if len(config) != 2 {
		t.Errorf("Expected 2 config entries, got %d", len(config))
	}
}

func TestParsePluginConfigJSONNumericValue(t *testing.T) {
	jsonStr := `{"port":8300,"enabled":true}`
	config, err := parsePluginConfig(jsonStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if config["port"] != "8300" {
		t.Errorf("port = %s, want 8300", config["port"])
	}

	if config["enabled"] != "true" {
		t.Errorf("enabled = %s, want true", config["enabled"])
	}
}

func TestParsePluginConfigKeyValueWithEqualsInValue(t *testing.T) {
	kvStr := "key=value=with=equals"
	config, err := parsePluginConfig(kvStr)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	if config["key"] != "value=with=equals" {
		t.Errorf("key = %s, want value=with=equals", config["key"])
	}
}
