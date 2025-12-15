# BlazeLog Deployment Guide

## Quick Start (Development)

```bash
# Build binaries
make build

# Set required secrets
export BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
export BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
export BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)

# Start server
./build/blazelog-server -c configs/server.yaml
```

Web UI: http://localhost:8080 | gRPC: localhost:9443 | Metrics: localhost:9090/metrics

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

# Build all binaries
make build

# Install to GOPATH/bin
make install
```

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

### Development Profile (SQLite)

```bash
cd deployments/docker

# Configure secrets
cp config/server.env.example .env
# Edit .env with generated secrets:
# BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
# BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)

# Start server only
docker compose --profile dev up -d
```

### Production Profile (ClickHouse)

```bash
cd deployments/docker

# Configure secrets
cp config/server.env.example .env
# Edit .env:
# BLAZELOG_MASTER_KEY=<generated>
# BLAZELOG_JWT_SECRET=<generated>
# BLAZELOG_CSRF_SECRET=<generated>
# CLICKHOUSE_PASSWORD=<generated>

# Start full stack
docker compose --profile prod up -d
```

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

### Docker Upgrade

```bash
cd deployments/docker

# Pull new images
docker compose pull

# Restart with new images
docker compose --profile prod up -d
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -u blazelog-server -n 50

# Common issues:
# - Missing BLAZELOG_MASTER_KEY → Set in /etc/blazelog/server.env
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
