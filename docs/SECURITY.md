# BlazeLog Security Guide

## Architecture Overview

```
                         ┌──────────────────────┐
                         │  Web UI (HTTPS)      │
                         │  CSRF + JWT Auth     │
                         └──────────┬───────────┘
                                    │
┌─────────────┐   mTLS    ┌────────▼────────────┐   TLS 1.3    ┌─────────────┐
│   Agents    │──────────►│   BlazeLog Server   │◄────────────►│ ClickHouse  │
└─────────────┘           └────────┬────────────┘              └─────────────┘
                                   │
                         ┌────────▼────────────┐
                         │  SQLite (Encrypted) │
                         │  Master Key AES-256 │
                         └─────────────────────┘
```

---

## Authentication

### JWT Authentication

- Algorithm: HMAC-SHA256
- Token lifetime: 24 hours
- Refresh: Automatic with valid token
- Secret: `BLAZELOG_JWT_SECRET` environment variable

### Password Requirements

- Minimum length: 12 characters
- Hashing: bcrypt (cost factor 12)
- No password reuse enforcement (yet)

### Account Lockout

| Condition | Action |
|-----------|--------|
| 5 failed logins in 15min | Account locked 30min |
| Locked account login | Shows "locked" message |
| After lockout expires | Counter resets |

---

## Authorization (RBAC)

### Roles

| Role | Capabilities |
|------|-------------|
| `admin` | Full access, user management, system config |
| `operator` | View logs, manage connections, view settings |
| `viewer` | Read-only log access |

### Resource Access

- Users can always access their own profile
- Users can change their own password
- Admin bypass applies to all protected routes
- Role checks via middleware on all `/api/*` routes

---

## Transport Security

### gRPC (Agent Connections)

- Protocol: gRPC over HTTP/2
- TLS: TLS 1.3 minimum
- Authentication: mTLS (mutual TLS)
- Client verification: Required when TLS enabled

### HTTP API

- Protocol: HTTP/1.1 or HTTP/2
- TLS: TLS 1.3 minimum (in production)
- HSTS: Enabled with 1-year max-age

### Certificate Requirements

```bash
# Generate CA (once)
blazectl ca init

# Server certificate (SAN required)
blazectl cert server \
  --cn server.example.com \
  --san DNS:server.example.com,IP:10.0.0.1

# Agent certificates (per agent)
blazectl cert agent --name agent-hostname
```

---

## Data Protection

### Encryption at Rest

| Data Type | Protection |
|-----------|------------|
| User passwords | bcrypt hash |
| SSH keys | AES-256-GCM (master key) |
| Connection credentials | AES-256-GCM (master key) |
| SQLite database | File permissions |

### Master Key

```bash
# Generate secure master key
openssl rand -base64 32

# Set as environment variable (not in config files!)
export BLAZELOG_MASTER_KEY="<generated-key>"
```

**Warning:** Losing the master key means losing access to encrypted credentials.

### Secrets Management

| Secret | Storage | Access |
|--------|---------|--------|
| `BLAZELOG_MASTER_KEY` | Environment only | Never logged |
| `BLAZELOG_JWT_SECRET` | Environment only | Never logged |
| `BLAZELOG_CSRF_SECRET` | Environment only | Never logged |
| SSH keys | Encrypted files | Decrypted in memory |

---

## Security Headers

All HTTP responses include:

```http
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'
Referrer-Policy: strict-origin-when-cross-origin
```

---

## Input Validation

### SQL Injection Protection

- All queries use parameterized statements
- No string concatenation in SQL
- ORM/query builder pattern

### Path Traversal Protection

- Log paths validated against whitelist
- No `..` in path components
- Absolute paths required

### XSS Protection

- HTML templating with auto-escaping (templ)
- CSP headers restrict inline scripts
- User input sanitized on display

---

## Rate Limiting

### Login Endpoint

```
Endpoint: POST /api/auth/login
Limit: 5 attempts per 15 minutes per IP
Lockout: 30 minutes after 5 failures
```

### API Endpoints

```
Default: 100 requests per minute per IP
Configurable per endpoint
```

### Implementation

- IP-based tracking with periodic cleanup
- Sliding window algorithm
- Memory-efficient (no external dependencies)

---

## SSH Security

### Host Key Verification

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `strict` | Reject unknown hosts | Production (pre-configured) |
| `tofu` | Trust first, reject changes | Balanced (default) |
| `warn` | Accept all, log warnings | Development only |

### SSH Audit Logging

All SSH operations logged to audit file:

```json
{"timestamp":"2024-01-01T00:00:00Z","event":"connect","host":"server.example.com","user":"blazelog"}
{"timestamp":"2024-01-01T00:00:01Z","event":"file_read","host":"server.example.com","path":"/var/log/nginx/access.log"}
```

### Jump Host Support

For accessing internal networks:

```yaml
ssh_connections:
  - name: "internal"
    host: "internal.example.com:22"
    jump_host:
      host: "bastion.example.com:22"
```

---

## Systemd Hardening

Service files include these protections:

| Setting | Effect |
|---------|--------|
| `NoNewPrivileges=yes` | Prevent privilege escalation |
| `ProtectSystem=strict` | Read-only filesystem (except allowed paths) |
| `ProtectHome=yes` | No access to home directories |
| `PrivateTmp=yes` | Isolated /tmp |
| `PrivateDevices=yes` | No access to devices |
| `RestrictAddressFamilies` | Only IPv4, IPv6, Unix sockets |
| `MemoryDenyWriteExecute=yes` | No W+X memory |
| `LockPersonality=yes` | Lock execution domain |
| `SystemCallArchitectures=native` | Native syscalls only |

---

## Production Hardening Checklist

### Pre-Deployment

- [ ] Generate unique `BLAZELOG_MASTER_KEY`
- [ ] Generate unique `BLAZELOG_JWT_SECRET`
- [ ] Generate unique `BLAZELOG_CSRF_SECRET`
- [ ] Generate TLS certificates (CA + server + agents)
- [ ] Enable mTLS in server config
- [ ] Set `host_key_policy: strict` for SSH
- [ ] Pre-populate known_hosts file

### Network

- [ ] Firewall: Allow 8080, 9443 from trusted sources only
- [ ] Firewall: Block 9090 (metrics) from public
- [ ] Use reverse proxy (nginx/traefik) with HTTPS
- [ ] Enable HSTS preload if using public domain

### Monitoring

- [ ] Enable Prometheus metrics
- [ ] Set up alerting for auth failures
- [ ] Monitor `/health/ready` endpoint
- [ ] Review SSH audit logs periodically

### Access Control

- [ ] Create individual user accounts (no shared admin)
- [ ] Use `viewer` role for read-only access
- [ ] Rotate JWT secret periodically
- [ ] Review user list quarterly

### Updates

- [ ] Subscribe to security advisories
- [ ] Test updates in staging first
- [ ] Keep dependencies updated
- [ ] Monitor CVE databases for Go dependencies

---

## Incident Response

### Suspected Compromise

1. **Isolate**: Disconnect server from network
2. **Preserve**: Snapshot logs before changes
3. **Rotate**: All secrets (master key, JWT, CSRF)
4. **Regenerate**: All TLS certificates
5. **Audit**: Review SSH audit logs
6. **Reset**: All user passwords

### Log Locations

| Log | Location |
|-----|----------|
| Application | `journalctl -u blazelog-server` |
| SSH audit | `/var/log/blazelog/ssh-audit.log` |
| Auth events | Application logs (JSON format) |

### Contact

Report security issues: security@good-yellow-bee.com (PGP available)

---

## See Also

- [mTLS Guide](guides/mtls.md) - Certificate management details
- [SSH Collection](guides/ssh-collection.md) - SSH security configuration
- [Configuration Reference](CONFIGURATION.md) - All security settings
- [Architecture Overview](ARCHITECTURE.md) - Security model
- [API Guide](api/API_GUIDE.md) - API authentication
