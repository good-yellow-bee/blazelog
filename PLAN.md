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
│   │   ├── sqlite.go       # SQLite (default)
│   │   ├── postgres.go     # PostgreSQL (production)
│   │   └── clickhouse.go   # ClickHouse (high volume)
│   └── security/           # Security utilities
│       ├── tls.go          # mTLS management
│       ├── ssh.go          # SSH key management
│       ├── crypto.go       # Encryption utilities
│       └── auth.go         # Authentication
├── pkg/                    # Public packages
│   ├── config/             # Configuration management
│   └── logger/             # Structured logging
├── web/                    # Web UI (SvelteKit or React)
│   ├── src/
│   └── build/
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

## Implementation Phases

### Phase 1: Core Foundation
**Goal:** Basic log parsing and local file analysis

**Tasks:**
1. Initialize Go module and project structure
2. Implement log parser interface
3. Implement Nginx access/error log parser
4. Implement Apache access/error log parser
5. Implement basic CLI for local file analysis
6. Add pattern matching (regex) support
7. Add structured output (JSON, table)
8. Write unit tests for parsers

**Deliverable:** CLI that can parse Nginx/Apache logs locally

---

### Phase 2: Application Log Parsers
**Goal:** Support for Magento, PrestaShop, WordPress logs

**Tasks:**
1. Research Magento log formats (system.log, exception.log, debug.log)
2. Implement Magento log parser
3. Research PrestaShop log formats
4. Implement PrestaShop log parser
5. Research WordPress log formats (debug.log, PHP errors)
6. Implement WordPress log parser
7. Add auto-detection of log format
8. Write unit tests for all parsers

**Deliverable:** CLI supports all 5 log types

---

### Phase 3: Real-time Streaming & Alerting
**Goal:** Tail logs and trigger alerts

**Tasks:**
1. Implement file tailing (follow mode) with fsnotify
2. Implement in-memory sliding window for aggregations
3. Design alert rule schema (YAML)
4. Implement threshold-based alerts (e.g., >100 errors/5min)
5. Implement pattern-based alerts (e.g., "FATAL" detected)
6. Add alert cooldown/deduplication
7. Implement alert state management
8. Write integration tests

**Deliverable:** CLI can tail logs and trigger alerts based on rules

---

### Phase 4: Notification System
**Goal:** Send alerts via Email, Slack, Teams

**Tasks:**
1. Design notifier interface
2. Implement Email notifier (SMTP with TLS)
3. Implement Slack notifier (webhook + API)
4. Implement Microsoft Teams notifier (webhook)
5. Add notification templates (customizable messages)
6. Add notification rate limiting
7. Add notification history/audit log
8. Write integration tests with mocks

**Deliverable:** Alerts can be sent via Email, Slack, or Teams

---

### Phase 5: Agent-Server Architecture
**Goal:** Distributed log collection with agents

**Tasks:**
1. Define gRPC protocol (protobuf schemas)
2. Implement gRPC server in central server
3. Implement gRPC client in agent
4. Implement mTLS for agent-server communication
5. Implement certificate generation CLI (blazectl)
6. Add agent registration and authentication
7. Implement log buffering in agent (for network outages)
8. Add heartbeat/health monitoring
9. Write integration tests

**Deliverable:** Agents can securely stream logs to central server

---

### Phase 6: SSH Connector
**Goal:** Pull logs from servers via SSH

**Tasks:**
1. Implement SSH client with key-based authentication
2. Implement secure credential storage (encrypted at rest)
3. Add support for jump hosts/bastion
4. Implement remote file tailing over SSH
5. Implement batch file download over SSH (SCP/SFTP)
6. Add connection pooling and retry logic
7. Add SSH host key verification
8. Write security tests

**Security Features:**
- SSH key-based auth only (no passwords)
- Encrypted credential storage (AES-256-GCM)
- Host key fingerprint verification
- Connection timeout and rate limiting
- Audit logging for all SSH operations

**Deliverable:** Server can securely pull logs via SSH

---

### Phase 7: Storage Layer
**Goal:** Persist logs and metadata

**Tasks:**
1. Design storage interface
2. Implement SQLite storage (default, single-node)
3. Implement PostgreSQL storage (production)
4. Design database schema (logs, alerts, projects, users)
5. Implement log retention policies
6. Implement log indexing for search
7. Add database migrations
8. Write storage tests

**Deliverable:** Logs are persisted with configurable retention

---

### Phase 8: REST API
**Goal:** API for Web UI and integrations

**Tasks:**
1. Design REST API schema (OpenAPI)
2. Implement authentication (JWT)
3. Implement authorization (RBAC)
4. Implement log query endpoints
5. Implement alert management endpoints
6. Implement project/connection endpoints
7. Implement user management endpoints
8. Implement WebSocket for real-time log streaming
9. Add API rate limiting
10. Write API tests

**Deliverable:** Full REST API for all operations

---

### Phase 9: Web UI
**Goal:** Web-based dashboard

**Tasks:**
1. Set up frontend project (SvelteKit or React)
2. Implement authentication pages (login, register)
3. Implement dashboard with metrics/charts
4. Implement log viewer with search and filters
5. Implement real-time log streaming view
6. Implement alert configuration UI
7. Implement project/connection management UI
8. Implement user management UI
9. Add responsive design
10. Write E2E tests

**Deliverable:** Fully functional Web UI

---

### Phase 10: Batch Processing
**Goal:** Analyze historical logs

**Tasks:**
1. Implement batch processing mode in CLI
2. Add support for date range queries
3. Implement parallel file processing
4. Add report generation (summary, top errors, etc.)
5. Implement export (CSV, JSON)
6. Add scheduled batch jobs (cron-like)
7. Write performance tests

**Deliverable:** Batch analysis of historical logs

---

### Phase 11: Production Hardening
**Goal:** Production-ready deployment

**Tasks:**
1. Add comprehensive logging and metrics (Prometheus)
2. Implement graceful shutdown
3. Add health check endpoints
4. Create Docker images
5. Create systemd service files
6. Create Kubernetes manifests
7. Write deployment documentation
8. Security audit
9. Performance optimization
10. Load testing

**Deliverable:** Production-ready deployment artifacts

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
- **Database:** SQLite (dev), PostgreSQL (prod), ClickHouse (high volume)
- **SSH:** golang.org/x/crypto/ssh
- **File Watching:** fsnotify
- **Config:** viper
- **Logging:** zerolog or slog

### Frontend
- **Framework:** SvelteKit (recommended) or React
- **UI Library:** Tailwind CSS + shadcn/ui
- **Charts:** Chart.js or Apache ECharts
- **State:** Svelte stores or Zustand

### DevOps
- **Containers:** Docker
- **Orchestration:** Docker Compose, Kubernetes
- **CI/CD:** GitHub Actions

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

## Open Questions

1. **Storage for high-volume deployments:**
   - ClickHouse vs Elasticsearch vs TimescaleDB?

2. **Frontend framework:**
   - SvelteKit (lighter, faster) vs React (more common)?

3. **Additional notification channels:**
   - PagerDuty, Discord, Telegram, webhook?

4. **Additional log sources (future):**
   - Kubernetes, Docker, CloudWatch, journald?

---

## Timeline Estimate

This plan represents approximately 11 phases of development. The actual timeline depends on:
- Team size and experience
- Full-time vs part-time development
- Scope adjustments based on priorities

Recommended approach: Start with Phases 1-4 for a working MVP (local analysis + notifications), then expand based on needs.
