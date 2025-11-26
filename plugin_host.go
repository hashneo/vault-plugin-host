// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"vault-plugin-host/handlers"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	backendplugin "github.com/hashicorp/vault/sdk/plugin"
)

// versionedPluginSet creates a plugin set that supports versions 3, 4, and 5
var versionedPluginSet = map[int]plugin.PluginSet{
	3: {
		"backend": &backendplugin.GRPCBackendPlugin{},
	},
	4: {
		"backend": &backendplugin.GRPCBackendPlugin{},
	},
	5: {
		"backend": &backendplugin.GRPCBackendPlugin{},
	},
}

// PluginHost manages the plugin lifecycle and HTTP server
type PluginHost struct {
	backend    logical.Backend
	client     *plugin.Client
	pluginCmd  *exec.Cmd
	storage    *InMemoryStorage
	logger     hclog.Logger
	pluginPath string
	config     map[string]string
	mountPath  string
	oasDoc     *framework.OASDocument
	handler    *handlers.Handler
	mu         sync.RWMutex
}

// NewPluginHost creates a new plugin host
func NewPluginHost(pluginPath string, verbose bool, config map[string]string, mountPath string) (*PluginHost, error) {
	logLevel := hclog.Info
	if verbose {
		logLevel = hclog.Debug
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin-host",
		Level:  logLevel,
		Output: os.Stdout,
	})

	if config == nil {
		config = make(map[string]string)
	}

	storage := NewInMemoryStorage()
	handler := handlers.NewHandler(nil, storage, logger, mountPath)

	return &PluginHost{
		pluginPath: pluginPath,
		storage:    storage,
		logger:     logger,
		config:     config,
		mountPath:  mountPath,
		handler:    handler,
	}, nil
}

// Start launches the plugin process
func (h *PluginHost) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.backend != nil {
		return fmt.Errorf("plugin already started")
	}

	pluginLogger := h.logger.Named("plugin")

	var cmd *exec.Cmd
	var clientConfig *plugin.ClientConfig

	// Check if attach string was provided via command-line flag
	if attachString != nil && *attachString != "" {
		h.logger.Info("parsing plugin attach string", "string", *attachString)

		parts := strings.Split(strings.TrimSuffix(*attachString, "|"), "|")
		if len(parts) < 5 {
			return fmt.Errorf("invalid attach string format, expected 'version|maxversion|network|socket|protocol|'")
		}

		protoVersion, _ := strconv.Atoi(parts[1])
		network := parts[2]
		socketPath := parts[3]
		protoType := parts[4]

		var protocol plugin.Protocol
		if protoType == "grpc" {
			protocol = plugin.ProtocolGRPC
		} else {
			protocol = plugin.ProtocolNetRPC
		}

		h.logger.Info("attaching to plugin",
			"socket", socketPath,
			"protocol", protoType,
			"version", protoVersion)

		external := &plugin.ExternalConfig{
			Protocol:        protocol,
			ProtocolVersion: protoVersion,
			Addr: &net.UnixAddr{
				Name: socketPath,
				Net:  network,
			},
		}

		clientConfig = &plugin.ClientConfig{
			HandshakeConfig:  backendplugin.HandshakeConfig,
			VersionedPlugins: versionedPluginSet,
			External:         external,
			Logger:           pluginLogger,
		}
	} else {
		// Start plugin process manually and capture reattach info
		h.logger.Info("starting plugin process manually to capture reattach info")

		cmd = exec.Command(h.pluginPath)
		cmd.Env = append(os.Environ(),
			"PLUGIN_PROTOCOL_VERSIONS=4",
			"VAULT_BACKEND_PLUGIN=6669da05-b1c8-4f49-97d9-c8e5bed98e20",
			"VAULT_PLUGIN_AUTOMTLS_ENABLED=true",
			"VAULT_VERSION=1.18.0",
		)

		// Capture stdout to get reattach info
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		// Start the process
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start plugin process: %w", err)
		}

		// Read the reattach string from stdout
		scanner := bufio.NewScanner(stdout)
		var reattachInfo string
		for scanner.Scan() {
			line := scanner.Text()
			h.logger.Debug("plugin output", "line", line)
			// Look for the reattach string (format: 1|5|unix|/path|grpc|)
			if strings.Contains(line, "|unix|") || strings.Contains(line, "|tcp|") {
				reattachInfo = strings.TrimSpace(line)
				break
			}
		}

		if reattachInfo == "" {
			cmd.Process.Kill()
			return fmt.Errorf("failed to get reattach info from plugin")
		}

		h.logger.Info("captured reattach info", "info", reattachInfo)

		// Parse the reattach string
		parts := strings.Split(strings.TrimSuffix(reattachInfo, "|"), "|")
		if len(parts) < 5 {
			cmd.Process.Kill()
			return fmt.Errorf("invalid reattach string format: %s", reattachInfo)
		}

		protoVersion, _ := strconv.Atoi(parts[1])
		network := parts[2]
		socketPath := parts[3]
		protoType := parts[4]

		var protocol plugin.Protocol
		if protoType == "grpc" {
			protocol = plugin.ProtocolGRPC
		} else {
			protocol = plugin.ProtocolNetRPC
		}

		// Store the command so we can kill it later
		h.mu.Unlock()
		h.pluginCmd = cmd
		h.mu.Lock()

		external := &plugin.ExternalConfig{
			Protocol:        protocol,
			ProtocolVersion: protoVersion,
			Addr: &net.UnixAddr{
				Name: socketPath,
				Net:  network,
			},
		}

		clientConfig = &plugin.ClientConfig{
			HandshakeConfig:  backendplugin.HandshakeConfig,
			VersionedPlugins: versionedPluginSet,
			External:         external,
			Logger:           pluginLogger,
		}
	}

	client := plugin.NewClient(clientConfig)

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	h.logger.Debug("attempting to dispense backend plugin",
		"external", clientConfig.External != nil,
		"versions", fmt.Sprintf("%v", clientConfig.VersionedPlugins))

	raw, err := rpcClient.Dispense("backend")
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	backend, ok := raw.(logical.Backend)
	if !ok {
		client.Kill()
		return fmt.Errorf("dispensed plugin is not a logical.Backend")
	}

	systemView := &TestSystemView{}
	backendConfig := &logical.BackendConfig{
		BackendUUID:         "6669da05-b1c8-4f49-97d9-c8e5bed98e20",
		StorageView:         h.storage,
		Logger:              pluginLogger,
		System:              systemView,
		Config:              h.config,
		EventsSender:        nil,
		ObservationRecorder: nil,
	}

	if err := backend.Setup(context.Background(), backendConfig); err != nil {
		client.Kill()
		return fmt.Errorf("failed to setup backend: %w", err)
	}

	h.backend = backend
	h.client = client
	h.handler.SetBackend(backend)

	// Initialize backend lifecycle functions
	h.initializeBackendLifecycle(backend)

	h.logger.Info("plugin started successfully")

	// List all available paths from the plugin
	h.listPluginPaths()

	return nil
}

// Stop stops the plugin
func (h *PluginHost) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.backend != nil {
		// Call cleanup lifecycle functions
		h.cleanupBackendLifecycle()
		h.backend.Cleanup(context.Background())
	}
	if h.client != nil {
		h.client.Kill()
	}
	if h.pluginCmd != nil && h.pluginCmd.Process != nil {
		h.logger.Info("killing manually started plugin process", "pid", h.pluginCmd.Process.Pid)
		h.pluginCmd.Process.Kill()
		h.pluginCmd.Wait() // Clean up zombie process
	}

	h.backend = nil
	h.client = nil
	h.pluginCmd = nil
	h.handler.SetBackend(nil)
	h.logger.Info("plugin stopped")
}

// initializeBackendLifecycle initializes backend lifecycle functions
func (h *PluginHost) initializeBackendLifecycle(backend logical.Backend) {
	ctx := context.Background()

	// Call Initialize method (standard logical.Backend interface)
	h.logger.Info("calling backend Initialize")
	if err := backend.Initialize(ctx, &logical.InitializationRequest{
		Storage: h.storage,
	}); err != nil {
		h.logger.Error("Initialize failed", "error", err)
	}
}

// cleanupBackendLifecycle handles cleanup of backend lifecycle functions
func (h *PluginHost) cleanupBackendLifecycle() {
	if h.backend == nil {
		return
	}

	ctx := context.Background()

	// Call InvalidateKey method (standard logical.Backend interface)
	h.logger.Info("calling backend InvalidateKey for shutdown")
	h.backend.InvalidateKey(ctx, "shutdown")

	// Note: Cleanup() is called separately in Stop() method
}

// listPluginPaths displays all paths and operations supported by the plugin
func (h *PluginHost) listPluginPaths() {
	ctx := context.Background()

	h.logger.Info("=== Plugin Paths and Operations ===")

	// Use HelpOperation to get the OpenAPI document with all paths
	req := &logical.Request{
		Operation: logical.HelpOperation,
		Storage:   h.storage,
		Data:      map[string]interface{}{"requestResponsePrefix": ""},
	}

	resp, err := h.backend.HandleRequest(ctx, req)
	if err != nil {
		h.logger.Error("Failed to get plugin paths", "error", err)
		return
	}

	if resp == nil || resp.Data == nil {
		h.logger.Warn("No path information available from plugin")
		return
	}

	var backendDoc *framework.OASDocument

	// Normalize response type
	switch v := resp.Data["openapi"].(type) {
	case *framework.OASDocument:
		backendDoc = v
	case map[string]interface{}:
		backendDoc, err = framework.NewOASDocumentFromMap(v)
		if err != nil {
			h.logger.Error("Failed to parse OpenAPI document", "error", err)
			return
		}
	default:
		h.logger.Warn("No OpenAPI document available from plugin")
		return
	}

	// Store the document for later use
	h.oasDoc = backendDoc

	// Display all paths and their operations
	if backendDoc != nil && backendDoc.Paths != nil {
		for path, pathItem := range backendDoc.Paths {
			var operations []string
			if pathItem.Get != nil {
				operations = append(operations, "GET")
			}
			if pathItem.Post != nil {
				operations = append(operations, "POST")
			}
			if pathItem.Patch != nil {
				operations = append(operations, "PATCH")
			}
			if pathItem.Delete != nil {
				operations = append(operations, "DELETE")
			}
			if len(pathItem.Parameters) > 0 {
				operations = append(operations, "LIST")
			}

			// Build full curl-ready path
			fullPath := fmt.Sprintf("/v1/%s/%s", h.mountPath, strings.TrimPrefix(path, "/"))
			h.logger.Info("Path:", "path", fullPath, "operations", operations)

			// Display path description if available
			if pathItem.Description != "" {
				h.logger.Debug("  Description:", "desc", pathItem.Description)
			}
		}
	}

	h.logger.Info("=== End of Plugin Paths ===")
}

// GetUsageInfo generates usage information based on the loaded schema
func (h *PluginHost) GetUsageInfo(port string) string {
	h.mu.RLock()
	doc := h.oasDoc
	mountPath := h.mountPath
	h.mu.RUnlock()

	var info strings.Builder
	info.WriteString("Plugin Test Server\n\n")
	info.WriteString("Available endpoints:\n")

	if doc != nil && doc.Paths != nil {
		for path, pathItem := range doc.Paths {
			fullPath := fmt.Sprintf("/v1/%s/%s", mountPath, strings.TrimPrefix(path, "/"))

			if pathItem.Get != nil {
				desc := ""
				if pathItem.Get.Summary != "" {
					desc = " - " + pathItem.Get.Summary
				}
				info.WriteString(fmt.Sprintf("  GET    %-40s%s\n", fullPath, desc))
			}
			if pathItem.Post != nil {
				desc := ""
				if pathItem.Post.Summary != "" {
					desc = " - " + pathItem.Post.Summary
				}
				info.WriteString(fmt.Sprintf("  POST   %-40s%s\n", fullPath, desc))
			}
			if pathItem.Patch != nil {
				desc := ""
				if pathItem.Patch.Summary != "" {
					desc = " - " + pathItem.Patch.Summary
				}
				info.WriteString(fmt.Sprintf("  PATCH  %-40s%s\n", fullPath, desc))
			}
			if pathItem.Delete != nil {
				desc := ""
				if pathItem.Delete.Summary != "" {
					desc = " - " + pathItem.Delete.Summary
				}
				info.WriteString(fmt.Sprintf("  DELETE %-40s%s\n", fullPath, desc))
			}
		}
	}

	info.WriteString("\nSystem endpoints:\n")
	info.WriteString("  GET    /v1/sys/health                           - Check plugin health\n")
	info.WriteString("  GET    /v1/sys/storage                          - View storage contents\n")
	info.WriteString("  GET    /v1/sys/plugins/catalog/openapi          - Get OpenAPI specification\n")
	info.WriteString("\\nWeb UI:\\n")
	info.WriteString(fmt.Sprintf("  http://localhost:%s/ui/                       - Access web interface\n", port))

	return info.String()
}

// GetOpenAPIDoc returns the stored OpenAPI document
func (h *PluginHost) GetOpenAPIDoc() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.oasDoc
}
