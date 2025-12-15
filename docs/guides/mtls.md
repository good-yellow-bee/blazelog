# mTLS Certificate Management

This guide covers setting up mutual TLS (mTLS) for secure agent-server communication.

---

## Overview

BlazeLog uses mutual TLS for agent-server authentication:

```
┌─────────────┐                    ┌─────────────┐
│   Agent     │◄──── mTLS ────────►│   Server    │
│             │                    │             │
│ - Client    │                    │ - Server    │
│   Cert      │                    │   Cert      │
│ - CA Cert   │                    │ - CA Cert   │
└─────────────┘                    └─────────────┘
```

- **Server authenticates to agents** via server certificate
- **Agents authenticate to server** via client certificates
- **Both verify the other** using the shared CA certificate

---

## Certificate Hierarchy

```
CA (Certificate Authority)
├── Server Certificate (server auth)
└── Agent Certificates (client auth)
    ├── agent1.crt
    ├── agent2.crt
    └── ...
```

---

## Quick Start

### 1. Initialize CA

```bash
# Create CA (one-time)
blazectl ca init

# Output:
# Created CA certificate: /etc/blazelog/ca/ca.crt
# Created CA private key: /etc/blazelog/ca/ca.key
```

### 2. Generate Server Certificate

```bash
# Generate server cert with hostname(s)
blazectl cert server --dns server.example.com

# For multiple hostnames
blazectl cert server --dns server.example.com --dns server.local

# Output:
# Created server certificate: /etc/blazelog/server.crt
# Created server private key: /etc/blazelog/server.key
```

### 3. Generate Agent Certificates

```bash
# Generate agent cert
blazectl cert agent --name agent1

# With custom validity
blazectl cert agent --name agent1 --days 365

# Output:
# Created agent certificate: /etc/blazelog/agents/agent1.crt
# Created agent private key: /etc/blazelog/agents/agent1.key
```

### 4. Distribute Certificates

```bash
# Copy to agent host
scp /etc/blazelog/ca/ca.crt agent-host:/etc/blazelog/
scp /etc/blazelog/agents/agent1.crt agent-host:/etc/blazelog/agent.crt
scp /etc/blazelog/agents/agent1.key agent-host:/etc/blazelog/agent.key
```

---

## Configuration

### Server Configuration

```yaml
# server.yaml
server:
  grpc_address: ":9443"
  tls:
    enabled: true
    cert_file: "/etc/blazelog/server.crt"
    key_file: "/etc/blazelog/server.key"
    ca_file: "/etc/blazelog/ca/ca.crt"
    client_auth: true  # Require client certificates
```

### Agent Configuration

```yaml
# agent.yaml
server:
  address: "server.example.com:9443"
  tls:
    enabled: true
    cert_file: "/etc/blazelog/agent.crt"
    key_file: "/etc/blazelog/agent.key"
    ca_file: "/etc/blazelog/ca.crt"
```

---

## Certificate Details

### CA Certificate

| Property | Value |
|----------|-------|
| Validity | 10 years (default) |
| Key Size | 4096 bits RSA |
| Usage | Certificate signing |
| Location | `/etc/blazelog/ca/ca.crt` |

### Server Certificate

| Property | Value |
|----------|-------|
| Validity | 1 year (default) |
| Key Size | 4096 bits RSA |
| Extended Key Usage | Server Auth |
| SANs | localhost, 127.0.0.1, ::1, custom hosts |
| Location | `/etc/blazelog/server.crt` |

### Agent Certificate

| Property | Value |
|----------|-------|
| Validity | 1 year (default) |
| Key Size | 4096 bits RSA |
| Extended Key Usage | Client Auth |
| SANs | Agent hostname |
| Location | `/etc/blazelog/agents/<name>.crt` |

---

## Certificate Commands

### View Certificate Details

```bash
# View certificate
openssl x509 -in /etc/blazelog/server.crt -text -noout

# View expiration
openssl x509 -in /etc/blazelog/server.crt -enddate -noout

# View SANs
openssl x509 -in /etc/blazelog/server.crt -text -noout | grep -A1 "Subject Alternative Name"
```

### Verify Certificate Chain

```bash
# Verify server cert against CA
openssl verify -CAfile /etc/blazelog/ca/ca.crt /etc/blazelog/server.crt

# Verify agent cert against CA
openssl verify -CAfile /etc/blazelog/ca/ca.crt /etc/blazelog/agents/agent1.crt
```

### Test TLS Connection

```bash
# Test server TLS (without client auth)
openssl s_client -connect server.example.com:9443 \
  -CAfile /etc/blazelog/ca/ca.crt

# Test mTLS (with client auth)
openssl s_client -connect server.example.com:9443 \
  -CAfile /etc/blazelog/ca/ca.crt \
  -cert /etc/blazelog/agent.crt \
  -key /etc/blazelog/agent.key
```

---

## Certificate Rotation

### Manual Rotation

1. **Generate new certificate:**
   ```bash
   blazectl cert server --dns server.example.com
   # or
   blazectl cert agent --name agent1
   ```

2. **Distribute to server/agent**

3. **Restart service:**
   ```bash
   sudo systemctl restart blazelog-server
   # or
   sudo systemctl restart blazelog-agent
   ```

### Automated Rotation (Future)

Certificate auto-rotation is planned for a future release. Until then:

- Monitor certificate expiration
- Set calendar reminders 30 days before expiry
- Use monitoring tools to alert on expiring certs

### Check Expiration

```bash
# Check all certificates
for cert in /etc/blazelog/*.crt /etc/blazelog/agents/*.crt; do
  echo "=== $cert ==="
  openssl x509 -in "$cert" -enddate -noout
done
```

---

## Revoking Certificates

Currently, certificate revocation is handled by:

1. **Stop using the certificate** - Remove from agent
2. **Regenerate with same name** - Overwrites old cert
3. **Delete old certificate files**

### Full Revocation (Paranoid)

If you suspect a certificate is compromised:

1. **Regenerate CA:**
   ```bash
   rm -rf /etc/blazelog/ca/
   blazectl ca init
   ```

2. **Regenerate all certificates:**
   ```bash
   blazectl cert server --dns server.example.com
   blazectl cert agent --name agent1
   blazectl cert agent --name agent2
   # ... for all agents
   ```

3. **Redistribute to all agents**

---

## Security Best Practices

### File Permissions

```bash
# CA private key - most sensitive
chmod 400 /etc/blazelog/ca/ca.key
chown root:root /etc/blazelog/ca/ca.key

# Server key
chmod 400 /etc/blazelog/server.key
chown blazelog:blazelog /etc/blazelog/server.key

# Agent key
chmod 400 /etc/blazelog/agent.key
chown blazelog:blazelog /etc/blazelog/agent.key

# Certificates (public, but restricted)
chmod 644 /etc/blazelog/*.crt
chmod 644 /etc/blazelog/ca/ca.crt
```

### CA Protection

- Store CA key offline or on HSM for production
- Never expose CA key to network
- Consider separate CA for BlazeLog only

### TLS Version

BlazeLog enforces TLS 1.3 minimum:

```yaml
# server.yaml (default, no config needed)
server:
  tls:
    min_version: "1.3"
```

---

## Docker Deployment

### Volume Mounts

```yaml
# docker-compose.yml
services:
  blazelog-server:
    volumes:
      - ./certs/ca.crt:/etc/blazelog/ca/ca.crt:ro
      - ./certs/server.crt:/etc/blazelog/server.crt:ro
      - ./certs/server.key:/etc/blazelog/server.key:ro

  blazelog-agent:
    volumes:
      - ./certs/ca.crt:/etc/blazelog/ca.crt:ro
      - ./certs/agent.crt:/etc/blazelog/agent.crt:ro
      - ./certs/agent.key:/etc/blazelog/agent.key:ro
```

### Kubernetes Secrets

```bash
# Create TLS secret for server
kubectl create secret tls blazelog-server-tls \
  --cert=/etc/blazelog/server.crt \
  --key=/etc/blazelog/server.key

# Create generic secret for CA
kubectl create secret generic blazelog-ca \
  --from-file=ca.crt=/etc/blazelog/ca/ca.crt
```

---

## Troubleshooting

### "certificate signed by unknown authority"

**Cause:** CA certificate not installed or wrong CA
**Solution:**
```bash
# Verify CA matches
openssl verify -CAfile /etc/blazelog/ca.crt /etc/blazelog/agent.crt
```

### "certificate has expired"

**Cause:** Certificate validity period passed
**Solution:**
```bash
# Check expiration
openssl x509 -in /etc/blazelog/server.crt -enddate -noout

# Regenerate
blazectl cert server --dns server.example.com
```

### "remote error: tls: bad certificate"

**Cause:** Client certificate rejected by server
**Solutions:**
- Verify client cert is signed by same CA
- Check Extended Key Usage includes Client Auth
- Verify cert hasn't expired

### "certificate name does not match"

**Cause:** Server hostname not in certificate SANs
**Solution:**
```bash
# Regenerate with correct hostname
blazectl cert server --dns correct-hostname.example.com
```

---

## See Also

- [Security Guide](../SECURITY.md) - Full security architecture
- [Deployment Guide](../DEPLOYMENT.md) - Installation and setup
- [SSH Collection Guide](ssh-collection.md) - SSH key management
- [Troubleshooting Guide](../TROUBLESHOOTING.md) - General troubleshooting
