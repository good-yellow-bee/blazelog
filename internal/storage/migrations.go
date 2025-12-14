package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Migration represents a database migration.
type Migration struct {
	Version int
	Name    string
	Up      string
}

// migrations holds all database migrations in order.
var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up: `
			-- Users table
			CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				username TEXT UNIQUE NOT NULL,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				role TEXT NOT NULL DEFAULT 'viewer',
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);

			-- Projects table
			CREATE TABLE IF NOT EXISTS projects (
				id TEXT PRIMARY KEY,
				name TEXT UNIQUE NOT NULL,
				description TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);

			-- Project-User junction table (many-to-many)
			CREATE TABLE IF NOT EXISTS project_users (
				project_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				role TEXT NOT NULL DEFAULT 'viewer',
				PRIMARY KEY (project_id, user_id),
				FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);

			-- Alert rules table
			CREATE TABLE IF NOT EXISTS alerts (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				type TEXT NOT NULL,
				condition_json TEXT NOT NULL,
				severity TEXT NOT NULL,
				window_ns INTEGER NOT NULL,
				cooldown_ns INTEGER NOT NULL,
				notify_json TEXT NOT NULL,
				enabled INTEGER NOT NULL DEFAULT 1,
				project_id TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
			);

			-- Connections table
			CREATE TABLE IF NOT EXISTS connections (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				host TEXT,
				port INTEGER,
				user TEXT,
				credentials_encrypted BLOB,
				status TEXT NOT NULL DEFAULT 'unknown',
				last_tested_at DATETIME,
				project_id TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
			);

			-- Indexes
			CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
			CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
			CREATE INDEX IF NOT EXISTS idx_alerts_project ON alerts(project_id);
			CREATE INDEX IF NOT EXISTS idx_alerts_enabled ON alerts(enabled);
			CREATE INDEX IF NOT EXISTS idx_connections_project ON connections(project_id);
			CREATE INDEX IF NOT EXISTS idx_connections_name ON connections(name);
		`,
	},
}

// runMigrations applies all pending migrations.
func runMigrations(db *sql.DB) error {
	// Create migrations table if not exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		// Run migration in transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for migration %d: %w", m.Version, err)
		}

		_, err = tx.Exec(m.Up)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %d (%s): %w", m.Version, m.Name, err)
		}

		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
			m.Version, m.Name, time.Now(),
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}
