# Vault Plugin Host

A standalone HTTP server for testing and developing HashiCorp Vault plugins without running a full Vault instance.

## Features

- **Plugin Launching**: Launch Vault plugins directly from binary
- **Plugin Attachment**: Attach to already-running plugin processes for debugging
- **Auto-Discovery**: Automatically discovers and displays all plugin paths and operations via OpenAPI schema
- **Plugin Configuration**: Pass configuration options to plugins via JSON or key=value format
- **In-Memory Storage**: Provides in-memory storage backend for plugin data
- **HTTP API**: RESTful API that mimics Vault's HTTP interface
- **Health & Storage Endpoints**: Built-in endpoints for monitoring and inspecting storage

## Installation

### Using Make (Recommended)

```bash
make build
# Binary will be in bin/vault-plugin-host
```

### Manual Build

```bash
go build -o bin/vault-plugin-host .
```

### Other Make Targets

```bash
make test              # Run all tests
make test-integration  # Run integration tests with KV plugin
make clean            # Clean build artifacts
make run              # Build and run with default KV plugin
make fmt              # Format code
make vet              # Run go vet
make lint             # Run formatting and vetting
```

## Usage

### Basic Usage - Launch Plugin

```bash
./bin/vault-plugin-host -plugin /path/to/plugin-binary
```

### With Custom Port and Mount Path

```bash
./bin/vault-plugin-host \
  -plugin /path/to/plugin-binary \
  -port 8300 \
  -mount myauth
```

### With Plugin Configuration

Pass configuration options in JSON format:

```bash
./bin/vault-plugin-host \
  -plugin /path/to/plugin-binary \
  -config '{"tenant_id":"abc123","region":"us-west"}'
```

Or use key=value pairs:

```bash
./bin/vault-plugin-host \
  -plugin /path/to/plugin-binary \
  -config 'tenant_id=abc123,region=us-west'
```

### Attach to Running Plugin

For debugging or development with an already-running plugin process:

```bash
./bin/vault-plugin-host -attach
```

You'll be prompted to enter the plugin attach string in the format:
```
1|4|unix|/path/to/socket|grpc|
```

### Enable Verbose Logging

```bash
./bin/vault-plugin-host -plugin /path/to/plugin-binary -v
```

## Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-plugin` | Path to plugin binary | (required in non-attach mode) |
| `-port` | HTTP server port | `8300` |
| `-mount` | Mount path for the plugin under /v1/ | `plugin` |
| `-config` | Plugin configuration (JSON or key=value) | `""` |
| `-attach` | Enable attach mode for debugging | `false` |
| `-v` | Enable verbose logging | `false` |

## API Endpoints

### Root Endpoint
```bash
GET http://localhost:8300/
```
Returns a list of all available plugin endpoints discovered from the OpenAPI schema.

### Plugin Endpoints
All plugin endpoints are accessible under `/v1/{mount}/`:

```bash
# Example: If plugin provides a login endpoint
POST http://localhost:8300/v1/plugin/login
Content-Type: application/json

{
  "password": "super-secret-password"
}
```

### System Endpoints

#### Health Check
```bash
GET http://localhost:8300/v1/sys/health
```

Returns plugin status:
```json
{
  "plugin_running": true,
  "storage_entries": 0
}
```

#### Storage Inspection
```bash
GET http://localhost:8300/v1/sys/storage
```

Returns all data stored by the plugin:
```json
{
  "storage": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

## Example Workflow

1. **Start the plugin host:**
```bash
./bin/vault-plugin-host -plugin ./my-auth-plugin -v
```

2. **View available endpoints:**
```bash
curl http://localhost:8300/
```

3. **Call plugin endpoints:**
```bash
curl -X POST http://localhost:8300/v1/plugin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"secret"}'
```

4. **Check plugin health:**
```bash
curl http://localhost:8300/v1/sys/health
```

5. **Inspect storage:**
```bash
curl http://localhost:8300/v1/sys/storage
```

## Features in Detail

### OpenAPI Schema Discovery

When the plugin starts, the host automatically queries the plugin for its OpenAPI schema and displays all available paths with their supported operations:

```
INFO  plugin started successfully
INFO  === Plugin Paths and Operations ===
INFO  Path: path=/v1/plugin/login operations=[POST]
INFO  Path: path=/v1/plugin/users/{name} operations=[GET DELETE]
INFO  Path: path=/v1/plugin/config operations=[POST GET]
INFO  === End of Plugin Paths ===
```

### In-Memory Storage

The plugin host provides an in-memory storage backend that implements `logical.Storage`. This allows plugins to store and retrieve data during testing without requiring a persistent storage backend.

### Plugin Configuration

Configuration passed via the `-config` flag is provided to the plugin through the `logical.BackendConfig.Config` map during the plugin's `Setup()` call. This is the standard way Vault passes configuration to plugins.

## Project Structure

```
vault-plugin-host/
├── main.go              # Entry point and CLI setup
├── plugin_host.go       # Plugin lifecycle management
├── storage.go           # In-memory storage implementation
├── system_view.go       # SystemView stub implementation
├── config.go            # Configuration parsing
├── handlers/            # HTTP handlers package
│   ├── handlers.go      # HTTP request handlers
│   └── handlers_test.go # Handler tests
├── *_test.go            # Test files
├── Makefile             # Build and test automation
└── README.md            # This file
```

## Development

The plugin host implements the necessary Vault SDK interfaces:
- `logical.Storage` - In-memory storage backend (storage.go)
- `logical.SystemView` - Mock system view with stub implementations (system_view.go)
- HTTP handlers - Separate package for clean interface boundaries (handlers/)

The codebase is organized into logical components:
- **Plugin Management**: Plugin lifecycle, process management, OpenAPI discovery
- **Storage Layer**: Thread-safe in-memory storage
- **HTTP Layer**: RESTful API handlers in separate package
- **Configuration**: Flexible config parsing (JSON and key=value)

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -cover ./...

# Run tests verbosely
go test -v ./...

# Run specific package tests
go test -v ./handlers
```

### Code Coverage

The project maintains comprehensive test coverage:
- Unit tests for all components
- HTTP handler tests with mock backends
- Storage concurrency tests
- Configuration parsing tests
- Integration tests with real plugins

This allows plugins to run in isolation without requiring a full Vault installation.

## License

Copyright (c) 2025 Steven Taylor

SPDX-License-Identifier: MIT

See [LICENSE](LICENSE) file for details.
