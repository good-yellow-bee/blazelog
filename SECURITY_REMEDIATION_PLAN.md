# Security Remediation Plan

Comprehensive plan addressing 51 security and quality issues identified in BlazeLog codebase review.

---

## Phase 1: CRITICAL Issues (7 tasks)

### Task 1.1: Remove Hardcoded Secrets from Script
**File:** `scripts/blazelog-start.sh:10-26`
**Issue:** Hardcoded BLAZELOG_MASTER_KEY, BLAZELOG_DB_KEY, BLAZELOG_JWT_SECRET, BLAZELOG_CSRF_SECRET in version control.
**Fix:**
```bash
# Remove default values, fail if not set
if [ -z "$BLAZELOG_MASTER_KEY" ]; then
    echo "ERROR: BLAZELOG_MASTER_KEY not set" >&2
    exit 1
fi
# Repeat for other secrets
```
**Verification:** `grep -r "MBmg2c" scripts/` returns nothing

---

### Task 1.2: Fix Template Injection (text/template → html/template)
**File:** `internal/notifier/templates.go:7`
**Issue:** Using `text/template` for HTML emails allows XSS in email clients.
**Fix:**
```go
import "html/template"  // NOT text/template

// Line 52: Already parses HTML, just need import change
```
**Verification:** Build succeeds, email templates render with escaped HTML

---

### Task 1.3: Fix Double-Close Panic in ConnManager
**File:** `internal/agent/connmgr.go:260-261`
**Issue:** `Close()` panics if called twice due to closing already-closed channel.
**Fix:**
```go
type ConnManager struct {
    // ...
    closed atomic.Bool
}

func (m *ConnManager) Close() error {
    if m.closed.Swap(true) {
        return nil  // Already closed
    }
    close(m.stopCh)
    // ... rest of cleanup
}
```
**Verification:** Unit test calling `Close()` twice doesn't panic

---

### Task 1.4: Add Project Authorization to Alerts Handler
**File:** `internal/api/alerts/handler.go:133-157, 228-248, 252-347, 349-377, 379-428`
**Issue:** No project access validation - users can access any project's alerts.
**Fix:**
```go
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    projectID := r.URL.Query().Get("project_id")

    // Add access check
    userID := middleware.GetUserID(ctx)
    role := middleware.GetRole(ctx)
    access, err := middleware.GetProjectAccess(ctx, userID, role, h.store)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "failed to get project access")
        return
    }

    if projectID != "" && !access.CanAccessProject(projectID) {
        jsonError(w, 403, "FORBIDDEN", "no access to project")
        return
    }
    // ... rest of handler with filtered project list
}
```
**Apply to:** List, GetByID, Update, Delete, History endpoints
**Verification:** Non-admin user cannot access alerts for unassigned project

---

### Task 1.5: Add Project Authorization to Connections Handler
**File:** `internal/api/connections/handler.go:106-130, 133-205, 207-228, 230-316`
**Issue:** No project access validation for connections.
**Fix:** Same pattern as Task 1.4 - add `GetProjectAccess` check to List, Create, GetByID, Update
**Verification:** Non-admin user cannot access connections for unassigned project

---

### Task 1.6: Add Project Authorization to Projects Handler
**File:** `internal/api/projects/handler.go:174-195, 287-324`
**Issue:** GetByID and GetUsers bypass project membership check.
**Fix:**
```go
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := chi.URLParam(r, "id")
    userID := middleware.GetUserID(ctx)
    role := middleware.GetRole(ctx)

    // Non-admins must have project access
    if role != models.RoleAdmin {
        hasAccess, err := h.storage.Projects().UserHasAccess(ctx, userID, id)
        if err != nil || !hasAccess {
            jsonError(w, 403, "FORBIDDEN", "no access to project")
            return
        }
    }
    // ... rest of handler
}
```
**Verification:** Non-admin cannot retrieve arbitrary project details

---

### Task 1.7: Add gRPC Message Size Limits
**File:** `internal/server/server.go:90`
**Issue:** No limits on gRPC message size - DoS via memory exhaustion.
**Fix:**
```go
import "google.golang.org/grpc"

opts = append(opts,
    grpc.MaxRecvMsgSize(4*1024*1024),   // 4MB max receive
    grpc.MaxSendMsgSize(4*1024*1024),   // 4MB max send
    grpc.MaxConcurrentStreams(100),      // Limit concurrent streams
)
s.grpcServer = grpc.NewServer(opts...)
```
**Verification:** Test with oversized batch returns error

---

## Phase 2: HIGH Issues (17 tasks)

### Task 2.1: Fix Race Condition in LogBuffer Flush
**File:** `internal/storage/log_buffer.go:120-146`
**Issue:** Lock released before flush completes, causing order disruption on error.
**Fix:**
```go
func (b *LogBuffer) Flush() error {
    b.mu.Lock()
    if len(b.buffer) == 0 {
        b.mu.Unlock()
        return nil
    }

    toFlush := b.buffer
    b.buffer = make([]*LogRecord, 0, b.batchSize)
    b.mu.Unlock()

    if err := b.repo.InsertBatch(ctx, toFlush); err != nil {
        b.mu.Lock()
        // Prepend failed entries, maintaining order
        b.buffer = append(toFlush, b.buffer...)
        b.mu.Unlock()
        return err
    }
    return nil
}
```
**Note:** Current implementation is actually correct - document the ordering guarantee

---

### Task 2.2: Fix Goroutine Leak on Context Cancellation
**File:** `internal/agent/tailer.go` (verify exact location)
**Issue:** Goroutines not properly cleaned up on context cancel.
**Fix:** Ensure all goroutines check `ctx.Done()` and exit cleanly
**Verification:** `go test -race` passes, no leaked goroutines

---

### Task 2.3: Fix Race in Buffer Replay
**File:** `internal/agent/buffer.go` (verify exact location)
**Issue:** Concurrent access during replay.
**Fix:** Add proper synchronization with mutex
**Verification:** `go test -race` passes

---

### Task 2.4: Fix Memory Pressure in Compaction
**File:** `internal/agent/buffer.go` (verify exact location)
**Issue:** Memory pressure during compaction operations.
**Fix:** Implement incremental compaction or streaming approach
**Verification:** Memory profiling shows stable usage

---

### Task 2.5: Add Batch Size Validation
**File:** `internal/server/handler.go:161-162`
**Issue:** No server-side enforcement of MaxBatchSize.
**Fix:**
```go
func (h *Handler) StreamLogs(stream blazelogv1.LogService_StreamLogsServer) error {
    for {
        batch, err := stream.Recv()
        // ...
        if len(batch.Entries) > 100 {
            return status.Errorf(codes.InvalidArgument, "batch size %d exceeds max 100", len(batch.Entries))
        }
        // ...
    }
}
```
**Verification:** Batch with 101 entries returns error

---

### Task 2.6: Add Stream Timeout/Deadline
**File:** `internal/server/handler.go:137-188`
**Issue:** Streams run indefinitely without timeout.
**Fix:**
```go
func (h *Handler) StreamLogs(stream blazelogv1.LogService_StreamLogsServer) error {
    idleTimeout := time.NewTimer(5 * time.Minute)
    defer idleTimeout.Stop()

    for {
        select {
        case <-idleTimeout.C:
            return status.Error(codes.DeadlineExceeded, "idle timeout")
        default:
            batch, err := stream.Recv()
            idleTimeout.Reset(5 * time.Minute)
            // ...
        }
    }
}
```
**Verification:** Idle stream closes after timeout

---

### Task 2.7: Fix Agent Activity Race Condition
**File:** `internal/server/handler.go:194-197`
**Issue:** `e.lastActive = time.Now()` without synchronization.
**Fix:**
```go
type agentEntry struct {
    info       *blazelogv1.AgentInfo
    lastActive atomic.Value  // Change to atomic
}

// Update usage:
e.lastActive.Store(time.Now())
```
**Verification:** `go test -race` passes

---

### Task 2.8: Fix CSP unsafe-inline/unsafe-eval
**File:** `internal/api/middleware/headers.go:21-26`
**Issue:** CSP allows unsafe-inline and unsafe-eval.
**Fix:** Use nonces for inline scripts or refactor to external files
```go
// Generate nonce per request
nonce := generateNonce()
w.Header().Set("Content-Security-Policy",
    fmt.Sprintf("script-src 'self' 'nonce-%s' https://cdn.jsdelivr.net;", nonce))
```
**Note:** Requires template changes to add nonce to script tags
**Verification:** Browser console shows no CSP violations

---

### Task 2.9: Add InsecureSkipVerify Warning
**File:** `internal/security/tls.go:25, 59-60`
**Issue:** InsecureSkipVerify can be enabled without warning.
**Fix:**
```go
if cfg.InsecureSkipVerify {
    log.Printf("CRITICAL: TLS verification disabled - INSECURE, do not use in production")
}
```
**Verification:** Log warning appears when flag enabled

---

### Task 2.10: Fix Session Cookie Secure Flag
**File:** `internal/web/handlers/auth.go:99`
**Issue:** Secure flag based on `r.TLS` which is nil behind proxy.
**Fix:**
```go
// Use config setting or check X-Forwarded-Proto
secure := h.config.UseSecureCookies
if !secure && r.Header.Get("X-Forwarded-Proto") == "https" {
    secure = true
}
http.SetCookie(w, &http.Cookie{
    // ...
    Secure: secure,
})
```
**Verification:** Cookie has Secure flag behind HTTPS proxy

---

### Task 2.11: Add Session Store Size Limits
**File:** `internal/web/session/store.go`
**Issue:** No max size limit on sessions map.
**Fix:**
```go
const maxSessions = 10000

func (s *Store) Create(...) (*Session, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if len(s.sessions) >= maxSessions {
        // Evict oldest sessions
        s.evictOldest(maxSessions / 10)
    }
    // ...
}
```
**Verification:** Sessions capped at limit

---

### Task 2.12: Fix Connection Leak on Context Cancel
**File:** `internal/agent/connmgr.go` (verify exact location)
**Issue:** Connection not closed on context cancellation.
**Fix:** Ensure connections are closed in defer or select on ctx.Done()
**Verification:** No leaked connections after context cancel

---

### Task 2.13: Fix Deferred Rollback After Commit
**File:** `internal/storage/clickhouse.go:262-263`
**Issue:** `defer tx.Rollback()` runs after successful commit.
**Fix:**
```go
func (c *ClickHouseStorage) InsertBatch(ctx context.Context, records []*LogRecord) error {
    tx, err := c.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }

    committed := false
    defer func() {
        if !committed {
            tx.Rollback()
        }
    }()

    // ... batch insert ...

    if err := tx.Commit(); err != nil {
        return err
    }
    committed = true
    return nil
}
```
**Verification:** No "rollback after commit" errors in logs

---

### Task 2.14: Fix Sliding Window Unbounded Memory
**File:** `internal/alerting/window.go:20, 46-48`
**Issue:** 100K events per rule, no global limit.
**Fix:**
```go
const (
    maxEventsPerRule = 10000   // Reduce from 100K
    maxTotalEvents   = 100000  // Global cap
)

func (m *WindowManager) Add(ruleID string, event Event) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.totalEvents >= maxTotalEvents {
        m.evictOldestGlobal()
    }
    // ...
}
```
**Verification:** Memory usage capped under load

---

### Task 2.15: Fix Alert Channel Drops
**File:** `internal/alerting/engine.go:62, 107-111`
**Issue:** Alerts silently dropped when channel full.
**Fix:**
```go
var droppedAlerts atomic.Uint64

select {
case e.alerts <- alert:
    // sent
default:
    droppedAlerts.Add(1)
    log.Printf("WARNING: alert dropped (buffer full), total dropped: %d", droppedAlerts.Load())
}
```
**Verification:** Dropped alerts logged and counted

---

### Task 2.16: Fix HTTP Response Body Memory Leak
**File:** `internal/notifier/slack.go:79`, `internal/notifier/teams.go:79`
**Issue:** `io.ReadAll` on error response without size limit.
**Fix:**
```go
if resp.StatusCode != http.StatusOK {
    body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
    if err != nil {
        return fmt.Errorf("status %d (body read error)", resp.StatusCode)
    }
    return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
}
```
**Verification:** Large error responses don't exhaust memory

---

### Task 2.17: Add Scanner Buffer Size Limit
**File:** `internal/parser/parser.go:128`
**Issue:** Scanner has no buffer configuration for large lines.
**Fix:**
```go
scanner := bufio.NewScanner(r)
scanner.Buffer(make([]byte, 64*1024), 1024*1024)  // 1MB max line
```
**Verification:** Large log lines (>64KB) parse correctly

---

## Phase 3: MEDIUM Issues (15 tasks)

### Task 3.1: Fix ReDoS via matches Operator
**File:** `internal/query/sql_builder.go:335-336`
**Issue:** User regex patterns can cause catastrophic backtracking.
**Fix:**
```go
// Option 1: Add timeout to regex operations
// Option 2: Validate regex complexity before use
// Option 3: Remove matches operator

func validateRegexComplexity(pattern string) error {
    // Check for known ReDoS patterns: (a+)+, (a|a)+, etc.
    dangerousPatterns := []string{`(\w+)+`, `(a+)+`, `(a|a)+`}
    for _, dp := range dangerousPatterns {
        if strings.Contains(pattern, dp) {
            return errors.New("potentially dangerous regex pattern")
        }
    }
    return nil
}
```
**Verification:** Known ReDoS patterns rejected

---

### Task 3.2: Add AST Depth/Complexity Limits
**File:** `internal/query/dsl.go:40-69`
**Issue:** No limits on query complexity.
**Fix:**
```go
const (
    maxASTDepth = 20
    maxASTNodes = 100
)

type validationVisitor struct {
    // ...
    depth     int
    nodeCount int
}

func (v *validationVisitor) Visit(node ast.Node) ast.Visitor {
    v.nodeCount++
    if v.nodeCount > maxASTNodes {
        v.err = errors.New("query too complex: too many nodes")
        return nil
    }
    // ...
}
```
**Verification:** Complex queries rejected

---

### Task 3.3: Fix bcrypt Cost Mismatch
**Files:** `internal/api/users/handler.go:184,424,509`, `internal/storage/sqlite.go:128`, `cmd/blazectl/cmd/user.go:192,286`
**Issue:** Using `bcrypt.DefaultCost` (10) but docs say 12.
**Fix:**
```go
const bcryptCost = 12

// Replace all occurrences:
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
```
**Verification:** All password hashes use cost 12

---

### Task 3.4: Fix Session Fixation Risk
**File:** `internal/web/handlers/auth.go:87-102`
**Issue:** Old session not invalidated before creating new one.
**Fix:**
```go
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    // ... authentication ...

    // Delete any existing session first
    if cookie, err := r.Cookie("session_id"); err == nil {
        h.sessions.Delete(cookie.Value)
    }

    // Create new session
    sess, err := h.sessions.CreateWithTTL(...)
}
```
**Verification:** Old session invalidated on login

---

### Task 3.5: Add Rate Limiting to Export Endpoints
**File:** `internal/web/handlers/logs.go:249-345`
**Issue:** Export allows 10K records with no rate limit.
**Fix:**
```go
// Add to router or handler
func (h *Handler) ExportLogs(w http.ResponseWriter, r *http.Request) {
    if !h.exportLimiter.Allow() {
        http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
        return
    }
    // ...
}
```
**Verification:** Rapid export requests rate limited

---

### Task 3.6: Add Rate Limiting to gRPC Registration
**File:** `internal/server/handler.go:94-134`
**Issue:** No rate limit on agent registration.
**Fix:**
```go
var registrationLimiter = rate.NewLimiter(10, 50)  // 10/sec, burst 50

func (h *Handler) Register(ctx context.Context, req *blazelogv1.RegisterRequest) (*blazelogv1.RegisterResponse, error) {
    if !registrationLimiter.Allow() {
        return nil, status.Error(codes.ResourceExhausted, "registration rate limit exceeded")
    }
    // ...
}
```
**Verification:** Rapid registrations rate limited

---

### Task 3.7: Add JSON Parsing Size Limits
**File:** `internal/parser/custom.go:207`, `internal/parser/magento.go:142,163`, `internal/parser/prestashop.go:139,160`
**Issue:** Unbounded JSON parsing in parsers.
**Fix:**
```go
const maxJSONSize = 1024 * 1024  // 1MB

func (p *CustomParser) parseJSON(line string) (*models.LogEntry, error) {
    if len(line) > maxJSONSize {
        return nil, fmt.Errorf("%w: JSON too large", ErrInvalidFormat)
    }
    // ...
}
```
**Verification:** Large JSON logs rejected gracefully

---

### Task 3.8: Fix PolicyWarn Accepts All Keys
**File:** `internal/ssh/` (verify exact location)
**Issue:** PolicyWarn accepts unknown host keys without storage.
**Fix:** Log warning and store key on first connection
**Verification:** Unknown host key logged, subsequent connections verified

---

### Task 3.9: Fix SMTP Credentials in Memory
**File:** `internal/notifier/email.go:16-23`
**Issue:** Password stored as plain string, not zeroed.
**Fix:**
```go
type EmailConfig struct {
    // ...
    password []byte  // Use byte slice for secure zeroing
}

func (n *EmailNotifier) Close() error {
    // Zero password
    for i := range n.password {
        n.password[i] = 0
    }
    return nil
}
```
**Verification:** Password zeroed after use

---

### Task 3.10: Add SSE Stream Timeout
**File:** `internal/web/handlers/logs.go:379-494`
**Issue:** SSE streams have no max duration.
**Fix:**
```go
const maxStreamDuration = 30 * time.Minute

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
    streamTimeout := time.After(maxStreamDuration)
    for {
        select {
        case <-ctx.Done():
            return
        case <-streamTimeout:
            return
        case <-ticker.C:
            // ...
        }
    }
}
```
**Verification:** Streams close after 30 minutes

---

### Task 3.11: Handle JSON Marshal Errors (Insert)
**File:** `internal/storage/clickhouse.go:283-284`
**Issue:** JSON marshal errors silently ignored.
**Fix:**
```go
fieldsJSON, err := json.Marshal(entry.Fields)
if err != nil {
    log.Printf("failed to marshal fields: %v", err)
    fieldsJSON = []byte("{}")
}
```
**Verification:** Marshal errors logged

---

### Task 3.12: Handle JSON Unmarshal Errors (Query)
**File:** `internal/storage/clickhouse.go:364-365`
**Issue:** JSON unmarshal errors silently ignored.
**Fix:**
```go
if fieldsJSON != "" {
    if err := json.Unmarshal([]byte(fieldsJSON), &entry.Fields); err != nil {
        log.Printf("failed to unmarshal fields: %v", err)
    }
}
```
**Verification:** Unmarshal errors logged

---

### Task 3.13: Handle JSON Encode Errors in API
**Files:** All API handlers (44+ locations)
**Issue:** `json.NewEncoder(w).Encode()` errors ignored.
**Fix:**
```go
if err := json.NewEncoder(w).Encode(resp); err != nil {
    log.Printf("json encode error: %v", err)
}
```
**Verification:** Encode errors logged

---

### Task 3.14: Validate Project Existence on Alert Create
**File:** `internal/api/alerts/handler.go:159-226`
**Issue:** ProjectID not validated on alert creation.
**Fix:**
```go
if req.ProjectID != "" {
    project, err := h.storage.Projects().GetByID(ctx, req.ProjectID)
    if err != nil || project == nil {
        jsonError(w, 400, "INVALID_PROJECT", "project does not exist")
        return
    }
}
```
**Verification:** Alert with invalid project_id rejected

---

### Task 3.15: Fix Token Rotation Error Logging
**File:** `internal/api/auth/tokens.go:72`
**Issue:** Revocation errors silently ignored.
**Fix:**
```go
if err := s.RevokeRefreshToken(ctx, oldPlainToken); err != nil {
    log.Printf("failed to revoke old token during rotation: %v", err)
}
```
**Verification:** Revocation errors logged

---

## Phase 4: LOW Issues (12 tasks)

### Task 4.1: Fix rand.Read Error Ignored
**File:** `internal/storage/sqlite.go:191-193`
**Fix:**
```go
if _, err := rand.Read(b); err != nil {
    return "", fmt.Errorf("failed to generate random password: %w", err)
}
```

---

### Task 4.2: Fix Double-Close on stopCh (Handler)
**File:** `internal/server/handler.go:89-91`
**Fix:** Add `sync.Once` or atomic flag to prevent double close

---

### Task 4.3: Add Keepalive Configuration
**File:** `internal/server/server.go:90`
**Fix:**
```go
import "google.golang.org/grpc/keepalive"

opts = append(opts,
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle: 15 * time.Minute,
        Time:              5 * time.Minute,
        Timeout:           1 * time.Minute,
    }),
)
```

---

### Task 4.4: Validate Sequence Numbers
**File:** `internal/server/handler.go:167,182`
**Fix:** Track and validate sequence numbers per agent for ordering

---

### Task 4.5: Validate ProjectID on Batch
**File:** `internal/server/processor.go:70`
**Fix:** Validate project exists before storing logs

---

### Task 4.6: Fix Connection Not Closed on Ping Fail
**File:** `internal/storage/clickhouse.go:91-103`
**Fix:**
```go
if err := db.PingContext(ctx); err != nil {
    db.Close()
    return fmt.Errorf("ping clickhouse: %w", err)
}
```

---

### Task 4.7: Fix TOCTOU in LogBuffer
**File:** `internal/storage/log_buffer.go:77-117`
**Note:** Document the benign nature of this race

---

### Task 4.8: Fix Operator Validation (Left Side Only)
**File:** `internal/query/dsl.go:134-142`
**Fix:** Check both left AND right sides for identifiers

---

### Task 4.9: Return Error for Unknown Operators
**File:** `internal/query/sql_builder.go:352-368`
**Fix:**
```go
default:
    return "", fmt.Errorf("unknown operator: %s", op)
```

---

### Task 4.10: Add Lockout Persistence
**File:** `internal/api/auth/lockout.go`
**Note:** Document limitation for single-instance deployments or implement DB persistence

---

### Task 4.11: Add Input Validation for AgentInfo
**File:** `internal/server/handler.go:94-134`
**Fix:** Validate hostname/name length, sanitize for logging

---

### Task 4.12: Add Size Limits for LogEntry Fields
**File:** `internal/server/processor.go:38-55`
**Fix:** Validate message/raw/source field sizes before storage

---

## Implementation Order

1. **Phase 1 (CRITICAL)** - Immediate priority
   - Security vulnerabilities that could lead to unauthorized access
   - Panics and crashes

2. **Phase 2 (HIGH)** - Next sprint
   - Race conditions and memory issues
   - Resource exhaustion vectors

3. **Phase 3 (MEDIUM)** - Following sprint
   - Defense-in-depth improvements
   - Error handling enhancements

4. **Phase 4 (LOW)** - Backlog
   - Minor improvements
   - Documentation updates

---

## Verification Checklist

After each phase:
- [ ] All tests pass: `make test`
- [ ] Race detector clean: `go test -race ./...`
- [ ] Linter passes: `make lint`
- [ ] Security scan: Review addressed issues
- [ ] Manual testing of affected features

---

## Notes

- Some issues may require architectural decisions (e.g., CSP nonces require template changes)
- Consider impact on existing deployments when changing password hashing cost
- Document any breaking changes for users upgrading
