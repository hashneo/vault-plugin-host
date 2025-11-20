// Copyright (c) 2025 Steven Taylor
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parsePluginConfig parses the plugin configuration string
// Supports both JSON format: '{"key":"value"}' and key=value format: 'key1=value1,key2=value2'
func parsePluginConfig(configStr string) (map[string]string, error) {
	if configStr == "" {
		return make(map[string]string), nil
	}

	// Try to parse as JSON first
	if strings.HasPrefix(strings.TrimSpace(configStr), "{") {
		var jsonConfig map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &jsonConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config as JSON: %w", err)
		}

		// Convert to map[string]string
		result := make(map[string]string)
		for k, v := range jsonConfig {
			result[k] = fmt.Sprintf("%v", v)
		}
		return result, nil
	}

	// Parse as key=value pairs
	result := make(map[string]string)
	pairs := strings.Split(configStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config format: expected key=value, got %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		result[key] = value
	}

	return result, nil
}
