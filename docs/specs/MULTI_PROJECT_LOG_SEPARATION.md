# Multi-Project Log Separation Specification

**Status:** Draft
**Author:** Claude
**Created:** 2026-01-24
**Last Updated:** 2026-01-24

---

## 1. Executive Summary

BlazeLog currently stores all logs in a single ClickHouse table without project association. While the system has full project CRUD support (including user roles and membership), logs are not linked to projects, making multi-tenant log isolation impossible.

This specification defines the changes required to implement project-scoped log separation, enabling:
- Logs to be associated with specific projects
- Project-based filtering in queries and UI
- Role-based access control for log data
- Project-aware dashboards and analytics

---

## 2. Current State Analysis

### 2.1 What Works

| Component | Status | Notes |
|-----------|--------|-------|
| Projects table | ✓ Full | CRUD, SQLite storage |
| Project membership | ✓ Full | User roles: admin, operator, viewer |
| Alerts | ✓ Has project_id | FK to projects table |
| Connections | ✓ Has project_id | FK to projects table |
| Alert History | ✓ Has project_id | Tracking per project |

### 2.2 What's Missing

| Component | Status | Gap |
|-----------|--------|-----|
| ClickHouse logs table | ✗ | No `project_id` column |
| LogRecord struct | ✗ | No `ProjectID` field |
| LogFilter struct | ✗ | No `ProjectID` filter |
| AggregationFilter | ✗ | No `ProjectID` filter |
| Agent config | ✗ | No project assignment |
| Proto messages | ✗ | No project_id in LogBatch/LogEntry |
| Logs API | ✗ | No project filtering parameter |
| Logs UI | ✗ | No project selector |
| Materialized views | ✗ | No project dimension |

---

## 3. Architecture Design

### 3.1 Project-Log Association Model

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Project   │────<│    Agent    │────<│     Log     │
│  (SQLite)   │     │  (Config)   │     │(ClickHouse) │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                    │
       │            project_id            project_id
       │                   │                    │
       └───────────────────┴────────────────────┘
```

**Design Decision:** Logs inherit project_id from the agent that sends them. This is simpler than per-source project assignment and matches common deployment patterns.

### 3.2 Data Flow

```
Agent Config (project_id)
    │
    ▼
Agent Registration (project_id in RegisterRequest)
    │
    ▼
LogBatch gRPC (project_id in message)
    │
    ▼
Server receives batch → validates project exists
    │
    ▼
ClickHouse INSERT (project_id column)
    │
    ▼
Query with project_id filter → RBAC check → results
```

---

## 4. Detailed Changes

### 4.1 Database Schema Changes

#### 4.1.1 ClickHouse Logs Table

**File:** `internal/storage/clickhouse.go`

**Current Schema (lines 120-144):**
```sql
CREATE TABLE IF NOT EXISTS logs (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(3, 'UTC'),
    level LowCardinality(String),
    message String,
    source String,
    type LowCardinality(String),
    raw String,
    agent_id String,
    file_path String,
    line_number Int64,
    fields String,
    labels String,
    http_status UInt16 DEFAULT 0,
    http_method LowCardinality(String) DEFAULT '',
    uri String DEFAULT '',
    _date Date DEFAULT toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(_date)
ORDER BY (agent_id, type, level, timestamp, id)
TTL _date + INTERVAL %d DAY DELETE
SETTINGS index_granularity = 8192
```

**New Schema:**
```sql
CREATE TABLE IF NOT EXISTS logs (
    id UUID DEFAULT generateUUIDv4(),
    project_id String,                          -- NEW: Project identifier
    timestamp DateTime64(3, 'UTC'),
    level LowCardinality(String),
    message String,
    source String,
    type LowCardinality(String),
    raw String,
    agent_id String,
    file_path String,
    line_number Int64,
    fields String,
    labels String,
    http_status UInt16 DEFAULT 0,
    http_method LowCardinality(String) DEFAULT '',
    uri String DEFAULT '',
    _date Date DEFAULT toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(_date)
ORDER BY (project_id, agent_id, type, level, timestamp, id)  -- project_id first
TTL _date + INTERVAL %d DAY DELETE
SETTINGS index_granularity = 8192
```

**New Index:**
```sql
ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_project_id project_id TYPE bloom_filter(0.01) GRANULARITY 4
```

#### 4.1.2 Migration Strategy

Since ClickHouse doesn't support traditional ALTER TABLE ADD COLUMN with defaults for existing data, implement a migration:

```sql
-- Option A: Add column with default (new logs only)
ALTER TABLE logs ADD COLUMN IF NOT EXISTS project_id String DEFAULT '' AFTER id;

-- Option B: For existing data, update with a default project
-- This should be optional/configurable
ALTER TABLE logs UPDATE project_id = 'default' WHERE project_id = '';
```

**Recommendation:** Use Option A. Existing logs without project_id remain accessible but are filtered out when a specific project is selected. Add a "All Projects" / "Unassigned" option in UI.

#### 4.1.3 Materialized Views Update

**File:** `internal/storage/clickhouse.go` (lines 169-213)

Update all materialized views to include `project_id`:

```sql
-- logs_hourly_errors_mv
CREATE MATERIALIZED VIEW IF NOT EXISTS logs_hourly_errors_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (project_id, agent_id, type, level, hour)  -- Add project_id
AS SELECT
    project_id,                                      -- Add project_id
    agent_id,
    type,
    level,
    toStartOfHour(timestamp) AS hour,
    count() AS count
FROM logs
WHERE level IN ('error', 'fatal', 'warning')
GROUP BY project_id, agent_id, type, level, hour    -- Add project_id

-- logs_daily_volume_mv
CREATE MATERIALIZED VIEW IF NOT EXISTS logs_daily_volume_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (project_id, agent_id, type, day)          -- Add project_id
AS SELECT
    project_id,                                      -- Add project_id
    agent_id,
    type,
    toDate(timestamp) AS day,
    count() AS total_count,
    countIf(level = 'error') AS error_count,
    countIf(level = 'fatal') AS fatal_count,
    countIf(level = 'warning') AS warning_count
FROM logs
GROUP BY project_id, agent_id, type, day            -- Add project_id

-- logs_http_stats_mv
CREATE MATERIALIZED VIEW IF NOT EXISTS logs_http_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (project_id, agent_id, hour, http_status)  -- Add project_id
AS SELECT
    project_id,                                      -- Add project_id
    agent_id,
    toStartOfHour(timestamp) AS hour,
    http_status,
    count() AS count
FROM logs
WHERE http_status > 0
GROUP BY project_id, agent_id, hour, http_status    -- Add project_id
```

---

### 4.2 Storage Layer Changes

#### 4.2.1 LogRecord Struct

**File:** `internal/storage/log_storage.go`

**Current (lines 66-108):**
```go
type LogRecord struct {
    ID         string
    Timestamp  time.Time
    Level      string
    Message    string
    Source     string
    Type       string
    Raw        string
    AgentID    string
    FilePath   string
    LineNumber int64
    Fields     map[string]interface{}
    Labels     map[string]string
    HTTPStatus int
    HTTPMethod string
    URI        string
}
```

**New:**
```go
type LogRecord struct {
    ID         string
    ProjectID  string    // NEW: Project this log belongs to
    Timestamp  time.Time
    Level      string
    Message    string
    Source     string
    Type       string
    Raw        string
    AgentID    string
    FilePath   string
    LineNumber int64
    Fields     map[string]interface{}
    Labels     map[string]string
    HTTPStatus int
    HTTPMethod string
    URI        string
}
```

#### 4.2.2 LogFilter Struct

**File:** `internal/storage/log_storage.go`

**Current (lines 110-141):**
```go
type LogFilter struct {
    StartTime time.Time
    EndTime   time.Time
    AgentID   string
    Level     string
    Levels    []string
    Type      string
    Types     []string
    Source    string
    FilePath  string
    MessageContains string
    SearchMode      SearchMode
    Limit     int
    Offset    int
    OrderBy   string
    OrderDesc bool
    FilterExpr string
    FilterSQL  string
    FilterArgs []any
}
```

**New:**
```go
type LogFilter struct {
    // Project filter (NEW)
    ProjectID  string   // Single project filter
    ProjectIDs []string // Multiple projects filter (for users with access to multiple)

    // Time range
    StartTime time.Time
    EndTime   time.Time

    // Existing filters...
    AgentID         string
    Level           string
    Levels          []string
    Type            string
    Types           []string
    Source          string
    FilePath        string
    MessageContains string
    SearchMode      SearchMode
    Limit           int
    Offset          int
    OrderBy         string
    OrderDesc       bool
    FilterExpr      string
    FilterSQL       string
    FilterArgs      []any
}
```

#### 4.2.3 AggregationFilter Struct

**File:** `internal/storage/log_storage.go`

**Current (lines 155-161):**
```go
type AggregationFilter struct {
    StartTime time.Time
    EndTime   time.Time
    AgentID   string
    Type      string
}
```

**New:**
```go
type AggregationFilter struct {
    ProjectID  string   // NEW: Single project filter
    ProjectIDs []string // NEW: Multiple projects filter
    StartTime  time.Time
    EndTime    time.Time
    AgentID    string
    Type       string
}
```

#### 4.2.4 Query Builder Updates

**File:** `internal/storage/clickhouse.go`

Update `InsertBatch` (lines 241-299):
```go
stmt, err := tx.PrepareContext(ctx, `
    INSERT INTO logs (
        id, project_id, timestamp, level, message, source, type, raw,
        agent_id, file_path, line_number, fields, labels,
        http_status, http_method, uri
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
// ... add entry.ProjectID to ExecContext args
```

Update `buildQuery` (lines 432-588):
```go
// Add project filter in WHERE clause builder
if filter.ProjectID != "" {
    conditions = append(conditions, "project_id = ?")
    args = append(args, filter.ProjectID)
}
if len(filter.ProjectIDs) > 0 {
    placeholders := make([]string, len(filter.ProjectIDs))
    for i, p := range filter.ProjectIDs {
        placeholders[i] = "?"
        args = append(args, p)
    }
    conditions = append(conditions, fmt.Sprintf("project_id IN (%s)", strings.Join(placeholders, ", ")))
}
```

Update `buildAggregationWhere` (lines 763-785):
```go
func (r *clickhouseLogRepo) buildAggregationWhere(filter *AggregationFilter) ([]interface{}, string) {
    var conditions []string
    var args []interface{}

    // NEW: Project filter
    if filter.ProjectID != "" {
        conditions = append(conditions, "project_id = ?")
        args = append(args, filter.ProjectID)
    }
    if len(filter.ProjectIDs) > 0 {
        placeholders := make([]string, len(filter.ProjectIDs))
        for i, p := range filter.ProjectIDs {
            placeholders[i] = "?"
            args = append(args, p)
        }
        conditions = append(conditions, fmt.Sprintf("project_id IN (%s)", strings.Join(placeholders, ", ")))
    }

    // Existing filters...
    if !filter.StartTime.IsZero() {
        conditions = append(conditions, "timestamp >= ?")
        args = append(args, filter.StartTime)
    }
    // ...
}
```

---

### 4.3 Protocol Buffer Changes

#### 4.3.1 LogEntry Message

**File:** `proto/blazelog/v1/log.proto`

**Current (lines 11-42):**
```protobuf
message LogEntry {
  google.protobuf.Timestamp timestamp = 1;
  LogLevel level = 2;
  string message = 3;
  string source = 4;
  LogType type = 5;
  string raw = 6;
  google.protobuf.Struct fields = 7;
  map<string, string> labels = 8;
  int64 line_number = 9;
  string file_path = 10;
}
```

**New:**
```protobuf
message LogEntry {
  google.protobuf.Timestamp timestamp = 1;
  LogLevel level = 2;
  string message = 3;
  string source = 4;
  LogType type = 5;
  string raw = 6;
  google.protobuf.Struct fields = 7;
  map<string, string> labels = 8;
  int64 line_number = 9;
  string file_path = 10;
  // Field 11 reserved for future use
  string project_id = 12;  // NEW: Project identifier (inherited from agent)
}
```

#### 4.3.2 LogBatch Message

**File:** `proto/blazelog/v1/log.proto`

**Current (lines 44-54):**
```protobuf
message LogBatch {
  repeated LogEntry entries = 1;
  string agent_id = 2;
  uint64 sequence = 3;
}
```

**New:**
```protobuf
message LogBatch {
  repeated LogEntry entries = 1;
  string agent_id = 2;
  uint64 sequence = 3;
  string project_id = 4;  // NEW: Project for all entries in batch
}
```

#### 4.3.3 Agent Registration

**File:** `proto/blazelog/v1/agent.proto`

Update `RegisterRequest`:
```protobuf
message RegisterRequest {
  AgentInfo agent = 1;
  string project_id = 2;  // NEW: Project this agent belongs to
}
```

Update `AgentInfo`:
```protobuf
message AgentInfo {
  string agent_id = 1;
  string name = 2;
  string hostname = 3;
  map<string, string> labels = 4;
  repeated string sources = 5;
  string project_id = 6;  // NEW: Project association
}
```

---

### 4.4 Agent Configuration Changes

#### 4.4.1 Config Struct

**File:** `cmd/agent/config.go`

**Current AgentConfig (lines 43-48):**
```go
type AgentConfig struct {
    ID            string        `yaml:"id"`
    Name          string        `yaml:"name"`
    BatchSize     int           `yaml:"batch_size"`
    FlushInterval time.Duration `yaml:"flush_interval"`
}
```

**New:**
```go
type AgentConfig struct {
    ID            string        `yaml:"id"`
    Name          string        `yaml:"name"`
    ProjectID     string        `yaml:"project_id"`     // NEW: Project assignment
    BatchSize     int           `yaml:"batch_size"`
    FlushInterval time.Duration `yaml:"flush_interval"`
}
```

#### 4.4.2 Config Validation

**File:** `cmd/agent/config.go`

Update `Validate()`:
```go
func (c *Config) Validate() error {
    if c.Server.Address == "" {
        return fmt.Errorf("server.address is required")
    }
    // NEW: Project ID validation (optional but recommended)
    if c.Agent.ProjectID == "" {
        // Warning log instead of error for backward compatibility
        log.Println("warning: agent.project_id not set, logs will be unassigned")
    }
    // ... existing validations
}
```

#### 4.4.3 Example Agent Config

**File:** `configs/agent.yaml`

```yaml
server:
  address: "localhost:9090"
  tls:
    enabled: true
    cert_file: "/etc/blazelog/certs/agent.crt"
    key_file: "/etc/blazelog/certs/agent.key"
    ca_file: "/etc/blazelog/certs/ca.crt"

agent:
  name: "production-web-01"
  project_id: "proj_abc123"           # NEW: Assign to project
  batch_size: 100
  flush_interval: 1s

sources:
  - name: "nginx-access"
    type: "nginx"
    path: "/var/log/nginx/access.log"

labels:
  environment: "production"
  datacenter: "us-east-1"
```

---

### 4.5 Server-Side Changes

#### 4.5.1 gRPC Handler Updates

**File:** `internal/server/grpc_handler.go` (or equivalent)

Update log batch handling:
```go
func (s *Server) StreamLogs(stream pb.LogService_StreamLogsServer) error {
    for {
        batch, err := stream.Recv()
        if err != nil {
            return err
        }

        // Validate project exists (NEW)
        if batch.ProjectId != "" {
            project, err := s.storage.Projects().GetByID(ctx, batch.ProjectId)
            if err != nil || project == nil {
                return status.Errorf(codes.InvalidArgument, "invalid project_id: %s", batch.ProjectId)
            }
        }

        // Convert to LogRecords with project_id
        records := make([]*storage.LogRecord, len(batch.Entries))
        for i, entry := range batch.Entries {
            records[i] = &storage.LogRecord{
                ProjectID: batch.ProjectId,  // NEW: Set project from batch
                // ... other fields
            }
        }

        // Insert batch
        if err := s.logStorage.Logs().InsertBatch(ctx, records); err != nil {
            return err
        }
    }
}
```

#### 4.5.2 Agent Registration Handler

Update agent registration to store project association:
```go
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
    // Validate project if specified
    if req.ProjectId != "" {
        project, err := s.storage.Projects().GetByID(ctx, req.ProjectId)
        if err != nil || project == nil {
            return nil, status.Errorf(codes.InvalidArgument, "invalid project_id")
        }
    }

    // Store agent with project association (may need new agents table)
    // ...
}
```

---

### 4.6 API Layer Changes

#### 4.6.1 Logs Handler

**File:** `internal/api/logs/handler.go`

Update query parameters:
```go
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    filter := &storage.LogFilter{}

    // NEW: Project filter
    if projectID := r.URL.Query().Get("project_id"); projectID != "" {
        filter.ProjectID = projectID
    }
    if projectIDs := r.URL.Query().Get("project_ids"); projectIDs != "" {
        filter.ProjectIDs = strings.Split(projectIDs, ",")
    }

    // RBAC check (NEW)
    user := auth.GetUser(r.Context())
    if filter.ProjectID != "" {
        if !h.canAccessProject(user, filter.ProjectID) {
            jsonError(w, http.StatusForbidden, "FORBIDDEN", "no access to project")
            return
        }
    }

    // Existing filter parsing...
}
```

#### 4.6.2 Stats/Stream Endpoints

Apply same pattern to:
- `GET /api/v1/logs/stats`
- `GET /api/v1/logs/stream` (WebSocket)
- `GET /api/v1/dashboard/stats`

---

### 4.7 Web UI Changes

#### 4.7.1 Logs Page Filter

**File:** `internal/web/templates/pages/logs.templ`

Add project selector to filter panel:
```html
<!-- Project Filter (NEW) -->
<div class="mb-4">
  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
    Project
  </label>
  <select
    name="project_id"
    x-model="filters.projectId"
    @change="applyFilters()"
    class="w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700"
  >
    <option value="">All Projects</option>
    <option value="__unassigned__">Unassigned Logs</option>
    for _, project := range projects {
      <option value={ project.ID }>{ project.Name }</option>
    }
  </select>
</div>
```

#### 4.7.2 Logs Handler

**File:** `internal/web/handlers/logs.go`

Pass projects to template:
```go
func (h *Handler) LogsPage(w http.ResponseWriter, r *http.Request) {
    user := auth.GetUser(r.Context())

    // Get user's accessible projects
    projects, err := h.storage.Projects().GetProjectsForUser(r.Context(), user.ID)
    if err != nil {
        // handle error
    }

    // Render with projects
    templates.LogsPage(projects).Render(r.Context(), w)
}
```

#### 4.7.3 Dashboard Updates

Update dashboard to show project-specific stats or add project selector.

---

## 5. Migration Plan

### 5.1 Phase 1: Database Schema (Non-Breaking)

1. Add `project_id` column to ClickHouse with default empty string
2. Add bloom filter index on `project_id`
3. No code changes yet - existing functionality works

```sql
ALTER TABLE logs ADD COLUMN IF NOT EXISTS project_id String DEFAULT '' AFTER id;
ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_project_id project_id TYPE bloom_filter(0.01) GRANULARITY 4;
```

### 5.2 Phase 2: Storage Layer

1. Update `LogRecord` struct with `ProjectID`
2. Update `LogFilter` and `AggregationFilter`
3. Update query builders
4. Update INSERT statements
5. All existing queries continue to work (project_id filter is optional)

### 5.3 Phase 3: Protocol & Agent

1. Update proto files
2. Run `make proto` to regenerate
3. Update agent config struct
4. Update agent to send project_id in batches
5. Server accepts but doesn't require project_id (backward compatible)

### 5.4 Phase 4: API Layer

1. Add `project_id` query parameter to logs endpoints
2. Add RBAC checks for project access
3. Existing API calls without project_id continue to work

### 5.5 Phase 5: UI Layer

1. Add project selector to logs page
2. Update dashboard for project awareness
3. Add "Unassigned" filter option

### 5.6 Phase 6: Materialized Views (Optional)

1. Drop existing materialized views
2. Create new views with project_id
3. Backfill from logs table

---

## 6. API Reference

### 6.1 Updated Endpoints

#### GET /api/v1/logs

**New Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| project_id | string | Filter by single project ID |
| project_ids | string | Comma-separated list of project IDs |

**Example:**
```
GET /api/v1/logs?project_id=proj_abc123&level=error&limit=50
```

#### GET /api/v1/logs/stats

**New Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| project_id | string | Get stats for specific project |

#### GET /api/v1/logs/stream (WebSocket)

**New Connection Parameters:**
```
ws://host/api/v1/logs/stream?project_id=proj_abc123
```

---

## 7. Security Considerations

### 7.1 Access Control

- Users can only query logs for projects they have membership in
- Admin users can query all projects
- API validates project access before executing queries
- WebSocket streams filtered by project access

### 7.2 Data Isolation

- Project ID is included in all log queries by default for non-admin users
- Empty project_id filter returns only "unassigned" logs (legacy)
- No cross-project data leakage in aggregations

### 7.3 Audit Trail

- Log access requests should be audited with user and project context
- Failed access attempts should be logged

---

## 8. Testing Requirements

### 8.1 Unit Tests

- LogRecord serialization with project_id
- LogFilter query building with project filters
- AggregationFilter query building

### 8.2 Integration Tests

- ClickHouse queries with project filtering
- Agent registration with project
- Log batch insertion with project

### 8.3 E2E Tests

- Project selector in UI
- Log filtering by project
- RBAC enforcement

---

## 9. Backward Compatibility

| Component | Compatibility | Notes |
|-----------|---------------|-------|
| Existing logs | ✓ | project_id defaults to empty string |
| Existing agents | ✓ | project_id optional in config |
| Existing API calls | ✓ | project_id filter optional |
| Existing UI | ✓ | Shows all logs if no project selected |

---

## 10. Open Questions

1. **Default Project:** Should there be a "default" project for logs without assignment?
2. **Project Deletion:** What happens to logs when a project is deleted? (Orphan or cascade?)
3. **Agent Reassignment:** Can an agent switch projects? What happens to historical logs?
4. **Multi-Project Agents:** Should one agent be able to send logs to multiple projects?

---

## 11. Files to Modify Summary

| File | Changes |
|------|---------|
| `internal/storage/clickhouse.go` | Schema, indexes, queries, materialized views |
| `internal/storage/log_storage.go` | LogRecord, LogFilter, AggregationFilter structs |
| `proto/blazelog/v1/log.proto` | LogEntry, LogBatch messages |
| `proto/blazelog/v1/agent.proto` | RegisterRequest, AgentInfo messages |
| `cmd/agent/config.go` | AgentConfig struct |
| `internal/server/grpc_handler.go` | Batch handling, agent registration |
| `internal/api/logs/handler.go` | Query parameter, RBAC |
| `internal/web/templates/pages/logs.templ` | Project selector |
| `internal/web/handlers/logs.go` | Pass projects to template |
| `configs/agent.yaml` | Example config |

---

## 12. Estimated Effort

| Phase | Scope |
|-------|-------|
| Phase 1: Database | Schema changes, migration |
| Phase 2: Storage | Struct updates, query builders |
| Phase 3: Protocol | Proto changes, agent config |
| Phase 4: API | Endpoint updates, RBAC |
| Phase 5: UI | Project selector, filtering |
| Phase 6: Views | Materialized view recreation |

---

## 13. Success Criteria

1. Logs can be filtered by project in API and UI
2. Agents can be configured with a project ID
3. New logs are stored with project association
4. Existing logs remain accessible (as "unassigned")
5. RBAC prevents cross-project access
6. Dashboard shows project-specific metrics
7. All existing functionality continues to work
