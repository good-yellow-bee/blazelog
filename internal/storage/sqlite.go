package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// SQLiteStorage implements Storage using SQLite.
type SQLiteStorage struct {
	path      string
	masterKey []byte
	db        *sql.DB

	users        *sqliteUserRepo
	projects     *sqliteProjectRepo
	alerts       *sqliteAlertRepo
	connections  *sqliteConnectionRepo
	tokens       *sqliteTokenRepo
	alertHistory *sqliteAlertHistoryRepo
}

// NewSQLiteStorage creates a new SQLite storage.
func NewSQLiteStorage(path string, masterKey []byte) *SQLiteStorage {
	return &SQLiteStorage{
		path:      path,
		masterKey: masterKey,
	}
}

// Open initializes the database connection.
func (s *SQLiteStorage) Open() error {
	// Connection string
	dsn := fmt.Sprintf("file:%s", s.path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite is single-writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Keep connection alive

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	// Enable foreign keys and WAL mode
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return fmt.Errorf("execute %s: %w", pragma, err)
		}
	}

	s.db = db

	// Initialize repositories
	s.users = &sqliteUserRepo{db: db}
	s.projects = &sqliteProjectRepo{db: db}
	s.alerts = &sqliteAlertRepo{db: db}
	s.connections = &sqliteConnectionRepo{db: db, masterKey: s.masterKey}
	s.tokens = &sqliteTokenRepo{db: db}
	s.alertHistory = &sqliteAlertHistoryRepo{db: db}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate runs database migrations.
func (s *SQLiteStorage) Migrate() error {
	return runMigrations(s.db)
}

// EnsureAdminUser creates default admin if no users exist.
func (s *SQLiteStorage) EnsureAdminUser() error {
	count, err := s.Users().Count(context.Background())
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil // Users exist, skip
	}

	// Generate random password
	password := generateRandomPassword(16)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	admin := &models.User{
		ID:           uuid.New().String(),
		Username:     "admin",
		Email:        "admin@localhost",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.Users().Create(context.Background(), admin); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	fmt.Printf("\n")
	fmt.Printf("===========================================\n")
	fmt.Printf("  DEFAULT ADMIN USER CREATED\n")
	fmt.Printf("  Username: admin\n")
	fmt.Printf("  Password: %s\n", password)
	fmt.Printf("  CHANGE THIS PASSWORD IMMEDIATELY!\n")
	fmt.Printf("===========================================\n")
	fmt.Printf("\n")

	return nil
}

// Users returns the user repository.
func (s *SQLiteStorage) Users() UserRepository {
	return s.users
}

// Projects returns the project repository.
func (s *SQLiteStorage) Projects() ProjectRepository {
	return s.projects
}

// Alerts returns the alert repository.
func (s *SQLiteStorage) Alerts() AlertRepository {
	return s.alerts
}

// Connections returns the connection repository.
func (s *SQLiteStorage) Connections() ConnectionRepository {
	return s.connections
}

// Tokens returns the token repository.
func (s *SQLiteStorage) Tokens() TokenRepository {
	return s.tokens
}

// AlertHistory returns the alert history repository.
func (s *SQLiteStorage) AlertHistory() AlertHistoryRepository {
	return s.alertHistory
}

// generateRandomPassword generates a random password of the specified length.
func generateRandomPassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:length]
}
