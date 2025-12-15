# BlazeLog Makefile
# Build, test, and manage the project

.PHONY: all build build-agent build-server build-cli test lint clean install help \
	proto proto-deps proto-lint proto-generate proto-clean \
	templ-generate templ-watch web-build web-watch dev-web

# Go parameters
GOCMD=/opt/homebrew/bin/go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary names
BINARY_CLI=blazelog
BINARY_AGENT=blazelog-agent
BINARY_SERVER=blazelog-server

# Build directories
BUILD_DIR=build
CMD_DIR=cmd

# Version info (can be overridden)
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Linker flags for version info
LDFLAGS=-ldflags "-s -w \
	-X github.com/good-yellow-bee/blazelog/pkg/config.Version=$(VERSION) \
	-X github.com/good-yellow-bee/blazelog/pkg/config.Commit=$(COMMIT) \
	-X github.com/good-yellow-bee/blazelog/pkg/config.BuildTime=$(BUILD_TIME)"

# Build flags for static binary
STATIC_FLAGS=CGO_ENABLED=0

# Default target
all: build

## build: Build all binaries
build: build-cli build-agent build-server

## build-cli: Build the CLI tool
build-cli:
	@echo "Building $(BINARY_CLI)..."
	@mkdir -p $(BUILD_DIR)
	$(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) ./$(CMD_DIR)/blazectl

## build-agent: Build the agent binary
build-agent:
	@echo "Building $(BINARY_AGENT)..."
	@mkdir -p $(BUILD_DIR)
	$(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT) ./$(CMD_DIR)/agent

## build-server: Build the server binary
build-server: templ-generate web-build
	@echo "Building $(BINARY_SERVER)..."
	@mkdir -p $(BUILD_DIR)
	$(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SERVER) ./$(CMD_DIR)/server

## build-all-platforms: Build for all platforms
build-all-platforms:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-linux-amd64 ./$(CMD_DIR)/blazectl
	GOOS=linux GOARCH=amd64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT)-linux-amd64 ./$(CMD_DIR)/agent
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-linux-arm64 ./$(CMD_DIR)/blazectl
	GOOS=linux GOARCH=arm64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT)-linux-arm64 ./$(CMD_DIR)/agent
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-darwin-amd64 ./$(CMD_DIR)/blazectl
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-darwin-arm64 ./$(CMD_DIR)/blazectl
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-windows-amd64.exe ./$(CMD_DIR)/blazectl

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linters
lint:
	@echo "Running linters..."
	$(GOVET) ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## tidy: Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## install: Install binaries to GOPATH/bin
install: build
	@echo "Installing binaries..."
	cp $(BUILD_DIR)/$(BINARY_CLI) $(shell $(GOCMD) env GOPATH)/bin/
	@echo "Installed $(BINARY_CLI) to $(shell $(GOCMD) env GOPATH)/bin/"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## generate: Run go generate
generate:
	@echo "Running go generate..."
	$(GOCMD) generate ./...

## templ-generate: Generate Go code from .templ files
templ-generate:
	@echo "Generating templ components..."
	$(HOME)/go/bin/templ generate

## templ-watch: Watch and regenerate templ files (development)
templ-watch:
	@echo "Watching templ files..."
	$(HOME)/go/bin/templ generate --watch

## web-build: Build Tailwind CSS
web-build:
	@echo "Building Tailwind CSS..."
	$(HOME)/.local/bin/tailwindcss -i web/static/css/input.css -o web/static/css/output.css --minify

## web-watch: Watch and rebuild Tailwind CSS (development)
web-watch:
	@echo "Watching Tailwind CSS..."
	$(HOME)/.local/bin/tailwindcss -i web/static/css/input.css -o web/static/css/output.css --watch

## dev-web: Start web development watchers
dev-web:
	@echo "Starting web development mode..."
	@make -j2 templ-watch web-watch

## help: Show this help
help:
	@echo "BlazeLog - Universal Log Analyzer"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

# Proto generation targets
PROTO_DIR=proto

## proto-deps: Install buf and protoc plugins
proto-deps:
	@echo "Installing buf..."
	@$(GOCMD) install github.com/bufbuild/buf/cmd/buf@latest
	@echo "Installing protoc-gen-go..."
	@$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@echo "Installing protoc-gen-go-grpc..."
	@$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## proto-lint: Lint proto files
proto-lint:
	@echo "Linting proto files..."
	@cd $(PROTO_DIR) && buf lint

## proto-generate: Generate Go code from proto files
proto-generate:
	@echo "Generating Go code from protos..."
	@cd $(PROTO_DIR) && buf generate

## proto: Lint and generate proto files
proto: proto-lint proto-generate
	@echo "Proto generation complete"

## proto-clean: Clean generated proto files
proto-clean:
	@echo "Cleaning generated proto files..."
	@rm -rf internal/proto/blazelog
