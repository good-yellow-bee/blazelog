# BlazeLog First Run - Multi-Server Setup (Agent Per Container)

This guide shows how to simulate multiple servers locally using separate containers, each with its
own BlazeLog agent.

## Example Setup

Each container simulates a separate server with its own agent:

| App | Container | Agent Inside |
|-----|-----------|--------------|
| App 1 | app1 | yes |
| App 2 | app2 | yes |
| App 3 | app3 | yes |
| App 4 | app4 | yes |

```
+------------------+     +------------------+
|  App 1           |     |  App 2           |
|  +-----------+   |     |  +-----------+   |
|  | Agent     |---+-----+--| Agent     |   |
|  +-----------+   |     |  +-----------+   |
+------------------+     +------------------+
         |                        |
         +---------+--------------+
                   v
         +------------------+
         |  BlazeLog Server |
         |  localhost:9443  |
         +------------------+
                   ^
         +---------+--------------+
         |                        |
+------------------+     +------------------+
|  App 3           |     |  App 4           |
|  +-----------+   |     |  +-----------+   |
|  | Agent     |---+-----+--| Agent     |   |
|  +-----------+   |     |  +-----------+   |
+------------------+     +------------------+
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

### App 1

**Directory structure:**
```
app1/
├── docker-compose.yml
├── blazelog/
│   ├── blazelog-agent      # Copy from blazelog/build/
│   └── agent.yaml
```

**Copy agent binary:**
```bash
cp /path/to/blazelog/build/blazelog-agent ./app1/blazelog/
```

**Create `app1/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"  # Mac/Windows
  # address: "172.17.0.1:9443"          # Linux (docker0 bridge IP)
  tls:
    enabled: false

agent:
  name: "app1-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "app-logs"
    type: "raw"
    path: "/var/log/app/*.log"
    follow: true

  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

labels:
  project: "app1"
  environment: "development"
```

**Update `app1/docker-compose.yml`:**
```yaml
services:
  app1:
    # ... existing config ...
    volumes:
      - ./blazelog:/opt/blazelog:ro
    # Option A: Run agent in background (add to entrypoint)
    command: >
      bash -c "
        /opt/blazelog/blazelog-agent -c /opt/blazelog/agent.yaml &
        exec your-main-command
      "
    # Option B: Or add to existing entrypoint script
```

---

### App 2

**Create `app2/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "app2-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "apache-access"
    type: "apache"
    path: "/var/log/apache2/access.log"
    follow: true

labels:
  project: "app2"
  environment: "development"
```

---

### App 3

**Create `app3/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "app3-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

  - name: "app-logs"
    type: "raw"
    path: "/var/log/app/*.log"
    follow: true

labels:
  project: "app3"
  environment: "development"
```

---

### App 4

**Create `app4/blazelog/agent.yaml`:**
```yaml
server:
  address: "host.docker.internal:9443"
  tls:
    enabled: false

agent:
  name: "app4-server"
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "app-logs"
    type: "raw"
    path: "/app/logs/*.log"
    follow: true

labels:
  project: "app4"
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
  app1:
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
- `project:app1`
- `project:app2`
- `project:app3`
- `project:app4`

---

## Quick Copy Script

Run this to set up all containers:

```bash
#!/bin/bash
BLAZELOG_PATH="/path/to/blazelog"
PROJECTS=("app1" "app2" "app3" "app4")

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
docker exec -it app1 bash
curl -v telnet://host.docker.internal:9443

# Check agent logs inside container
docker exec -it app1 cat /var/log/blazelog-agent.log

# Verify agent is running
docker exec -it app1 ps aux | grep blazelog

# Check BlazeLog server logs
docker compose -f /path/to/blazelog/deployments/docker/docker-compose.yml logs
```
