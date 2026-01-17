# Troubleshooting Guide

This guide covers common issues and their solutions for BlazeLog.

---

## Quick Diagnostics

### Check Service Status

```bash
# Systemd
sudo systemctl status blazelog-server
sudo systemctl status blazelog-agent

# Docker
docker ps | grep blazelog
docker logs blazelog-server
docker logs blazelog-agent
```

### Check Logs

```bash
# Systemd
journalctl -u blazelog-server -f
journalctl -u blazelog-agent -f

# Docker
docker logs -f blazelog-server
docker logs -f blazelog-agent
```

### Health Checks

```bash
# Server health
curl http://localhost:8080/health
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready

# Check specific components
curl http://localhost:8080/health | jq .
```

---

## Server Issues

### Server Won't Start

**Symptom:** Server fails to start, exits immediately

**Check 1: Missing environment variables**
```bash
# Required variables
echo $BLAZELOG_MASTER_KEY
echo $BLAZELOG_DB_KEY
echo $BLAZELOG_JWT_SECRET

# Set if missing
export BLAZELOG_MASTER_KEY=$(openssl rand -hex 32)
export BLAZELOG_DB_KEY=$(openssl rand -hex 32)
export BLAZELOG_JWT_SECRET=$(openssl rand -hex 32)
```

**Check 2: Port already in use**
```bash
# Check what's using the ports
lsof -i :8080  # HTTP
lsof -i :9443  # gRPC

# Kill conflicting process or change ports in config
```

**Check 3: Configuration errors**
```bash
# Validate config by running with the config file
blazelog-server --config server.yaml

# Common errors:
# - Invalid YAML syntax
# - Missing required fields
# - Invalid paths
```

**Check 4: Permission issues**
```bash
# Check data directory permissions
ls -la /var/lib/blazelog/
# Should be owned by blazelog user

# Fix permissions
sudo chown -R blazelog:blazelog /var/lib/blazelog/
```

### Database Errors

**Symptom:** "database is locked" or connection errors

**SQLite locked:**
```bash
# Check for multiple processes
ps aux | grep blazelog

# Only one server should access SQLite
# Kill duplicate processes
```

**ClickHouse connection failed:**
```bash
# Test ClickHouse connectivity
curl http://localhost:8123/

# Check ClickHouse is running
docker ps | grep clickhouse
# or
systemctl status clickhouse-server

# Verify connection string in config
# clickhouse://user:password@host:9000/database
```

### High Memory Usage

**Symptom:** Server using excessive memory

**Causes and solutions:**

1. **Too many buffered logs:**
   ```yaml
   # Reduce buffer sizes in server.yaml
   buffer:
     max_size: "50MB"  # Reduce from default
   ```

2. **Too many concurrent connections:**
   ```yaml
   # Limit gRPC connections
   grpc:
     max_concurrent_streams: 100
   ```

3. **Memory leak (rare):**
   ```bash
   # Monitor memory growth
   watch -n 5 'ps aux | grep blazelog'

   # Restart as temporary fix
   sudo systemctl restart blazelog-server
   ```

### High CPU Usage

**Symptom:** Server CPU at 100%

**Causes and solutions:**

1. **Too many alert rules:**
   - Review and consolidate rules
   - Increase cooldown periods

2. **Complex regex patterns:**
   - Simplify regex in alert rules
   - Use log_type filters to reduce scope

3. **Too many agents:**
   - Scale horizontally (multiple servers)
   - Increase server resources

---

## Agent Issues

### Agent Can't Connect to Server

**Symptom:** "connection refused" or timeout errors

**Check 1: Server is running**
```bash
# From agent host
curl http://server-host:8080/health
```

**Check 2: Network connectivity**
```bash
# Test TCP connection
nc -zv server-host 9443

# Check firewall
sudo iptables -L | grep 9443
```

**Check 3: TLS/mTLS configuration**
```bash
# Verify certificates exist
ls -la /etc/blazelog/agent.crt
ls -la /etc/blazelog/agent.key
ls -la /etc/blazelog/ca.crt

# Test TLS connection
openssl s_client -connect server-host:9443 \
  -cert /etc/blazelog/agent.crt \
  -key /etc/blazelog/agent.key \
  -CAfile /etc/blazelog/ca.crt
```

**Check 4: Agent configuration**
```yaml
# agent.yaml - verify server address
server:
  address: "server-host:9443"  # Correct host and port
  tls:
    enabled: true
    cert_file: "/etc/blazelog/agent.crt"
    key_file: "/etc/blazelog/agent.key"
    ca_file: "/etc/blazelog/ca.crt"
```

### Certificate Errors

**Symptom:** "x509: certificate signed by unknown authority"

**Solution 1: Verify CA certificate**
```bash
# Ensure agent has correct CA
openssl verify -CAfile /etc/blazelog/ca.crt /etc/blazelog/agent.crt
```

**Solution 2: Regenerate certificates**
```bash
# On server
blazectl ca init
blazectl cert server --dns server-host
blazectl cert agent --name agent1

# Copy to agent
scp /etc/blazelog/ca.crt agent-host:/etc/blazelog/
scp /etc/blazelog/agents/agent1.crt agent-host:/etc/blazelog/agent.crt
scp /etc/blazelog/agents/agent1.key agent-host:/etc/blazelog/agent.key
```

**Symptom:** "certificate has expired"

**Solution:** Regenerate certificates (default validity: 1 year)
```bash
blazectl cert agent --name agent1 --days 365
# Distribute new certificates
```

### Log File Permission Denied

**Symptom:** Agent can't read log files

**Check permissions:**
```bash
# Check agent user
ps aux | grep blazelog-agent

# Check log file permissions
ls -la /var/log/nginx/access.log

# Add agent user to appropriate group
sudo usermod -a -G adm blazelog
# or
sudo usermod -a -G nginx blazelog

# Restart agent
sudo systemctl restart blazelog-agent
```

### Agent Buffer Full

**Symptom:** "buffer full, dropping logs"

**Causes and solutions:**

1. **Network issues:**
   - Check connectivity to server
   - Review agent logs for errors

2. **Server overloaded:**
   - Check server health
   - Scale server resources

3. **Increase buffer:**
   ```yaml
   # agent.yaml
   buffer:
     max_size: "200MB"  # Increase from default
     path: "/var/lib/blazelog/buffer"
   ```

---

## SSH Collection Issues

### SSH Authentication Failed

**Symptom:** "authentication failed" when connecting via SSH

**Check 1: Key file permissions**
```bash
# Key must be readable only by owner
chmod 600 /etc/blazelog/ssh/server.key
ls -la /etc/blazelog/ssh/server.key
# Should show -rw-------
```

**Check 2: Key format**
```bash
# Verify key type (Ed25519 recommended)
ssh-keygen -l -f /etc/blazelog/ssh/server.key

# Test manually
ssh -i /etc/blazelog/ssh/server.key user@host "echo ok"
```

**Check 3: Target server authorized_keys**
```bash
# On target server
cat ~/.ssh/authorized_keys
# Should contain the public key
```

### Host Key Verification Failed

**Symptom:** "host key verification failed"

**Solution 1: Add host key**
```bash
# Get host key fingerprint
ssh-keyscan -H target-host >> /etc/blazelog/known_hosts
```

**Solution 2: Use TOFU mode (development only)**
```yaml
# server.yaml
ssh:
  host_key_policy: "tofu"  # Trust on first use
```

**Solution 3: Disable verification (NOT recommended)**
```yaml
# server.yaml
ssh:
  host_key_policy: "warn"  # Log warning but continue
```

### SSH Connection Timeout

**Symptom:** "connection timed out"

**Check 1: Network connectivity**
```bash
# Test basic connectivity
ping target-host
nc -zv target-host 22
```

**Check 2: SSH service on target**
```bash
# On target server
systemctl status sshd
```

**Check 3: Firewall rules**
```bash
# Check target firewall allows SSH from server
sudo iptables -L | grep ssh
```

**Check 4: Jump host configuration**
```yaml
# If using bastion, verify jump host config
ssh:
  connections:
    - name: "internal-server"
      host: "internal:22"
      user: "blazelog"
      key_file: "/etc/blazelog/ssh/key"
      jump_host: "bastion.example.com:22"
      jump_user: "jump"
      jump_key_file: "/etc/blazelog/ssh/bastion.key"
```

---

## Web UI Issues

### Can't Log In

**Symptom:** Login fails with valid credentials

**Check 1: User exists**
```bash
# Via API
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

**Check 2: Account locked**
- After 5 failed attempts, account locks for 30 minutes
- Wait for lockout to expire or restart server

**Check 3: JWT secret configured**
```bash
echo $BLAZELOG_JWT_SECRET
# Must be set and consistent across restarts
```

### SSE Streaming Not Working

**Symptom:** Real-time log view doesn't update

**Check 1: Browser support**
- SSE requires EventSource support
- Check browser console for errors

**Check 2: Proxy configuration**
```nginx
# Nginx proxy must allow SSE
location /api/v1/logs/stream {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
}
```

**Check 3: Firewall/load balancer**
- Long-lived connections may be terminated
- Increase timeout settings

### Session Expired

**Symptom:** Logged out unexpectedly

**Causes:**
- JWT token expired (default: 24h)
- Server restarted with different JWT secret
- Clock skew between client and server

**Solution:** Re-login or increase session duration:
```yaml
# server.yaml
auth:
  session_duration: "72h"  # Increase from default 24h
```

---

## Alert Issues

### Alerts Not Triggering

**Symptom:** Expected alerts don't fire

**Check 1: Rule is enabled**
```yaml
# alerts.yaml - ensure enabled is true or absent
- name: "My Rule"
  enabled: true  # or remove this line (defaults to true)
```

**Check 2: Pattern matches**
```bash
# Test regex pattern manually
echo "test log line" | grep -P "your-pattern"
```

**Check 3: Log type filter**
```yaml
# Ensure log_type matches
condition:
  log_type: "nginx"  # Must match actual log type
```

**Check 4: Label filter**
```yaml
# Ensure labels match agent labels
labels:
  project: "myapp"  # Agent must have this label
```

### Too Many Alerts

**Symptom:** Alert spam

**Solution 1: Add cooldown**
```yaml
- name: "My Rule"
  cooldown: "15m"  # Minimum time between alerts
```

**Solution 2: Increase threshold**
```yaml
condition:
  threshold: 100  # Raise the bar
  window: "10m"   # Or increase window
```

**Solution 3: Use rate limiting**
```yaml
# server.yaml
notifications:
  rate_limit:
    max_per_window: 10
    window: "1m"
```

### Notifications Not Sent

**Symptom:** Alert triggers but no notification

**Check 1: Notify channels configured**
```yaml
# alerts.yaml
- name: "My Rule"
  notify:
    - "slack"  # Must be configured in server.yaml
```

**Check 2: Channel configuration**
```yaml
# server.yaml - verify credentials
notifications:
  slack:
    webhook_url_env: "SLACK_WEBHOOK_URL"
```

**Check 3: Rate limiting**
```bash
# Check if notifications are being dropped
curl http://localhost:8080/metrics | grep rate_limit
```

See [Notifications Guide](guides/notifications.md) for channel-specific troubleshooting.

---

## Log Collection Issues

### Missing Logs

**Symptom:** Some log entries not appearing

**Check 1: File path correct**
```yaml
# agent.yaml
sources:
  - path: "/var/log/nginx/access.log"  # Verify path exists
```

**Check 2: Glob pattern works**
```bash
# Test glob pattern
ls /var/log/nginx/*.log
```

**Check 3: Log rotation**
- BlazeLog handles rotation automatically
- Check agent logs for rotation events

**Check 4: File encoding**
- Logs must be UTF-8 encoded
- Binary files are not supported

### Parser Errors

**Symptom:** "failed to parse log line"

**Check 1: Correct parser type**
```yaml
# agent.yaml
sources:
  - path: "/var/log/nginx/access.log"
    type: "nginx"  # Must match actual log format
```

**Check 2: Custom log format**
- If using custom Nginx/Apache format, try `type: "auto"`
- Consider custom regex parser

**Check 3: Log file content**
```bash
# Check first few lines
head -5 /var/log/nginx/access.log
# Should match expected format
```

### Log Rotation Issues

**Symptom:** Logs stop after rotation

**Check 1: Agent detects rotation**
- Agent watches for file changes via fsnotify
- Check agent logs for rotation events

**Check 2: File path changed**
```yaml
# Use glob pattern for rotated files
sources:
  - path: "/var/log/nginx/*.log"  # Catches rotated files
```

**Check 3: Inode reuse (rare)**
- Restart agent if rotation detection fails
- This is usually a filesystem edge case

---

## Performance Issues

### Slow Log Queries

**Symptom:** Web UI searches take too long

**SQLite (development):**
- Consider migrating to ClickHouse
- Reduce retention period
- Add indexes (advanced)

**ClickHouse:**
```sql
-- Check table sizes
SELECT
    table,
    formatReadableSize(sum(bytes)) as size
FROM system.parts
WHERE database = 'blazelog'
GROUP BY table;

-- Check query performance
SELECT *
FROM system.query_log
WHERE query_duration_ms > 1000
ORDER BY event_time DESC
LIMIT 10;
```

### High Network Usage

**Symptom:** Excessive bandwidth between agent and server

**Solution 1: Increase batch size**
```yaml
# agent.yaml
batch:
  max_size: 1000  # Fewer, larger batches
  max_wait: "5s"
```

**Solution 2: Filter at source**
- Only collect necessary log files
- Use log_type filters in alert rules

---

## Getting Help

If this guide doesn't resolve your issue:

1. **Check GitHub Issues:** https://github.com/good-yellow-bee/blazelog/issues
2. **Gather diagnostics:**
   ```bash
   blazelog-server --version
   blazelog-agent --version
   cat /etc/blazelog/server.yaml
   journalctl -u blazelog-server --since "1 hour ago"
   ```
3. **Open an issue** with:
   - BlazeLog version
   - OS and Go version
   - Configuration (redact secrets)
   - Error messages
   - Steps to reproduce

---

## See Also

- [Deployment Guide](DEPLOYMENT.md) - Installation and setup
- [Configuration Reference](CONFIGURATION.md) - All configuration options
- [Security Guide](SECURITY.md) - Security hardening
- [mTLS Guide](guides/mtls.md) - Certificate management
- [SSH Collection Guide](guides/ssh-collection.md) - SSH setup
