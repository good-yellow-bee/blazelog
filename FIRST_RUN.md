# BlazeLog First Run - Multi-Server Setup (Agent Per Container)

## Your Setup

Each container simulates a separate server with its own agent:

| Project | Container | Agent Inside |
|---------|-----------|--------------|
| PrestaShop 1 | prestashop1 | ✅ |
| PrestaShop 2 | prestashop2 | ✅ |
| Magento | magento | ✅ |
| Custom | custom | ✅ |

```
┌──────────────────┐     ┌──────────────────┐
│  PrestaShop 1    │     │  PrestaShop 2    │
│  ┌────────────┐  │     │  ┌────────────┐  │
│  │ Agent      │──┼─────┼──│ Agent      │  │
│  └────────────┘  │     │  └────────────┘  │
└──────────────────┘     └──────────────────┘
         │                        │
         └────────┬───────────────┘
                  ▼
         ┌──────────────────┐
         │  BlazeLog Server │
         │  localhost:9443  │
         └──────────────────┘
                  ▲
         ┌────────┴───────────────┐
         │                        │
┌──────────────────┐     ┌──────────────────┐
│  Magento         │     │  Custom App      │
│  ┌────────────┐  │     │  ┌────────────┐  │
│  │ Agent      │──┼─────┼──│ Agent      │  │
│  └────────────┘  │     │  └────────────┘  │
└──────────────────┘     └──────────────────┘
```

---

## Step 1: Start BlazeLog Server

```bash
cd /path/to/blazelog
docker compose --profile dev up -d
# Server at localhost:8080 (UI) and localhost:9443 (gRPC)
```

---

## Step 2: Build Agent Binary

```bash
cd /path/to/blazelog
make build-agent
# Creates: ./build/blazelog-agent
```

---

## Step 3: Setup Each Container

### PrestaShop 1

**Directory structure:**
```
prestashop1/
├── docker-compose.yml
├── blazelog/
│   ├── blazelog-agent      # Copy from blazelog/build/
│   └── agent.yaml
```

**Copy agent binary:**
```bash
cp /path/to/blazelog/build/blazelog-agent ./prestashop1/blazelog/
```

**Create `prestashop1/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"  # Mac/Windows
  # address: "172.17.0.1:9443"          # Linux (docker0 bridge IP)
  tls:
    enabled: false

agent:
  name: "prestashop1-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "prestashop-logs"
    type: "prestashop"
    path: "/var/www/html/var/logs/*.log"
    follow: true

  - name: "apache-access"
    type: "apache"
    path: "/var/log/apache2/access.log"
    follow: true

  - name: "apache-error"
    type: "apache"
    path: "/var/log/apache2/error.log"
    follow: true

labels:
  project: "prestashop1"
  environment: "development"
```

**Update `prestashop1/docker-compose.yml`:**
```yaml
services:
  prestashop:
    # ... existing config ...
    volumes:
      - ./blazelog:/opt/blazelog:ro
    # Option A: Run agent in background (add to entrypoint)
    command: >
      bash -c "
        /opt/blazelog/blazelog-agent -c /opt/blazelog/agent.yaml &
        apache2-foreground
      "
    # Option B: Or add to existing entrypoint script
```

---

### PrestaShop 2

**Create `prestashop2/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "prestashop2-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "prestashop-logs"
    type: "prestashop"
    path: "/var/www/html/var/logs/*.log"
    follow: true

  - name: "apache-access"
    type: "apache"
    path: "/var/log/apache2/access.log"
    follow: true

labels:
  project: "prestashop2"
  environment: "development"
```

---

### Magento

**Create `magento/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "magento-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "system"
    type: "magento"
    path: "/var/www/html/var/log/system.log"
    follow: true

  - name: "exception"
    type: "magento"
    path: "/var/www/html/var/log/exception.log"
    follow: true

  - name: "debug"
    type: "magento"
    path: "/var/www/html/var/log/debug.log"
    follow: true

  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

labels:
  project: "magento"
  environment: "development"
```

---

### Custom App

**Create `custom/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "custom-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "app-logs"
    type: "raw"
    path: "/app/logs/*.log"
    follow: true

labels:
  project: "custom"
  environment: "development"
```

---

## Step 4: Docker Compose Integration Options

### Option A: Background Process (Simple)

```yaml
services:
  app:
    volumes:
      - ./blazelog:/opt/blazelog:ro
    command: >
      bash -c "
        chmod +x /opt/blazelog/blazelog-agent &&
        /opt/blazelog/blazelog-agent -c /opt/blazelog/agent.yaml &
        exec your-main-command
      "
```

### Option B: Supervisor (Robust)

Add supervisord to manage both app and agent:

```dockerfile
# In your Dockerfile
RUN apt-get update && apt-get install -y supervisor
COPY supervisord.conf /etc/supervisor/conf.d/
```

```ini
# supervisord.conf
[program:app]
command=/usr/sbin/apache2ctl -D FOREGROUND
autostart=true
autorestart=true

[program:blazelog-agent]
command=/opt/blazelog/blazelog-agent -c /opt/blazelog/agent.yaml
autostart=true
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
```

### Option C: Sidecar Container (Cleanest)

```yaml
services:
  prestashop:
    volumes:
      - logs:/var/www/html/var/logs

  blazelog-agent:
    image: alpine:latest
    volumes:
      - ./blazelog:/opt/blazelog:ro
      - logs:/var/log/app:ro
    command: /opt/blazelog/blazelog-agent -c /opt/blazelog/agent.yaml
    depends_on:
      - prestashop

volumes:
  logs:
```

---

## Step 5: Network Configuration

### Mac/Windows (Docker Desktop)

Use `host.docker.internal:9443` - works out of the box.

### Linux

Option 1 - Use docker0 bridge IP:
```yaml
server:
  address: "172.17.0.1:9443"
```

Option 2 - Add host network alias:
```yaml
# docker-compose.yml
services:
  app:
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

Option 3 - Shared network with BlazeLog:
```yaml
# In your app's docker-compose.yml
networks:
  default:
    name: blazelog_blazelog
    external: true

# Then use:
server:
  address: "blazelog-server:9443"
```

---

## Step 6: View Logs

Open http://localhost:8080

Filter by project:
- `project:prestashop1`
- `project:prestashop2`
- `project:magento`
- `project:custom`

---

## Quick Copy Script

Run this to set up all containers:

```bash
#!/bin/bash
BLAZELOG_PATH="/path/to/blazelog"
PROJECTS=("prestashop1" "prestashop2" "magento" "custom")

for proj in "${PROJECTS[@]}"; do
  mkdir -p "$proj/blazelog"
  cp "$BLAZELOG_PATH/build/blazelog-agent" "$proj/blazelog/"
  echo "Created $proj/blazelog/ - add agent.yaml"
done
```

---

## Available Parsers

| Parser | Log Types |
|--------|-----------|
| `prestashop` | PrestaShop application logs |
| `magento` | Magento system/exception/debug logs |
| `nginx` | Nginx access/error logs |
| `apache` | Apache access/error logs |
| `raw` | Any plain text logs (no parsing) |

---

## Troubleshooting

```bash
# Test connectivity from inside container
docker exec -it prestashop1 bash
curl -v telnet://host.docker.internal:9443

# Check agent logs inside container
docker exec -it prestashop1 cat /var/log/blazelog-agent.log

# Verify agent is running
docker exec -it prestashop1 ps aux | grep blazelog

# Check BlazeLog server logs
docker compose -f /path/to/blazelog/deployments/docker/docker-compose.yml logs
```
