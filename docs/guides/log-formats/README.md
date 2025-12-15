# Log Formats Guide

BlazeLog supports multiple log formats with automatic detection and intelligent parsing.

---

## Supported Formats

| Type | Description | Auto-Detect |
|------|-------------|-------------|
| [`nginx`](nginx.md) | Nginx access and error logs | Yes |
| [`apache`](apache.md) | Apache access and error logs | Yes |
| [`magento`](magento.md) | Magento 2 system/exception logs | Yes |
| [`prestashop`](prestashop.md) | PrestaShop application logs | Yes |
| [`wordpress`](wordpress.md) | WordPress debug.log | Yes |
| [`auto`](custom.md) | Automatic detection | - |

---

## Auto-Detection

BlazeLog can automatically detect log formats using the `type: "auto"` setting:

```yaml
# agent.yaml
sources:
  - name: "auto-detect"
    path: "/var/log/app/*.log"
    type: "auto"  # Detect format from content
```

### How It Works

1. Agent reads the first few lines of the log file
2. Each parser's `CanParse()` method is tested against the lines
3. First matching parser is selected
4. If no parser matches, raw lines are forwarded

### Detection Priority

Parsers are tested in order of specificity:
1. Magento (Monolog format with brackets)
2. PrestaShop (PrestaShop-specific patterns)
3. WordPress (PHP error format)
4. Nginx Access (combined/common format)
5. Nginx Error (error format)
6. Apache Access (CLF/combined)
7. Apache Error (Apache error format)

---

## Agent Configuration

### Basic Configuration

```yaml
# agent.yaml
sources:
  - name: "nginx-access"
    path: "/var/log/nginx/access.log"
    type: "nginx"
    follow: true  # Tail mode

  - name: "magento-logs"
    path: "/var/www/magento/var/log/*.log"
    type: "magento"
    follow: true
```

### Multiple Sources

```yaml
sources:
  # Web server logs
  - name: "nginx-access"
    path: "/var/log/nginx/access.log"
    type: "nginx"

  - name: "nginx-error"
    path: "/var/log/nginx/error.log"
    type: "nginx"

  # Application logs
  - name: "magento-system"
    path: "/var/www/magento/var/log/system.log"
    type: "magento"

  - name: "magento-exception"
    path: "/var/www/magento/var/log/exception.log"
    type: "magento"
```

### Glob Patterns

```yaml
sources:
  # All log files in directory
  - name: "all-logs"
    path: "/var/log/app/*.log"
    type: "auto"

  # Recursive
  - name: "nested-logs"
    path: "/var/www/**/var/log/*.log"
    type: "auto"
```

---

## Parsed Fields

All log entries include common fields:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | time | When the event occurred |
| `level` | string | Log level (debug, info, warning, error, fatal) |
| `message` | string | Main message content |
| `source` | string | Source identifier |
| `type` | string | Log type (nginx, magento, etc.) |
| `raw` | string | Original unparsed line (optional) |
| `file_path` | string | Source file path |
| `line_number` | int | Line number in source file |
| `labels` | map | Custom key-value labels |
| `fields` | map | Format-specific parsed fields |

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Debugging information |
| `info` | Informational messages |
| `warning` | Warning conditions |
| `error` | Error conditions |
| `fatal` | Critical/fatal errors |
| `unknown` | Level could not be determined |

---

## CLI Commands

### Parse Single File

```bash
# Auto-detect format
blazectl parse auto /var/log/nginx/access.log

# Explicit format
blazectl parse nginx /var/log/nginx/access.log
blazectl parse magento /var/www/magento/var/log/system.log
```

### Output Formats

```bash
# JSON output
blazectl parse nginx access.log --format json

# Pretty JSON
blazectl parse nginx access.log --format json-pretty

# Table format
blazectl parse nginx access.log --format table

# Plain text
blazectl parse nginx access.log --format plain
```

### Tail Mode

```bash
# Follow log file
blazectl tail /var/log/nginx/access.log

# With alerts
blazectl tail /var/log/nginx/access.log --alerts alerts.yaml
```

---

## Labels

Labels are key-value pairs attached to log entries for filtering:

```yaml
# agent.yaml
labels:
  environment: "production"
  project: "ecommerce"
  server: "web01"

sources:
  - name: "nginx"
    path: "/var/log/nginx/access.log"
    type: "nginx"
```

Use labels in alert rules:

```yaml
# alerts.yaml
rules:
  - name: "Production Errors"
    type: "pattern"
    condition:
      pattern: "error"
    labels:
      environment: "production"  # Only match production logs
```

---

## Format-Specific Guides

- [Nginx Logs](nginx.md) - Access and error log formats
- [Apache Logs](apache.md) - Common and combined formats
- [Magento Logs](magento.md) - Monolog format with multiline support
- [PrestaShop Logs](prestashop.md) - PrestaShop application logs
- [WordPress Logs](wordpress.md) - debug.log and PHP errors
- [Custom Patterns](custom.md) - Auto-detection and custom formats

---

## See Also

- [Alert Rules Reference](../alerts.md) - Create alerts based on parsed fields
- [Configuration Reference](../../CONFIGURATION.md) - Full agent configuration
