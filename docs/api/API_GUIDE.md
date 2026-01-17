# REST API Guide

This guide covers using the BlazeLog REST API for programmatic access.

---

## Overview

| Property | Value |
|----------|-------|
| Base URL | `http://your-server:8080/api/v1` |
| Authentication | JWT Bearer Token |
| Content-Type | `application/json` |
| Full Spec | [openapi.yaml](openapi.yaml) |

---

## Authentication

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}'
```

Response:
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

### Using the Token

Include the access token in all subsequent requests:

```bash
curl http://localhost:8080/api/v1/logs \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json"
```

### Token Refresh

Access tokens expire after 15 minutes by default. Use the refresh token to get a new pair:

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "YOUR_REFRESH_TOKEN"}'
```

### Logout

Revoke the refresh token:

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "YOUR_REFRESH_TOKEN"}'
```

---

## Response Format

### Success Response

```json
{
  "data": { ... }
}
```

### Paginated Response

```json
{
  "data": {
    "items": [ ... ],
    "total": 1234,
    "page": 1,
    "per_page": 50,
    "total_pages": 25
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Start time is required"
  }
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (success, no body) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 429 | Rate Limited / Account Locked |
| 500 | Internal Server Error |

---

## Logs

### Query Logs

```bash
# Basic query - last 24 hours
curl "http://localhost:8080/api/v1/logs?start=2024-01-01T00:00:00Z" \
  -H "Authorization: Bearer TOKEN"

# With filters
curl "http://localhost:8080/api/v1/logs?start=2024-01-01T00:00:00Z&level=error&type=nginx&q=500" \
  -H "Authorization: Bearer TOKEN"

# Pagination
curl "http://localhost:8080/api/v1/logs?start=2024-01-01T00:00:00Z&page=2&per_page=100" \
  -H "Authorization: Bearer TOKEN"
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `start` | datetime | Start time (required, RFC3339) |
| `end` | datetime | End time (default: now) |
| `agent_id` | string | Filter by agent ID |
| `level` | string | Filter by level (debug, info, warning, error, fatal) |
| `levels` | string | Comma-separated levels |
| `type` | string | Filter by log type |
| `source` | string | Filter by source |
| `q` | string | Search query |
| `search_mode` | string | token, substring, or phrase |
| `page` | integer | Page number (default: 1) |
| `per_page` | integer | Results per page (default: 50, max: 1000) |
| `order` | string | Sort field (timestamp, level) |
| `order_dir` | string | Sort direction (asc, desc) |

### Get Log Statistics

```bash
curl "http://localhost:8080/api/v1/logs/stats?start=2024-01-01T00:00:00Z&interval=hour" \
  -H "Authorization: Bearer TOKEN"
```

Response:
```json
{
  "data": {
    "total_logs": 15234,
    "error_count": 42,
    "warning_count": 156,
    "fatal_count": 0,
    "volume": [
      {"timestamp": "2024-01-01T00:00:00Z", "total_count": 120, "error_count": 5},
      {"timestamp": "2024-01-01T01:00:00Z", "total_count": 98, "error_count": 2}
    ],
    "top_sources": [
      {"source": "nginx", "count": 8000},
      {"source": "magento", "count": 4000}
    ],
    "http_stats": {
      "total_2xx": 7500,
      "total_3xx": 200,
      "total_4xx": 250,
      "total_5xx": 50
    }
  }
}
```

### Stream Logs (SSE)

```bash
curl -N "http://localhost:8080/api/v1/logs/stream?level=error" \
  -H "Authorization: Bearer TOKEN" \
  -H "Accept: text/event-stream"
```

Events:
```
event: log
data: {"id":"abc123","timestamp":"2024-01-01T10:30:00Z","level":"error","message":"..."}

event: log
data: {"id":"abc124","timestamp":"2024-01-01T10:30:01Z","level":"error","message":"..."}
```

---

## Alerts

### List Alerts

```bash
curl "http://localhost:8080/api/v1/alerts" \
  -H "Authorization: Bearer TOKEN"
```

### Get Alert

```bash
curl "http://localhost:8080/api/v1/alerts/123" \
  -H "Authorization: Bearer TOKEN"
```

### Create Alert

```bash
curl -X POST "http://localhost:8080/api/v1/alerts" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Error Rate",
    "description": "Alert on high error rate",
    "type": "threshold",
    "condition": {
      "field": "level",
      "value": "error",
      "threshold": 100,
      "window": "5m"
    },
    "severity": "high",
    "cooldown": "15m",
    "notify": ["slack", "email"],
    "enabled": true
  }'
```

### Update Alert

```bash
curl -X PUT "http://localhost:8080/api/v1/alerts/123" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": false
  }'
```

### Delete Alert

```bash
curl -X DELETE "http://localhost:8080/api/v1/alerts/123" \
  -H "Authorization: Bearer TOKEN"
```

---

## Projects

### List Projects

```bash
curl "http://localhost:8080/api/v1/projects" \
  -H "Authorization: Bearer TOKEN"
```

### Create Project

```bash
curl -X POST "http://localhost:8080/api/v1/projects" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-web",
    "description": "Production web servers"
  }'
```

---

## Users

### Get Current User

```bash
curl "http://localhost:8080/api/v1/users/me" \
  -H "Authorization: Bearer TOKEN"
```

### Change Password

```bash
curl -X PUT "http://localhost:8080/api/v1/users/me/password" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "current_password": "old-password",
    "new_password": "new-password"
  }'
```

### List Users (Admin)

```bash
curl "http://localhost:8080/api/v1/users" \
  -H "Authorization: Bearer TOKEN"
```

### Create User (Admin)

```bash
curl -X POST "http://localhost:8080/api/v1/users" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "jdoe",
    "email": "jdoe@example.com",
    "password": "secure-password",
    "role": "operator"
  }'
```

---

## SSH Connections (Admin)

### List Connections

```bash
curl "http://localhost:8080/api/v1/connections" \
  -H "Authorization: Bearer TOKEN"
```

### Create Connection

```bash
curl -X POST "http://localhost:8080/api/v1/connections" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server-1",
    "host": "web1.example.com:22",
    "user": "blazelog",
    "key_file": "/etc/blazelog/ssh/web1.key",
    "sources": [
      {"path": "/var/log/nginx/*.log", "type": "nginx"}
    ]
  }'
```

---

## Code Examples

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

func main() {
    // Login
    loginBody := `{"username":"admin","password":"password"}`
    resp, _ := http.Post(
        "http://localhost:8080/api/v1/auth/login",
        "application/json",
        strings.NewReader(loginBody),
    )

    var loginResp struct {
        Data struct {
            AccessToken string `json:"access_token"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&loginResp)
    resp.Body.Close()

    // Query logs
    req, _ := http.NewRequest("GET",
        "http://localhost:8080/api/v1/logs?start=2024-01-01T00:00:00Z&level=error",
        nil)
    req.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)

    resp, _ = http.DefaultClient.Do(req)
    defer resp.Body.Close()

    var logsResp struct {
        Data struct {
            Items []map[string]interface{} `json:"items"`
            Total int                       `json:"total"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&logsResp)

    fmt.Printf("Found %d error logs\n", logsResp.Data.Total)
}
```

### Python

```python
import requests

# Login
login_resp = requests.post(
    "http://localhost:8080/api/v1/auth/login",
    json={"username": "admin", "password": "password"}
)
token = login_resp.json()["data"]["access_token"]

headers = {"Authorization": f"Bearer {token}"}

# Query logs
logs_resp = requests.get(
    "http://localhost:8080/api/v1/logs",
    headers=headers,
    params={
        "start": "2024-01-01T00:00:00Z",
        "level": "error",
        "per_page": 100
    }
)
logs = logs_resp.json()["data"]
print(f"Found {logs['total']} error logs")

# Stream logs (SSE)
import sseclient

response = requests.get(
    "http://localhost:8080/api/v1/logs/stream",
    headers={**headers, "Accept": "text/event-stream"},
    params={"level": "error"},
    stream=True
)
client = sseclient.SSEClient(response)
for event in client.events():
    if event.event == "log":
        print(event.data)
```

### JavaScript/Node.js

```javascript
const fetch = require('node-fetch');

async function main() {
    // Login
    const loginResp = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'password' })
    });
    const { data: { access_token } } = await loginResp.json();

    // Query logs
    const logsResp = await fetch(
        'http://localhost:8080/api/v1/logs?start=2024-01-01T00:00:00Z&level=error',
        { headers: { 'Authorization': `Bearer ${access_token}` } }
    );
    const logs = await logsResp.json();
    console.log(`Found ${logs.data.total} error logs`);
}

main();
```

---

## Rate Limiting

API requests are rate-limited:

| Endpoint | Limit |
|----------|-------|
| `/auth/login` | 5 requests/minute per IP |
| Other endpoints | 100 requests/minute per user |

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1704067260
```

---

## See Also

- [openapi.yaml](openapi.yaml) - Full OpenAPI specification
- [Web UI Guide](../guides/web-ui.md) - Web interface
- [Configuration Reference](../CONFIGURATION.md) - Server configuration
