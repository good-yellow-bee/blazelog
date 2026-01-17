# BlazeLog CLI Reference

## Overview

BlazeLog provides two CLI tools:

- **`blazectl`** — Administrative CLI for managing users, projects, and parsing logs
- **`blazelog-server`** — Server daemon

All management commands use the default database at `./data/blazelog.db`. Use `--db <path>` to override.
`blazectl` requires `BLAZELOG_DB_KEY` to open the encrypted database. Set `BLAZELOG_MASTER_KEY` when managing encrypted SSH credentials.

---

## User Management

### List users

```bash
blazectl user list
```

Displays all users with their ID, username, email, role, and creation date.

### Create user

```bash
blazectl user create --username <name> --email <email> [--role <role>]
```

**Flags:**
- `--username` (required) — Username (3-50 chars, alphanumeric/underscore/dash)
- `--email` (required) — Email address
- `--role` — Role: `admin`, `operator`, or `viewer` (default: `viewer`)

Password is prompted interactively.

**Password requirements:**
- Minimum 12 characters
- At least 1 uppercase letter (A-Z)
- At least 1 lowercase letter (a-z)
- At least 1 digit (0-9)
- At least 1 special character (!@#$%^&*...)

**Example:**

```bash
blazectl user create --username alice --email alice@example.com --role operator
```

### Change password

```bash
blazectl user passwd --username <name>
```

Prompts for new password. All existing sessions are revoked.

---

## Project Management

### List projects

```bash
blazectl project list
```

Displays all projects with ID, name, description, member count, and creation date.

### Create project

```bash
blazectl project create --name <name> [--description <desc>]
```

**Flags:**
- `--name` (required) — Project name (unique)
- `--description` — Optional description

**Example:**

```bash
blazectl project create --name ecommerce-prod --description "Production e-commerce platform"
```

### Show project details

```bash
blazectl project show --name <name>
blazectl project show --id <uuid>
```

Displays project ID, name, description, member count, and timestamps.

### Update project

```bash
blazectl project update --name <name> [--new-name <new>] [--description <desc>]
blazectl project update --id <uuid> [--new-name <new>] [--description <desc>]
```

**Flags:**
- `--name` or `--id` — Identify the project
- `--new-name` — New project name
- `--description` — New description

### Delete project

```bash
blazectl project delete --name <name>
blazectl project delete --id <uuid> --force
```

**Flags:**
- `--force` — Skip confirmation prompt

Deleting a project removes all memberships. Alerts and other resources have their project association cleared.

---

## Project Member Management

### List members

```bash
blazectl project members --name <name>
blazectl project members --id <uuid>
```

Displays user ID, username, email, and role for each member.

### Add or update member

```bash
blazectl project add-member --name <project> --username <user> --role <role>
blazectl project add-member --id <project-id> --user-id <user-id> --role <role>
```

**Flags:**
- `--name` or `--id` — Identify the project
- `--username` or `--user-id` — Identify the user
- `--role` — Role: `admin`, `operator`, or `viewer` (default: `viewer`)

If the user is already a member, their role is updated (upsert behavior).

**Example:**

```bash
blazectl project add-member --name ecommerce-prod --username alice --role admin
```

### Remove member

```bash
blazectl project remove-member --name <project> --username <user>
blazectl project remove-member --id <project-id> --user-id <user-id>
```

---

## SSH Connection Management

SSH connections allow the server to pull logs from remote servers.

### List connections

```bash
blazectl ssh list
blazectl ssh list --project <name>
```

Displays all SSH connections with ID, name, host, port, status, and project.

### Create connection

```bash
blazectl ssh create --name <name> --host <host> --user <user> --project <project> [flags]
```

**Flags:**
- `--name` (required) — Connection name (unique)
- `--host` (required) — SSH host address
- `--port` — SSH port (default: 22)
- `--user` (required) — SSH username
- `--key-file` — Path to private key file
- `--project` (required) — Project name

If `--key-file` is provided, prompts for passphrase. Otherwise prompts for password.

**Example:**

```bash
blazectl ssh create --name prod-web --host web.example.com --user loguser --project myapp
blazectl ssh create --name prod-web --host web.example.com:2222 --user loguser --project myapp --key-file ~/.ssh/id_ed25519
```

### Show connection details

```bash
blazectl ssh show --name <name>
blazectl ssh show --id <uuid>
```

Displays connection ID, name, type, host, port, user, status, project, and timestamps.

### Test connection

```bash
blazectl ssh test --name <name>
blazectl ssh test --id <uuid>
```

Attempts to connect via SSH and updates the connection status.

### Delete connection

```bash
blazectl ssh delete --name <name>
blazectl ssh delete --id <uuid> --force
```

**Flags:**
- `--force` — Skip confirmation prompt

---

## Log Parsing

### Parse log files

```bash
blazectl parse <format> <file> [flags]
```

**Formats:** `nginx`, `apache`, `magento`, `prestashop`, `wordpress`, `syslog`, `json`, `auto`

**Flags:**
- `--output`, `-o` — Output format: `table`, `json`, `plain`
- `--verbose`, `-v` — Verbose output

**Examples:**

```bash
# Auto-detect format
blazectl parse auto /var/log/nginx/access.log

# Specific parser with JSON output
blazectl parse nginx /var/log/nginx/access.log -o json

# Parse multiple files
blazectl parse magento /var/www/magento/var/log/*.log
```

### Tail log files

```bash
blazectl tail <file> [flags]
```

**Flags:**
- `--follow`, `-f` — Follow file for new entries
- `--lines`, `-n` — Number of lines to show (default: 10)
- `--format` — Parser type (default: auto)

---

## Certificate Management

### Initialize CA

```bash
blazectl ca init
```

Creates a new Certificate Authority for mTLS.

### Generate server certificate

```bash
blazectl cert server --output <dir> --cn <common-name> [--san <SANs>]
```

### Generate agent certificate

```bash
blazectl cert agent --name <agent-name> --output <dir>
```

---

## Global Options

All commands support:

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output |
| `--output` | `-o` | Output format: `table`, `json`, `plain` |
| `--db` | | Database path (default: `./data/blazelog.db`) |

---

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `BLAZELOG_MASTER_KEY` | Encryption key for server | Server only |
| `BLAZELOG_JWT_SECRET` | JWT signing secret | Server only |
| `BLAZELOG_CSRF_SECRET` | CSRF protection (enables Web UI) | Optional |
| `BLAZELOG_WEB_UI_ENABLED` | Set to `false` to disable Web UI | Optional (default: `true`) |

---

## Examples

### Initial setup

```bash
# Start server
export BLAZELOG_MASTER_KEY=$(openssl rand -base64 32)
export BLAZELOG_JWT_SECRET=$(openssl rand -base64 32)
export BLAZELOG_CSRF_SECRET=$(openssl rand -base64 32)
./blazelog-server

# Create admin user
blazectl user create --username admin --email admin@example.com --role admin

# Create project and add members
blazectl project create --name myapp --description "My Application"
blazectl project add-member --name myapp --username admin --role admin
```

### API-only mode (no Web UI)

```bash
export BLAZELOG_WEB_UI_ENABLED=false
./blazelog-server

# Manage via CLI only
blazectl user list
blazectl project list
```
