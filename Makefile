# BlazeLog Makefile
# Build, test, and manage the project

.PHONY: all build build-agent build-server build-cli test lint clean install help \
	proto proto-deps proto-lint proto-generate proto-clean \
	templ-generate templ-watch web-build web-watch dev-web \
	docker-build docker-push install-systemd benchmark

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

# Build flags
STATIC_FLAGS=CGO_ENABLED=0
SQLCIPHER_FLAGS=CGO_ENABLED=1
SQLCIPHER_TAGS=-tags sqlcipher

# Default target
all: build

## build: Build all binaries
build: build-cli build-agent build-server

## build-cli: Build the CLI tool
build-cli:
	@echo "Building $(BINARY_CLI)..."
	@mkdir -p $(BUILD_DIR)
	$(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) ./$(CMD_DIR)/blazectl

## build-agent: Build the agent binary
build-agent:
	@echo "Building $(BINARY_AGENT)..."
	@mkdir -p $(BUILD_DIR)
	$(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT) ./$(CMD_DIR)/agent

## build-server: Build the server binary
build-server: templ-generate web-build
	@echo "Building $(BINARY_SERVER)..."
	@mkdir -p $(BUILD_DIR)
	$(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SERVER) ./$(CMD_DIR)/server

## build-all-platforms: Build for all platforms
build-all-platforms:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-linux-amd64 ./$(CMD_DIR)/blazectl
	GOOS=linux GOARCH=amd64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT)-linux-amd64 ./$(CMD_DIR)/agent
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-linux-arm64 ./$(CMD_DIR)/blazectl
	GOOS=linux GOARCH=arm64 $(STATIC_FLAGS) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT)-linux-arm64 ./$(CMD_DIR)/agent
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-darwin-amd64 ./$(CMD_DIR)/blazectl
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-darwin-arm64 ./$(CMD_DIR)/blazectl
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(SQLCIPHER_FLAGS) $(GOBUILD) $(SQLCIPHER_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI)-windows-amd64.exe ./$(CMD_DIR)/blazectl

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
	$(HOME)/.local/bin/tailwindcss -i web/static/css/input.css -o internal/web/static/css/output.css --minify

## web-watch: Watch and rebuild Tailwind CSS (development)
web-watch:
	@echo "Watching Tailwind CSS..."
	$(HOME)/.local/bin/tailwindcss -i web/static/css/input.css -o internal/web/static/css/output.css --watch

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

# Docker targets
DOCKER_REGISTRY?=ghcr.io/good-yellow-bee
DOCKER_TAG?=$(VERSION)

## docker-build: Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.server \
		-t $(DOCKER_REGISTRY)/blazelog-server:$(DOCKER_TAG) .
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.agent \
		-t $(DOCKER_REGISTRY)/blazelog-agent:$(DOCKER_TAG) .

## docker-build-multiarch: Build multi-arch Docker images (requires buildx)
docker-build-multiarch:
	@echo "Building multi-arch Docker images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.server \
		-t $(DOCKER_REGISTRY)/blazelog-server:$(DOCKER_TAG) .
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.agent \
		-t $(DOCKER_REGISTRY)/blazelog-agent:$(DOCKER_TAG) .

## docker-push: Push Docker images to registry
docker-push:
	@echo "Pushing Docker images..."
	docker push $(DOCKER_REGISTRY)/blazelog-server:$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/blazelog-agent:$(DOCKER_TAG)

## docker-compose-dev: Start development environment
docker-compose-dev:
	cd deployments/docker && docker compose --profile dev up -d

## docker-compose-prod: Start production environment
docker-compose-prod:
	cd deployments/docker && docker compose --profile prod up -d

## docker-compose-down: Stop all containers
docker-compose-down:
	cd deployments/docker && docker compose down

# Systemd targets
## install-systemd: Install systemd service files (requires sudo)
install-systemd:
	@echo "Installing systemd services..."
	@echo "Creating blazelog user..."
	sudo useradd -r -s /bin/false blazelog 2>/dev/null || true
	@echo "Creating directories..."
	sudo mkdir -p /etc/blazelog/certs /var/lib/blazelog /var/log/blazelog
	sudo chown -R blazelog:blazelog /var/lib/blazelog /var/log/blazelog
	@echo "Installing binaries..."
	sudo cp $(BUILD_DIR)/$(BINARY_SERVER) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(BINARY_AGENT) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(BINARY_CLI) /usr/local/bin/
	@echo "Installing configs..."
	sudo cp configs/server.yaml /etc/blazelog/
	sudo cp configs/agent.yaml /etc/blazelog/
	@echo "Installing service files..."
	sudo cp deployments/systemd/blazelog-server.service /etc/systemd/system/
	sudo cp deployments/systemd/blazelog-agent.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "Done! Configure /etc/blazelog/server.env with secrets, then:"
	@echo "  sudo systemctl enable --now blazelog-server"

# Benchmark targets
## benchmark: Run Go benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/api/
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/server/
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/storage/

## benchmark-api: Run API benchmarks only
benchmark-api:
	@echo "Running API benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/api/

## benchmark-grpc: Run gRPC benchmarks only
benchmark-grpc:
	@echo "Running gRPC benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/server/

## load-test: Run load tests (requires server running and k6 installed)
load-test:
	@echo "Running load tests..."
	./scripts/load-test.sh --all

## load-test-go: Run Go benchmarks only
load-test-go:
	@echo "Running Go benchmarks..."
	./scripts/load-test.sh --go-only

## load-test-k6: Run k6 load tests only
load-test-k6:
	@echo "Running k6 load tests..."
	./scripts/load-test.sh --k6-only
