# BlazeLog

**Fast, secure, universal log analyzer with multi-platform support**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

BlazeLog is a universal log analyzer built in Go that provides real-time streaming and batch processing capabilities. It features an agent-server architecture with secure mTLS/gRPC communication, SSH-based log collection, and a web-based UI for management and visualization.

## Features

- **Multi-format parsing** — Nginx, Apache, Magento, PrestaShop, WordPress, custom regex
- **Real-time streaming** — Tail logs with fsnotify, handle log rotation
- **Alert rules engine** — Pattern matching (regex) + threshold detection with sliding windows
- **Notifications** — Email (SMTP/TLS), Slack, Microsoft Teams
- **Distributed collection** — Lightweight agents with mTLS/gRPC, offline buffering
- **SSH collection** — Pull logs from remote servers via SSH
- **Web dashboard** — Templ + HTMX + Alpine.js, real-time metrics, log search
- **CLI management** — Full project/user management via `blazectl`

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              BLAZELOG SYSTEM                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                │
│  │   Server A   │     │   Server B   │     │   Server C   │                │
│  │  (Magento)   │     │ (PrestaShop) │     │  (WordPress) │                │
│  │ ┌──────────┐ │     │ ┌──────────┐ │     │ ┌──────────┐ │                │
│  │ │ BlazeLog │ │     │ │ BlazeLog │ │     │ │ BlazeLog │ │                │
│  │ │  Agent   │ │     │ │  Agent   │ │     │ │  Agent   │ │                │
│  │ └────┬─────┘ │     │ └────┬─────┘ │     │ └────┬─────┘ │                │
│  └──────┼───────┘     └──────┼───────┘     └──────┼───────┘                │
│         │  mTLS/gRPC         │                    │                         │
│         └────────────────────┼────────────────────┘                         │
│                              ▼                                              │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                     BLAZELOG CENTRAL SERVER                        │     │
│  │ Log Processor │ Alert Engine │ Notifier │ SSH Connector │ REST API│     │
│  └───────────────────────────────────────────────────────────────────┘     │
│                              │                                              │
│                              ▼                                              │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                         WEB UI                                     │     │
│  │   Dashboard │ Log Search │ Alerts │ Projects │ Settings │ Users   │     │
│  └───────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+

### Build

```bash
make build
```

Note: `blazelog-server` and `blazectl` require SQLCipher (CGO + `-tags sqlcipher`).

### Parse logs

```bash
# Auto-detect format
blazelog parse auto /var/log/nginx/access.log

# Specific parser
blazelog parse nginx /var/log/nginx/access.log
blazelog parse magento /var/www/magento/var/log/system.log

# Output as JSON
blazelog parse auto /var/log/*.log --format json
```

### Tail logs with alerts

```bash
# Real-time tailing
blazelog tail /var/log/nginx/*.log --follow

# With notifications
blazelog tail /var/log/*.log --notify-email admin@example.com
blazelog tail /var/log/*.log --notify-slack https://hooks.slack.com/...
```

## Agent Installation

Zero-dependency single binary. Just copy and run.

```bash
# Download
wget https://releases.blazelog.example.com/agent/latest/linux-amd64/blazelog-agent
chmod +x blazelog-agent
sudo mv blazelog-agent /usr/local/bin/

# Initialize
sudo blazelog-agent init --server blazelog.example.com:9443

# Install as service
sudo blazelog-agent install-service
sudo systemctl enable --now blazelog-agent
```

**Supported platforms:** linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

## Configuration

### Agent (agent.yaml)

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

labels:
  environment: "production"
  project: "ecommerce-main"
```

### Alert Rules (alerts.yaml)

```yaml
rules:
  - name: "High Error Rate"
    type: "threshold"
    condition:
      field: "level"
      value: "error"
      threshold: 100
      window: "5m"
    severity: "critical"
    notify: ["slack", "email"]

  - name: "Fatal Error Detected"
    type: "pattern"
    condition:
      pattern: "FATAL|CRITICAL"
    severity: "critical"
    notify: ["slack", "teams", "email"]
    cooldown: "15m"
```

See [configs/](configs/) for full examples.

## Documentation

- [Deployment Guide](docs/DEPLOYMENT.md)
- [Configuration Reference](docs/CONFIGURATION.md)
- [First Run (Multi-Server Demo)](docs/guides/first-run.md)

## CLI Management

BlazeLog includes `blazectl` for full administrative control:

```bash
# User management
blazectl user list
blazectl user create --username admin --email admin@example.com --role admin
blazectl user passwd --username admin

# Project management
blazectl project list
blazectl project create --name myapp --description "My Application"
blazectl project add-member --name myapp --username alice --role operator
blazectl project members --name myapp
```

API-only mode (disable Web UI):

```bash
export BLAZELOG_WEB_UI_ENABLED=false
./blazelog-server
```

See [docs/CLI.md](docs/CLI.md) for full reference.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22+ |
| Agent-Server | gRPC with mTLS |
| Log Storage | ClickHouse |
| Config Storage | SQLite |
| HTTP Router | chi |
| Templating | Templ |
| Interactivity | HTMX + Alpine.js |
| Styling | Tailwind CSS |
| Charts | Apache ECharts |

## Project Status

| Stage | Description | Status |
|-------|-------------|--------|
| A | CLI Foundation (parsers) | ✅ Complete |
| B | Real-time & Alerting | ✅ Complete |
| C | Distributed Collection | ✅ Complete |
| D | Storage (ClickHouse) | ✅ Complete |
| E | REST API | ✅ Complete |
| F | Web UI | ✅ Complete |
| G | CLI Management | ✅ Complete |
| H | Production Hardening | 🔄 In Progress |

See [PLAN.md](PLAN.md) for detailed milestones.

## Contributing

Contributions welcome! Please read the contributing guidelines before submitting PRs.

## License

MIT License - see [LICENSE](LICENSE) for details.
