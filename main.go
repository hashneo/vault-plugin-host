// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

//go:embed web
var webFS embed.FS

var (
	pluginPath   = flag.String("plugin", "", "Path to plugin binary")
	port         = flag.String("port", "8300", "HTTP server port")
	mount        = flag.String("mount", "plugin", "Mount path for the plugin (under /v1/)")
	verbose      = flag.Bool("v", false, "Enable verbose logging")
	attach       = flag.Bool("attach", false, "Enable attach mode (reads plugin attach string from stdin or prompts)")
	pluginConfig = flag.String("config", "", "Plugin configuration options in JSON format or key=value pairs separated by commas")

	attachString *string
)

func main() {
	flag.Parse()

	var absPath string
	var err error

	// Check if -attach flag was provided
	if *attach {
		fmt.Print("Enter plugin attach string (format: 1|4|unix|/path/to/socket|grpc|): ")

		reader := bufio.NewReader(os.Stdin)

		value, err := reader.ReadString('\n')

		if err != nil && err.Error() != "unexpected newline" {
			log.Fatalf("Failed to read input: %v", err)
		}
		if value != "" {
			attachString = &value
			fmt.Printf("Using attach config: %s\n", value)
		}
	} else {
		// Determine plugin path
		path := *pluginPath
		if path == "" {
			log.Fatalf("Plugin path required when not in attach mode. Use -plugin flag to specify the plugin binary.")
		}

		absPath, err = filepath.Abs(path)
		if err != nil {
			log.Fatalf("Failed to resolve plugin path: %v", err)
		}

		// Verify the plugin file exists
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("Plugin binary not found at path: %s", absPath)
			}
			log.Fatalf("Failed to access plugin binary: %v", err)
		}
	}

	// Parse plugin configuration
	config, err := parsePluginConfig(*pluginConfig)
	if err != nil {
		log.Fatalf("Failed to parse plugin config: %v", err)
	}

	if len(config) > 0 {
		fmt.Printf("Plugin config: %v\n", config)
	}

	fmt.Printf("Plugin: %s\n", absPath)
	fmt.Printf("Starting HTTP server on port %s...\n\n", *port)

	host, err := NewPluginHost(absPath, *verbose, config, *mount)
	if err != nil {
		log.Fatalf("Failed to create plugin host: %v", err)
	}

	if err := host.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
	defer host.Stop()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		host.Stop()
		os.Exit(0)
	}()

	// CORS middleware
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r)
		}
	}

	// Setup HTTP handlers with CORS
	mountPath := "/v1/" + *mount + "/"
	http.HandleFunc(mountPath, corsMiddleware(host.handler.HandleRequest))
	http.HandleFunc("/v1/sys/health", corsMiddleware(host.handler.HandleHealth))
	http.HandleFunc("/v1/sys/storage", corsMiddleware(host.handler.HandleStorage))
	http.HandleFunc("/v1/sys/plugins/catalog/openapi", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		host.handler.HandleOpenAPI(w, r, host.GetOpenAPIDoc())
	}))

	// Serve embedded web UI
	webContentFS, err := fs.Sub(webFS, "web")
	if err == nil {
		http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(webContentFS))))
	}

	// Root handler with usage info
	http.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, host.GetUsageInfo(*port))
		} else {
			host.handler.HandleRequest(w, r)
		}
	}))

	addr := ":" + *port
	fmt.Printf("Server ready! Try:\n")
	fmt.Printf("  curl http://localhost:%s/v1/%s/ \\\n", *port, *mount)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
