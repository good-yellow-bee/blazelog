// Package storage provides database storage interfaces and implementations.
package storage

import (
	"context"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Storage is the main interface for database operations.
type Storage interface {
	// Open initializes the database connection.
	Open() error
	// Close closes the database connection.
	Close() error
	// Migrate runs database migrations.
	Migrate() error
	// EnsureAdminUser creates default admin if no users exist using secure bootstrap credentials.
	EnsureAdminUser() error

	// Repository accessors
	Users() UserRepository
	Projects() ProjectRepository
	Alerts() AlertRepository
	Connections() ConnectionRepository
	Tokens() TokenRepository
	AlertHistory() AlertHistoryRepository
}

// UserRepository defines operations for user management.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*models.User, error)
	Count(ctx context.Context) (int64, error)
}

// ProjectRepository defines operations for project management.
type ProjectRepository interface {
	Create(ctx context.Context, project *models.Project) error
	GetByID(ctx context.Context, id string) (*models.Project, error)
	GetByName(ctx context.Context, name string) (*models.Project, error)
	Update(ctx context.Context, project *models.Project) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*models.Project, error)
	AddUser(ctx context.Context, projectID, userID string, role models.Role) error
	RemoveUser(ctx context.Context, projectID, userID string) error
	GetUsers(ctx context.Context, projectID string) ([]*models.User, error)
	GetProjectMembers(ctx context.Context, projectID string) ([]*models.ProjectMember, error)
	GetProjectsForUser(ctx context.Context, userID string) ([]*models.Project, error)
}

// AlertRepository defines operations for alert rule management.
type AlertRepository interface {
	Create(ctx context.Context, alert *models.AlertRule) error
	GetByID(ctx context.Context, id string) (*models.AlertRule, error)
	Update(ctx context.Context, alert *models.AlertRule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*models.AlertRule, error)
	ListByProject(ctx context.Context, projectID string) ([]*models.AlertRule, error)
	ListEnabled(ctx context.Context) ([]*models.AlertRule, error)
	SetEnabled(ctx context.Context, id string, enabled bool) error
}

// ConnectionRepository defines operations for connection management.
type ConnectionRepository interface {
	Create(ctx context.Context, conn *models.Connection) error
	GetByID(ctx context.Context, id string) (*models.Connection, error)
	GetByName(ctx context.Context, name string) (*models.Connection, error)
	Update(ctx context.Context, conn *models.Connection) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*models.Connection, error)
	ListByProject(ctx context.Context, projectID string) ([]*models.Connection, error)
	UpdateStatus(ctx context.Context, id string, status models.ConnectionStatus, testedAt time.Time) error
	EncryptCredentials(plaintext []byte) ([]byte, error)
	DecryptCredentials(encrypted []byte) ([]byte, error)
}

// TokenRepository defines operations for refresh token management.
type TokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// AlertHistoryRepository defines operations for alert history.
type AlertHistoryRepository interface {
	Create(ctx context.Context, history *models.AlertHistory) error
	List(ctx context.Context, limit, offset int) ([]*models.AlertHistory, int64, error)
	ListByAlert(ctx context.Context, alertID string, limit, offset int) ([]*models.AlertHistory, int64, error)
	ListByProject(ctx context.Context, projectID string, limit, offset int) ([]*models.AlertHistory, int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}
