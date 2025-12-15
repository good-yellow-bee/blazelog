# Contributing Guide

Welcome to BlazeLog! This guide covers setting up the development environment and contributing code.

---

## Development Setup

### Prerequisites

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | For Tailwind CSS |
| Docker | 24+ | For ClickHouse |
| Make | - | Build automation |

### Clone and Build

```bash
# Clone repository
git clone https://github.com/good-yellow-bee/blazelog.git
cd blazelog

# Download dependencies
make deps

# Build all binaries
make build

# Run tests
make test
```

### Install Development Tools

```bash
# Install Go tools
make proto-deps          # protoc plugins
go install github.com/a-h/templ/cmd/templ@latest

# Install Tailwind CSS
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
mv tailwindcss-linux-x64 ~/.local/bin/tailwindcss

# Install linter (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

## Project Structure

```
blazelog/
├── cmd/                    # Main applications
│   ├── agent/              # Agent binary
│   ├── server/             # Server binary
│   └── blazectl/           # CLI tool
├── internal/               # Private packages
│   ├── agent/              # Agent runtime
│   ├── alerting/           # Alert engine
│   ├── api/                # REST API
│   ├── batch/              # Batch processing
│   ├── models/             # Data models
│   ├── notifier/           # Notifications
│   ├── parser/             # Log parsers
│   ├── proto/              # Generated protobuf
│   ├── security/           # TLS, auth
│   ├── server/             # gRPC server
│   ├── ssh/                # SSH client
│   ├── storage/            # Database layer
│   ├── tailer/             # File watching
│   └── web/                # Web UI
├── pkg/                    # Public packages
│   └── config/             # Configuration
├── proto/                  # Protobuf definitions
├── web/                    # Web assets
│   └── static/             # CSS, JS
├── configs/                # Example configs
├── docs/                   # Documentation
├── deployments/            # Docker, K8s
└── scripts/                # Helper scripts
```

---

## Make Commands

```bash
# Building
make build              # Build all binaries
make build-agent        # Build agent only
make build-server       # Build server only
make build-cli          # Build CLI only

# Testing
make test               # Run all tests
make test-coverage      # Run with coverage report

# Code Quality
make fmt                # Format code
make lint               # Run linters
make vet                # Run go vet

# Proto Generation
make proto-deps         # Install protoc plugins
make proto              # Generate Go from proto
make proto-clean        # Clean generated files

# Web Development
make templ-generate     # Generate from .templ files
make web-build          # Build Tailwind CSS
make dev-web            # Watch mode for web dev

# Cleanup
make clean              # Remove build artifacts
make tidy               # Tidy go.mod
```

---

## Development Workflow

### Running Locally

```bash
# Terminal 1: Start ClickHouse
docker run -d --name clickhouse \
  -p 9000:9000 -p 8123:8123 \
  clickhouse/clickhouse-server:latest

# Terminal 2: Start server
export BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
export BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)
export BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
./build/blazelog-server --config configs/server.yaml

# Terminal 3: Start agent (optional)
./build/blazelog-agent --config configs/agent.yaml
```

### Web Development

For live reload during web development:

```bash
# Terminal 1: Watch Templ files
make templ-watch

# Terminal 2: Watch Tailwind CSS
make web-watch

# Terminal 3: Run server
go run ./cmd/server --config configs/server.yaml
```

---

## Adding Features

### Adding a Log Parser

1. Create parser file in `internal/parser/`:

```go
// internal/parser/myformat.go
package parser

import (
    "regexp"
    "github.com/good-yellow-bee/blazelog/internal/models"
)

type MyFormatParser struct {
    pattern *regexp.Regexp
}

func NewMyFormatParser() *MyFormatParser {
    return &MyFormatParser{
        pattern: regexp.MustCompile(`^(?P<timestamp>\S+) (?P<level>\w+) (?P<message>.*)$`),
    }
}

func (p *MyFormatParser) Parse(line string) (*models.LogEntry, error) {
    match := p.pattern.FindStringSubmatch(line)
    if match == nil {
        return nil, ErrNoMatch
    }
    // Parse and return LogEntry...
}

func (p *MyFormatParser) Type() models.LogType {
    return models.LogTypeCustom // or add new type
}

func (p *MyFormatParser) Name() string {
    return "myformat"
}
```

2. Register in `internal/parser/registry.go`

3. Add tests in `internal/parser/myformat_test.go`

### Adding a Notifier

1. Create notifier in `internal/notifier/`:

```go
// internal/notifier/mynotifier.go
package notifier

type MyNotifier struct {
    config MyNotifierConfig
}

type MyNotifierConfig struct {
    Endpoint string
    Token    string
}

func NewMyNotifier(cfg MyNotifierConfig) *MyNotifier {
    return &MyNotifier{config: cfg}
}

func (n *MyNotifier) Send(alert Alert) error {
    // Send notification...
    return nil
}

func (n *MyNotifier) Name() string {
    return "mynotifier"
}
```

2. Register in notifier factory

3. Add configuration support

4. Add tests

### Adding API Endpoints

1. Add handler in `internal/api/`:

```go
// internal/api/myhandler.go
func (h *Handler) GetMyResource(w http.ResponseWriter, r *http.Request) {
    // Handle request...
}
```

2. Register route in router

3. Update `docs/api/openapi.yaml`

4. Add tests

---

## Code Style

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `go fmt` before committing
- Add doc comments for exported functions
- Keep functions focused and testable

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to parse log: %w", err)
}

// Bad: Lose error context
if err != nil {
    return errors.New("parse failed")
}
```

### Testing

```go
// Table-driven tests
func TestParser(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *LogEntry
        wantErr bool
    }{
        {"valid line", "...", &LogEntry{...}, false},
        {"invalid line", "...", nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parser.Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
            }
            // Compare got and want...
        })
    }
}
```

---

## Pull Request Process

### Before Submitting

1. **Run tests**: `make test`
2. **Run linter**: `make lint`
3. **Format code**: `make fmt`
4. **Update docs** if needed

### PR Guidelines

- Keep PRs focused (one feature/fix per PR)
- Write clear commit messages
- Add tests for new features
- Update documentation
- Reference related issues

### Commit Messages

```
<type>: <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

Example:
```
feat: Add PagerDuty notifier

- Add pagerduty package with Events API v2 support
- Add configuration options for routing keys
- Add tests for event formatting

Closes #123
```

---

## Testing Guidelines

### Unit Tests

Test individual functions:

```bash
go test -v ./internal/parser/...
```

### Integration Tests

Test component interactions (requires ClickHouse):

```bash
go test -v -tags=integration ./...
```

### Benchmarks

```bash
go test -bench=. ./internal/parser/...
```

---

## Debugging

### Server Logs

```bash
# Verbose logging
./build/blazelog-server --config configs/server.yaml --log-level debug
```

### Agent Logs

```bash
./build/blazelog-agent --config configs/agent.yaml --log-level debug
```

### gRPC Debugging

```bash
# Enable gRPC tracing
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info
```

---

## Release Process

1. Update version in `VERSION`
2. Update `CHANGELOG.md`
3. Create tag: `git tag v1.2.3`
4. Push: `git push origin v1.2.3`
5. CI builds and publishes releases

---

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/good-yellow-bee/blazelog/issues)
- **Discussions**: [GitHub Discussions](https://github.com/good-yellow-bee/blazelog/discussions)

---

## See Also

- [Architecture Overview](ARCHITECTURE.md) - System design
- [Protocol Specification](protocol.md) - gRPC protocol
- [Configuration Reference](CONFIGURATION.md) - All settings
