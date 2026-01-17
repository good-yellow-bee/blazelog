# BlazeLog Configuration Reference

## Environment Variables

### Required (Server)

| Variable | Description | Generation |
|----------|-------------|------------|
| `BLAZELOG_MASTER_KEY` | Encryption key for sensitive data | `openssl rand -base64 32` |
| `BLAZELOG_DB_KEY` | SQLCipher database key | `openssl rand -base64 32` |
| `BLAZELOG_JWT_SECRET` | JWT signing secret | `openssl rand -base64 32` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `BLAZELOG_CSRF_SECRET` | CSRF protection secret (enables Web UI) | - |
| `BLAZELOG_WEB_UI_ENABLED` | Enable Web UI (`true`/`false`) | `true` |
| `CLICKHOUSE_PASSWORD` | ClickHouse password (prod profile) | - |

### Disabling Web UI

To run BlazeLog in API-only mode (for security or CLI-only deployments):

```bash
export BLAZELOG_WEB_UI_ENABLED=false
./blazelog-server
```

When disabled:
- All `/api/v1/*` endpoints remain available
- Web UI routes return 404
- CLI management still works via `blazectl`

---

## Server Configuration

Location: `/etc/blazelog/server.yaml`

### Full Reference

```yaml
# Server network settings
server:
  # gRPC listen address for agent connections
  grpc_address: ":9443"  # default

  # HTTP listen address for REST API and Web UI
  http_address: ":8080"  # default

  # HTTPS configuration for HTTP API
  http_tls:
    enabled: false
    cert_file: "/etc/blazelog/certs/http.crt"
    key_file: "/etc/blazelog/certs/http.key"

  # TLS/mTLS configuration
  tls:
    # Enable mTLS (false for development)
    enabled: false

    # Server certificate
    cert_file: "/etc/blazelog/certs/server.crt"

    # Server private key
    key_file: "/etc/blazelog/certs/server.key"

    # CA certificate for client verification
    client_ca_file: "/etc/blazelog/certs/ca.crt"

# Metrics endpoint configuration
metrics:
  # Enable Prometheus metrics (default: true)
  enabled: true

  # Metrics server address (separate from main API)
  address: ":9090"  # default

# SQLite database (metadata, users, connections)
database:
  # Database file path
  path: "./data/blazelog.db"  # default

# Authentication settings
auth:
  # JWT secret environment variable name
  jwt_secret_env: "BLAZELOG_JWT_SECRET"

  # CSRF secret environment variable name (Web UI)
  csrf_secret_env: "BLAZELOG_CSRF_SECRET"

  # Access token TTL (default: 15m)
  access_token_ttl: "15m"

  # Refresh token TTL (default: 168h / 7 days)
  refresh_token_ttl: "168h"

  # Login rate limit per IP (default: 5/15m)
  rate_limit_per_ip: 5

  # API rate limit per user (default: 100/min)
  rate_limit_per_user: 100

  # Failed attempts before lockout (default: 5)
  lockout_threshold: 5

  # Lockout duration (default: 30m)
  lockout_duration: "30m"

```

---

## Agent Configuration

Location: `/etc/blazelog/agent.yaml`

### Full Reference

```yaml
# Server connection
server:
  # BlazeLog server address (host:port)
  address: "localhost:9443"

  # TLS configuration
  tls:
    # Enable mTLS
    enabled: false

    # Agent certificate
    cert_file: "/etc/blazelog/certs/agent.crt"

    # Agent private key
    key_file: "/etc/blazelog/certs/agent.key"

    # CA certificate for server verification
    ca_file: "/etc/blazelog/certs/ca.crt"

    # Skip server certificate verification (dev only!)
    insecure_skip_verify: false

# Agent settings
agent:
  # Agent name (defaults to hostname)
  name: "my-server"

  # Entries per batch
  batch_size: 100  # default

  # Batch flush interval
  flush_interval: 1s  # default

# Log sources to collect
sources:
  # Nginx logs
  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

  - name: "nginx-error"
    type: "nginx"
    path: "/var/log/nginx/error.log"
    follow: true

  # Application logs with glob patterns
  - name: "app-logs"
    type: "auto"
    path: "/var/log/app/*.log"
    follow: true

# Labels for categorization
labels:
  environment: "production"
  project: "my-project"
  team: "ops"
```

---

## Log Source Types

| Type | Description | Examples |
|------|-------------|----------|
| `nginx` | Nginx access/error logs | combined, common, error |
| `apache` | Apache httpd logs | access, error |
| `magento` | Magento 2 logs | system.log, exception.log |
| `prestashop` | PrestaShop logs | var/logs/*.log |
| `wordpress` | WordPress debug logs | debug.log |
| `syslog` | Standard syslog format | /var/log/syslog |
| `json` | JSON-formatted logs | Structured logs |
| `auto` | Auto-detect format | Any log type |

---

## TLS/mTLS Setup

### Certificate Generation

```bash
# Initialize Certificate Authority
blazectl ca init

# Generate server certificate
blazectl cert server \
  --output /etc/blazelog/certs/ \
  --cn server.example.com \
  --san DNS:server.example.com,IP:10.0.0.1

# Generate agent certificate
blazectl cert agent \
  --name agent-1 \
  --output /etc/blazelog/certs/
```

### Certificate Files

| File | Purpose | Location |
|------|---------|----------|
| `ca.crt` | CA certificate | All nodes |
| `server.crt` | Server certificate | Server only |
| `server.key` | Server private key | Server only |
| `agent.crt` | Agent certificate | Agent only |
| `agent.key` | Agent private key | Agent only |

### TLS Versions

BlazeLog enforces TLS 1.3 minimum for all connections.

---

## Rate Limiting

Built-in rate limiting is enabled by default:

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/api/auth/login` | 5 attempts | 15 minutes |
| `/api/auth/login` (failed) | Lockout after 5 failures | 30 minutes |
| API endpoints | 100 requests | per minute |

---

## Storage Backends

### SQLite (Default)

```yaml
database:
  path: "./data/blazelog.db"
```

- Used for: metadata, users, connections, tokens
- Good for: development, small deployments

### ClickHouse (Production)

Configure via Docker Compose prod profile or external ClickHouse:

```yaml
clickhouse:
  address: "localhost:9000"
  database: "blazelog"
  user: "blazelog"
  password_env: "CLICKHOUSE_PASSWORD"
```

- Used for: log storage, high-volume queries
- Good for: production, large-scale deployments

---

## Configuration Validation

```bash
# Validate server config
blazelog-server -c /etc/blazelog/server.yaml validate

# Validate agent config
blazelog-agent -c /etc/blazelog/agent.yaml validate
```

---

## Common Configurations

### Development

```yaml
server:
  grpc_address: ":9443"
  http_address: ":8080"
  tls:
    enabled: false

database:
  path: "./data/blazelog.db"
```

### Production (Single Node)

```yaml
server:
  grpc_address: ":9443"
  http_address: ":8080"
  tls:
    enabled: true
    cert_file: "/etc/blazelog/certs/server.crt"
    key_file: "/etc/blazelog/certs/server.key"
    client_ca_file: "/etc/blazelog/certs/ca.crt"

metrics:
  enabled: true
  address: ":9090"

database:
  path: "/var/lib/blazelog/blazelog.db"

```

### Production (ClickHouse)

Use Docker Compose with `--profile prod` or configure external ClickHouse.

---

## See Also

- [CLI Reference](CLI.md) - Full CLI command documentation
- [Alert Rules Reference](guides/alerts.md) - Alert configuration details
- [Notification Setup](guides/notifications.md) - Email/Slack/Teams configuration
- [Log Formats](guides/log-formats/README.md) - Supported log formats
- [mTLS Guide](guides/mtls.md) - Certificate configuration
- [Deployment Guide](DEPLOYMENT.md) - Installation and deployment
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues
