package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

type sqliteAlertHistoryRepo struct {
	db *sql.DB
}

func (r *sqliteAlertHistoryRepo) Create(ctx context.Context, h *models.AlertHistory) error {
	query := `
		INSERT INTO alert_history (id, alert_id, alert_name, severity, message,
			matched_logs, notified_at, project_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		h.ID, h.AlertID, h.AlertName, h.Severity, h.Message,
		h.MatchedLogs, h.NotifiedAt, nullString(h.ProjectID), h.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert history: %w", err)
	}
	return nil
}

func (r *sqliteAlertHistoryRepo) List(ctx context.Context, limit, offset int) ([]*models.AlertHistory, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM alert_history").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count alert history: %w", err)
	}

	query := `
		SELECT id, alert_id, alert_name, severity, message, matched_logs,
			notified_at, project_id, created_at
		FROM alert_history ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query alert history: %w", err)
	}
	defer rows.Close()

	histories, err := r.scanHistories(rows)
	if err != nil {
		return nil, 0, err
	}
	return histories, total, rows.Err()
}

func (r *sqliteAlertHistoryRepo) ListByAlert(ctx context.Context, alertID string, limit, offset int) ([]*models.AlertHistory, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM alert_history WHERE alert_id = ?", alertID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count alert history by alert: %w", err)
	}

	query := `
		SELECT id, alert_id, alert_name, severity, message, matched_logs,
			notified_at, project_id, created_at
		FROM alert_history WHERE alert_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, alertID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query alert history by alert: %w", err)
	}
	defer rows.Close()

	histories, err := r.scanHistories(rows)
	if err != nil {
		return nil, 0, err
	}
	return histories, total, rows.Err()
}

func (r *sqliteAlertHistoryRepo) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]*models.AlertHistory, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM alert_history WHERE project_id = ?", projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count alert history by project: %w", err)
	}

	query := `
		SELECT id, alert_id, alert_name, severity, message, matched_logs,
			notified_at, project_id, created_at
		FROM alert_history WHERE project_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query alert history by project: %w", err)
	}
	defer rows.Close()

	histories, err := r.scanHistories(rows)
	if err != nil {
		return nil, 0, err
	}
	return histories, total, rows.Err()
}

func (r *sqliteAlertHistoryRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM alert_history WHERE created_at < ?", before)
	if err != nil {
		return 0, fmt.Errorf("delete alert history: %w", err)
	}
	return result.RowsAffected()
}

func (r *sqliteAlertHistoryRepo) scanHistories(rows *sql.Rows) ([]*models.AlertHistory, error) {
	var histories []*models.AlertHistory
	for rows.Next() {
		h := &models.AlertHistory{}
		var projectID sql.NullString
		err := rows.Scan(&h.ID, &h.AlertID, &h.AlertName, &h.Severity, &h.Message,
			&h.MatchedLogs, &h.NotifiedAt, &projectID, &h.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan alert history: %w", err)
		}
		h.ProjectID = projectID.String
		histories = append(histories, h)
	}
	return histories, nil
}
