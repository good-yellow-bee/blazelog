# Notifications Guide

This guide covers how to configure notification channels in BlazeLog for alert delivery.

---

## Overview

BlazeLog supports three notification channels:

| Channel | Use Case |
|---------|----------|
| **Email** | Formal notifications, compliance requirements |
| **Slack** | Team chat, quick response |
| **Microsoft Teams** | Enterprise environments, Microsoft ecosystem |

---

## Email Notifications

Email notifications use SMTP with TLS support.

### Server Configuration

```yaml
# server.yaml
notifications:
  email:
    smtp_host: "smtp.example.com"
    smtp_port: 587              # 587 for STARTTLS, 465 for implicit TLS
    username: "alerts@example.com"
    password_env: "SMTP_PASSWORD"  # Read from environment variable
    from: "BlazeLog <alerts@example.com>"
    recipients:
      - "ops@example.com"
      - "dev-team@example.com"
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `smtp_host` | **Yes** | SMTP server hostname |
| `smtp_port` | **Yes** | SMTP port (587 or 465) |
| `username` | No | SMTP authentication username |
| `password` | No | SMTP password (plain text) |
| `password_env` | No | Environment variable containing password (recommended) |
| `from` | **Yes** | Sender address (can include name) |
| `recipients` | **Yes** | List of recipient email addresses |

### TLS/Port Selection

| Port | Protocol | Description |
|------|----------|-------------|
| 587 | STARTTLS | Standard submission port, upgrades to TLS |
| 465 | Implicit TLS | Direct TLS connection (SMTPS) |
| 25 | Plain/STARTTLS | Not recommended (often blocked) |

### Example: Gmail SMTP

```yaml
notifications:
  email:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "yourname@gmail.com"
    password_env: "GMAIL_APP_PASSWORD"
    from: "BlazeLog Alerts <yourname@gmail.com>"
    recipients:
      - "team@yourcompany.com"
```

**Note:** Gmail requires an [App Password](https://support.google.com/accounts/answer/185833) when 2FA is enabled.

### Example: Amazon SES

```yaml
notifications:
  email:
    smtp_host: "email-smtp.us-east-1.amazonaws.com"
    smtp_port: 587
    username_env: "SES_USERNAME"
    password_env: "SES_PASSWORD"
    from: "noreply@yourdomain.com"
    recipients:
      - "alerts@yourcompany.com"
```

### Testing Email

```bash
# Set password
export SMTP_PASSWORD="your-password"

# Start server and trigger a test alert
blazelog-server --config server.yaml

# Check server logs for delivery status
journalctl -u blazelog-server | grep -i email
```

---

## Slack Notifications

Slack notifications use incoming webhooks to post alerts to channels.

### Creating a Webhook

1. Go to [Slack API: Incoming Webhooks](https://api.slack.com/messaging/webhooks)
2. Click **Create your Slack app** or select an existing app
3. Enable **Incoming Webhooks**
4. Click **Add New Webhook to Workspace**
5. Select the channel for alerts
6. Copy the webhook URL

### Server Configuration

```yaml
# server.yaml
notifications:
  slack:
    webhook_url_env: "SLACK_WEBHOOK_URL"  # Recommended: use env var
    # Or: webhook_url: "https://hooks.slack.com/services/T.../B.../..."
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `webhook_url` | **Yes** | Slack incoming webhook URL |
| `webhook_url_env` | No | Environment variable containing webhook URL |

### Message Format

BlazeLog sends alerts using Slack Block Kit for rich formatting:

```
┌─────────────────────────────────────────┐
│ 🔴 BlazeLog Alert: High Error Rate     │
├─────────────────────────────────────────┤
│ Severity: 🔴 CRITICAL                   │
│ Time: 2024-01-15 14:30:22 UTC          │
├─────────────────────────────────────────┤
│ Message: More than 100 errors in 5 min │
├─────────────────────────────────────────┤
│ Count: 127                              │
│ Threshold: 100 in 5m                    │
├─────────────────────────────────────────┤
│ Rule: More than 100 errors in 5 minutes│
│ Labels: project=ecommerce              │
└─────────────────────────────────────────┘
```

### Severity Colors

| Severity | Emoji |
|----------|-------|
| Critical | 🔴 Red circle |
| High | 🟠 Orange circle |
| Medium | 🟡 Yellow circle |
| Low | 🟢 Green circle |

### Testing Slack

```bash
# Set webhook URL
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/T.../B.../..."

# Start server
blazelog-server --config server.yaml

# Watch the Slack channel for test alerts
```

---

## Microsoft Teams Notifications

Teams notifications use incoming webhooks with Adaptive Cards.

### Creating a Webhook

1. In Teams, go to the channel for alerts
2. Click **...** → **Connectors** (or **Manage channel** → **Edit** → **Connectors**)
3. Find **Incoming Webhook** and click **Configure**
4. Give it a name (e.g., "BlazeLog Alerts")
5. Optionally upload an icon
6. Click **Create** and copy the webhook URL

### Server Configuration

```yaml
# server.yaml
notifications:
  teams:
    webhook_url_env: "TEAMS_WEBHOOK_URL"  # Recommended: use env var
    # Or: webhook_url: "https://outlook.office.com/webhook/..."
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `webhook_url` | **Yes** | Teams incoming webhook URL |
| `webhook_url_env` | No | Environment variable containing webhook URL |

### Message Format

BlazeLog sends Adaptive Cards to Teams:

```
╔═════════════════════════════════════════╗
║ 🔴 BlazeLog Alert: High Error Rate     ║
╠═════════════════════════════════════════╣
║ Severity: 🔴 CRITICAL                   ║
║ Time: 2024-01-15 14:30:22 UTC          ║
╠═════════════════════════════════════════╣
║ Message: More than 100 errors in 5 min ║
╠═════════════════════════════════════════╣
║ Count: 127                              ║
║ Threshold: 100 in 5m                    ║
╠═════════════════════════════════════════╣
║ Rule: More than 100 errors in 5 minutes║
║ Labels: project=ecommerce              ║
╚═════════════════════════════════════════╝
```

### Severity Styles

| Severity | Adaptive Card Style |
|----------|---------------------|
| Critical | `attention` (red) |
| High | `warning` (orange/yellow) |
| Medium | `accent` (blue) |
| Low | `good` (green) |

### Testing Teams

```bash
# Set webhook URL
export TEAMS_WEBHOOK_URL="https://outlook.office.com/webhook/..."

# Start server
blazelog-server --config server.yaml

# Watch the Teams channel for test alerts
```

---

## Rate Limiting

BlazeLog includes rate limiting to prevent notification spam.

### Default Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `max_per_window` | 10 | Maximum notifications per window |
| `window` | 1 minute | Sliding window duration |
| `enabled` | true | Whether rate limiting is active |

### Configuration

```yaml
# server.yaml
notifications:
  rate_limit:
    max_per_window: 10
    window: "1m"
    enabled: true
```

### Behavior

- Rate limiting applies globally across all channels
- Notifications exceeding the limit are dropped
- Dropped count is available in metrics
- Use rule-level `cooldown` for per-rule throttling

### Monitoring

Check rate limit status via metrics:

```
# Prometheus metric
blazelog_notifications_rate_limited_total
```

---

## Multiple Channels

Configure multiple channels and select per-rule:

### Server Configuration

```yaml
# server.yaml
notifications:
  email:
    smtp_host: "smtp.example.com"
    smtp_port: 587
    username: "alerts@example.com"
    password_env: "SMTP_PASSWORD"
    from: "BlazeLog <alerts@example.com>"
    recipients:
      - "ops@example.com"

  slack:
    webhook_url_env: "SLACK_WEBHOOK_URL"

  teams:
    webhook_url_env: "TEAMS_WEBHOOK_URL"

  rate_limit:
    max_per_window: 20
    window: "1m"
```

### Alert Rule Selection

```yaml
# alerts.yaml
rules:
  - name: "Critical Alert"
    type: "pattern"
    condition:
      pattern: "FATAL"
    severity: "critical"
    notify:
      - "slack"
      - "teams"
      - "email"  # All channels

  - name: "High Alert"
    type: "threshold"
    condition:
      field: "level"
      value: "error"
      threshold: 100
      window: "5m"
    severity: "high"
    notify:
      - "slack"    # Slack only

  - name: "Medium Alert"
    type: "pattern"
    condition:
      pattern: "Warning"
    severity: "medium"
    notify:
      - "email"    # Email only
```

---

## Environment Variables

Use environment variables for sensitive data:

| Variable | Description |
|----------|-------------|
| `SMTP_PASSWORD` | Email SMTP password |
| `SLACK_WEBHOOK_URL` | Slack webhook URL |
| `TEAMS_WEBHOOK_URL` | Teams webhook URL |

### Setting Variables

```bash
# Systemd service
sudo systemctl edit blazelog-server
# Add:
# [Service]
# Environment="SMTP_PASSWORD=xxx"
# Environment="SLACK_WEBHOOK_URL=https://..."

# Docker
docker run -e SMTP_PASSWORD=xxx -e SLACK_WEBHOOK_URL=https://...

# Manual
export SMTP_PASSWORD=xxx
export SLACK_WEBHOOK_URL=https://...
blazelog-server --config server.yaml
```

---

## Troubleshooting

### Email Issues

**Problem:** Emails not delivered
- Check SMTP credentials
- Verify port (587 vs 465)
- Check spam/junk folder
- Review server logs for SMTP errors

**Problem:** Authentication failed
```bash
# Test SMTP connection
openssl s_client -starttls smtp -connect smtp.example.com:587
```

**Problem:** TLS errors
- Ensure correct port for TLS type
- Check certificate validity

### Slack Issues

**Problem:** Notifications not appearing
- Verify webhook URL is correct and active
- Check Slack app permissions
- Review server logs for HTTP errors

**Problem:** 404 error
- Webhook may have been deleted, recreate it

**Problem:** Rate limited by Slack
- Slack limits: ~1 message/second/webhook
- Use BlazeLog rate limiting and cooldowns

### Teams Issues

**Problem:** Notifications not appearing
- Verify webhook URL is correct
- Check connector is still enabled
- Review server logs for HTTP errors

**Problem:** 400 Bad Request
- Adaptive Card may be malformed
- Check server logs for details

**Problem:** Webhook disabled
- Connectors can be disabled by admins
- Check channel connector settings

### General Issues

**Problem:** No notifications at all
- Check that alert rules have `notify` configured
- Verify notification channels are configured in server.yaml
- Check rate limiting isn't dropping everything
- Review server logs

**Problem:** Too many notifications
- Increase `cooldown` on rules
- Lower rate limit `max_per_window`
- Adjust threshold values

---

## See Also

- [Alert Rules Reference](alerts.md) - Configure alert rules
- [Configuration Reference](../CONFIGURATION.md) - Full server configuration
- [Troubleshooting Guide](../TROUBLESHOOTING.md) - General troubleshooting
