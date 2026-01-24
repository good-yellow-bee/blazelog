# Magento Log Format

BlazeLog parses Magento 2 logs which use the Monolog format.

---

## Supported Formats

### Monolog Format

BlazeLog supports both classic and ISO 8601 timestamp formats:

```
[YYYY-MM-DD HH:MM:SS] channel.LEVEL: message {context} [extra]
[YYYY-MM-DDTHH:MM:SS.ffffff±HH:MM] channel.LEVEL: message {context} [extra]
```

Examples:
```
[2024-01-15 10:30:00] main.ERROR: Exception message {"exception":"[object] (Exception: ...)"} []
[2024-01-15T10:30:00.123456+00:00] main.INFO: Order placed {"order_id":12345} []
```

### Stack Traces (Multiline)

BlazeLog handles multiline stack traces automatically:

```
[2024-01-15 10:30:00] main.CRITICAL: Uncaught Exception: Something went wrong {"exception":"[object]"} []
#0 /var/www/magento/vendor/magento/framework/App/Http.php(116): ...
#1 /var/www/magento/pub/index.php(30): ...
```

---

## Log Files

| File | Purpose |
|------|---------|
| `var/log/system.log` | General system events |
| `var/log/exception.log` | Exceptions and errors |
| `var/log/debug.log` | Debug information (when enabled) |
| `var/log/cron.log` | Cron job execution |
| `var/report/` | Detailed error reports (separate files) |

### Default Locations

| Environment | Path |
|-------------|------|
| Standard | `/var/www/magento/var/log/` |
| Docker | `/var/www/html/var/log/` |
| Composer | `<project>/var/log/` |

---

## Agent Configuration

```yaml
# agent.yaml
sources:
  # Core log files
  - name: "magento-system"
    path: "/var/www/magento/var/log/system.log"
    type: "magento"
    follow: true

  - name: "magento-exception"
    path: "/var/www/magento/var/log/exception.log"
    type: "magento"
    follow: true

  - name: "magento-debug"
    path: "/var/www/magento/var/log/debug.log"
    type: "magento"
    follow: true

  # All logs with glob
  - name: "magento-all"
    path: "/var/www/magento/var/log/*.log"
    type: "magento"
    follow: true

# Add labels for filtering
labels:
  project: "magento-store"
  environment: "production"
```

---

## Parsed Fields

| Field | Type | Description |
|-------|------|-------------|
| `channel` | string | Monolog channel (main, cron, etc.) |
| `magento_level` | string | Original Magento level |
| `context` | object | Structured context data |
| `extra` | array | Extra Monolog data |
| `is_exception` | bool | Whether entry contains exception |
| `exception_class` | string | Exception class name |
| `exception_file` | string | File where exception occurred |
| `exception_line` | int | Line number of exception |
| `stack_trace` | string | Full stack trace (multiline) |
| `stack_frame_count` | int | Number of stack frames |
| `multiline` | bool | Whether entry spans multiple lines |

### Log Level Mapping

| Monolog Level | BlazeLog Level |
|---------------|----------------|
| DEBUG | `debug` |
| INFO, NOTICE | `info` |
| WARNING | `warning` |
| ERROR | `error` |
| CRITICAL, ALERT, EMERGENCY | `fatal` |

---

## Alert Rules

### Critical Exceptions

```yaml
- name: "Magento Critical"
  description: "Critical error in Magento"
  type: "pattern"
  condition:
    pattern: "CRITICAL|EMERGENCY"
    log_type: "magento"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Any Exception

```yaml
- name: "Magento Exception"
  description: "Exception detected in Magento"
  type: "pattern"
  condition:
    pattern: "Exception|Stack trace"
    log_type: "magento"
  severity: "high"
  notify:
    - "slack"
  cooldown: "10m"
```

### Database Errors

```yaml
- name: "Database Error"
  description: "Database connection or query error"
  type: "pattern"
  condition:
    pattern: "SQLSTATE|MySQL|database|Zend_Db"
    case_sensitive: false
    log_type: "magento"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Payment Errors

```yaml
- name: "Payment Error"
  description: "Payment processing failure"
  type: "pattern"
  condition:
    pattern: "payment|PayPal|Stripe|Braintree|transaction"
    case_sensitive: false
    log_type: "magento"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Cache Issues

```yaml
- name: "Cache Error"
  description: "Cache system issues"
  type: "pattern"
  condition:
    pattern: "cache|Redis|Varnish|memcache"
    case_sensitive: false
    log_type: "magento"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "15m"
```

### Indexer Failures

```yaml
- name: "Indexer Failed"
  description: "Indexer process failure"
  type: "pattern"
  condition:
    pattern: "indexer|reindex|Mview"
    case_sensitive: false
    log_type: "magento"
  severity: "high"
  notify:
    - "slack"
  cooldown: "30m"
```

### Cron Job Errors

```yaml
- name: "Cron Error"
  description: "Cron job execution failure"
  type: "pattern"
  condition:
    pattern: "cron.*error|cron.*failed|cron.*exception"
    case_sensitive: false
    log_type: "magento"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "30m"
```

### High Error Rate

```yaml
- name: "Magento Error Spike"
  description: "High rate of errors"
  type: "threshold"
  condition:
    field: "level"
    value: "error"
    threshold: 50
    window: "5m"
    log_type: "magento"
  severity: "high"
  notify:
    - "slack"
```

---

## Magento Configuration

### Enable Debug Logging

```php
// app/etc/env.php
'system' => [
    'default' => [
        'dev' => [
            'debug' => '1'
        ]
    ]
]
```

Or via admin: **Stores → Configuration → Advanced → Developer → Debug**.

### Increase Log Detail

```php
// In custom module
$logger->debug('Message', ['context' => $data]);
$logger->error('Error occurred', ['exception' => $e]);
```

### Custom Log Channels

```xml
<!-- etc/di.xml -->
<type name="Vendor\Module\Logger">
    <arguments>
        <argument name="name" xsi:type="string">custom_channel</argument>
    </arguments>
</type>
```

---

## Common Exceptions

| Exception | Typical Cause |
|-----------|---------------|
| `LocalizedException` | User-facing errors |
| `CouldNotSaveException` | Database save failures |
| `NoSuchEntityException` | Missing product/customer/etc. |
| `AuthenticationException` | Login failures |
| `InputException` | Invalid input data |
| `SecurityViolationException` | CSRF/security issues |

---

## Performance Considerations

Magento logs can be verbose. Consider:

1. **Filter by level in production:**
   ```yaml
   # Only collect ERROR and above
   sources:
     - path: "/var/www/magento/var/log/system.log"
       type: "magento"
       # Use alert rules to filter INFO/DEBUG
   ```

2. **Separate exception.log monitoring:**
   ```yaml
   # Higher priority for exception.log
   sources:
     - name: "exceptions"
       path: "/var/www/magento/var/log/exception.log"
       type: "magento"
   ```

3. **Use labels for filtering:**
   ```yaml
   labels:
     log_file: "exception"
   ```

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
