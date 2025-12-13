#!/bin/bash
set -e

# BlazeLog GitHub Project Setup Script
# Creates project board, labels, milestones, and issues
# Compatible with bash 3.2+ (macOS default)

REPO="${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"
echo "Setting up GitHub Project for: $REPO"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check gh CLI is installed and authenticated
if ! command -v gh &> /dev/null; then
    log_error "GitHub CLI (gh) is not installed. Install from https://cli.github.com/"
    exit 1
fi

if ! gh auth status &> /dev/null; then
    log_error "GitHub CLI is not authenticated. Run 'gh auth login' first."
    exit 1
fi

# ============================================================================
# CREATE LABELS
# ============================================================================
log_info "Creating labels..."

create_label() {
    local name=$1
    local color=$2
    local desc=$3
    if gh label create "$name" --color "$color" --description "$desc" --repo "$REPO" 2>/dev/null; then
        log_info "Created label: $name"
    else
        log_warn "Label already exists: $name"
    fi
}

create_label "stage-a" "1D76DB" "Stage A - CLI Foundation"
create_label "stage-b" "0E8A16" "Stage B - Real-time & Alerting"
create_label "stage-c" "D93F0B" "Stage C - Distributed Collection"
create_label "stage-d" "FBCA04" "Stage D - SSH Collection"
create_label "stage-e" "6F42C1" "Stage E - Storage"
create_label "stage-f" "E99695" "Stage F - REST API"
create_label "stage-g" "C2E0C6" "Stage G - Web UI"
create_label "stage-h" "BFD4F2" "Stage H - Batch & Production"
create_label "effort-small" "C5DEF5" "Small effort milestone"
create_label "effort-medium" "FEF2C0" "Medium effort milestone"

# ============================================================================
# CREATE GITHUB MILESTONES
# ============================================================================
log_info "Creating GitHub milestones..."

create_milestone() {
    local title=$1
    local desc=$2
    if gh api "repos/$REPO/milestones" --method POST -f title="$title" -f description="$desc" 2>/dev/null; then
        log_info "Created milestone: $title"
    else
        log_warn "Milestone may already exist: $title"
    fi
}

create_milestone "Stage A: CLI Foundation" "Working CLI that parses all log types locally"
create_milestone "Stage B: Real-time & Alerting" "CLI can tail logs and send notifications"
create_milestone "Stage C: Distributed Collection" "Agent-server architecture with secure communication"
create_milestone "Stage D: SSH Collection" "Server can pull logs via SSH"
create_milestone "Stage E: Storage" "Persistent storage with search"
create_milestone "Stage F: REST API" "Full API for web UI"
create_milestone "Stage G: Web UI" "Complete web dashboard"
create_milestone "Stage H: Batch & Production" "Production-ready system"

# ============================================================================
# CREATE ISSUES
# ============================================================================
log_info "Creating issues for all 30 milestones..."

# Stage A Issues
gh issue create --repo "$REPO" --title "[Milestone 1] Project Setup & Parser Interface" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 1: Project Setup & Parser Interface

### Tasks
- [ ] Initialize Go module with \`go mod init\`
- [ ] Create project directory structure
- [ ] Set up Makefile with build targets
- [ ] Implement \`Parser\` interface
- [ ] Create \`LogEntry\` model
- [ ] Set up basic CLI with cobra/urfave

### Deliverable
Empty CLI skeleton with parser interface

### Acceptance Criteria
- [ ] \`go build ./...\` succeeds
- [ ] \`blazelog --help\` shows usage
- [ ] Parser interface defined in \`internal/parser/parser.go\`" && log_info "Created: Milestone 1" || log_warn "Issue may exist: Milestone 1"

gh issue create --repo "$REPO" --title "[Milestone 2] Nginx Parser" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 2: Nginx Parser

### Tasks
- [ ] Research Nginx log formats (combined, custom)
- [ ] Implement Nginx access log parser
- [ ] Implement Nginx error log parser
- [ ] Add CLI command: \`blazelog parse nginx <file>\`
- [ ] Write unit tests

### Deliverable
\`blazelog parse nginx /var/log/nginx/access.log\`

### Acceptance Criteria
- [ ] Parses standard combined format
- [ ] Parses error log format
- [ ] Unit tests pass with >80% coverage
- [ ] Handles malformed lines gracefully" && log_info "Created: Milestone 2" || log_warn "Issue may exist: Milestone 2"

gh issue create --repo "$REPO" --title "[Milestone 3] Apache Parser" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 3: Apache Parser

### Tasks
- [ ] Research Apache log formats (common, combined, error)
- [ ] Implement Apache access log parser
- [ ] Implement Apache error log parser
- [ ] Add CLI command: \`blazelog parse apache <file>\`
- [ ] Write unit tests

### Deliverable
\`blazelog parse apache /var/log/apache2/access.log\`

### Acceptance Criteria
- [ ] Parses common and combined formats
- [ ] Parses error log format
- [ ] Unit tests pass" && log_info "Created: Milestone 3" || log_warn "Issue may exist: Milestone 3"

gh issue create --repo "$REPO" --title "[Milestone 4] Magento Parser" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 4: Magento Parser

### Tasks
- [ ] Research Magento log formats (system.log, exception.log, debug.log)
- [ ] Implement Magento log parser (handles multiline stack traces)
- [ ] Add CLI command: \`blazelog parse magento <file>\`
- [ ] Write unit tests

### Deliverable
\`blazelog parse magento /var/www/magento/var/log/system.log\`

### Acceptance Criteria
- [ ] Parses system.log format
- [ ] Parses exception.log with stack traces
- [ ] Handles multiline entries correctly" && log_info "Created: Milestone 4" || log_warn "Issue may exist: Milestone 4"

gh issue create --repo "$REPO" --title "[Milestone 5] PrestaShop Parser" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 5: PrestaShop Parser

### Tasks
- [ ] Research PrestaShop log formats
- [ ] Implement PrestaShop log parser
- [ ] Add CLI command: \`blazelog parse prestashop <file>\`
- [ ] Write unit tests

### Deliverable
\`blazelog parse prestashop /var/www/prestashop/var/logs/*.log\`

### Acceptance Criteria
- [ ] Parses PrestaShop log format
- [ ] Unit tests pass" && log_info "Created: Milestone 5" || log_warn "Issue may exist: Milestone 5"

gh issue create --repo "$REPO" --title "[Milestone 6] WordPress Parser & Auto-Detection" --label "stage-a" --label "effort-small" --milestone "Stage A: CLI Foundation" --body "## Milestone 6: WordPress Parser & Auto-Detection

### Tasks
- [ ] Research WordPress log formats (debug.log, PHP errors)
- [ ] Implement WordPress log parser
- [ ] Implement auto-detection of log format
- [ ] Add CLI command: \`blazelog parse auto <file>\`
- [ ] Add output formats (JSON, table, plain)
- [ ] Write unit tests

### Deliverable
\`blazelog parse auto /var/log/*.log --format json\`

### Acceptance Criteria
- [ ] Parses WordPress debug.log
- [ ] Auto-detects log format correctly
- [ ] Outputs JSON, table, plain text" && log_info "Created: Milestone 6" || log_warn "Issue may exist: Milestone 6"

# Stage B Issues
gh issue create --repo "$REPO" --title "[Milestone 7] File Tailing" --label "stage-b" --label "effort-small" --milestone "Stage B: Real-time & Alerting" --body "## Milestone 7: File Tailing

### Tasks
- [ ] Implement file tailing with fsnotify
- [ ] Handle log rotation gracefully
- [ ] Add CLI command: \`blazelog tail <file>\`
- [ ] Support multiple files with glob patterns
- [ ] Write integration tests

### Deliverable
\`blazelog tail /var/log/nginx/*.log --follow\`

### Acceptance Criteria
- [ ] Tails file and outputs new lines
- [ ] Handles log rotation (file recreated)
- [ ] Supports glob patterns" && log_info "Created: Milestone 7" || log_warn "Issue may exist: Milestone 7"

gh issue create --repo "$REPO" --title "[Milestone 8] Alert Rules Engine" --label "stage-b" --label "effort-medium" --milestone "Stage B: Real-time & Alerting" --body "## Milestone 8: Alert Rules Engine

### Tasks
- [ ] Design alert rule YAML schema
- [ ] Implement rule parser
- [ ] Implement pattern-based matching (regex)
- [ ] Implement threshold detection (count in window)
- [ ] Add sliding window aggregation
- [ ] Add alert cooldown/deduplication

### Deliverable
Alert rules loaded from YAML, evaluated in memory

### Acceptance Criteria
- [ ] YAML schema documented
- [ ] Pattern alerts trigger on match
- [ ] Threshold alerts trigger correctly
- [ ] Cooldown prevents alert spam" && log_info "Created: Milestone 8" || log_warn "Issue may exist: Milestone 8"

gh issue create --repo "$REPO" --title "[Milestone 9] Email Notifications" --label "stage-b" --label "effort-small" --milestone "Stage B: Real-time & Alerting" --body "## Milestone 9: Email Notifications

### Tasks
- [ ] Design notifier interface
- [ ] Implement SMTP client with TLS
- [ ] Implement email templates (HTML + plain text)
- [ ] Add CLI flag: \`--notify-email\`
- [ ] Write tests with mock SMTP

### Deliverable
\`blazelog tail ... --notify-email admin@example.com\`

### Acceptance Criteria
- [ ] Sends email via SMTP/TLS
- [ ] HTML and plain text templates
- [ ] Tests pass with mock SMTP" && log_info "Created: Milestone 9" || log_warn "Issue may exist: Milestone 9"

gh issue create --repo "$REPO" --title "[Milestone 10] Slack Notifications" --label "stage-b" --label "effort-small" --milestone "Stage B: Real-time & Alerting" --body "## Milestone 10: Slack Notifications

### Tasks
- [ ] Implement Slack webhook notifier
- [ ] Implement Slack message formatting (blocks)
- [ ] Add CLI flag: \`--notify-slack\`
- [ ] Write tests

### Deliverable
\`blazelog tail ... --notify-slack webhook-url\`

### Acceptance Criteria
- [ ] Sends to Slack webhook
- [ ] Uses Block Kit formatting
- [ ] Tests pass" && log_info "Created: Milestone 10" || log_warn "Issue may exist: Milestone 10"

gh issue create --repo "$REPO" --title "[Milestone 11] Teams Notifications" --label "stage-b" --label "effort-small" --milestone "Stage B: Real-time & Alerting" --body "## Milestone 11: Teams Notifications

### Tasks
- [ ] Implement Microsoft Teams webhook notifier
- [ ] Implement Teams adaptive card formatting
- [ ] Add CLI flag: \`--notify-teams\`
- [ ] Add notification rate limiting (all channels)
- [ ] Write tests

### Deliverable
\`blazelog tail ... --notify-teams webhook-url\`

### Acceptance Criteria
- [ ] Sends to Teams webhook
- [ ] Uses Adaptive Card formatting
- [ ] Rate limiting works across all channels" && log_info "Created: Milestone 11" || log_warn "Issue may exist: Milestone 11"

# Stage C Issues
gh issue create --repo "$REPO" --title "[Milestone 12] gRPC Protocol Definition" --label "stage-c" --label "effort-small" --milestone "Stage C: Distributed Collection" --body "## Milestone 12: gRPC Protocol Definition

### Tasks
- [ ] Define protobuf schemas (LogEntry, AgentInfo, etc.)
- [ ] Generate Go code from protos
- [ ] Design streaming protocol
- [ ] Document protocol

### Deliverable
\`.proto\` files and generated Go code

### Acceptance Criteria
- [ ] Protobuf schemas in \`proto/\` directory
- [ ] Generated Go code compiles
- [ ] Protocol documented" && log_info "Created: Milestone 12" || log_warn "Issue may exist: Milestone 12"

gh issue create --repo "$REPO" --title "[Milestone 13] Agent Basic Implementation" --label "stage-c" --label "effort-medium" --milestone "Stage C: Distributed Collection" --body "## Milestone 13: Agent Basic Implementation

### Tasks
- [ ] Create agent CLI binary (\`blazelog-agent\`)
- [ ] Implement config file loading
- [ ] Implement log collection from local files
- [ ] Implement gRPC client (insecure for now)
- [ ] Write integration tests

### Deliverable
Agent that sends logs to server (no TLS yet)

### Acceptance Criteria
- [ ] Agent binary builds
- [ ] Loads config from YAML
- [ ] Connects to server and streams logs" && log_info "Created: Milestone 13" || log_warn "Issue may exist: Milestone 13"

gh issue create --repo "$REPO" --title "[Milestone 14] Server Log Receiver" --label "stage-c" --label "effort-medium" --milestone "Stage C: Distributed Collection" --body "## Milestone 14: Server Log Receiver

### Tasks
- [ ] Create server binary (\`blazelog-server\`)
- [ ] Implement gRPC server
- [ ] Implement log receiver and processor pipeline
- [ ] Add basic console output for received logs
- [ ] Write integration tests

### Deliverable
Server receives and displays logs from agents

### Acceptance Criteria
- [ ] Server binary builds
- [ ] Accepts agent connections
- [ ] Outputs received logs to console" && log_info "Created: Milestone 14" || log_warn "Issue may exist: Milestone 14"

gh issue create --repo "$REPO" --title "[Milestone 15] mTLS Security" --label "stage-c" --label "effort-medium" --milestone "Stage C: Distributed Collection" --body "## Milestone 15: mTLS Security

### Tasks
- [ ] Implement CA certificate generation (\`blazectl ca init\`)
- [ ] Implement agent certificate generation (\`blazectl cert agent\`)
- [ ] Implement server certificate generation (\`blazectl cert server\`)
- [ ] Add mTLS to gRPC client/server
- [ ] Implement certificate validation
- [ ] Write security tests

### Deliverable
Secure agent-server communication with mTLS

### Acceptance Criteria
- [ ] CA can be initialized
- [ ] Certificates can be generated
- [ ] mTLS connection works
- [ ] Invalid certs rejected" && log_info "Created: Milestone 15" || log_warn "Issue may exist: Milestone 15"

gh issue create --repo "$REPO" --title "[Milestone 16] Agent Reliability" --label "stage-c" --label "effort-medium" --milestone "Stage C: Distributed Collection" --body "## Milestone 16: Agent Reliability

### Tasks
- [ ] Implement local buffer for network outages
- [ ] Implement reconnection with backoff
- [ ] Implement heartbeat/health check
- [ ] Add agent registration flow
- [ ] Write chaos tests (network failures)

### Deliverable
Agent handles network issues gracefully

### Acceptance Criteria
- [ ] Buffers logs when disconnected
- [ ] Reconnects with exponential backoff
- [ ] Sends buffered logs after reconnect" && log_info "Created: Milestone 16" || log_warn "Issue may exist: Milestone 16"

# Stage D Issues
gh issue create --repo "$REPO" --title "[Milestone 17] SSH Connector Base" --label "stage-d" --label "effort-medium" --milestone "Stage D: SSH Collection" --body "## Milestone 17: SSH Connector Base

### Tasks
- [ ] Implement SSH client with key authentication
- [ ] Implement remote file reading
- [ ] Implement remote file tailing
- [ ] Add connection management in config
- [ ] Write integration tests

### Deliverable
Server can read logs from remote servers via SSH

### Acceptance Criteria
- [ ] Connects via SSH key
- [ ] Reads remote files
- [ ] Tails remote files" && log_info "Created: Milestone 17" || log_warn "Issue may exist: Milestone 17"

gh issue create --repo "$REPO" --title "[Milestone 18] SSH Security Hardening" --label "stage-d" --label "effort-medium" --milestone "Stage D: SSH Collection" --body "## Milestone 18: SSH Security Hardening

### Tasks
- [ ] Implement encrypted credential storage (AES-256-GCM)
- [ ] Implement host key verification
- [ ] Add jump host/bastion support
- [ ] Add connection pooling
- [ ] Add audit logging for SSH operations
- [ ] Write security tests

### Deliverable
Production-ready secure SSH connector

### Acceptance Criteria
- [ ] Credentials encrypted at rest
- [ ] Host keys verified
- [ ] Jump hosts work
- [ ] All SSH ops logged" && log_info "Created: Milestone 18" || log_warn "Issue may exist: Milestone 18"

# Stage E Issues
gh issue create --repo "$REPO" --title "[Milestone 19] SQLite for Config" --label "stage-e" --label "effort-small" --milestone "Stage E: Storage" --body "## Milestone 19: SQLite for Config

### Tasks
- [ ] Design SQLite schema (users, projects, alerts, connections)
- [ ] Implement SQLite storage layer
- [ ] Implement database migrations
- [ ] Add config persistence to server
- [ ] Write storage tests

### Deliverable
Server persists configuration in SQLite

### Acceptance Criteria
- [ ] Schema created
- [ ] Migrations work
- [ ] Config persists across restarts" && log_info "Created: Milestone 19" || log_warn "Issue may exist: Milestone 19"

gh issue create --repo "$REPO" --title "[Milestone 20] ClickHouse Setup" --label "stage-e" --label "effort-medium" --milestone "Stage E: Storage" --body "## Milestone 20: ClickHouse Setup

### Tasks
- [ ] Design ClickHouse schema for logs
- [ ] Implement ClickHouse client
- [ ] Implement log insertion (batched)
- [ ] Implement basic log queries
- [ ] Write integration tests

### Deliverable
Logs stored in ClickHouse

### Acceptance Criteria
- [ ] Schema optimized for time-series
- [ ] Batch inserts work
- [ ] Basic queries work" && log_info "Created: Milestone 20" || log_warn "Issue may exist: Milestone 20"

gh issue create --repo "$REPO" --title "[Milestone 21] ClickHouse Advanced" --label "stage-e" --label "effort-medium" --milestone "Stage E: Storage" --body "## Milestone 21: ClickHouse Advanced

### Tasks
- [ ] Create materialized views for dashboards
- [ ] Implement full-text search
- [ ] Implement TTL retention policies
- [ ] Implement log aggregation queries
- [ ] Performance tuning
- [ ] Write performance tests

### Deliverable
Fast search and analytics on billions of logs

### Acceptance Criteria
- [ ] Materialized views for common queries
- [ ] Full-text search works
- [ ] TTL auto-deletes old logs
- [ ] Query performance acceptable" && log_info "Created: Milestone 21" || log_warn "Issue may exist: Milestone 21"

# Stage F Issues
gh issue create --repo "$REPO" --title "[Milestone 22] API Auth & Users" --label "stage-f" --label "effort-medium" --milestone "Stage F: REST API" --body "## Milestone 22: API Auth & Users

### Tasks
- [ ] Set up HTTP router (chi)
- [ ] Implement JWT authentication
- [ ] Implement user registration/login endpoints
- [ ] Implement RBAC (Admin, Operator, Viewer)
- [ ] Add API rate limiting
- [ ] Write API tests

### Deliverable
\`/api/v1/auth/*\` and \`/api/v1/users/*\` endpoints

### Acceptance Criteria
- [ ] JWT auth works
- [ ] RBAC enforced
- [ ] Rate limiting works" && log_info "Created: Milestone 22" || log_warn "Issue may exist: Milestone 22"

gh issue create --repo "$REPO" --title "[Milestone 23] API Logs & Search" --label "stage-f" --label "effort-medium" --milestone "Stage F: REST API" --body "## Milestone 23: API Logs & Search

### Tasks
- [ ] Implement log query endpoint
- [ ] Implement log search with filters
- [ ] Implement SSE for real-time streaming
- [ ] Add pagination
- [ ] Write API tests

### Deliverable
\`/api/v1/logs/*\` endpoints with real-time streaming

### Acceptance Criteria
- [ ] Log queries work
- [ ] Filters work (level, source, time)
- [ ] SSE streams logs in real-time" && log_info "Created: Milestone 23" || log_warn "Issue may exist: Milestone 23"

gh issue create --repo "$REPO" --title "[Milestone 24] API Alerts & Projects" --label "stage-f" --label "effort-medium" --milestone "Stage F: REST API" --body "## Milestone 24: API Alerts & Projects

### Tasks
- [ ] Implement alert rules CRUD endpoints
- [ ] Implement alert history endpoint
- [ ] Implement projects CRUD endpoints
- [ ] Implement connections CRUD endpoints
- [ ] Generate OpenAPI spec
- [ ] Write API tests

### Deliverable
Full REST API complete

### Acceptance Criteria
- [ ] All CRUD endpoints work
- [ ] OpenAPI spec generated
- [ ] API tests pass" && log_info "Created: Milestone 24" || log_warn "Issue may exist: Milestone 24"

# Stage G Issues
gh issue create --repo "$REPO" --title "[Milestone 25] Web UI Setup" --label "stage-g" --label "effort-medium" --milestone "Stage G: Web UI" --body "## Milestone 25: Web UI Setup

### Tasks
- [ ] Set up Templ templates
- [ ] Configure Tailwind CSS
- [ ] Integrate HTMX and Alpine.js
- [ ] Create base layout template
- [ ] Implement login/register pages
- [ ] Embed static assets in binary

### Deliverable
Login page working

### Acceptance Criteria
- [ ] Templ templates compile
- [ ] Tailwind CSS works
- [ ] Login/register functional
- [ ] Assets embedded in binary" && log_info "Created: Milestone 25" || log_warn "Issue may exist: Milestone 25"

gh issue create --repo "$REPO" --title "[Milestone 26] Dashboard" --label "stage-g" --label "effort-medium" --milestone "Stage G: Web UI" --body "## Milestone 26: Dashboard

### Tasks
- [ ] Create dashboard layout
- [ ] Implement metrics cards (log counts, error rates)
- [ ] Implement charts (ECharts)
- [ ] Add time range selector
- [ ] Add auto-refresh

### Deliverable
Dashboard with real-time metrics

### Acceptance Criteria
- [ ] Shows log counts
- [ ] Shows error rates
- [ ] Charts render
- [ ] Auto-refreshes" && log_info "Created: Milestone 26" || log_warn "Issue may exist: Milestone 26"

gh issue create --repo "$REPO" --title "[Milestone 27] Log Viewer" --label "stage-g" --label "effort-medium" --milestone "Stage G: Web UI" --body "## Milestone 27: Log Viewer

### Tasks
- [ ] Implement log list view with pagination
- [ ] Implement search and filters
- [ ] Implement log detail view
- [ ] Implement real-time tail view (SSE)
- [ ] Add export functionality

### Deliverable
Full log viewer with search

### Acceptance Criteria
- [ ] Pagination works
- [ ] Search works
- [ ] Real-time tail works
- [ ] Export to JSON/CSV" && log_info "Created: Milestone 27" || log_warn "Issue may exist: Milestone 27"

gh issue create --repo "$REPO" --title "[Milestone 28] Settings & Management" --label "stage-g" --label "effort-medium" --milestone "Stage G: Web UI" --body "## Milestone 28: Settings & Management

### Tasks
- [ ] Implement alert rules management UI
- [ ] Implement projects management UI
- [ ] Implement connections management UI
- [ ] Implement user management UI (admin only)
- [ ] Add responsive design
- [ ] Write E2E tests

### Deliverable
Complete Web UI

### Acceptance Criteria
- [ ] All management UIs work
- [ ] Responsive on mobile
- [ ] E2E tests pass" && log_info "Created: Milestone 28" || log_warn "Issue may exist: Milestone 28"

# Stage H Issues
gh issue create --repo "$REPO" --title "[Milestone 29] Batch Processing" --label "stage-h" --label "effort-medium" --milestone "Stage H: Batch & Production" --body "## Milestone 29: Batch Processing

### Tasks
- [ ] Implement batch analysis mode
- [ ] Add date range support
- [ ] Implement parallel processing
- [ ] Add report generation
- [ ] Add export (CSV, JSON)
- [ ] Write performance tests

### Deliverable
\`blazelog analyze --from 2024-01-01 --to 2024-01-31\`

### Acceptance Criteria
- [ ] Batch mode works
- [ ] Date range filtering
- [ ] Reports generated
- [ ] Export works" && log_info "Created: Milestone 29" || log_warn "Issue may exist: Milestone 29"

gh issue create --repo "$REPO" --title "[Milestone 30] Production Hardening" --label "stage-h" --label "effort-medium" --milestone "Stage H: Batch & Production" --body "## Milestone 30: Production Hardening

### Tasks
- [ ] Add Prometheus metrics
- [ ] Add health check endpoints
- [ ] Implement graceful shutdown
- [ ] Create Docker images
- [ ] Create systemd service files
- [ ] Write deployment documentation
- [ ] Security audit
- [ ] Load testing

### Deliverable
Production-ready deployment

### Acceptance Criteria
- [ ] Prometheus metrics exposed
- [ ] Health checks work
- [ ] Docker images built
- [ ] Systemd services work
- [ ] Documentation complete" && log_info "Created: Milestone 30" || log_warn "Issue may exist: Milestone 30"

# ============================================================================
# SUMMARY
# ============================================================================
echo ""
echo "=============================================="
echo "GitHub Project Setup Complete!"
echo "=============================================="
echo ""
echo "Created:"
echo "  - 10 labels (stages + effort)"
echo "  - 8 GitHub milestones (stages A-H)"
echo "  - 30 issues (one per milestone)"
echo ""
echo "Next steps:"
echo "  1. Go to your repo's Projects tab"
echo "  2. Create a new project 'BlazeLog Development'"
echo "  3. Add all issues to the project board"
echo "  4. Organize into columns (Backlog, Ready, In Progress, etc.)"
echo ""
echo "Workflow:"
echo "  1. Pick an issue from 'Ready'"
echo "  2. Create branch: git checkout -b feature/milestone-XX-name"
echo "  3. Implement and commit"
echo "  4. Create PR with 'Closes #XX' in description"
echo "  5. Issue auto-closes when PR merges"
echo ""
