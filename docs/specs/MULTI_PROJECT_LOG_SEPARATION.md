# Multi-Project Log Separation Specification

**Status:** Draft
**Author:** Claude
**Created:** 2026-01-24
**Last Updated:** 2026-01-24
**Version:** 1.1

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

## 7. Access Control Model (Role-Based Fallback)

This section defines how users access logs, alerts, connections, and other project-scoped data based on their role and project assignments. The model ensures backward compatibility while enabling strict project isolation for users who configure it.

### 7.1 Design Principles

1. **Graceful Degradation**: Users without project assignments still have access (not empty screens)
2. **Backward Compatibility**: Existing deployments work without changes
3. **Progressive Isolation**: Organizations can adopt project scoping incrementally
4. **Admin Override**: Admins always have full visibility

### 7.2 Access Control Matrix

| Role | Project Assignments | Logs Access | Alerts Access | Connections Access | Dashboard |
|------|---------------------|-------------|---------------|-------------------|-----------|
| **Admin** | Any/None | All projects + unassigned | All projects + unassigned | All projects + unassigned | All projects aggregated |
| **Operator** | Yes (N projects) | N projects + unassigned | N projects + unassigned | N projects + unassigned | N projects aggregated |
| **Operator** | No (0 projects) | All (legacy mode)* | All (legacy mode)* | All (legacy mode)* | All (legacy mode)* |
| **Viewer** | Yes (N projects) | N projects only | N projects only (read) | N projects only (read) | N projects aggregated |
| **Viewer** | No (0 projects) | Unassigned only | Unassigned only (read) | Unassigned only (read) | Unassigned only |

*\*Legacy mode: Show warning banner encouraging project assignment*

### 7.3 Detailed Access Rules

#### 7.3.1 Admin Role
```
CAN ACCESS:
  - All logs (any project_id, including empty/unassigned)
  - All alerts (any project_id)
  - All connections (any project_id)
  - All dashboard stats (aggregated across all projects)
  - Project management (CRUD)
  - User management (CRUD)

NOTES:
  - Project assignment is optional for admins
  - Admins can filter by specific project but see all by default
```

#### 7.3.2 Operator Role (With Project Assignments)
```
CAN ACCESS:
  - Logs where project_id IN (assigned_projects) OR project_id = '' (unassigned)
  - Alerts where project_id IN (assigned_projects) OR project_id = '' (unassigned)
  - Connections where project_id IN (assigned_projects) OR project_id = '' (unassigned)
  - Dashboard stats aggregated from assigned projects + unassigned

CAN MODIFY:
  - Create/update/delete alerts for assigned projects
  - Create/update/delete connections for assigned projects

CANNOT ACCESS:
  - Logs/alerts/connections from other projects
  - Project management
  - User management
```

#### 7.3.3 Operator Role (No Project Assignments - Legacy Mode)
```
CAN ACCESS:
  - All logs (legacy behavior, backward compatible)
  - All alerts (legacy behavior)
  - All connections (legacy behavior)
  - All dashboard stats

UI BEHAVIOR:
  - Show warning banner: "You are not assigned to any projects.
    Contact your admin to get project access for better data isolation."
  - Encourage transition to project-based access

NOTES:
  - This mode exists for backward compatibility
  - New deployments should assign operators to projects
```

#### 7.3.4 Viewer Role (With Project Assignments)
```
CAN ACCESS:
  - Logs where project_id IN (assigned_projects) — NO unassigned
  - Alerts where project_id IN (assigned_projects) (read-only)
  - Connections where project_id IN (assigned_projects) (read-only)
  - Dashboard stats for assigned projects only

CANNOT ACCESS:
  - Unassigned logs (stricter than operator for data hygiene)
  - Create/modify alerts or connections
  - Project management
  - User management
```

#### 7.3.5 Viewer Role (No Project Assignments)
```
CAN ACCESS:
  - Unassigned logs only (project_id = '')
  - Unassigned alerts only (read-only)
  - Unassigned connections only (read-only)
  - Dashboard stats for unassigned data only

NOTES:
  - Most restricted access level
  - Intended as a "waiting room" state until assigned to projects
```

### 7.4 Implementation Details

#### 7.4.1 Access Check Function

**File:** `internal/auth/project_access.go` (new file)

```go
package auth

import "context"

// ProjectAccess defines what projects a user can access
type ProjectAccess struct {
    AllProjects      bool     // Admin override - can see everything
    ProjectIDs       []string // Specific project IDs user can access
    IncludeUnassigned bool    // Can see logs with empty project_id
    LegacyMode       bool     // Operator with no assignments (show warning)
}

// GetProjectAccess returns the project access rules for a user
func GetProjectAccess(ctx context.Context, user *User, storage Storage) (*ProjectAccess, error) {
    // Admins can access everything
    if user.Role == RoleAdmin {
        return &ProjectAccess{
            AllProjects:       true,
            IncludeUnassigned: true,
        }, nil
    }

    // Get user's project assignments
    projects, err := storage.Projects().GetProjectsForUser(ctx, user.ID)
    if err != nil {
        return nil, err
    }

    projectIDs := make([]string, len(projects))
    for i, p := range projects {
        projectIDs[i] = p.ID
    }

    // Operator with no assignments = legacy mode (full access + warning)
    if user.Role == RoleOperator && len(projectIDs) == 0 {
        return &ProjectAccess{
            AllProjects:       true,
            IncludeUnassigned: true,
            LegacyMode:        true, // Triggers UI warning
        }, nil
    }

    // Operator with assignments = assigned projects + unassigned
    if user.Role == RoleOperator {
        return &ProjectAccess{
            ProjectIDs:        projectIDs,
            IncludeUnassigned: true,
        }, nil
    }

    // Viewer with assignments = assigned projects only (no unassigned)
    if user.Role == RoleViewer && len(projectIDs) > 0 {
        return &ProjectAccess{
            ProjectIDs:        projectIDs,
            IncludeUnassigned: false,
        }, nil
    }

    // Viewer with no assignments = unassigned only
    return &ProjectAccess{
        ProjectIDs:        []string{},
        IncludeUnassigned: true, // Only unassigned visible
    }, nil
}

// CanAccessProject checks if user can access a specific project
func (pa *ProjectAccess) CanAccessProject(projectID string) bool {
    if pa.AllProjects {
        return true
    }
    if projectID == "" {
        return pa.IncludeUnassigned
    }
    for _, id := range pa.ProjectIDs {
        if id == projectID {
            return true
        }
    }
    return false
}
```

#### 7.4.2 Query Filter Application

**File:** `internal/api/logs/handler.go`

```go
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
    user := auth.GetUserFromContext(r.Context())

    // Get user's project access rules
    access, err := auth.GetProjectAccess(r.Context(), user, h.storage)
    if err != nil {
        jsonError(w, 500, "INTERNAL_ERROR", "failed to get project access")
        return
    }

    filter := &storage.LogFilter{}

    // Apply project access restrictions
    if !access.AllProjects {
        // User can only see specific projects
        filter.ProjectIDs = access.ProjectIDs
        filter.IncludeUnassigned = access.IncludeUnassigned
    }

    // User-requested project filter (must be subset of allowed)
    if requestedProject := r.URL.Query().Get("project_id"); requestedProject != "" {
        if !access.CanAccessProject(requestedProject) {
            jsonError(w, 403, "FORBIDDEN", "no access to project")
            return
        }
        filter.ProjectID = requestedProject
        filter.ProjectIDs = nil // Override with specific project
    }

    // ... rest of query handling
}
```

#### 7.4.3 Updated LogFilter Struct

**File:** `internal/storage/log_storage.go`

```go
type LogFilter struct {
    // Project access control (NEW)
    ProjectID         string   // Filter by single project
    ProjectIDs        []string // Filter by multiple projects (user's accessible projects)
    IncludeUnassigned bool     // Include logs where project_id = ''

    // ... existing fields
}
```

#### 7.4.4 Query Builder Update

**File:** `internal/storage/clickhouse.go`

```go
func (r *clickhouseLogRepo) buildProjectFilter(filter *LogFilter) (string, []interface{}) {
    var conditions []string
    var args []interface{}

    // Specific single project filter
    if filter.ProjectID != "" {
        return "project_id = ?", []interface{}{filter.ProjectID}
    }

    // Multiple projects filter (user's accessible projects)
    if len(filter.ProjectIDs) > 0 {
        placeholders := make([]string, len(filter.ProjectIDs))
        for i, p := range filter.ProjectIDs {
            placeholders[i] = "?"
            args = append(args, p)
        }
        conditions = append(conditions, fmt.Sprintf("project_id IN (%s)", strings.Join(placeholders, ",")))
    }

    // Include unassigned logs
    if filter.IncludeUnassigned {
        conditions = append(conditions, "project_id = ''")
    }

    if len(conditions) == 0 {
        return "", nil // No project filter (admin sees all)
    }

    return "(" + strings.Join(conditions, " OR ") + ")", args
}
```

### 7.5 UI Indicators

#### 7.5.1 Legacy Mode Warning Banner

When an operator is in legacy mode (no project assignments), show a dismissible warning:

```html
<div class="bg-amber-50 border-l-4 border-amber-400 p-4 mb-4">
  <div class="flex">
    <div class="flex-shrink-0">
      <svg class="h-5 w-5 text-amber-400" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
      </svg>
    </div>
    <div class="ml-3">
      <p class="text-sm text-amber-700">
        <strong>No project assignment.</strong>
        You're seeing all data. Contact your administrator to get assigned to specific projects for better data isolation.
      </p>
    </div>
  </div>
</div>
```

#### 7.5.2 Project Selector Behavior

The project dropdown in the logs page adapts to user access:

| User State | Dropdown Options |
|------------|------------------|
| Admin | "All Projects" + all project list + "Unassigned" |
| Operator (with projects) | Assigned projects + "Unassigned" |
| Operator (legacy mode) | "All Projects" + all project list + "Unassigned" |
| Viewer (with projects) | Assigned projects only |
| Viewer (no projects) | "Unassigned" only |

### 7.6 Data Isolation Guarantees

1. **Query-Level Enforcement**: All log queries include project_id filter based on user access
2. **No Client-Side Filtering**: Server enforces access, not UI
3. **API Validation**: All endpoints validate project access before returning data
4. **WebSocket Streams**: Real-time log streams filtered by project access
5. **Aggregations**: Dashboard stats only aggregate accessible projects

### 7.7 Audit Trail

- Log access requests should be audited with user and project context
- Failed access attempts (403 responses) should be logged with:
  - User ID
  - Requested project ID
  - User's actual project access list
  - Timestamp
  - Endpoint accessed

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

1. ~~**Default Project:** Should there be a "default" project for logs without assignment?~~
   **RESOLVED:** No default project. Unassigned logs (empty project_id) are accessible based on role. See Section 7.

2. ~~**User Access Without Projects:** What do users see if not assigned to any project?~~
   **RESOLVED:** Role-Based Fallback model implemented. See Section 7.2 for access matrix.

3. **Project Deletion:** What happens to logs when a project is deleted? (Orphan or cascade?)
   - *Recommendation:* Orphan logs (set project_id to empty). They become "unassigned" and visible per access rules.

4. **Agent Reassignment:** Can an agent switch projects? What happens to historical logs?
   - *Recommendation:* Historical logs keep original project_id. New logs use new project_id.

5. **Multi-Project Agents:** Should one agent be able to send logs to multiple projects?
   - *Recommendation:* No. One agent = one project. Use multiple agents for multi-project collection.

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
