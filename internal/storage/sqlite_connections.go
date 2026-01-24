package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/security"
)

type sqliteConnectionRepo struct {
	db        *sql.DB
	masterKey []byte
}

func (r *sqliteConnectionRepo) Create(ctx context.Context, conn *models.Connection) error {
	query := `
		INSERT INTO connections (id, name, type, host, port, user, credentials_encrypted,
			status, last_tested_at, project_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		conn.ID, conn.Name, conn.Type, conn.Host, conn.Port, conn.User,
		conn.CredentialsEncrypted, conn.Status, conn.LastTestedAt,
		nullString(conn.ProjectID), conn.CreatedAt, conn.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	return nil
}

func (r *sqliteConnectionRepo) GetByID(ctx context.Context, id string) (*models.Connection, error) {
	query := `
		SELECT id, name, type, host, port, user, credentials_encrypted,
			status, last_tested_at, project_id, created_at, updated_at
		FROM connections WHERE id = ?
	`
	return r.scanConnection(r.db.QueryRowContext(ctx, query, id))
}

func (r *sqliteConnectionRepo) GetByName(ctx context.Context, name string) (*models.Connection, error) {
	query := `
		SELECT id, name, type, host, port, user, credentials_encrypted,
			status, last_tested_at, project_id, created_at, updated_at
		FROM connections WHERE name = ?
	`
	return r.scanConnection(r.db.QueryRowContext(ctx, query, name))
}

func (r *sqliteConnectionRepo) Update(ctx context.Context, conn *models.Connection) error {
	query := `
		UPDATE connections SET name = ?, type = ?, host = ?, port = ?, user = ?,
			credentials_encrypted = ?, status = ?, last_tested_at = ?,
			project_id = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query,
		conn.Name, conn.Type, conn.Host, conn.Port, conn.User,
		conn.CredentialsEncrypted, conn.Status, conn.LastTestedAt,
		nullString(conn.ProjectID), conn.UpdatedAt,
		conn.ID,
	)
	if err != nil {
		return fmt.Errorf("update connection: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", conn.ID)
	}
	return nil
}

func (r *sqliteConnectionRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM connections WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", id)
	}
	return nil
}

func (r *sqliteConnectionRepo) List(ctx context.Context) ([]*models.Connection, error) {
	query := `
		SELECT id, name, type, host, port, user, credentials_encrypted,
			status, last_tested_at, project_id, created_at, updated_at
		FROM connections ORDER BY name
	`
	return r.queryConnections(ctx, query)
}

func (r *sqliteConnectionRepo) ListByProject(ctx context.Context, projectID string) ([]*models.Connection, error) {
	query := `
		SELECT id, name, type, host, port, user, credentials_encrypted,
			status, last_tested_at, project_id, created_at, updated_at
		FROM connections WHERE project_id = ? ORDER BY name
	`
	return r.queryConnectionsWithArg(ctx, query, projectID)
}

func (r *sqliteConnectionRepo) UpdateStatus(ctx context.Context, id string, status models.ConnectionStatus, testedAt time.Time) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE connections SET status = ?, last_tested_at = ?, updated_at = ? WHERE id = ?",
		status, testedAt, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update connection status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", id)
	}
	return nil
}

// EncryptCredentials encrypts credentials for storage.
func (r *sqliteConnectionRepo) EncryptCredentials(plaintext []byte) ([]byte, error) {
	if len(r.masterKey) == 0 {
		return nil, fmt.Errorf("master key not set")
	}
	data, err := security.Encrypt(plaintext, r.masterKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}
	return json.Marshal(data)
}

// DecryptCredentials decrypts credentials from storage.
func (r *sqliteConnectionRepo) DecryptCredentials(encrypted []byte) ([]byte, error) {
	if len(r.masterKey) == 0 {
		return nil, fmt.Errorf("master key not set")
	}
	if len(encrypted) == 0 {
		return nil, nil
	}
	var data security.EncryptedData
	if err := json.Unmarshal(encrypted, &data); err != nil {
		return nil, fmt.Errorf("unmarshal encrypted data: %w", err)
	}
	return security.Decrypt(&data, r.masterKey)
}

func (r *sqliteConnectionRepo) queryConnections(ctx context.Context, query string) ([]*models.Connection, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()
	return r.scanConnections(rows)
}

func (r *sqliteConnectionRepo) queryConnectionsWithArg(ctx context.Context, query string, arg interface{}) ([]*models.Connection, error) {
	rows, err := r.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()
	return r.scanConnections(rows)
}

func (r *sqliteConnectionRepo) scanConnections(rows *sql.Rows) ([]*models.Connection, error) {
	var conns []*models.Connection
	for rows.Next() {
		conn, err := r.scanConnectionRow(rows)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
	}
	return conns, rows.Err()
}

func (r *sqliteConnectionRepo) scanConnection(row *sql.Row) (*models.Connection, error) {
	conn := &models.Connection{}
	var host, user, projectID sql.NullString
	var port sql.NullInt64
	var lastTestedAt sql.NullTime

	err := row.Scan(
		&conn.ID, &conn.Name, &conn.Type, &host, &port, &user,
		&conn.CredentialsEncrypted, &conn.Status, &lastTestedAt,
		&projectID, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		//nolint:nilnil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan connection: %w", err)
	}

	conn.Host = host.String
	conn.User = user.String
	conn.ProjectID = projectID.String
	conn.Port = int(port.Int64)
	if lastTestedAt.Valid {
		conn.LastTestedAt = &lastTestedAt.Time
	}

	return conn, nil
}

func (r *sqliteConnectionRepo) scanConnectionRow(rows *sql.Rows) (*models.Connection, error) {
	conn := &models.Connection{}
	var host, user, projectID sql.NullString
	var port sql.NullInt64
	var lastTestedAt sql.NullTime

	err := rows.Scan(
		&conn.ID, &conn.Name, &conn.Type, &host, &port, &user,
		&conn.CredentialsEncrypted, &conn.Status, &lastTestedAt,
		&projectID, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan connection: %w", err)
	}

	conn.Host = host.String
	conn.User = user.String
	conn.ProjectID = projectID.String
	conn.Port = int(port.Int64)
	if lastTestedAt.Valid {
		conn.LastTestedAt = &lastTestedAt.Time
	}

	return conn, nil
}
