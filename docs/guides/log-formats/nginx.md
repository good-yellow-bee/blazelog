# Nginx Log Format

BlazeLog parses Nginx access and error logs.

---

## Supported Formats

### Access Logs

**Combined format** (most common):
```
$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
```

Example:
```
192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/products HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0..."
```

**Common format**:
```
$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent
```

Example:
```
192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/products HTTP/1.1" 200 1234
```

### Error Logs

```
YYYY/MM/DD HH:MM:SS [level] pid#tid: *connection message
```

Example:
```
2024/01/15 10:30:00 [error] 12345#0: *100 upstream timed out, client: 192.168.1.1
```

---

## Default Log Locations

| Distribution | Access Log | Error Log |
|--------------|------------|-----------|
| Ubuntu/Debian | `/var/log/nginx/access.log` | `/var/log/nginx/error.log` |
| CentOS/RHEL | `/var/log/nginx/access.log` | `/var/log/nginx/error.log` |
| macOS (Homebrew) | `/usr/local/var/log/nginx/access.log` | `/usr/local/var/log/nginx/error.log` |

---

## Agent Configuration

```yaml
# agent.yaml
sources:
  - name: "nginx-access"
    path: "/var/log/nginx/access.log"
    type: "nginx"
    follow: true

  - name: "nginx-error"
    path: "/var/log/nginx/error.log"
    type: "nginx"
    follow: true

  # Multiple sites
  - name: "nginx-all"
    path: "/var/log/nginx/*.log"
    type: "nginx"
    follow: true
```

---

## Parsed Fields

### Access Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `remote_addr` | string | Client IP address |
| `remote_user` | string | Authenticated user (if any) |
| `method` | string | HTTP method (GET, POST, etc.) |
| `request_uri` | string | Request path |
| `protocol` | string | HTTP protocol version |
| `status` | int | HTTP status code |
| `body_bytes_sent` | int | Response body size in bytes |
| `http_referer` | string | Referer header (combined only) |
| `http_user_agent` | string | User agent (combined only) |

### Error Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `nginx_level` | string | Nginx error level |
| `pid` | int | Process ID |
| `tid` | int | Thread ID |
| `connection` | int | Connection number |
| `client` | string | Client IP |
| `upstream` | string | Upstream server (if applicable) |

### Log Level Mapping

| HTTP Status | Log Level |
|-------------|-----------|
| 100-399 | `info` |
| 400-499 | `warning` |
| 500-599 | `error` |

| Nginx Error Level | Log Level |
|-------------------|-----------|
| debug | `debug` |
| info, notice | `info` |
| warn | `warning` |
| error | `error` |
| crit, alert, emerg | `fatal` |

---

## Alert Rules

### High 5xx Error Rate

```yaml
- name: "Nginx 5xx Spike"
  description: "High rate of server errors"
  type: "threshold"
  condition:
    field: "status"
    operator: ">="
    value: 500
    threshold: 50
    window: "1m"
    log_type: "nginx"
  severity: "high"
  notify:
    - "slack"
```

### High 4xx Error Rate

```yaml
- name: "Nginx 4xx Spike"
  description: "High rate of client errors"
  type: "threshold"
  condition:
    field: "status"
    operator: ">="
    value: 400
    threshold: 100
    window: "5m"
    log_type: "nginx"
  severity: "medium"
  notify:
    - "slack"
```

### Slow Requests

```yaml
- name: "Slow Requests"
  description: "Requests taking more than 5 seconds"
  type: "threshold"
  condition:
    field: "request_time"
    operator: ">"
    value: 5.0
    threshold: 20
    window: "5m"
    log_type: "nginx"
  severity: "medium"
  notify:
    - "slack"
```

**Note:** Requires `$request_time` in Nginx log format.

### Upstream Timeout

```yaml
- name: "Upstream Timeout"
  description: "Backend server timeout detected"
  type: "pattern"
  condition:
    pattern: "upstream timed out"
    log_type: "nginx"
  severity: "high"
  notify:
    - "slack"
  cooldown: "5m"
```

### Large Response Bodies

```yaml
- name: "Large Responses"
  description: "Responses over 10MB"
  type: "threshold"
  condition:
    field: "body_bytes_sent"
    operator: ">"
    value: 10485760  # 10MB
    threshold: 10
    window: "10m"
    log_type: "nginx"
  severity: "low"
  notify:
    - "email"
```

---

## Custom Nginx Formats

If you use a custom Nginx log format, use `type: "auto"`:

```yaml
sources:
  - name: "nginx-custom"
    path: "/var/log/nginx/custom.log"
    type: "auto"  # Auto-detect or fall back to raw
```

For complex custom formats, see [Custom Patterns](custom.md).

---

## Nginx Configuration Tips

### Enable Request Time

Add `$request_time` to your log format for latency monitoring:

```nginx
log_format timed '$remote_addr - $remote_user [$time_local] '
                 '"$request" $status $body_bytes_sent '
                 '"$http_referer" "$http_user_agent" '
                 '$request_time';
```

### Separate Error Logs Per Site

```nginx
server {
    server_name example.com;
    access_log /var/log/nginx/example.com.access.log;
    error_log /var/log/nginx/example.com.error.log;
}
```

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
