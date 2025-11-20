.PHONY: all build test clean run help docker-build docker-run

# Binary name
BINARY_NAME=vault-plugin-host

# Build directory
BUILD_DIR=bin

# Docker parameters
DOCKER_IMAGE=vault-plugin-host
DOCKER_TAG=latest

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"

all: clean build

help: ## Display this help screen
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	@$(GOTEST) -v ./...

test-integration: build ## Run integration test with KV plugin
	@echo "Running integration test with KV plugin..."
	@if [ ! -f ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv ]; then \
		echo "Error: KV plugin not found at ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv"; \
		exit 1; \
	fi
	@echo "Starting plugin host in background..."
	@$(BUILD_DIR)/$(BINARY_NAME) -plugin ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv -config="version=2" > /tmp/plugin-host.log 2>&1 & \
	PID=$$!; \
	sleep 2; \
	echo "Testing health endpoint..."; \
	curl -s http://localhost:8300/v1/sys/health | grep -q "plugin_running" && echo "✓ Health check passed" || (echo "✗ Health check failed"; kill $$PID; exit 1); \
	echo "Testing write operation..."; \
	curl -s -X POST http://localhost:8300/v1/plugin/data/test -H "Content-Type: application/json" -d '{"data":{"foo":"bar"}}' | grep -q "created_time" && echo "✓ Write test passed" || (echo "✗ Write test failed"; kill $$PID; exit 1); \
	echo "All tests passed!"; \
	kill $$PID 2>/dev/null || true

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

tidy: ## Run go mod tidy
	@echo "Running go mod tidy..."
	@$(GOMOD) tidy

run: build ## Build and run with default KV plugin
	@if [ ! -f ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv ]; then \
		echo "Error: KV plugin not found at ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv"; \
		echo "Usage: make run PLUGIN=/path/to/plugin CONFIG='key=value'"; \
		exit 1; \
	fi
	@$(BUILD_DIR)/$(BINARY_NAME) -plugin ../vault-plugin-secrets-kv/bin/vault-plugin-secrets-kv -config="version=2" -v

run-plugin: build ## Run with custom plugin (usage: make run-plugin PLUGIN=/path/to/plugin CONFIG='key=value')
	@if [ -z "$(PLUGIN)" ]; then \
		echo "Error: PLUGIN variable not set"; \
		echo "Usage: make run-plugin PLUGIN=/path/to/plugin [CONFIG='key=value']"; \
		exit 1; \
	fi
	@$(BUILD_DIR)/$(BINARY_NAME) -plugin $(PLUGIN) $(if $(CONFIG),-config="$(CONFIG)") -v

install: build ## Install binary to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

fmt: ## Format Go code
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@$(GOCMD) vet ./...

lint: fmt vet ## Run formatting and vetting

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built successfully"

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	@docker run --rm -p 8200:8200 $(DOCKER_IMAGE):$(DOCKER_TAG) --help

docker-run-plugin: docker-build ## Run Docker container with plugin (usage: make docker-run-plugin PLUGIN=/path/to/plugin)
	@if [ -z "$(PLUGIN)" ]; then \
		echo "Error: PLUGIN variable not set"; \
		echo "Usage: make docker-run-plugin PLUGIN=/path/to/plugin [CONFIG='key=value']"; \
		exit 1; \
	fi
	@echo "Running Docker container with plugin..."
	@docker run --rm -p 8200:8200 -v $(PLUGIN):/plugin:ro $(DOCKER_IMAGE):$(DOCKER_TAG) -plugin /plugin $(if $(CONFIG),-config="$(CONFIG)") -v

.DEFAULT_GOAL := help
