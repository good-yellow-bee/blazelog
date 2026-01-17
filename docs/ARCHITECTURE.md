# Architecture Overview

This document describes the internal architecture of BlazeLog.

---

## System Components

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Log Sources                                 │
├─────────────────────┬───────────────────────┬───────────────────────────┤
│     Log Files       │    Remote Servers     │      Applications         │
│   (/var/log/*)      │      (via SSH)        │    (stdout/stderr)        │
└─────────┬───────────┴───────────┬───────────┴───────────┬───────────────┘
          │                       │                       │
          ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          BlazeLog Agents                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                │
│  │  Tailer  │  │  Parser  │  │  Buffer  │  │  gRPC    │                │
│  │ (watch)  │→ │ (parse)  │→ │ (batch)  │→ │ Client   │                │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘                │
└─────────────────────────────────┬───────────────────────────────────────┘
                                  │ gRPC + mTLS
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          BlazeLog Server                                 │
│                                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                │
│  │  gRPC    │→ │  Batch   │→ │ Storage  │  │  Alert   │                │
│  │ Server   │  │ Writer   │  │ (Click-  │← │ Engine   │                │
│  └──────────┘  └──────────┘  │  House)  │  └──────────┘                │
│                              └──────────┘        │                      │
│  ┌──────────┐  ┌──────────┐                      ▼                      │
│  │   SSH    │  │   REST   │              ┌──────────┐                  │
│  │ Collector│  │   API    │              │ Notifier │                  │
│  └──────────┘  └──────────┘              └──────────┘                  │
│                      │                         │                        │
│  ┌──────────┐        │                         ▼                        │
│  │  Web UI  │        │              ┌─────────────────────┐            │
│  │ (Templ)  │        │              │ Email/Slack/Teams   │            │
│  └──────────┘        │              └─────────────────────┘            │
└──────────────────────┼──────────────────────────────────────────────────┘
                       │
                       ▼
              ┌────────────────┐
              │    Clients     │
              │  (Web, API)    │
              └────────────────┘
```

---

## Component Details

### Agent Components

| Component | Package | Description |
|-----------|---------|-------------|
| Tailer | `internal/tailer` | Watches log files using fsnotify |
| Parser | `internal/parser` | Parses raw lines into LogEntry |
| Buffer | `internal/agent` | Buffers entries for batching |
| gRPC Client | `internal/agent` | Streams logs to server |

**Data Flow:**
1. Tailer watches files for new content
2. New lines passed to appropriate parser
3. Parsed LogEntry added to buffer
4. Buffer flushes to server periodically or when full

### Server Components

| Component | Package | Description |
|-----------|---------|-------------|
| gRPC Server | `internal/server` | Receives logs from agents |
| Batch Writer | `internal/batch` | Batches for efficient storage |
| Storage | `internal/storage` | Persists to ClickHouse/SQLite |
| Alert Engine | `internal/alerting` | Evaluates alert rules |
| Notifier | `internal/notifier` | Sends notifications |
| SSH Collector | `internal/ssh` | Agentless collection |
| REST API | `internal/api` | HTTP API endpoints |
| Web UI | `internal/web` | Web dashboard |

---

## Data Flow

### Log Ingestion (Agent Mode)

```
Log File → Tailer → Parser → Buffer → gRPC Stream → Server
                                                      │
                                                      ▼
                                               Batch Writer
                                                      │
                                                      ▼
                                               ClickHouse
```

1. **Tailer** detects new content via fsnotify
2. **Parser** converts raw lines to structured LogEntry
3. **Buffer** collects entries (max 100 or 1 second)
4. **gRPC Stream** sends LogBatch to server
5. **Batch Writer** buffers for ClickHouse (5000 rows or 5 seconds)
6. **ClickHouse** stores logs with MergeTree engine

### Log Ingestion (SSH Mode)

```
Remote Server → SSH Client → Tailer → Parser → Batch Writer → ClickHouse
```

1. **SSH Client** connects to remote server
2. **Tailer** (remote) reads log files via SSH
3. Same parsing and storage pipeline

### Alert Processing

```
ClickHouse → Alert Engine → Rule Matcher → Notifier → Email/Slack/Teams
```

1. **Alert Engine** runs on configurable interval (default 30s)
2. **Rule Matcher** evaluates pattern/threshold rules
3. **Notifier** dispatches to configured channels
4. **Cooldown** prevents duplicate alerts

---

## Parser Architecture

```
internal/parser/
├── parser.go          # Parser interface
├── registry.go        # Parser registration
├── nginx_access.go    # Nginx access log
├── nginx_error.go     # Nginx error log
├── apache_access.go   # Apache access log
├── apache_error.go    # Apache error log
├── magento.go         # Magento Monolog
├── prestashop.go      # PrestaShop logs
├── wordpress.go       # WordPress debug.log
└── raw.go             # Fallback (raw line)
```

### Parser Interface

```go
type Parser interface {
    Parse(line string) (*models.LogEntry, error)
    Type() models.LogType
    Name() string
}
```

### Auto-Detection

1. Each parser attempts to parse the line
2. First successful match determines parser
3. Parser is cached per-file for efficiency
4. Falls back to raw parser if no match

---

## Storage Layer

### Dual Database Strategy

| Database | Purpose | Data |
|----------|---------|------|
| SQLite | Configuration | Users, alerts, projects, SSH connections |
| ClickHouse | Log Data | All ingested log entries |

### ClickHouse Schema

```sql
CREATE TABLE logs (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(3, 'UTC'),
    level LowCardinality(String),
    message String,
    source String,
    type LowCardinality(String),
    file_path String,
    line_number Int64,
    raw String,
    agent_id String,
    fields String,
    labels String,
    http_status UInt16 DEFAULT 0,
    http_method LowCardinality(String) DEFAULT '',
    uri String DEFAULT '',
    _date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(_date)
ORDER BY (agent_id, type, level, timestamp, id)
TTL _date + INTERVAL 30 DAY DELETE;
```

### Batch Insert

- Batches inserts for efficiency (1000 rows or 5 seconds)
- Uses ClickHouse's batch insert API
- Handles backpressure via buffering

---

## Alert Engine

### Rule Types

| Type | Evaluation |
|------|------------|
| Pattern | Regex match on log message |
| Threshold | Count over sliding time window |

### Evaluation Loop

```
┌───────────┐
│  Timer    │ (30s interval)
└─────┬─────┘
      ▼
┌───────────┐
│  Load     │ (get enabled rules)
│  Rules    │
└─────┬─────┘
      ▼
┌───────────┐
│  Query    │ (ClickHouse aggregations)
│  Logs     │
└─────┬─────┘
      ▼
┌───────────┐
│  Match    │ (evaluate conditions)
│  Rules    │
└─────┬─────┘
      ▼
┌───────────┐
│  Check    │ (skip if in cooldown)
│ Cooldown  │
└─────┬─────┘
      ▼
┌───────────┐
│  Notify   │ (dispatch to channels)
└───────────┘
```

---

## Security Model

### Authentication Layers

| Layer | Mechanism |
|-------|-----------|
| Agent → Server | mTLS (mutual TLS) |
| User → API | JWT tokens |
| User → Web | Session cookies |
| SSH Collection | SSH key auth |

### Authorization (RBAC)

| Role | Permissions |
|------|-------------|
| admin | Full access |
| operator | Manage alerts, view all |
| viewer | Read-only access |

### Key Security Features

- JWT with refresh tokens
- Account lockout after failed logins
- Rate limiting on sensitive endpoints
- CSRF protection on web forms
- Password hashing with bcrypt
- SSH key encryption at rest

---

## Web Architecture

### Technology Stack

| Component | Technology |
|-----------|------------|
| Templates | Templ (type-safe Go templates) |
| Interactivity | Alpine.js |
| Dynamic Updates | HTMX + SSE |
| Styling | Tailwind CSS |
| Charts | ECharts |
| HTTP Router | chi |

### Page Flow

```
Browser Request → chi Router → Handler → Templ Template → HTML Response
                                  │
                                  ▼
                              Storage
```

### Real-time Updates

- Dashboard stats via HTMX polling
- Log streaming via SSE (Server-Sent Events)
- Auto-refresh with configurable interval

---

## Packages Overview

```
internal/
├── agent/        # Agent runtime
├── alerting/     # Alert rules and engine
├── api/          # REST API handlers
├── batch/        # Batch writer for ClickHouse
├── metrics/      # Prometheus metrics
├── models/       # Data models
├── notifier/     # Email, Slack, Teams
├── parser/       # Log parsers
├── proto/        # Generated protobuf
├── security/     # TLS, certs, auth
├── server/       # gRPC server
├── ssh/          # SSH client
├── storage/      # SQLite + ClickHouse
├── tailer/       # File watching
└── web/          # Web UI
```

---

## Configuration Flow

```
YAML Files → Viper → Config Struct → Components
```

1. Load from YAML files
2. Override with environment variables
3. Validate configuration
4. Initialize components with config

---

## Deployment Modes

### Binary Mode

```
blazelog-server (single binary)
├── gRPC Server (9443)
├── HTTP Server (8080)
├── Web UI
├── SSH Collector
└── All processing

blazelog-agent (single binary)
├── File Tailer
├── Parser
├── Buffer
└── gRPC Client
```

### Docker Mode

```
docker-compose.yml
├── blazelog-server
├── blazelog-agent (optional, for local files)
├── clickhouse
└── (nginx reverse proxy)
```

### Kubernetes Mode

```
Namespace: blazelog
├── Deployment: blazelog-server
├── DaemonSet: blazelog-agent
├── StatefulSet: clickhouse
├── Services, ConfigMaps, Secrets
└── Ingress
```

---

## See Also

- [Protocol Specification](protocol.md) - gRPC protocol details
- [Configuration Reference](CONFIGURATION.md) - All configuration options
- [Security Guide](SECURITY.md) - Security architecture
- [Deployment Guide](DEPLOYMENT.md) - Installation options
