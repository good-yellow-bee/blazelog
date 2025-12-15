# Alert Rules Reference

This guide covers how to configure alert rules in BlazeLog. Alert rules evaluate log entries and trigger notifications when conditions are met.

---

## Overview

BlazeLog supports two types of alert rules:

| Type | Description | Use Case |
|------|-------------|----------|
| **pattern** | Triggers on regex pattern match | Detect specific errors, exceptions, keywords |
| **threshold** | Triggers when count exceeds limit in time window | Detect error rate spikes, volume anomalies |

---

## Configuration File

Alert rules are defined in `alerts.yaml`:

```yaml
rules:
  - name: "Rule Name"
    description: "What this rule detects"
    type: "pattern"  # or "threshold"
    condition:
      # ... type-specific fields
    severity: "high"
    notify:
      - "slack"
      - "email"
    cooldown: "5m"
    labels:
      project: "ecommerce"
    enabled: true
```

---

## Rule Schema

### Common Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | Unique identifier for the rule |
| `description` | string | No | - | Human-readable description |
| `type` | string | **Yes** | - | `"pattern"` or `"threshold"` |
| `condition` | object | **Yes** | - | Trigger conditions (type-specific) |
| `severity` | string | No | `"medium"` | `"low"`, `"medium"`, `"high"`, `"critical"` |
| `notify` | list | No | `[]` | Notification channels: `"email"`, `"slack"`, `"teams"` |
| `cooldown` | duration | No | - | Minimum time between repeated alerts (e.g., `"5m"`, `"1h"`) |
| `labels` | map | No | `{}` | Filter logs by label (e.g., `project: "myapp"`) |
| `enabled` | boolean | No | `true` | Whether the rule is active |

---

## Pattern-Based Rules

Pattern rules trigger immediately when a log entry matches a regex pattern.

### Pattern Condition Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `pattern` | string | **Yes** | - | Regex pattern to match against log message |
| `case_sensitive` | boolean | No | `false` | Enable case-sensitive matching |
| `log_type` | string | No | - | Filter by log type (e.g., `"nginx"`, `"magento"`) |

### Pattern Examples

**Basic Pattern - Fatal Errors:**
```yaml
- name: "Fatal Error Detected"
  description: "FATAL or CRITICAL error in logs"
  type: "pattern"
  condition:
    pattern: "FATAL|CRITICAL"
    case_sensitive: false
  severity: "critical"
  notify:
    - "slack"
    - "email"
  cooldown: "15m"
```

**Log Type Filtered - WordPress Database:**
```yaml
- name: "WordPress Database Error"
  description: "Database connection issues in WordPress"
  type: "pattern"
  condition:
    pattern: "Error establishing a database connection"
    log_type: "wordpress"
  severity: "high"
  notify:
    - "slack"
  cooldown: "5m"
```

**Magento Exceptions:**
```yaml
- name: "Magento Exception"
  description: "Exception detected in Magento logs"
  type: "pattern"
  condition:
    pattern: "Exception|Stack trace"
    log_type: "magento"
  severity: "medium"
  notify:
    - "email"
  cooldown: "10m"
```

**Security - Authentication Failures:**
```yaml
- name: "Authentication Failure"
  description: "Multiple authentication failures detected"
  type: "pattern"
  condition:
    pattern: "authentication failed|invalid password|access denied"
    case_sensitive: false
  severity: "high"
  notify:
    - "slack"
    - "email"
  cooldown: "5m"
```

---

## Threshold-Based Rules

Threshold rules trigger when the count of matching entries exceeds a limit within a sliding time window.

### Threshold Condition Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `field` | string | No | - | Log field to check (e.g., `"level"`, `"status"`, `"message"`) |
| `value` | any | No | - | Value to match against |
| `operator` | string | No | `"=="` | Comparison: `"=="`, `"!="`, `">"`, `">="`, `"<"`, `"<="` |
| `threshold` | integer | **Yes** | - | Count that triggers the alert |
| `window` | duration | **Yes** | - | Time window for counting (e.g., `"5m"`, `"1h"`) |
| `log_type` | string | No | - | Filter by log type |

### Threshold Examples

**High Error Rate:**
```yaml
- name: "High Error Rate"
  description: "More than 100 errors in 5 minutes"
  type: "threshold"
  condition:
    field: "level"
    value: "error"
    threshold: 100
    window: "5m"
  severity: "critical"
  notify:
    - "slack"
    - "email"
  labels:
    project: "*"  # All projects
```

**Nginx 5xx Spike:**
```yaml
- name: "Nginx 5xx Spike"
  description: "High rate of 5xx errors from Nginx"
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
    - "teams"
```

**Slow Requests:**
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

---

## Severity Levels

| Level | Use Case | Color |
|-------|----------|-------|
| `low` | Informational, non-urgent | Blue |
| `medium` | Should investigate when possible | Yellow |
| `high` | Requires prompt attention | Orange |
| `critical` | Immediate action required | Red |

---

## Label Filtering

Labels filter which logs a rule applies to. Use `"*"` as wildcard.

```yaml
labels:
  project: "ecommerce"      # Exact match
  environment: "production" # Exact match
  team: "*"                 # Matches any value
```

Labels are matched against the `labels` field in log entries, which are set in agent configuration:

```yaml
# Agent config
labels:
  project: "ecommerce"
  environment: "production"
```

---

## Cooldown

Cooldown prevents alert spam by enforcing a minimum time between repeated alerts from the same rule.

```yaml
cooldown: "5m"   # 5 minutes
cooldown: "1h"   # 1 hour
cooldown: "30s"  # 30 seconds
```

**Behavior:**
- First match triggers immediately
- Subsequent matches within cooldown are suppressed
- After cooldown expires, next match triggers again

---

## Duration Formats

Durations use Go's `time.ParseDuration` format:

| Format | Example | Meaning |
|--------|---------|---------|
| `s` | `30s` | 30 seconds |
| `m` | `5m` | 5 minutes |
| `h` | `1h` | 1 hour |
| Combined | `1h30m` | 1 hour 30 minutes |

---

## Notification Channels

| Channel | Value | Setup |
|---------|-------|-------|
| Email | `"email"` | See [Notifications Guide](notifications.md#email) |
| Slack | `"slack"` | See [Notifications Guide](notifications.md#slack) |
| Microsoft Teams | `"teams"` | See [Notifications Guide](notifications.md#teams) |

---

## Disabling Rules

To temporarily disable a rule without removing it:

```yaml
- name: "Debug Messages"
  description: "Track debug messages (disabled)"
  type: "pattern"
  condition:
    pattern: "DEBUG"
  severity: "low"
  enabled: false  # Rule is inactive
```

---

## Regex Pattern Tips

### Common Patterns

| Pattern | Matches |
|---------|---------|
| `error\|Error\|ERROR` | Any case of "error" |
| `(?i)error` | Case-insensitive "error" (same as `case_sensitive: false`) |
| `Exception.*` | "Exception" followed by anything |
| `status=[45]\d\d` | HTTP status 4xx or 5xx |
| `\b500\b` | Word boundary: exactly "500" |

### Escaping

Special regex characters need escaping with `\`:

| Character | Escape | Example |
|-----------|--------|---------|
| `.` | `\.` | `file\.log` |
| `[` `]` | `\[` `\]` | `\[ERROR\]` |
| `(` `)` | `\(` `\)` | `func\(\)` |
| `$` | `\$` | `\$100` |

---

## Complete Configuration Example

```yaml
# /etc/blazelog/alerts.yaml

rules:
  # Critical: Immediate attention
  - name: "Fatal Error Detected"
    description: "FATAL or CRITICAL error in any log"
    type: "pattern"
    condition:
      pattern: "FATAL|CRITICAL"
      case_sensitive: false
    severity: "critical"
    notify:
      - "slack"
      - "teams"
      - "email"
    cooldown: "15m"

  # High: Error rate spike
  - name: "High Error Rate"
    description: "More than 100 errors in 5 minutes"
    type: "threshold"
    condition:
      field: "level"
      value: "error"
      threshold: 100
      window: "5m"
    severity: "critical"
    notify:
      - "slack"
      - "email"

  # High: Nginx server errors
  - name: "Nginx 5xx Spike"
    description: "High rate of 5xx errors"
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
      - "teams"

  # Medium: Slow requests
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

  # Medium: Magento exceptions
  - name: "Magento Exception"
    description: "Exception in Magento logs"
    type: "pattern"
    condition:
      pattern: "Exception|Stack trace"
      log_type: "magento"
    severity: "medium"
    notify:
      - "email"
    cooldown: "10m"

  # High: Security alerts
  - name: "Authentication Failure"
    description: "Authentication failures detected"
    type: "pattern"
    condition:
      pattern: "authentication failed|invalid password|access denied"
      case_sensitive: false
    severity: "high"
    notify:
      - "slack"
      - "email"
    cooldown: "5m"

  # Per-project filtering
  - name: "Production Errors"
    description: "Errors in production environment"
    type: "pattern"
    condition:
      pattern: "error|Error|ERROR"
    severity: "high"
    notify:
      - "slack"
    labels:
      environment: "production"
    cooldown: "5m"
```

---

## Validating Rules

Rules are validated when the server starts. Check server logs for validation errors:

```bash
# Check for validation errors
blazelog-server --config server.yaml 2>&1 | grep -i "rule"

# Common validation errors:
# - "rule name is required"
# - "pattern is required for pattern rule"
# - "threshold must be positive"
# - "window is required for threshold rule"
# - "invalid operator"
```

---

## See Also

- [Notifications Guide](notifications.md) - Configure Email, Slack, Teams
- [Configuration Reference](../CONFIGURATION.md) - Full server configuration
- [Log Formats](log-formats/README.md) - Supported log types
