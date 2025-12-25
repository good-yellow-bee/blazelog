# Security and Performance Review - BlazeLog

**Date:** 2025-12-25
**Reviewer:** Claude Code
**Codebase Version:** commit a6dfdc3

## Executive Summary

This review identified **3 high-severity**, **5 medium-severity**, and **4 low-severity** security issues, along with **4 performance concerns**. The codebase demonstrates solid security foundations including mTLS, bcrypt password hashing, JWT with refresh token rotation, RBAC, rate limiting, and account lockout. However, several vulnerabilities require attention.

---

## High Severity Issues

### 1. SSH Command Injection via Glob Pattern
**Location:** `internal/ssh/client.go:336`

**Issue:** The `ListFiles` function interpolates user-controlled glob patterns directly into a shell command without escaping:
```go
cmd := fmt.Sprintf("ls -1 %s 2>/dev/null || true", pattern)
```

**Impact:** Remote command execution on target SSH servers. An attacker with access to create SSH sources could inject commands like `; rm -rf /`.

**Recommendation:** Use proper shell escaping or execute `ls` without shell interpolation:
```go
cmd := fmt.Sprintf("ls -1 %q 2>/dev/null || true", pattern)
// Or better: use SFTP instead of shell commands
```

---

### 2. SQL Injection via ORDER BY Clause
**Location:** `internal/storage/clickhouse.go:556-563`

**Issue:** The `orderBy` parameter is directly interpolated into SQL without parameterization:
```go
sb.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir))
```

**Current Mitigation:** The API handler (`internal/api/logs/handler.go:208-214`) validates that `orderBy` is only `timestamp` or `level`.

**Risk:** If validation is bypassed or new code paths are added, SQL injection is possible.

**Recommendation:** Use an allowlist at the query builder level:
```go
allowedColumns := map[string]string{"timestamp": "timestamp", "level": "level"}
if col, ok := allowedColumns[filter.OrderBy]; ok {
    sb.WriteString(fmt.Sprintf(" ORDER BY %s %s", col, orderDir))
}
```

---

### 3. SSH Host Key Verification Disabled by Default
**Location:** `internal/ssh/client.go:92-93`

**Issue:** When no `HostKeyCallback` is provided, the code defaults to ignoring host keys:
```go
if hostKeyCallback == nil {
    hostKeyCallback = ssh.InsecureIgnoreHostKey()
}
```

**Impact:** Vulnerable to man-in-the-middle attacks on SSH connections.

**Recommendation:**
- Require explicit host key verification configuration
- Use known_hosts file integration (partially implemented in `internal/ssh/hostkey.go`)
- Log warnings when insecure mode is used
- Consider making host key verification mandatory for production

---

## Medium Severity Issues

### 4. SQL Injection in JSON Field Access
**Location:** `internal/query/sql_builder.go:297`

**Issue:** Property names for JSON field access are not escaped:
```go
return fmt.Sprintf("JSONExtractString(%s, '%s')", field.Column, propName)
```

**Impact:** If `propName` contains a single quote (e.g., `field's`), it could break the SQL and potentially allow injection.

**Recommendation:** Escape single quotes in property names:
```go
escapedName := strings.ReplaceAll(propName, "'", "\\'")
return fmt.Sprintf("JSONExtractString(%s, '%s')", field.Column, escapedName)
```

---

### 5. Rate Limiter Bypass via Header Spoofing
**Location:** `internal/api/middleware/ratelimit.go:136-156`

**Issue:** The `getClientIP` function trusts `X-Forwarded-For` and `X-Real-IP` headers unconditionally:
```go
if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
    // Takes first IP without validation
    return xff
}
```

**Impact:** Attackers can bypass rate limiting and brute-force protection by spoofing headers.

**Recommendation:**
- Only trust proxy headers when behind a known reverse proxy
- Add configuration option for trusted proxy ranges
- Validate IP address format before use
- Consider using a library like `realip` that handles edge cases

---

### 6. Web UI Login Missing Account Lockout
**Location:** `internal/web/handlers/auth.go:25-54`

**Issue:** The web UI login handler does not integrate with the `LockoutTracker`, unlike the API login handler.

**Impact:** Brute-force attacks against the web UI are not protected by account lockout.

**Recommendation:** Add lockout tracking to web login:
```go
// Check lockout
if h.lockoutTracker.IsLocked(username) {
    renderLoginError(w, r, "Account temporarily locked")
    return
}
// On failure:
h.lockoutTracker.RecordFailure(username)
// On success:
h.lockoutTracker.ClearFailures(username)
```

---

### 7. Remember Me Cookie/Session TTL Mismatch
**Location:** `internal/web/handlers/auth.go:65-79`

**Issue:** When "remember me" is checked, cookie max age is 30 days, but the underlying session has a 24-hour TTL.

**Impact:** Users may have a valid cookie but expired session, causing confusion. The session check correctly rejects expired sessions, but the UX is poor.

**Recommendation:** Extend session TTL when remember_me is enabled, or use separate long-lived tokens.

---

### 8. Plaintext Credentials in Configuration
**Location:** `cmd/server/config.go:48,85`

**Issue:** ClickHouse password and SSH credentials can be stored in plaintext in config files:
```yaml
clickhouse:
  password: "my_password"
ssh_connections:
  - password: "ssh_password"
```

**Impact:** Credential exposure if config files are compromised or committed to version control.

**Recommendation:**
- Add deprecation warnings for plaintext passwords
- Prefer environment variable references (`password_env`)
- Consider secrets management integration (Vault, AWS Secrets Manager)

---

## Low Severity Issues

### 9. In-Memory Lockout State Not Persistent
**Location:** `internal/api/auth/lockout.go`

**Issue:** Lockout tracking uses in-memory storage. Server restarts clear all lockout data.

**Impact:**
- Attackers can restart lockout timers by waiting for server restarts
- Multiple server instances don't share lockout state

**Recommendation:** Consider using Redis or database-backed lockout tracking for production deployments.

---

### 10. In-Memory Session Store
**Location:** `internal/web/session/store.go`

**Issue:** Sessions are stored in memory with no persistence or sharing mechanism.

**Impact:**
- Not suitable for horizontal scaling
- All sessions invalidated on restart

**Recommendation:** Document this limitation. Consider optional Redis-backed session store for multi-instance deployments.

---

### 11. Unbounded Agent Registration
**Location:** `internal/server/handler.go:55`

**Issue:** Registered agents are stored in `sync.Map` indefinitely with no cleanup.

**Impact:** Memory leak if many agents connect and disconnect over time.

**Recommendation:** Implement agent TTL with periodic cleanup, or remove agents on connection close.

---

### 12. Debug Information Leakage Potential
**Location:** Various log statements

**Issue:** Some log statements include internal paths and error details that could aid attackers.

**Example:** `internal/api/middleware/auth.go:72`
```go
log.Printf("JWT auth failed for %s: %v", r.RemoteAddr, err)
```

**Impact:** Low - logs are not exposed to clients, but could leak via log aggregation.

**Recommendation:** Sanitize error messages in production logs. Consider structured logging with severity levels.

---

## Performance Issues

### 1. DELETE with Prior COUNT in ClickHouse
**Location:** `internal/storage/clickhouse.go:414-429`

**Issue:** `DeleteBefore` runs a COUNT query before DELETE.

**Recommendation:** The count is returned for informational purposes. Consider making it optional or async.

---

### 2. SSE Polling Inefficiency
**Location:** `internal/api/logs/handler.go:565-628`

**Issue:** Log streaming uses 1-second polling interval.

**Recommendation:** Consider pub/sub architecture with WebSocket or true SSE push for real-time logs.

---

### 3. Sync.Map for Rate Limiter
**Location:** `internal/api/middleware/ratelimit.go`

**Issue:** `sync.Map` isn't optimal for high-contention write workloads.

**Recommendation:** Consider sharded maps or specialized rate limiting libraries.

---

### 4. No Connection Pool Limits for SSH
**Location:** `internal/ssh/pool.go`

**Issue:** SSH connection pool has no maximum connection limit per host.

**Recommendation:** Add configurable max connections per host to prevent resource exhaustion.

---

## Security Strengths

The codebase demonstrates many security best practices:

1. **mTLS for gRPC** - TLS 1.3 with mandatory client certificate verification
2. **bcrypt Password Hashing** - Industry standard with appropriate cost factor
3. **JWT with Refresh Token Rotation** - Short-lived access tokens with automatic rotation
4. **RBAC Implementation** - Clear role hierarchy (admin > operator > viewer)
5. **Account Lockout** - 5 failures trigger 15-minute lockout (API only)
6. **Rate Limiting** - Token bucket per IP and per user
7. **Security Headers** - CSP, X-Frame-Options, X-Content-Type-Options
8. **CSRF Protection** - Gorilla CSRF for web forms
9. **Parameterized SQL** - Most queries use parameterized statements
10. **AES-256-GCM Encryption** - Proper authenticated encryption with PBKDF2
11. **Secrets from Environment** - Sensitive values loaded from env vars

---

## Recommendations Summary

### Immediate Actions (High Priority)
1. Fix SSH command injection in `ListFiles`
2. Add SQL injection protection at query builder level
3. Enable SSH host key verification by default

### Short-term Actions (Medium Priority)
4. Escape JSON field names in SQL builder
5. Implement trusted proxy configuration for rate limiting
6. Add account lockout to web UI login
7. Warn on plaintext password configuration

### Long-term Improvements
8. Add Redis-backed session store option
9. Implement persistent lockout storage
10. Add agent cleanup mechanism
11. Consider real-time log streaming architecture

---

## Files Reviewed

- `internal/api/auth/*.go` - Authentication handlers and services
- `internal/api/middleware/*.go` - Security middleware
- `internal/security/*.go` - Cryptography and TLS
- `internal/storage/*.go` - Database interactions
- `internal/ssh/*.go` - SSH client and operations
- `internal/web/*.go` - Web UI handlers
- `internal/server/*.go` - gRPC server
- `internal/query/*.go` - Query DSL and SQL builder
- `cmd/server/config.go` - Configuration handling
