# Custom Log Formats

BlazeLog can handle custom log formats using auto-detection.

---

## Auto-Detection

Use `type: "auto"` to let BlazeLog detect the log format:

```yaml
# agent.yaml
sources:
  - name: "custom-app"
    path: "/var/log/myapp/*.log"
    type: "auto"
    follow: true
```

### How It Works

1. BlazeLog reads log lines
2. Each built-in parser tests the line format
3. If a parser matches, it's used for that file
4. If no parser matches, raw lines are forwarded

### Detection Order

1. Magento (Monolog format)
2. PrestaShop (Monolog format)
3. WordPress (PHP error format)
4. Nginx Access (combined/common)
5. Nginx Error
6. Apache Access (CLF/combined)
7. Apache Error

---

## Syslog Format

Standard syslog is partially supported via auto-detection:

```
Jan 15 10:30:00 hostname program[pid]: message
```

Example agent config:
```yaml
sources:
  - name: "syslog"
    path: "/var/log/syslog"
    type: "auto"
    follow: true
```

---

## JSON Logs

Structured JSON logs work with auto-detection:

```json
{"timestamp":"2024-01-15T10:30:00Z","level":"error","message":"Something failed"}
```

The raw JSON is preserved in the `raw` field and available for pattern matching.

```yaml
# Alert rule for JSON logs
- name: "JSON Error"
  type: "pattern"
  condition:
    pattern: "\"level\":\"error\""
  severity: "high"
  notify:
    - "slack"
```

---

## Raw Line Mode

When no parser matches, logs are forwarded as raw lines:

```yaml
sources:
  - name: "unknown-app"
    path: "/var/log/unknown.log"
    type: "auto"
```

In raw mode:
- `timestamp` = time when line was read
- `level` = unknown
- `message` = full line content
- `raw` = full line content

---

## Pattern-Based Alerting

Even without parsing, you can create powerful alerts using regex:

### Generic Error Detection

```yaml
- name: "Any Error"
  type: "pattern"
  condition:
    pattern: "error|Error|ERROR|failed|Failed|FAILED"
  severity: "medium"
  notify:
    - "slack"
  cooldown: "5m"
```

### Exception Stack Traces

```yaml
- name: "Stack Trace"
  type: "pattern"
  condition:
    pattern: "Exception|Traceback|at \\S+\\(|#\\d+ \\/"
  severity: "high"
  notify:
    - "slack"
  cooldown: "5m"
```

### Specific Application Errors

```yaml
- name: "MyApp Critical"
  type: "pattern"
  condition:
    pattern: "\\[CRITICAL\\]|\\[FATAL\\]"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

### IP Address Extraction

```yaml
- name: "Suspicious IP"
  type: "pattern"
  condition:
    pattern: "blocked.*\\b\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\b"
  severity: "high"
  notify:
    - "slack"
  cooldown: "10m"
```

---

## Common Log Formats

### Generic Application Log

```
YYYY-MM-DD HH:MM:SS LEVEL message
```

Detected by: Will fall back to raw mode, use pattern alerts.

### Docker/Container Logs

```
2024-01-15T10:30:00.123456789Z stdout P message
```

Detected by: Raw mode. Use pattern matching.

### Java Log4j

```
2024-01-15 10:30:00,123 [thread] LEVEL class - message
```

Detected by: Raw mode. Use pattern matching.

```yaml
- name: "Java Error"
  type: "pattern"
  condition:
    pattern: "\\sERROR\\s|\\sFATAL\\s"
  severity: "high"
  notify:
    - "slack"
```

### Python Logging

```
2024-01-15 10:30:00,123 - module - LEVEL - message
```

Detected by: Raw mode. Use pattern matching.

```yaml
- name: "Python Error"
  type: "pattern"
  condition:
    pattern: " - ERROR - | - CRITICAL - "
  severity: "high"
  notify:
    - "slack"
```

### Node.js/PM2

```json
{"timestamp":"2024-01-15T10:30:00.123Z","level":"error","message":"..."}
```

Detected by: Raw mode (JSON preserved).

---

## Tips for Custom Logs

### Use Specific Patterns

```yaml
# Bad - too broad
- name: "Error"
  condition:
    pattern: "error"  # Matches "no errors found" too

# Good - more specific
- name: "Error"
  condition:
    pattern: "\\[ERROR\\]|\\bERROR:"
```

### Combine with Labels

```yaml
# agent.yaml
sources:
  - name: "app1"
    path: "/var/log/app1.log"
    type: "auto"

labels:
  app: "app1"

# alerts.yaml
- name: "App1 Error"
  condition:
    pattern: "ERROR"
  labels:
    app: "app1"  # Only match app1 logs
```

### Use Cooldowns

Custom formats often produce verbose output:

```yaml
- name: "Custom Error"
  condition:
    pattern: "ERROR"
  cooldown: "15m"  # Avoid spam
```

---

## Future: Custom Parsers

Custom regex parsers are planned for a future release. This will allow:

```yaml
# Future feature - not yet implemented
sources:
  - name: "custom"
    path: "/var/log/custom.log"
    type: "custom"
    pattern: "^(?P<timestamp>\\S+) \\[(?P<level>\\w+)\\] (?P<message>.*)$"
    time_format: "2006-01-02T15:04:05Z07:00"
```

---

## See Also

- [Log Formats Overview](README.md)
- [Alert Rules Reference](../alerts.md)
- [Troubleshooting Guide](../../TROUBLESHOOTING.md)
