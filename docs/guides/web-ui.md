# Web UI Guide

BlazeLog provides a web-based user interface for monitoring logs, managing alerts, and configuring the system.

---

## Accessing the UI

### URL and Port

```yaml
# server.yaml
server:
  http_address: ":8080"
```

Access the UI at `http://your-server:8080`

### Authentication

BlazeLog uses session-based authentication:

1. Navigate to the login page
2. Enter your username and password
3. Sessions expire after 24 hours of inactivity

---

## Dashboard

The dashboard provides an overview of your log data.

### Stats Cards

| Metric | Description |
|--------|-------------|
| Total Logs | Count of logs in selected time range |
| Errors | Count of error-level logs |
| Warnings | Count of warning-level logs |
| Active Alerts | Currently active alert rules |

### Charts

| Chart | Description |
|-------|-------------|
| Log Volume | Time-series of total logs and errors |
| Log Levels | Donut chart showing level distribution |
| Top Sources | Horizontal bar chart of busiest sources |
| HTTP Status | Bar chart of 2xx/3xx/4xx/5xx responses |

### Time Range

Select the time range for dashboard data:
- 15 minutes
- 1 hour
- 6 hours
- 24 hours (default)
- 7 days
- 30 days

### Auto-Refresh

Enable automatic dashboard refresh:
- Off (manual only)
- 10 seconds
- 30 seconds (default)
- 1 minute
- 5 minutes

---

## Log Viewer

The log viewer provides real-time log viewing and search capabilities.

### Search and Filters

| Filter | Description |
|--------|-------------|
| Search | Full-text search across log messages |
| Time Range | Filter by time window |
| Level | Filter by log level (debug, info, warning, error, fatal) |
| Source | Filter by log source name |
| Type | Filter by log type (nginx, apache, magento, etc.) |
| Search Mode | Token (word match), Substring, or Phrase |

### Search Modes

- **Token**: Matches whole words (default, fastest)
- **Substring**: Matches any part of message
- **Phrase**: Matches exact phrase

### Live Streaming

Click **Live** to stream new logs in real-time via SSE (Server-Sent Events):
- Green indicator shows live mode is active
- New logs appear at the top of the table
- Click **Paused** to stop streaming

### Log Details

Click any log row to view details:
- Full timestamp and level
- Source and log type
- Agent ID and file path
- HTTP request details (if applicable)
- Full message (syntax highlighted)
- Parsed fields and labels

### Export

Export logs with current filters:
- **Formats**: JSON or CSV
- **Limits**: 100, 500, 1,000, or 5,000 records

---

## Settings

### Alert Rules

**Path**: `/settings/alerts`

**Access**: All users can view, admin/operator can edit

Manage alert rules:
- View all configured alerts
- Filter by project or type
- Create new alerts (admin/operator)
- Edit existing alerts (admin/operator)
- Delete alerts (admin only)

#### Creating an Alert

1. Click **Create Alert**
2. Fill in the form:
   - **Name**: Unique alert name
   - **Description**: Optional description
   - **Type**: Pattern (regex) or Threshold (count)
   - **Condition**: JSON condition (see [Alert Rules](alerts.md))
   - **Severity**: Low, Medium, High, Critical
   - **Window**: Time window for threshold alerts
   - **Cooldown**: Minimum time between notifications
   - **Project**: Optional project scope
   - **Channels**: Email, Slack, Teams
   - **Enabled**: Toggle alert on/off
3. Click **Save**

### Projects

**Path**: `/settings/projects`

**Access**: Admin only

Manage projects for organizing logs and alerts.

### SSH Connections

**Path**: `/settings/connections`

**Access**: Admin only

Manage SSH connections for agentless log collection (see [SSH Collection Guide](ssh-collection.md)).

### Users

**Path**: `/settings/users`

**Access**: Admin only

Manage user accounts and roles:
- Create new users
- Edit user roles
- Disable/enable accounts
- Reset passwords

---

## User Roles

| Role | View Logs | View Alerts | Edit Alerts | Delete Alerts | Settings |
|------|-----------|-------------|-------------|---------------|----------|
| viewer | Yes | Yes | No | No | No |
| operator | Yes | Yes | Yes | No | No |
| admin | Yes | Yes | Yes | Yes | Yes |

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Esc` | Close modal dialogs |
| `Enter` | Submit forms (when focused) |

---

## Browser Requirements

BlazeLog Web UI supports modern browsers:

| Browser | Minimum Version |
|---------|-----------------|
| Chrome | 90+ |
| Firefox | 88+ |
| Safari | 14+ |
| Edge | 90+ |

**Required Features**:
- JavaScript enabled
- Cookies enabled (for session)
- SSE support (for live streaming)

---

## Mobile Support

The UI is responsive and works on tablets and large phones. For best experience:
- Use landscape orientation on tablets
- Some tables may scroll horizontally on small screens
- Export functionality works on all devices

---

## Troubleshooting

### Can't Login

1. **Check credentials**: Verify username/password
2. **Check server logs**: Look for auth failures
3. **Clear cookies**: Try clearing site cookies

### Dashboard Not Loading

1. **Check browser console**: Look for JavaScript errors
2. **Check network tab**: Verify API requests succeed
3. **Check server health**: Ensure server is running

### Live Streaming Not Working

1. **Check SSE support**: Ensure browser supports SSE
2. **Check proxy config**: Reverse proxies may buffer SSE
3. **Check firewall**: Long-lived connections must be allowed

For reverse proxy SSE configuration:

```nginx
# Nginx
location /logs/stream {
    proxy_pass http://blazelog:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
}
```

```apache
# Apache
<Location /logs/stream>
    ProxyPass http://blazelog:8080/logs/stream
    ProxyPassReverse http://blazelog:8080/logs/stream
    SetEnv proxy-sendchunked 1
</Location>
```

### Slow Performance

1. **Reduce time range**: Use shorter time windows
2. **Add filters**: Filter by level, source, or type
3. **Check server resources**: Monitor CPU/memory

---

## See Also

- [Alert Rules Reference](alerts.md) - Alert configuration
- [Configuration Reference](../CONFIGURATION.md) - Server configuration
- [Troubleshooting Guide](../TROUBLESHOOTING.md) - General troubleshooting
