# BlazeLog - Universal Log Analyzer

## Overview

BlazeLog is a fast, powerful, and secure universal log analyzer built in Go with multi-platform support. It provides real-time streaming and batch processing capabilities with a web-based UI for management and visualization.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              BLAZELOG SYSTEM                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                │
│  │   Server A   │     │   Server B   │     │   Server C   │                │
│  │  (Magento)   │     │ (PrestaShop) │     │  (WordPress) │                │
│  │              │     │              │     │              │                │
│  │ ┌──────────┐ │     │ ┌──────────┐ │     │ ┌──────────┐ │                │
│  │ │ BlazeLog │ │     │ │ BlazeLog │ │     │ │ BlazeLog │ │                │
│  │ │  Agent   │ │     │ │  Agent   │ │     │ │  Agent   │ │                │
│  │ └────┬─────┘ │     │ └────┬─────┘ │     │ └────┬─────┘ │                │
│  └──────┼───────┘     └──────┼───────┘     └──────┼───────┘                │
│         │                    │                    │                         │
│         │  mTLS/gRPC         │  mTLS/gRPC         │  mTLS/gRPC             │
│         │                    │                    │                         │
│         └────────────────────┼────────────────────┘                         │
│                              │                                              │
│                              ▼                                              │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                     BLAZELOG CENTRAL SERVER                        │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │     │
│  │  │   Log       │  │   Alert     │  │  Notifier   │                │     │
│  │  │  Processor  │  │   Engine    │  │   Service   │                │     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │     │
│  │  │    SSH      │  │    REST     │  │   Storage   │                │     │
│  │  │  Connector  │  │    API      │  │   Engine    │                │     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                │     │
│  └───────────────────────────────────────────────────────────────────┘     │
│                              │                                              │
│                              ▼                                              │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                         WEB UI                                     │     │
│  │   Dashboard │ Log Search │ Alerts │ Projects │ Settings │ Users   │     │
│  └───────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Components

### 1. BlazeLog Agent (CLI)
Lightweight binary installed on remote servers for local log collection.

**Zero-Dependency Design:**
- Single static binary (~5-10MB)
- No runtime dependencies (no Java, Python, Node.js)
- No external packages required
- Just copy binary and run
- Cross-compiled for Linux (amd64, arm64), macOS, Windows

**Security Hardening:**
- Pure Go (no CGO) = no C vulnerabilities
- No shell execution, no external commands
- Outbound connections only (no listeners)
- Runs as unprivileged user
- Read-only log file access
- Built with: `-trimpath -ldflags="-s -w"` + PIE/RELRO

**Responsibilities:**
- Read logs from local filesystem
- Parse logs based on configured format (Magento, PrestaShop, WordPress, Nginx, Apache)
- Stream logs to central server via secure gRPC/mTLS
- Buffer logs during network outages
- Local alerting (optional standalone mode)

### 2. BlazeLog Central Server
Core processing engine and API server.

**Responsibilities:**
- Receive logs from agents (gRPC)
- Pull logs via SSH from remote servers
- Read logs from local filesystem
- Process and analyze logs (real-time + batch)
- Execute alert rules (threshold + pattern based)
- Send notifications (Email, Slack, Teams)
- Store logs and metadata
- Expose REST API for Web UI

### 3. BlazeLog Web UI
Web-based dashboard for management and visualization.

**Features:**
- Dashboard with summaries and metrics
- Log search and filtering
- Real-time log streaming view
- Alert configuration and history
- Project/connection management
- User management with RBAC

---

## Project Structure

```
blazelog/
├── cmd/
│   ├── agent/              # Agent CLI binary
│   │   └── main.go
│   ├── server/             # Central server binary
│   │   └── main.go
│   └── blazectl/           # Management CLI
│       └── main.go
├── internal/
│   ├── agent/              # Agent core logic
│   │   ├── collector/      # Log collection
│   │   ├── buffer/         # Offline buffering
│   │   └── transport/      # gRPC client
│   ├── server/             # Server core logic
│   │   ├── receiver/       # gRPC log receiver
│   │   ├── ssh/            # SSH connector
│   │   ├── processor/      # Log processing pipeline
│   │   ├── alerting/       # Alert engine
│   │   ├── notifier/       # Notification dispatchers
│   │   └── api/            # REST API handlers
│   ├── parser/             # Log parsers
│   │   ├── parser.go       # Parser interface
│   │   ├── magento.go
│   │   ├── prestashop.go
│   │   ├── wordpress.go
│   │   ├── nginx.go
│   │   ├── apache.go
│   │   └── custom.go       # Custom regex parser
│   ├── models/             # Data models
│   │   ├── log.go
│   │   ├── alert.go
│   │   ├── project.go
│   │   └── user.go
│   ├── storage/            # Storage layer
│   │   ├── storage.go      # Interface
│   │   ├── sqlite.go       # SQLite (config/metadata)
│   │   └── clickhouse.go   # ClickHouse (logs - primary)
│   └── security/           # Security utilities
│       ├── tls.go          # mTLS management
│       ├── ssh.go          # SSH key management
│       ├── crypto.go       # Encryption utilities
│       └── auth.go         # Authentication
├── pkg/                    # Public packages
│   ├── config/             # Configuration management
│   └── logger/             # Structured logging
├── web/                    # Web UI (Templ + HTMX)
│   ├── templates/          # Templ templates
│   ├── static/             # CSS, JS (Alpine.js, HTMX)
│   └── handlers/           # HTTP handlers for UI
├── configs/                # Example configurations
│   ├── agent.yaml
│   ├── server.yaml
│   └── alerts.yaml
├── scripts/                # Build and deployment scripts
├── deployments/            # Docker, systemd, etc.
│   ├── docker/
│   └── systemd/
├── docs/                   # Documentation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Implementation Milestones

Small, incremental milestones that each deliver usable value.

---

### STAGE A: CLI Foundation (Milestones 1-6)
*Goal: Working CLI that parses all log types locally*

---

#### Milestone 1: Project Setup & Parser Interface
**Tasks:**
1. Initialize Go module with `go mod init`
2. Create project directory structure
3. Set up Makefile with build targets
4. Implement `Parser` interface
5. Create `LogEntry` model
6. Set up basic CLI with cobra/urfave

**Deliverable:** Empty CLI skeleton with parser interface
**Effort:** Small

---

#### Milestone 2: Nginx Parser
**Tasks:**
1. Research Nginx log formats (combined, custom)
2. Implement Nginx access log parser
3. Implement Nginx error log parser
4. Add CLI command: `blazelog parse nginx <file>`
5. Write unit tests

**Deliverable:** `blazelog parse nginx /var/log/nginx/access.log`
**Effort:** Small

---

#### Milestone 3: Apache Parser
**Tasks:**
1. Research Apache log formats (common, combined, error)
2. Implement Apache access log parser
3. Implement Apache error log parser
4. Add CLI command: `blazelog parse apache <file>`
5. Write unit tests

**Deliverable:** `blazelog parse apache /var/log/apache2/access.log`
**Effort:** Small

---

#### Milestone 4: Magento Parser
**Tasks:**
1. Research Magento log formats (system.log, exception.log, debug.log)
2. Implement Magento log parser (handles multiline stack traces)
3. Add CLI command: `blazelog parse magento <file>`
4. Write unit tests

**Deliverable:** `blazelog parse magento /var/www/magento/var/log/system.log`
**Effort:** Small

---

#### Milestone 5: PrestaShop Parser
**Tasks:**
1. Research PrestaShop log formats
2. Implement PrestaShop log parser
3. Add CLI command: `blazelog parse prestashop <file>`
4. Write unit tests

**Deliverable:** `blazelog parse prestashop /var/www/prestashop/var/logs/*.log`
**Effort:** Small

---

#### Milestone 6: WordPress Parser & Auto-Detection
**Tasks:**
1. Research WordPress log formats (debug.log, PHP errors)
2. Implement WordPress log parser
3. Implement auto-detection of log format
4. Add CLI command: `blazelog parse auto <file>`
5. Add output formats (JSON, table, plain)
6. Write unit tests

**Deliverable:** `blazelog parse auto /var/log/*.log --format json`
**Effort:** Small

---

### STAGE B: Real-time & Alerting (Milestones 7-11)
*Goal: CLI can tail logs and send notifications*

---

#### Milestone 7: File Tailing
**Tasks:**
1. Implement file tailing with fsnotify
2. Handle log rotation gracefully
3. Add CLI command: `blazelog tail <file>`
4. Support multiple files with glob patterns
5. Write integration tests

**Deliverable:** `blazelog tail /var/log/nginx/*.log --follow`
**Effort:** Small

---

#### Milestone 8: Alert Rules Engine
**Tasks:**
1. Design alert rule YAML schema
2. Implement rule parser
3. Implement pattern-based matching (regex)
4. Implement threshold detection (count in window)
5. Add sliding window aggregation
6. Add alert cooldown/deduplication

**Deliverable:** Alert rules loaded from YAML, evaluated in memory
**Effort:** Medium

---

#### Milestone 9: Email Notifications
**Tasks:**
1. Design notifier interface
2. Implement SMTP client with TLS
3. Implement email templates (HTML + plain text)
4. Add CLI flag: `--notify-email`
5. Write tests with mock SMTP

**Deliverable:** `blazelog tail ... --notify-email admin@example.com`
**Effort:** Small

---

#### Milestone 10: Slack Notifications
**Tasks:**
1. Implement Slack webhook notifier
2. Implement Slack message formatting (blocks)
3. Add CLI flag: `--notify-slack`
4. Write tests

**Deliverable:** `blazelog tail ... --notify-slack webhook-url`
**Effort:** Small

---

#### Milestone 11: Teams Notifications
**Tasks:**
1. Implement Microsoft Teams webhook notifier
2. Implement Teams adaptive card formatting
3. Add CLI flag: `--notify-teams`
4. Add notification rate limiting (all channels)
5. Write tests

**Deliverable:** `blazelog tail ... --notify-teams webhook-url`
**Effort:** Small

---

### STAGE C: Distributed Collection (Milestones 12-16)
*Goal: Agent-server architecture with secure communication*

---

#### Milestone 12: gRPC Protocol Definition
**Tasks:**
1. Define protobuf schemas (LogEntry, AgentInfo, etc.)
2. Generate Go code from protos
3. Design streaming protocol
4. Document protocol

**Deliverable:** `.proto` files and generated Go code
**Effort:** Small

---

#### Milestone 13: Agent Basic Implementation
**Tasks:**
1. Create agent CLI binary (`blazelog-agent`)
2. Implement config file loading
3. Implement log collection from local files
4. Implement gRPC client (insecure for now)
5. Write integration tests

**Deliverable:** Agent that sends logs to server (no TLS yet)
**Effort:** Medium

---

#### Milestone 14: Server Log Receiver
**Tasks:**
1. Create server binary (`blazelog-server`)
2. Implement gRPC server
3. Implement log receiver and processor pipeline
4. Add basic console output for received logs
5. Write integration tests

**Deliverable:** Server receives and displays logs from agents
**Effort:** Medium

---

#### Milestone 15: mTLS Security
**Tasks:**
1. Implement CA certificate generation (`blazectl ca init`)
2. Implement agent certificate generation (`blazectl cert agent`)
3. Implement server certificate generation (`blazectl cert server`)
4. Add mTLS to gRPC client/server
5. Implement certificate validation
6. Write security tests

**Deliverable:** Secure agent-server communication with mTLS
**Effort:** Medium

---

#### Milestone 16: Agent Reliability
**Tasks:**
1. Implement local buffer for network outages
2. Implement reconnection with backoff
3. Implement heartbeat/health check
4. Add agent registration flow
5. Write chaos tests (network failures)

**Deliverable:** Agent handles network issues gracefully
**Effort:** Medium

---

### STAGE D: SSH Collection (Milestones 17-18)
*Goal: Server can pull logs via SSH*

---

#### Milestone 17: SSH Connector Base
**Tasks:**
1. Implement SSH client with key authentication
2. Implement remote file reading
3. Implement remote file tailing
4. Add connection management in config
5. Write integration tests

**Deliverable:** Server can read logs from remote servers via SSH
**Effort:** Medium

---

#### Milestone 18: SSH Security Hardening
**Tasks:**
1. Implement encrypted credential storage (AES-256-GCM)
2. Implement host key verification
3. Add jump host/bastion support
4. Add connection pooling
5. Add audit logging for SSH operations
6. Write security tests

**Deliverable:** Production-ready secure SSH connector
**Effort:** Medium

---

### STAGE E: Storage (Milestones 19-21)
*Goal: Persistent storage with search*

---

#### Milestone 19: SQLite for Config
**Tasks:**
1. Design SQLite schema (users, projects, alerts, connections)
2. Implement SQLite storage layer
3. Implement database migrations
4. Add config persistence to server
5. Write storage tests

**Deliverable:** Server persists configuration in SQLite
**Effort:** Small

---

#### Milestone 20: ClickHouse Setup
**Tasks:**
1. Design ClickHouse schema for logs
2. Implement ClickHouse client
3. Implement log insertion (batched)
4. Implement basic log queries
5. Write integration tests

**Deliverable:** Logs stored in ClickHouse
**Effort:** Medium

---

#### Milestone 21: ClickHouse Advanced
**Tasks:**
1. Create materialized views for dashboards
2. Implement full-text search
3. Implement TTL retention policies
4. Implement log aggregation queries
5. Performance tuning
6. Write performance tests

**Deliverable:** Fast search and analytics on billions of logs
**Effort:** Medium

---

### STAGE F: REST API (Milestones 22-24)
*Goal: Full API for web UI*

---

#### Milestone 22: API Auth & Users
**Tasks:**
1. Set up HTTP router (chi)
2. Implement JWT authentication
3. Implement user registration/login endpoints
4. Implement RBAC (Admin, Operator, Viewer)
5. Add API rate limiting
6. Write API tests

**Deliverable:** `/api/v1/auth/*` and `/api/v1/users/*` endpoints
**Effort:** Medium

---

#### Milestone 23: API Logs & Search
**Tasks:**
1. Implement log query endpoint
2. Implement log search with filters
3. Implement SSE for real-time streaming
4. Add pagination
5. Write API tests

**Deliverable:** `/api/v1/logs/*` endpoints with real-time streaming
**Effort:** Medium

---

#### Milestone 24: API Alerts & Projects
**Tasks:**
1. Implement alert rules CRUD endpoints
2. Implement alert history endpoint
3. Implement projects CRUD endpoints
4. Implement connections CRUD endpoints
5. Generate OpenAPI spec
6. Write API tests

**Deliverable:** Full REST API complete
**Effort:** Medium

---

### STAGE G: Web UI (Milestones 25-28)
*Goal: Complete web dashboard*

---

#### Milestone 25: Web UI Setup
**Tasks:**
1. Set up Templ templates
2. Configure Tailwind CSS
3. Integrate HTMX and Alpine.js
4. Create base layout template
5. Implement login/register pages
6. Embed static assets in binary

**Deliverable:** Login page working
**Effort:** Medium

---

#### Milestone 26: Dashboard
**Tasks:**
1. Create dashboard layout
2. Implement metrics cards (log counts, error rates)
3. Implement charts (ECharts)
4. Add time range selector
5. Add auto-refresh

**Deliverable:** Dashboard with real-time metrics
**Effort:** Medium

---

#### Milestone 27: Log Viewer
**Tasks:**
1. Implement log list view with pagination
2. Implement search and filters
3. Implement log detail view
4. Implement real-time tail view (SSE)
5. Add export functionality

**Deliverable:** Full log viewer with search
**Effort:** Medium

---

#### Milestone 28: Settings & Management
**Tasks:**
1. Implement alert rules management UI
2. Implement projects management UI
3. Implement connections management UI
4. Implement user management UI (admin only)
5. Add responsive design
6. Write E2E tests

**Deliverable:** Complete Web UI
**Effort:** Medium

---

### STAGE H: Batch & Production (Milestones 29-30)
*Goal: Production-ready system*

---

#### Milestone 29: Batch Processing
**Tasks:**
1. Implement batch analysis mode
2. Add date range support
3. Implement parallel processing
4. Add report generation
5. Add export (CSV, JSON)
6. Write performance tests

**Deliverable:** `blazelog analyze --from 2024-01-01 --to 2024-01-31`
**Effort:** Medium

---

#### Milestone 30: Production Hardening
**Tasks:**
1. Add Prometheus metrics
2. Add health check endpoints
3. Implement graceful shutdown
4. Create Docker images
5. Create systemd service files
6. Write deployment documentation
7. Security audit
8. Load testing

**Deliverable:** Production-ready deployment
**Effort:** Medium

---

## Milestone Summary

| Stage | Milestones | Description |
|-------|------------|-------------|
| A | 1-6 | CLI parses all log types |
| B | 7-11 | Real-time tailing + notifications |
| C | 12-16 | Agent-server with mTLS |
| D | 17-18 | SSH log collection |
| E | 19-21 | ClickHouse storage |
| F | 22-24 | REST API |
| G | 25-28 | Web UI |
| H | 29-30 | Batch + Production |

**Total: 30 milestones** (vs 11 large phases before)

Each milestone is:
- Self-contained and testable
- Delivers usable functionality
- Can be completed independently
- Small enough to review easily

---

## Security Architecture

### Agent-Server Communication
```
┌─────────────┐                    ┌─────────────┐
│   Agent     │◄──── mTLS ────────►│   Server    │
│             │                    │             │
│ - Client    │                    │ - Server    │
│   Cert      │                    │   Cert      │
│ - CA Cert   │                    │ - CA Cert   │
└─────────────┘                    └─────────────┘

- Mutual TLS (mTLS) for authentication
- TLS 1.3 minimum
- Certificate rotation support
- Certificate revocation list (CRL)
```

### SSH Security
```
- Ed25519 or RSA-4096 keys only
- No password authentication
- Host key verification (TOFU or pre-configured)
- Encrypted key storage (AES-256-GCM)
- Connection audit logging
- Rate limiting per host
- Jump host/bastion support
```

### Web UI Security
```
- JWT with short expiration + refresh tokens
- RBAC (Admin, Operator, Viewer roles)
- CSRF protection
- Rate limiting
- Security headers (CSP, HSTS, etc.)
- Password hashing (Argon2id)
- MFA support (TOTP)
```

### Data Security
```
- Encryption at rest for sensitive config
- Secure credential storage
- Log sanitization options (mask sensitive data)
- Audit logging for all operations
```

---

## Configuration Examples

### Agent Configuration (agent.yaml)
```yaml
server:
  address: "blazelog.example.com:9443"
  tls:
    cert_file: "/etc/blazelog/agent.crt"
    key_file: "/etc/blazelog/agent.key"
    ca_file: "/etc/blazelog/ca.crt"

sources:
  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

  - name: "magento-system"
    type: "magento"
    path: "/var/www/magento/var/log/system.log"
    follow: true

  - name: "magento-exception"
    type: "magento"
    path: "/var/www/magento/var/log/exception.log"
    follow: true

buffer:
  max_size: "100MB"
  path: "/var/lib/blazelog/buffer"

labels:
  environment: "production"
  project: "ecommerce-main"
```

### Server Configuration (server.yaml)
```yaml
server:
  grpc_address: ":9443"
  http_address: ":8080"

  tls:
    cert_file: "/etc/blazelog/server.crt"
    key_file: "/etc/blazelog/server.key"
    ca_file: "/etc/blazelog/ca.crt"

storage:
  type: "postgres"
  dsn: "postgres://blazelog:***@localhost/blazelog?sslmode=require"
  retention_days: 30

ssh_connections:
  - name: "staging-server"
    host: "staging.example.com:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/staging.key"
    sources:
      - path: "/var/log/nginx/*.log"
        type: "nginx"
      - path: "/var/www/prestashop/var/logs/*.log"
        type: "prestashop"

notifications:
  email:
    smtp_host: "smtp.example.com"
    smtp_port: 587
    username: "alerts@example.com"
    password_env: "SMTP_PASSWORD"
    from: "BlazeLog <alerts@example.com>"

  slack:
    webhook_url_env: "SLACK_WEBHOOK_URL"

  teams:
    webhook_url_env: "TEAMS_WEBHOOK_URL"

auth:
  jwt_secret_env: "JWT_SECRET"
  session_duration: "24h"
```

### Alert Rules (alerts.yaml)
```yaml
rules:
  - name: "High Error Rate"
    description: "More than 100 errors in 5 minutes"
    type: "threshold"
    condition:
      field: "level"
      value: "error"
      threshold: 100
      window: "5m"
    severity: "critical"
    notify:
      - "slack"
      - "email"
    labels:
      project: "*"

  - name: "Fatal Error Detected"
    description: "FATAL error in logs"
    type: "pattern"
    condition:
      pattern: "FATAL|CRITICAL"
      case_sensitive: false
    severity: "critical"
    notify:
      - "slack"
      - "teams"
      - "email"
    cooldown: "15m"

  - name: "WordPress Database Error"
    description: "Database connection issues"
    type: "pattern"
    condition:
      pattern: "Error establishing a database connection"
      log_type: "wordpress"
    severity: "high"
    notify:
      - "slack"
    cooldown: "5m"

  - name: "Nginx 5xx Spike"
    description: "High rate of 5xx errors"
    type: "threshold"
    condition:
      field: "status"
      operator: ">="
      value: 500
      threshold: 50
      window: "1m"
      log_type: "nginx"
    severity: "high"
    notify:
      - "slack"
```

---

## Technology Stack

### Backend
- **Language:** Go 1.22+
- **gRPC:** google.golang.org/grpc
- **HTTP Router:** chi or gin
- **Database:** SQLite (dev/config), ClickHouse (logs - handles billions of rows with full-text search)
- **SSH:** golang.org/x/crypto/ssh
- **File Watching:** fsnotify
- **Config:** viper
- **Logging:** zerolog or slog

### Frontend (Go-Native Stack)
- **Templating:** Templ (type-safe Go templates)
- **Interactivity:** HTMX + Alpine.js
- **Styling:** Tailwind CSS
- **Charts:** Apache ECharts (via HTMX partials)
- **Benefit:** Single binary deployment, SSR, minimal JS

### DevOps
- **Containers:** Docker
- **Orchestration:** Docker Compose, Kubernetes
- **CI/CD:** GitHub Actions

### Agent Build (Zero-Dependency)
```makefile
# Build static binary with all security flags
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath \
  -ldflags="-s -w -extldflags '-static'" \
  -tags netgo \
  -o blazelog-agent-linux-amd64 \
  ./cmd/agent

# Optional: compress with UPX (3-4MB final size)
upx --best blazelog-agent-linux-amd64
```

**Supported Platforms:**
- `linux/amd64` - Standard servers
- `linux/arm64` - ARM servers, Raspberry Pi
- `darwin/amd64` - macOS Intel
- `darwin/arm64` - macOS Apple Silicon
- `windows/amd64` - Windows servers

### Agent Deployment (One-Liner)
```bash
# Install agent on any Linux server (no dependencies!)
curl -fsSL https://blazelog.example.com/install-agent.sh | sh

# Or manual installation:
wget https://releases.blazelog.example.com/agent/latest/linux-amd64/blazelog-agent
chmod +x blazelog-agent
sudo mv blazelog-agent /usr/local/bin/
sudo blazelog-agent init --server blazelog.example.com:9443

# Systemd service (optional)
sudo blazelog-agent install-service
sudo systemctl enable --now blazelog-agent
```

---

## API Endpoints (Draft)

### Authentication
```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
```

### Logs
```
GET    /api/v1/logs                    # Query logs
GET    /api/v1/logs/stream             # WebSocket for real-time
GET    /api/v1/logs/stats              # Aggregated statistics
```

### Alerts
```
GET    /api/v1/alerts                  # List alerts
POST   /api/v1/alerts                  # Create alert rule
GET    /api/v1/alerts/:id              # Get alert rule
PUT    /api/v1/alerts/:id              # Update alert rule
DELETE /api/v1/alerts/:id              # Delete alert rule
GET    /api/v1/alerts/history          # Alert history
```

### Projects
```
GET    /api/v1/projects                # List projects
POST   /api/v1/projects                # Create project
GET    /api/v1/projects/:id            # Get project
PUT    /api/v1/projects/:id            # Update project
DELETE /api/v1/projects/:id            # Delete project
```

### Connections
```
GET    /api/v1/connections             # List connections
POST   /api/v1/connections             # Create connection
GET    /api/v1/connections/:id         # Get connection
PUT    /api/v1/connections/:id         # Update connection
DELETE /api/v1/connections/:id         # Delete connection
POST   /api/v1/connections/:id/test    # Test connection
```

### Users
```
GET    /api/v1/users                   # List users
POST   /api/v1/users                   # Create user
GET    /api/v1/users/:id               # Get user
PUT    /api/v1/users/:id               # Update user
DELETE /api/v1/users/:id               # Delete user
```

### System
```
GET    /api/v1/health                  # Health check
GET    /api/v1/metrics                 # Prometheus metrics
```

---

## Success Metrics

1. **Performance:**
   - Agent: <50MB memory, <5% CPU for 10k logs/sec
   - Server: Process 100k logs/sec
   - Query: <100ms for last 24h queries

2. **Reliability:**
   - Zero log loss during network outages (buffering)
   - 99.9% uptime for central server
   - Alert delivery <30 seconds

3. **Security:**
   - All communications encrypted
   - No plaintext credentials
   - Full audit trail

---

## Decisions Made

1. **Storage:** ClickHouse for logs (handles billions of rows, built-in FTS, SQL-like)
2. **Frontend:** Templ + HTMX + Alpine.js (Go-native, single binary deployment)
3. **Notifications:** Email, Slack, Teams only (additional channels deferred)

## Open Questions

1. **Additional log sources (future):**
   - Kubernetes, Docker, CloudWatch, journald?

2. **ClickHouse deployment:**
   - Embedded (chDB) vs external ClickHouse server?

---

## Timeline Estimate

This plan represents approximately 11 phases of development. The actual timeline depends on:
- Team size and experience
- Full-time vs part-time development
- Scope adjustments based on priorities

Recommended approach: Start with Phases 1-4 for a working MVP (local analysis + notifications), then expand based on needs.
