# WordPress Log Format

BlazeLog parses WordPress debug.log and PHP error logs.

---

## Supported Formats

### WordPress debug.log

```
[DD-Mon-YYYY HH:MM:SS UTC] PHP message
```

Example:
```
[15-Jan-2024 10:30:00 UTC] PHP Fatal error: Uncaught Error: Call to undefined function...
[15-Jan-2024 10:30:01 UTC] PHP Warning: fopen(): Unable to open file...
```

### PHP Error Format

```
[DD-Mon-YYYY HH:MM:SS Timezone] PHP Level: message in /path/to/file.php on line N
```

Example:
```
[15-Jan-2024 10:30:00 UTC] PHP Notice: Undefined variable $foo in /var/www/html/wp-content/themes/theme/functions.php on line 42
```

---

## Log Files

| File | Purpose |
|------|---------|
| `wp-content/debug.log` | WordPress debug output |
| `/var/log/php_errors.log` | PHP error log (if configured) |

### Default Locations

| Setup | Path |
|-------|------|
| Standard | `/var/www/html/wp-content/debug.log` |
| Multisite | `/var/www/html/wp-content/debug.log` (shared) |
| Docker | `/var/www/html/wp-content/debug.log` |

---

## Agent Configuration

```yaml
# agent.yaml
sources:
  - name: "wordpress-debug"
    path: "/var/www/html/wp-content/debug.log"
    type: "wordpress"
    follow: true

  # If PHP errors go to separate file
  - name: "php-errors"
    path: "/var/log/php_errors.log"
    type: "wordpress"
    follow: true

labels:
  project: "wordpress-site"
  environment: "production"
```

---

## Parsed Fields

| Field | Type | Description |
|-------|------|-------------|
| `php_level` | string | PHP error level |
| `file` | string | Source file path |
| `line` | int | Line number |
| `php_message` | string | Full PHP message |

### Log Level Mapping

| PHP Level | BlazeLog Level |
|-----------|----------------|
| Notice, Deprecated | `info` |
| Warning | `warning` |
| Error, Fatal error | `error` |
| Parse error | `fatal` |

---

## Alert Rules

### Fatal Errors

```yaml
- name: "WordPress Fatal Error"
  description: "PHP fatal error in WordPress"
  type: "pattern"
  condition:
    pattern: "Fatal error|Parse error"
    log_type: "wordpress"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Database Connection

```yaml
- name: "WordPress Database Error"
  description: "Database connection issues"
  type: "pattern"
  condition:
    pattern: "Error establishing a database connection|wpdb|mysql"
    case_sensitive: false
    log_type: "wordpress"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Memory Exhausted

```yaml
- name: "Memory Exhausted"
  description: "PHP memory limit reached"
  type: "pattern"
  condition:
    pattern: "Allowed memory size.*exhausted"
    log_type: "wordpress"
  severity: "high"
  notify:
    - "slack"
  cooldown: "10m"
```

### Plugin/Theme Errors

```yaml
- name: "Plugin Error"
  description: "Error in plugin code"
  type: "pattern"
  condition:
    pattern: "wp-content/plugins/.*error"
    case_sensitive: false
    log_type: "wordpress"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "15m"
```

```yaml
- name: "Theme Error"
  description: "Error in theme code"
  type: "pattern"
  condition:
    pattern: "wp-content/themes/.*error"
    case_sensitive: false
    log_type: "wordpress"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "15m"
```

### Deprecated Functions

```yaml
- name: "Deprecated Warning"
  description: "Deprecated function usage"
  type: "threshold"
  condition:
    pattern: "Deprecated"
    threshold: 50
    window: "1h"
    log_type: "wordpress"
  severity: "low"
  notify:
    - "email"
```

### High Error Rate

```yaml
- name: "WordPress Error Spike"
  description: "High rate of PHP errors"
  type: "threshold"
  condition:
    field: "level"
    value: "error"
    threshold: 20
    window: "5m"
    log_type: "wordpress"
  severity: "high"
  notify:
    - "slack"
```

---

## WordPress Configuration

### Enable Debug Logging

```php
// wp-config.php
define('WP_DEBUG', true);
define('WP_DEBUG_LOG', true);
define('WP_DEBUG_DISPLAY', false);  // Hide errors from visitors
```

### Custom Log Path

```php
// wp-config.php
define('WP_DEBUG_LOG', '/var/log/wordpress/debug.log');
```

Update agent configuration accordingly.

### Disable in Production (Alternative)

Instead of disabling debug logging, use BlazeLog to filter:

```php
// Keep logging enabled
define('WP_DEBUG', true);
define('WP_DEBUG_LOG', true);
define('WP_DEBUG_DISPLAY', false);
```

Then use alert rules to filter important errors only.

---

## Common Errors

| Error | Typical Cause |
|-------|---------------|
| "Allowed memory size exhausted" | Increase `memory_limit` in php.ini |
| "Maximum execution time exceeded" | Long-running script, increase `max_execution_time` |
| "Error establishing database connection" | Database down, wrong credentials |
| "Call to undefined function" | Missing plugin or theme |
| "Headers already sent" | Output before WordPress loads |

---

## Security Considerations

The debug.log can contain sensitive information:

1. **Restrict access via .htaccess:**
   ```apache
   <Files debug.log>
       Order allow,deny
       Deny from all
   </Files>
   ```

2. **Move log outside web root:**
   ```php
   define('WP_DEBUG_LOG', '/var/log/wordpress/debug.log');
   ```

3. **Rotate logs regularly:**
   ```bash
   # logrotate.d/wordpress
   /var/www/html/wp-content/debug.log {
       daily
       rotate 7
       compress
       missingok
       notifempty
   }
   ```

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
