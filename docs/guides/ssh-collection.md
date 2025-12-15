# SSH Log Collection Guide

This guide covers collecting logs from remote servers via SSH (agentless mode).

---

## Overview

SSH collection allows the BlazeLog server to pull logs directly from remote servers without installing agents:

```
┌─────────────┐         SSH          ┌─────────────┐
│   BlazeLog  │ ──────────────────── │   Remote    │
│   Server    │   (Key Auth)         │   Server    │
└─────────────┘                      └─────────────┘
       │
       └── Reads /var/log/*.log via SSH
```

### When to Use SSH vs Agents

| Use Case | Recommended |
|----------|-------------|
| Production servers | **Agent** (better reliability, lower latency) |
| Cannot install software | **SSH** |
| Legacy systems | **SSH** |
| Quick setup / testing | **SSH** |
| High-volume logs | **Agent** (local buffering) |
| Network restrictions | **Agent** (outbound only) |

---

## Quick Start

### 1. Create SSH User on Target Server

```bash
# On target server
sudo useradd -r -s /bin/bash blazelog
sudo mkdir -p /home/blazelog/.ssh
sudo chmod 700 /home/blazelog/.ssh
```

### 2. Generate SSH Key on BlazeLog Server

```bash
# Ed25519 (recommended)
ssh-keygen -t ed25519 -C "blazelog@server" -f /etc/blazelog/ssh/target.key -N ""

# Or RSA-4096
ssh-keygen -t rsa -b 4096 -C "blazelog@server" -f /etc/blazelog/ssh/target.key -N ""
```

### 3. Copy Public Key to Target

```bash
# Copy public key
ssh-copy-id -i /etc/blazelog/ssh/target.key.pub blazelog@target-server

# Or manually
cat /etc/blazelog/ssh/target.key.pub | ssh admin@target-server \
  "sudo tee /home/blazelog/.ssh/authorized_keys"
```

### 4. Grant Log Read Access

```bash
# On target server
# Option 1: Add to adm group (access to /var/log/)
sudo usermod -a -G adm blazelog

# Option 2: ACL for specific logs
sudo setfacl -R -m u:blazelog:r /var/log/nginx/
sudo setfacl -R -m u:blazelog:r /var/www/magento/var/log/
```

### 5. Configure BlazeLog Server

```yaml
# server.yaml
ssh_connections:
  - name: "web-server-1"
    host: "web1.example.com:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/target.key"
    sources:
      - path: "/var/log/nginx/access.log"
        type: "nginx"
      - path: "/var/log/nginx/error.log"
        type: "nginx"
```

---

## Configuration

### Connection Settings

```yaml
ssh_connections:
  - name: "production-web"           # Unique identifier
    host: "web.example.com:22"       # Host and port
    user: "blazelog"                 # SSH username
    key_file: "/etc/blazelog/ssh/key" # Private key path

    # Optional settings
    timeout: "30s"                   # Connection timeout
    keepalive_interval: "60s"        # SSH keepalive
    max_retries: 3                   # Reconnection attempts

    sources:
      - path: "/var/log/nginx/*.log"
        type: "nginx"
        follow: true                 # Tail mode
```

### Multiple Servers

```yaml
ssh_connections:
  - name: "web-1"
    host: "web1.example.com:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/web1.key"
    sources:
      - path: "/var/log/nginx/*.log"
        type: "nginx"

  - name: "web-2"
    host: "web2.example.com:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/web2.key"
    sources:
      - path: "/var/log/nginx/*.log"
        type: "nginx"

  - name: "magento"
    host: "magento.example.com:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/magento.key"
    sources:
      - path: "/var/www/magento/var/log/*.log"
        type: "magento"
```

---

## Host Key Verification

BlazeLog supports three host key policies:

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `strict` | Reject unknown hosts | Production |
| `tofu` | Trust on first use, verify after | Development |
| `warn` | Accept all, log warnings | Testing only |

### Configuration

```yaml
# server.yaml
ssh:
  host_key_policy: "tofu"  # Default
  known_hosts_file: "/etc/blazelog/ssh/known_hosts"
```

### Pre-populating Known Hosts

```bash
# Scan host keys
ssh-keyscan -H web1.example.com >> /etc/blazelog/ssh/known_hosts
ssh-keyscan -H web2.example.com >> /etc/blazelog/ssh/known_hosts

# Set strict policy
# server.yaml
ssh:
  host_key_policy: "strict"
```

### Policy Details

**Strict (Production):**
- Rejects connections to unknown hosts
- Pre-populate known_hosts before deployment
- Most secure option

**TOFU (Development):**
- Accepts unknown hosts on first connection
- Stores key for future verification
- Rejects if key changes (possible MITM)

**Warn (Testing Only):**
- Accepts all connections
- Logs warnings for unknown/changed keys
- **Never use in production**

---

## Jump Host / Bastion

For servers behind a bastion/jump host:

```yaml
ssh_connections:
  - name: "internal-server"
    host: "internal.private:22"
    user: "blazelog"
    key_file: "/etc/blazelog/ssh/internal.key"

    # Jump host configuration
    jump_host: "bastion.example.com:22"
    jump_user: "jump"
    jump_key_file: "/etc/blazelog/ssh/bastion.key"

    sources:
      - path: "/var/log/app/*.log"
        type: "auto"
```

### Multi-hop (Future)

Multiple jump hosts are planned for a future release.

---

## Key Encryption

SSH private keys are encrypted at rest using AES-256-GCM.

### Master Key

```bash
# Set master key (required)
export BLAZELOG_MASTER_KEY=$(openssl rand -hex 32)

# Store securely - needed on every server start
```

### Encrypted Key Storage

When you configure SSH connections, BlazeLog encrypts the keys:

1. Reads private key from `key_file`
2. Encrypts with master key
3. Stores encrypted version in database
4. Original file can be deleted (optional)

### Best Practices

- Use strong master key (256 bits)
- Store master key in secure vault (HashiCorp Vault, AWS Secrets Manager)
- Never log or expose master key
- Rotate master key periodically (requires re-encryption)

---

## Audit Logging

SSH operations are logged for security auditing:

```yaml
# server.yaml
ssh:
  audit_log: "/var/log/blazelog/ssh-audit.log"
```

### Logged Events

| Event | Description |
|-------|-------------|
| `SSH_CONNECT` | Connection established |
| `SSH_DISCONNECT` | Connection closed |
| `SSH_AUTH_SUCCESS` | Authentication succeeded |
| `SSH_AUTH_FAILURE` | Authentication failed |
| `SSH_HOST_KEY_ACCEPTED` | Host key verified |
| `SSH_HOST_KEY_REJECTED` | Host key mismatch (possible attack) |
| `SSH_FILE_READ` | File read operation |

### Log Format

```json
{"time":"2024-01-15T10:30:00Z","event":"SSH_CONNECT","host":"web1.example.com:22","user":"blazelog"}
{"time":"2024-01-15T10:30:01Z","event":"SSH_HOST_KEY_ACCEPTED","host":"web1.example.com:22","fingerprint":"SHA256:...","first_use":false}
{"time":"2024-01-15T10:30:02Z","event":"SSH_AUTH_SUCCESS","host":"web1.example.com:22","user":"blazelog"}
```

---

## Connection Pooling

BlazeLog maintains connection pools for efficiency:

```yaml
# server.yaml
ssh:
  pool:
    max_connections_per_host: 5
    idle_timeout: "5m"
    max_lifetime: "1h"
```

- Connections are reused for multiple file reads
- Idle connections are closed after timeout
- Maximum lifetime prevents stale connections

---

## Security Best Practices

### Key Management

```bash
# Key file permissions
chmod 600 /etc/blazelog/ssh/*.key
chown blazelog:blazelog /etc/blazelog/ssh/*.key
```

### Minimal Permissions

```bash
# On target server - restrict blazelog user
# 1. No sudo access
# 2. No shell access (optional)
sudo usermod -s /sbin/nologin blazelog

# 3. Restrict to specific directories
# In /etc/ssh/sshd_config:
Match User blazelog
    ForceCommand internal-sftp
    ChrootDirectory /var/log
```

### IP Restrictions

```bash
# On target server - restrict source IPs
# In /home/blazelog/.ssh/authorized_keys:
from="10.0.0.1,10.0.0.2" ssh-ed25519 AAAA... blazelog@server
```

### Rate Limiting

```yaml
# server.yaml
ssh:
  rate_limit:
    connections_per_minute: 10
    enabled: true
```

---

## Troubleshooting

### Connection Refused

```bash
# Check SSH service on target
ssh blazelog@target-server

# Check firewall
sudo iptables -L | grep ssh
```

### Permission Denied

```bash
# Verify key permissions
ls -la /etc/blazelog/ssh/

# Test key manually
ssh -i /etc/blazelog/ssh/target.key blazelog@target-server

# Check authorized_keys on target
cat /home/blazelog/.ssh/authorized_keys
```

### Host Key Verification Failed

```bash
# Get current host key
ssh-keyscan target-server

# Compare with stored
cat /etc/blazelog/ssh/known_hosts | grep target-server

# If intentionally changed, remove old entry
ssh-keygen -R target-server -f /etc/blazelog/ssh/known_hosts
```

### Can't Read Log Files

```bash
# Check permissions on target
ssh blazelog@target-server "ls -la /var/log/nginx/"

# Add to appropriate group
sudo usermod -a -G adm blazelog
sudo usermod -a -G nginx blazelog
```

### Connection Timeout

```yaml
# Increase timeout
ssh_connections:
  - name: "slow-server"
    timeout: "60s"
    keepalive_interval: "30s"
```

---

## See Also

- [mTLS Guide](mtls.md) - Agent certificate management
- [Security Guide](../SECURITY.md) - Full security architecture
- [Configuration Reference](../CONFIGURATION.md) - All SSH options
- [Troubleshooting Guide](../TROUBLESHOOTING.md) - General troubleshooting
