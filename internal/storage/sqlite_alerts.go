package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

type sqliteAlertRepo struct {
	db *sql.DB
}

func (r *sqliteAlertRepo) Create(ctx context.Context, alert *models.AlertRule) error {
	notifyJSON, err := json.Marshal(alert.Notify)
	if err != nil {
		return fmt.Errorf("marshal notify: %w", err)
	}

	query := `
		INSERT INTO alerts (id, name, description, type, condition_json, severity,
			window_ns, cooldown_ns, notify_json, enabled, project_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.ExecContext(ctx, query,
		alert.ID, alert.Name, alert.Description, alert.Type, alert.Condition, alert.Severity,
		alert.Window.Nanoseconds(), alert.Cooldown.Nanoseconds(), string(notifyJSON),
		boolToInt(alert.Enabled), nullString(alert.ProjectID),
		alert.CreatedAt, alert.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}
	return nil
}

func (r *sqliteAlertRepo) GetByID(ctx context.Context, id string) (*models.AlertRule, error) {
	query := `
		SELECT id, name, description, type, condition_json, severity,
			window_ns, cooldown_ns, notify_json, enabled, project_id, created_at, updated_at
		FROM alerts WHERE id = ?
	`
	return r.scanAlert(r.db.QueryRowContext(ctx, query, id))
}

func (r *sqliteAlertRepo) Update(ctx context.Context, alert *models.AlertRule) error {
	notifyJSON, err := json.Marshal(alert.Notify)
	if err != nil {
		return fmt.Errorf("marshal notify: %w", err)
	}

	query := `
		UPDATE alerts SET name = ?, description = ?, type = ?, condition_json = ?,
			severity = ?, window_ns = ?, cooldown_ns = ?, notify_json = ?,
			enabled = ?, project_id = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query,
		alert.Name, alert.Description, alert.Type, alert.Condition, alert.Severity,
		alert.Window.Nanoseconds(), alert.Cooldown.Nanoseconds(), string(notifyJSON),
		boolToInt(alert.Enabled), nullString(alert.ProjectID), alert.UpdatedAt,
		alert.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert not found: %s", alert.ID)
	}
	return nil
}

func (r *sqliteAlertRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM alerts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert not found: %s", id)
	}
	return nil
}

func (r *sqliteAlertRepo) List(ctx context.Context) ([]*models.AlertRule, error) {
	query := `
		SELECT id, name, description, type, condition_json, severity,
			window_ns, cooldown_ns, notify_json, enabled, project_id, created_at, updated_at
		FROM alerts ORDER BY name
	`
	return r.queryAlerts(ctx, query)
}

func (r *sqliteAlertRepo) ListByProject(ctx context.Context, projectID string) ([]*models.AlertRule, error) {
	query := `
		SELECT id, name, description, type, condition_json, severity,
			window_ns, cooldown_ns, notify_json, enabled, project_id, created_at, updated_at
		FROM alerts WHERE project_id = ? ORDER BY name
	`
	return r.queryAlertsWithArg(ctx, query, projectID)
}

func (r *sqliteAlertRepo) ListEnabled(ctx context.Context) ([]*models.AlertRule, error) {
	query := `
		SELECT id, name, description, type, condition_json, severity,
			window_ns, cooldown_ns, notify_json, enabled, project_id, created_at, updated_at
		FROM alerts WHERE enabled = 1 ORDER BY name
	`
	return r.queryAlerts(ctx, query)
}

func (r *sqliteAlertRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE alerts SET enabled = ?, updated_at = ? WHERE id = ?",
		boolToInt(enabled), time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("set alert enabled: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert not found: %s", id)
	}
	return nil
}

func (r *sqliteAlertRepo) queryAlerts(ctx context.Context, query string) ([]*models.AlertRule, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()
	return r.scanAlerts(rows)
}

func (r *sqliteAlertRepo) queryAlertsWithArg(ctx context.Context, query string, arg interface{}) ([]*models.AlertRule, error) {
	rows, err := r.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()
	return r.scanAlerts(rows)
}

func (r *sqliteAlertRepo) scanAlerts(rows *sql.Rows) ([]*models.AlertRule, error) {
	var alerts []*models.AlertRule
	for rows.Next() {
		alert, err := r.scanAlertRow(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *sqliteAlertRepo) scanAlert(row *sql.Row) (*models.AlertRule, error) {
	alert := &models.AlertRule{}
	var description, projectID sql.NullString
	var notifyJSON string
	var windowNS, cooldownNS int64
	var enabled int

	err := row.Scan(
		&alert.ID, &alert.Name, &description, &alert.Type, &alert.Condition, &alert.Severity,
		&windowNS, &cooldownNS, &notifyJSON, &enabled, &projectID,
		&alert.CreatedAt, &alert.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan alert: %w", err)
	}

	alert.Description = description.String
	alert.ProjectID = projectID.String
	alert.Window = time.Duration(windowNS)
	alert.Cooldown = time.Duration(cooldownNS)
	alert.Enabled = enabled != 0

	if err := json.Unmarshal([]byte(notifyJSON), &alert.Notify); err != nil {
		return nil, fmt.Errorf("unmarshal notify: %w", err)
	}

	return alert, nil
}

func (r *sqliteAlertRepo) scanAlertRow(rows *sql.Rows) (*models.AlertRule, error) {
	alert := &models.AlertRule{}
	var description, projectID sql.NullString
	var notifyJSON string
	var windowNS, cooldownNS int64
	var enabled int

	err := rows.Scan(
		&alert.ID, &alert.Name, &description, &alert.Type, &alert.Condition, &alert.Severity,
		&windowNS, &cooldownNS, &notifyJSON, &enabled, &projectID,
		&alert.CreatedAt, &alert.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan alert: %w", err)
	}

	alert.Description = description.String
	alert.ProjectID = projectID.String
	alert.Window = time.Duration(windowNS)
	alert.Cooldown = time.Duration(cooldownNS)
	alert.Enabled = enabled != 0

	if err := json.Unmarshal([]byte(notifyJSON), &alert.Notify); err != nil {
		return nil, fmt.Errorf("unmarshal notify: %w", err)
	}

	return alert, nil
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
