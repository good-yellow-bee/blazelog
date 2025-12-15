# PrestaShop Log Format

BlazeLog parses PrestaShop application logs.

---

## Supported Formats

### PrestaShop Log Format

PrestaShop uses a similar Monolog-style format:

```
[YYYY-MM-DD HH:MM:SS] channel.LEVEL: message {context} [extra]
```

Example:
```
[2024-01-15 10:30:00] prestashop.ERROR: Cart rule validation failed {"cart_rule_id":42} []
```

### Symfony/Debug Format

PrestaShop may also output Symfony-style logs:

```
[2024-01-15T10:30:00+00:00] app.ERROR: Error message
```

---

## Log Files

| File | Purpose |
|------|---------|
| `var/logs/prod.log` | Production logs |
| `var/logs/dev.log` | Development logs |
| `var/logs/*.log` | Various module logs |

### Default Locations

| Version | Path |
|---------|------|
| PrestaShop 1.7+ | `/var/www/prestashop/var/logs/` |
| PrestaShop 1.6 | `/var/www/prestashop/log/` |
| Docker | `/var/www/html/var/logs/` |

---

## Agent Configuration

```yaml
# agent.yaml
sources:
  # Production log
  - name: "prestashop-prod"
    path: "/var/www/prestashop/var/logs/prod.log"
    type: "prestashop"
    follow: true

  # All logs
  - name: "prestashop-all"
    path: "/var/www/prestashop/var/logs/*.log"
    type: "prestashop"
    follow: true

labels:
  project: "prestashop-store"
  environment: "production"
```

---

## Parsed Fields

| Field | Type | Description |
|-------|------|-------------|
| `channel` | string | Log channel |
| `prestashop_level` | string | Original level |
| `context` | object | Structured context |
| `extra` | array | Extra data |
| `module` | string | Module name (if present) |

### Log Level Mapping

| PrestaShop Level | BlazeLog Level |
|------------------|----------------|
| DEBUG | `debug` |
| INFO, NOTICE | `info` |
| WARNING | `warning` |
| ERROR | `error` |
| CRITICAL, ALERT, EMERGENCY | `fatal` |

---

## Alert Rules

### Critical Errors

```yaml
- name: "PrestaShop Critical"
  description: "Critical error in PrestaShop"
  type: "pattern"
  condition:
    pattern: "CRITICAL|EMERGENCY"
    log_type: "prestashop"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Database Errors

```yaml
- name: "PrestaShop Database Error"
  description: "Database connection or query error"
  type: "pattern"
  condition:
    pattern: "SQLSTATE|PDOException|database"
    case_sensitive: false
    log_type: "prestashop"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Payment Issues

```yaml
- name: "Payment Error"
  description: "Payment processing failure"
  type: "pattern"
  condition:
    pattern: "payment|PayPal|Stripe|transaction failed"
    case_sensitive: false
    log_type: "prestashop"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### Order Errors

```yaml
- name: "Order Error"
  description: "Order processing failure"
  type: "pattern"
  condition:
    pattern: "order.*error|cart.*error|checkout.*fail"
    case_sensitive: false
    log_type: "prestashop"
  severity: "high"
  notify:
    - "slack"
  cooldown: "5m"
```

### Module Errors

```yaml
- name: "Module Error"
  description: "Module execution error"
  type: "pattern"
  condition:
    pattern: "module.*error|hook.*error"
    case_sensitive: false
    log_type: "prestashop"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "15m"
```

### High Error Rate

```yaml
- name: "PrestaShop Error Spike"
  description: "High rate of errors"
  type: "threshold"
  condition:
    field: "level"
    value: "error"
    threshold: 30
    window: "5m"
    log_type: "prestashop"
  severity: "high"
  notify:
    - "slack"
```

---

## PrestaShop Configuration

### Enable Debug Mode

```php
// config/defines.inc.php
define('_PS_MODE_DEV_', true);
```

Or in Back Office: **Advanced Parameters → Performance → Debug mode**.

### Increase Log Detail

```php
// In custom module
PrestaShopLogger::addLog('Message', 1, null, 'Object', $id);
```

Log severities:
- 1 = Informative only
- 2 = Warning
- 3 = Error
- 4 = Major issue (crash)

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
