# Apache Log Format

BlazeLog parses Apache HTTPD access and error logs.

---

## Supported Formats

### Access Logs

**Combined format** (most common):
```
%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"
```

Example:
```
192.168.1.1 - john [15/Jan/2024:10:30:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com/" "Mozilla/5.0..."
```

**Common Log Format (CLF)**:
```
%h %l %u %t "%r" %>s %b
```

Example:
```
192.168.1.1 - john [15/Jan/2024:10:30:00 +0000] "GET /index.html HTTP/1.1" 200 1234
```

### Error Logs

**Apache 2.4+ format**:
```
[timestamp] [module:level] [pid tid] [client ip:port] message
```

Example:
```
[Mon Jan 15 10:30:00.123456 2024] [php:error] [pid 12345] [client 192.168.1.1:54321] PHP Fatal error: ...
```

**Apache 2.2 format**:
```
[timestamp] [level] [client ip] message
```

Example:
```
[Mon Jan 15 10:30:00 2024] [error] [client 192.168.1.1] File does not exist: /var/www/html/missing.html
```

---

## Default Log Locations

| Distribution | Access Log | Error Log |
|--------------|------------|-----------|
| Ubuntu/Debian | `/var/log/apache2/access.log` | `/var/log/apache2/error.log` |
| CentOS/RHEL | `/var/log/httpd/access_log` | `/var/log/httpd/error_log` |
| macOS | `/var/log/apache2/access_log` | `/var/log/apache2/error_log` |

---

## Agent Configuration

```yaml
# agent.yaml
sources:
  # Ubuntu/Debian
  - name: "apache-access"
    path: "/var/log/apache2/access.log"
    type: "apache"
    follow: true

  - name: "apache-error"
    path: "/var/log/apache2/error.log"
    type: "apache"
    follow: true

  # CentOS/RHEL
  - name: "httpd-access"
    path: "/var/log/httpd/access_log"
    type: "apache"
    follow: true

  - name: "httpd-error"
    path: "/var/log/httpd/error_log"
    type: "apache"
    follow: true

  # Virtual host logs
  - name: "vhosts"
    path: "/var/log/apache2/vhosts/*.log"
    type: "apache"
    follow: true
```

---

## Parsed Fields

### Access Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `remote_addr` | string | Client IP address |
| `ident` | string | RFC 1413 identity (usually "-") |
| `remote_user` | string | Authenticated user |
| `method` | string | HTTP method |
| `request_uri` | string | Request path |
| `protocol` | string | HTTP protocol |
| `status` | int | HTTP status code |
| `body_bytes_sent` | int | Response body size |
| `http_referer` | string | Referer header (combined) |
| `http_user_agent` | string | User agent (combined) |

### Error Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `apache_level` | string | Apache error level |
| `module` | string | Module name (Apache 2.4+) |
| `pid` | int | Process ID |
| `tid` | string | Thread ID (Apache 2.4+) |
| `client` | string | Client IP address |
| `client_port` | int | Client port (Apache 2.4+) |

### Log Level Mapping

| HTTP Status | Log Level |
|-------------|-----------|
| 100-399 | `info` |
| 400-499 | `warning` |
| 500-599 | `error` |

| Apache Error Level | Log Level |
|--------------------|-----------|
| trace*, debug | `debug` |
| info, notice | `info` |
| warn | `warning` |
| error | `error` |
| crit, alert, emerg | `fatal` |

---

## Alert Rules

### High Error Rate

```yaml
- name: "Apache 5xx Errors"
  description: "Server error rate spike"
  type: "threshold"
  condition:
    field: "status"
    operator: ">="
    value: 500
    threshold: 25
    window: "5m"
    log_type: "apache"
  severity: "high"
  notify:
    - "slack"
```

### PHP Errors

```yaml
- name: "PHP Fatal Error"
  description: "PHP fatal error detected"
  type: "pattern"
  condition:
    pattern: "PHP Fatal error"
    log_type: "apache"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### File Not Found

```yaml
- name: "404 Spike"
  description: "High rate of 404 errors"
  type: "threshold"
  condition:
    field: "status"
    value: 404
    threshold: 100
    window: "10m"
    log_type: "apache"
  severity: "medium"
  notify:
    - "slack"
```

### Permission Denied

```yaml
- name: "Permission Denied"
  description: "File permission issues"
  type: "pattern"
  condition:
    pattern: "Permission denied|access denied"
    case_sensitive: false
    log_type: "apache"
  severity: "high"
  notify:
    - "slack"
  cooldown: "10m"
```

### mod_security Blocks

```yaml
- name: "ModSecurity Block"
  description: "ModSecurity blocked request"
  type: "pattern"
  condition:
    pattern: "ModSecurity|mod_security"
    log_type: "apache"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "5m"
```

---

## Apache Configuration Tips

### Separate Log Files Per VHost

```apache
<VirtualHost *:80>
    ServerName example.com
    CustomLog /var/log/apache2/example.com-access.log combined
    ErrorLog /var/log/apache2/example.com-error.log
</VirtualHost>
```

### Enable Request Time

```apache
LogFormat "%h %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\" %D" combined_with_time
CustomLog /var/log/apache2/access.log combined_with_time
```

`%D` logs request time in microseconds.

### Increase Error Detail

```apache
# Apache 2.4+
ErrorLogFormat "[%{cu}t] [%-m:%l] [pid %P:tid %T] [client\ %a] %M"
LogLevel info
```

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
