# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build              # Build all binaries (CLI, agent, server)
make build-server       # Build server (runs templ-generate + web-build first)
make build-agent        # Build agent binary
make build-cli          # Build blazectl CLI

make test               # Run all tests with race detection + coverage
make test-coverage      # Generate HTML coverage report
make lint               # Run go vet + golangci-lint
make fmt                # Format code

make templ-generate     # Generate Go from .templ files
make templ-watch        # Watch mode for template development
make web-build          # Build Tailwind CSS
make web-watch          # Watch mode for CSS
make dev-web            # Run both watchers in parallel

make proto              # Lint + generate gRPC code from proto/
make docker-compose-dev # Start dev environment (server + SQLite)
```

### Running Locally

```bash
# Server (requires BLAZELOG_MASTER_KEY, BLAZELOG_JWT_SECRET, BLAZELOG_CSRF_SECRET)
./build/blazelog-server -c configs/server.yaml

# Agent
./build/blazelog-agent -c configs/agent.yaml
```

### Running Single Test

```bash
go test -v -run TestName ./internal/package/...
go test -v -run TestName ./internal/api/...
```

## Architecture

BlazeLog is a distributed log analysis platform with agent-server architecture.

### Binaries (cmd/)
- `server/` → Central server: gRPC receiver, HTTP API, Web UI
- `agent/` → Log collector: file tailing, parsing, streaming to server
- `blazectl/` → CLI management tool

### Core Packages (internal/)
- `api/` → HTTP REST API + routing (chi framework)
- `web/` → Web UI handlers + session management
- `server/` → gRPC server for agent communication
- `agent/` → Agent implementation
- `storage/` → SQLite (config) + ClickHouse (logs)
- `parser/` → Log format parsers (nginx, apache, magento, prestashop, wordpress)
- `alerting/` → Alert rules engine
- `notifier/` → Email, Slack, Teams notifications
- `ssh/` → SSH connection pooling for agentless collection

### Web Stack (internal/web/)
- **Templ** (.templ files) → Go template generation
- **HTMX** → Server-driven interactivity
- **Alpine.js** → Client-side reactivity
- **Tailwind CSS** → Styling (input.css → output.css)
- **ECharts** → Dashboard charts

Templates in `internal/web/templates/`: layouts, components, pages

### Storage Pattern
- **SQLite** (`storage/sqlite*.go`) → Config, users, alerts, SSH credentials
- **ClickHouse** (`storage/clickhouse.go`) → High-volume log data, analytics
- **LogBuffer** (`storage/log_buffer.go`) → In-memory batching before flush

### Auth Flow
- JWT tokens for API clients
- Session cookies for Web UI
- Hybrid middleware supports both
- Rate limiting + account lockout (5 attempts → 15 min)

### gRPC (proto/)
- Agent-server communication via mTLS
- Protocol defined in `proto/blazelog/`
- Generated code in `internal/proto/`

## E2E Tests

```bash
cd e2e
npm install
npm test           # Run Playwright tests
npm run test:ui    # Interactive mode
```
