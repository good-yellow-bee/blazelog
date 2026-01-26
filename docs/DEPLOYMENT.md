# BlazeLog Deployment Guide

## Quick Start (Development)

```bash
# Build binaries
make build

# Set required secrets
export BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
export BLAZELOG_DB_KEY=$(openssl rand -base64 32)
export BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
export BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)

# Start server
./build/blazelog-server -c configs/server.yaml
```

Web UI: http://localhost:8080 | gRPC: localhost:9443 | Metrics: localhost:9090/metrics

---

## Local Development Workflow

For active development with the start/stop scripts:

### Initial Setup

```bash
# 1. Create local env file with secrets (one-time)
cat > .env.local << 'EOF'
export BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
export BLAZELOG_DB_KEY=$(openssl rand -base64 32)
export BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
export BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)
EOF

# 2. Source and regenerate with actual values
source .env.local
cat > .env.local << EOF
export BLAZELOG_MASTER_KEY="$BLAZELOG_MASTER_KEY"
export BLAZELOG_DB_KEY="$BLAZELOG_DB_KEY"
export BLAZELOG_JWT_SECRET="$BLAZELOG_JWT_SECRET"
export BLAZELOG_CSRF_SECRET="$BLAZELOG_CSRF_SECRET"
EOF
```

### Development Cycle

```bash
# Build server (includes templ generate + tailwind build)
make build-server

# Start server + agent
./scripts/blazelog-start.sh

# Stop all
./scripts/blazelog-stop.sh

# Rebuild and restart after code changes
make build-server && ./scripts/blazelog-stop.sh && ./scripts/blazelog-start.sh
```

### Helper Commands

```bash
# Watch mode for templates (separate terminal)
make templ-watch

# Watch mode for CSS (separate terminal)
make web-watch

# Both watchers in parallel
make dev-web

# Run tests
make test

# Lint code
make lint
```

### Log Files

- Server logs: `logs/server.log`
- Agent logs: `logs/agent.log`
- PID files: `.blazelog-server.pid`, `.blazelog-agent.pid`

### Configs for Local Dev

- Server: `configs/server-dev.yaml`
- Agent: `configs/agent-inpost.yaml` (customize for your setup)

---

## Binary Installation

### Download Pre-built Binaries

```bash
# Download latest release
VERSION=v1.0.0
curl -LO https://github.com/good-yellow-bee/blazelog/releases/download/${VERSION}/blazelog-${VERSION}-linux-amd64.tar.gz

# Extract
tar -xzf blazelog-${VERSION}-linux-amd64.tar.gz
sudo mv blazelog-* /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/good-yellow-bee/blazelog.git
cd blazelog

# Install dependencies
make deps

# Install SQLCipher (required for server + blazectl)
# macOS (Homebrew):
#   brew install sqlcipher
# Linux (Debian/Ubuntu):
#   sudo apt-get install -y sqlcipher libsqlcipher-dev
#
# Point CGO at SQLCipher headers and libs if needed:
#   export CGO_ENABLED=1
#   export CGO_CFLAGS="-I$(brew --prefix sqlcipher)/include"
#   export CGO_LDFLAGS="-L$(brew --prefix sqlcipher)/lib -lsqlcipher"

# Build all binaries
make build

# Install to GOPATH/bin
make install
```

### Cross-Compilation Notes

The server and CLI binaries require SQLCipher (CGO-enabled). Cross-compiling CGO binaries requires platform-specific toolchains.

**Recommended: Use Docker for cross-platform builds**

```bash
# Build Linux AMD64 image
docker build --platform linux/amd64 \
  -f deployments/docker/Dockerfile.server \
  -t blazelog-server:linux-amd64 .

# Build Linux ARM64 image
docker build --platform linux/arm64 \
  -f deployments/docker/Dockerfile.server \
  -t blazelog-server:linux-arm64 .
```

**Native cross-compilation** (advanced):
- Requires cross-compiler toolchain (e.g., `gcc-aarch64-linux-gnu` for ARM64)
- Requires SQLCipher compiled for target platform
- Set `CC` and `CGO_CFLAGS`/`CGO_LDFLAGS` for target architecture

The `make build-all-platforms` target works for the agent (CGO_ENABLED=0) but **will fail** for server/CLI without cross-compilation setup.

### Setup Directories

```bash
# Create user and directories
sudo useradd -r -s /bin/false blazelog
sudo mkdir -p /etc/blazelog/certs /var/lib/blazelog /var/log/blazelog
sudo chown -R blazelog:blazelog /var/lib/blazelog /var/log/blazelog
```

---

## Docker Deployment

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+

### Directory Structure

```
deployments/docker/
├── config/
│   ├── server.yaml          # Development server config (SQLite)
│   ├── server-prod.yaml     # Production server config (ClickHouse)
│   ├── agent.yaml           # Agent config
│   ├── known_hosts          # SSH known hosts
│   └── server.env.example   # Legacy env example
├── scripts/
│   └── agent-entrypoint.sh  # Agent initialization script
├── docker-compose.yml       # Main compose file
├── docker-compose.override.yml  # Dev overrides (auto-loaded)
├── docker-compose.prod.yml  # Production overrides
├── Dockerfile.server        # Server image (distroless)
├── Dockerfile.agent         # Agent image (Alpine)
├── .dockerignore
└── .env.example             # Environment template
```

### Quick Start (Development)

```bash
cd deployments/docker

# 1. Create environment file
cp .env.example .env

# 2. Generate secrets
echo "BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)" >> .env
echo "BLAZELOG_DB_KEY=$(openssl rand -base64 32)" >> .env
echo "BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)" >> .env
echo "BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)" >> .env

# 3. Start server (SQLite mode)
docker compose --profile dev up -d

# 4. Access Web UI
open http://localhost:8080
```

First run prints a random admin password to server logs (change immediately).

### Production Deployment (ClickHouse)

```bash
cd deployments/docker

# 1. Create environment file
cp .env.example .env

# 2. Generate ALL secrets
cat >> .env << 'EOF'
BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
BLAZELOG_DB_KEY=$(openssl rand -base64 32)
BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)
CLICKHOUSE_PASSWORD=$(openssl rand -base64 32)
EOF

# 3. Start full stack with production config
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile prod up -d

# 4. Verify all services are healthy
docker compose ps
```

### Profiles

| Profile | Services | Storage | Use Case |
|---------|----------|---------|----------|
| `dev` | server | SQLite | Development, testing |
| `prod` | server, clickhouse, agent | ClickHouse | Production |

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `BLAZELOG_MASTER_KEY` | Yes | - | Encryption key for sensitive data |
| `BLAZELOG_DB_KEY` | Yes | - | SQLite SQLCipher database key |
| `BLAZELOG_JWT_SECRET` | Yes | - | JWT signing secret |
| `BLAZELOG_CSRF_SECRET` | No | - | Web UI CSRF protection |
| `CLICKHOUSE_PASSWORD` | Prod | - | ClickHouse password |
| `BLAZELOG_HTTP_PORT` | No | 8080 | HTTP API port |
| `BLAZELOG_GRPC_PORT` | No | 9443 | gRPC port |
| `BLAZELOG_METRICS_PORT` | No | 9090 | Prometheus metrics port |

### Volumes

| Volume | Purpose |
|--------|---------|
| `blazelog-data` | SQLite database, audit logs |
| `blazelog-certs` | mTLS certificates |
| `blazelog-ssh` | SSH private keys |
| `blazelog-clickhouse-data` | ClickHouse data (prod) |

### Building Multi-arch Images

```bash
# Create buildx builder
docker buildx create --name blazelog --use

# Build server (amd64 + arm64)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f deployments/docker/Dockerfile.server \
  -t ghcr.io/good-yellow-bee/blazelog-server:latest \
  --push .

# Build agent
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f deployments/docker/Dockerfile.agent \
  -t ghcr.io/good-yellow-bee/blazelog-agent:latest \
  --push .
```

### Container Health

```bash
# Check status
docker compose ps

# View logs
docker compose logs -f server

# Check health endpoint
curl http://localhost:8080/health/ready

# Health endpoints
# GET /health       - Basic health check
# GET /health/live  - Liveness probe (k8s)
# GET /health/ready - Readiness probe (includes DB checks)
```

### mTLS Configuration

To enable mutual TLS for agent-server communication:

```bash
# 1. Generate certificates (from project root)
./build/blazectl ca init
./build/blazectl cert server --output deployments/docker/certs/
./build/blazectl cert agent --name agent-1 --output deployments/docker/certs/

# 2. Copy certs to volume
docker volume create blazelog-certs
docker run --rm -v blazelog-certs:/certs -v $(pwd)/deployments/docker/certs:/src alpine cp -r /src/. /certs/

# 3. Update config/server.yaml to enable TLS
# server.tls.enabled: true
# server.tls.cert_file: /etc/blazelog/certs/server.crt
# ...

# 4. Restart
docker compose --profile prod restart
```

### Troubleshooting

```bash
# Server won't start
docker compose logs server | tail -50

# Check if ports are available
ss -tlnp | grep -E '8080|9443|9090'

# Database issues
docker compose exec server cat /data/blazelog.db  # Should exist

# ClickHouse connection issues
docker compose exec clickhouse clickhouse-client --query "SELECT 1"

# Reset everything
docker compose down -v
docker compose --profile dev up -d
```

---

## Systemd Service Setup

### Install Service Files

```bash
# Copy binaries
sudo cp build/blazelog-server /usr/local/bin/
sudo cp build/blazelog-agent /usr/local/bin/

# Copy configs
sudo cp configs/server.yaml /etc/blazelog/
sudo cp configs/agent.yaml /etc/blazelog/

# Copy service files
sudo cp deployments/systemd/blazelog-server.service /etc/systemd/system/
sudo cp deployments/systemd/blazelog-agent.service /etc/systemd/system/

# Configure secrets
sudo tee /etc/blazelog/server.env << 'EOF'
BLAZELOG_MASTER_KEY=<your-master-key>
BLAZELOG_DB_KEY=<your-db-key>
BLAZELOG_JWT_SECRET=<your-jwt-secret>
BLAZELOG_CSRF_SECRET=<your-csrf-secret>
EOF
sudo chmod 600 /etc/blazelog/server.env
```

### Start Services

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable and start server
sudo systemctl enable blazelog-server
sudo systemctl start blazelog-server

# Check status
sudo systemctl status blazelog-server
sudo journalctl -u blazelog-server -f
```

### Agent on Remote Hosts

```bash
# Copy agent binary and config to remote host
scp build/blazelog-agent user@remote:/tmp/
scp configs/agent.yaml user@remote:/tmp/

# On remote host:
sudo mv /tmp/blazelog-agent /usr/local/bin/
sudo mkdir -p /etc/blazelog /var/lib/blazelog
sudo mv /tmp/agent.yaml /etc/blazelog/

# Edit agent.yaml - set server address
sudo nano /etc/blazelog/agent.yaml

# Install and start service
sudo cp deployments/systemd/blazelog-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now blazelog-agent
```

---

## TLS/mTLS Configuration

### Generate Certificates

```bash
# Initialize CA
blazectl ca init

# Generate server certificate
blazectl cert server --output /etc/blazelog/certs/

# Generate agent certificate
blazectl cert agent --name agent-1 --output /etc/blazelog/certs/
```

### Enable mTLS on Server

```yaml
# /etc/blazelog/server.yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/blazelog/certs/server.crt"
    key_file: "/etc/blazelog/certs/server.key"
    client_ca_file: "/etc/blazelog/certs/ca.crt"
```

### Enable mTLS on Agent

```yaml
# /etc/blazelog/agent.yaml
server:
  address: "server.example.com:9443"
  tls:
    enabled: true
    cert_file: "/etc/blazelog/certs/agent.crt"
    key_file: "/etc/blazelog/certs/agent.key"
    ca_file: "/etc/blazelog/certs/ca.crt"
```

---

## Monitoring

### Prometheus Metrics

Metrics endpoint: `http://localhost:9090/metrics`

Available metrics:
- `blazelog_http_requests_total{method,path,status}` - HTTP request count
- `blazelog_http_request_duration_seconds{method,path}` - Request latency
- `blazelog_grpc_streams_active` - Active agent connections
- `blazelog_grpc_batches_total` - Log batches received
- `blazelog_grpc_entries_total` - Log entries processed
- `blazelog_buffer_pending_entries` - Pending buffer entries
- `blazelog_storage_query_duration_seconds` - Storage query latency
- `blazelog_auth_login_total{status}` - Login attempts
- `blazelog_build_info{version,commit,build_time}` - Build information

### Prometheus Scrape Config

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'blazelog'
    static_configs:
      - targets: ['localhost:9090']
```

### Health Endpoints

| Endpoint | Purpose | Success Code |
|----------|---------|--------------|
| `/health` | Basic check | 200 |
| `/health/live` | Liveness probe (k8s) | 200 |
| `/health/ready` | Readiness probe (k8s) | 200/503 |

Example response:
```json
{"status": "ready", "checks": {"sqlite": "ok"}}
```

---

## Upgrading

### Binary Upgrade

```bash
# Stop service
sudo systemctl stop blazelog-server

# Backup current binary
sudo cp /usr/local/bin/blazelog-server /usr/local/bin/blazelog-server.bak

# Install new binary
sudo cp build/blazelog-server /usr/local/bin/

# Start service
sudo systemctl start blazelog-server

# Verify
sudo journalctl -u blazelog-server -f
```

### SQLCipher Migration (v0.x → v1.0)

Version 1.0 introduces SQLCipher database encryption. Existing unencrypted databases must be migrated.

**Breaking Change:** Unencrypted SQLite databases cannot be opened after upgrading.

**Migration Steps:**

1. **Backup existing database**
   ```bash
   cp data/blazelog.db data/blazelog.db.backup
   ```

2. **Export data** (before upgrade)
   ```bash
   sqlite3 data/blazelog.db .dump > blazelog_export.sql
   ```

3. **Upgrade binaries** and set new environment variable
   ```bash
   export BLAZELOG_DB_KEY=$(openssl rand -base64 32)
   # Save this key securely - required to open database
   ```

4. **Create new encrypted database**
   ```bash
   rm data/blazelog.db  # Remove old unencrypted DB
   ./blazelog-server -c configs/server.yaml  # Creates new encrypted DB
   ```

5. **Re-import data** (optional - for preserving users/alerts)
   ```bash
   # Import via CLI after server creates schema
   blazectl user create --username admin --role admin
   ```

**Note:** Log data in ClickHouse is unaffected. Only SQLite config data requires migration.

### Docker Upgrade

```bash
cd deployments/docker

# Pull new images
docker compose pull

# Restart with new images
docker compose --profile prod up -d
```

### Backup & Restore

**SQLite (config database)**
```bash
# Backup (requires BLAZELOG_DB_KEY)
sqlite3 data/blazelog.db ".backup 'blazelog_backup.db'"

# Or using blazectl (planned feature)
# blazectl backup --output blazelog_backup.sql
```

**ClickHouse (log data)**
```bash
# Using clickhouse-client
clickhouse-client --query "SELECT * FROM logs FORMAT Native" > logs_backup.native

# Restore
clickhouse-client --query "INSERT INTO logs FORMAT Native" < logs_backup.native
```

**Full backup script example:**
```bash
#!/bin/bash
DATE=$(date +%Y%m%d)
BACKUP_DIR=/var/backups/blazelog

mkdir -p $BACKUP_DIR
cp data/blazelog.db $BACKUP_DIR/blazelog_$DATE.db
# Note: DB key required to restore
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -u blazelog-server -n 50

# Common issues:
# - Missing BLAZELOG_MASTER_KEY → Set in /etc/blazelog/server.env
# - Missing BLAZELOG_DB_KEY → Set in /etc/blazelog/server.env
# - Port already in use → Check with: ss -tlnp | grep 8080
# - Permission denied → Check directory ownership
```

### Agent Can't Connect

```bash
# Check server is listening
ss -tlnp | grep 9443

# Test connectivity
nc -zv server-address 9443

# Check TLS certificates
openssl s_client -connect server-address:9443

# Agent logs
sudo journalctl -u blazelog-agent -f
```

### High Memory Usage

```bash
# Check buffer settings in config
# Reduce batch_size or increase flush_interval

# Monitor metrics
curl http://localhost:9090/metrics | grep blazelog_buffer
```

---

## See Also

- [Configuration Reference](CONFIGURATION.md) - All configuration options
- [Security Guide](SECURITY.md) - Security hardening
- [mTLS Guide](guides/mtls.md) - Certificate setup
- [Performance Tuning](PERFORMANCE.md) - Optimization guide
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues
- [Architecture Overview](ARCHITECTURE.md) - System design
