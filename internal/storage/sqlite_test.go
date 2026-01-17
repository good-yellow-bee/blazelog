package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func setupTestDB(t *testing.T) (*SQLiteStorage, func()) {
	t.Helper()

	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "blazelog-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	masterKey := []byte("test-master-key-32-bytes-long!!!")
	dbKey := []byte("test-db-key-32-bytes-long!!!!!")

	store := NewSQLiteStorage(dbPath, masterKey, dbKey)
	if err := store.Open(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("open database: %v", err)
	}

	if err := store.Migrate(); err != nil {
		store.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("migrate database: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestSQLiteStorage_OpenClose(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify storage is open
	if store.db == nil {
		t.Fatal("database should be open")
	}
}

func TestSQLiteStorage_Migrate(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Verify tables exist by querying them
	tables := []string{"users", "projects", "alerts", "connections", "project_users", "schema_migrations"}
	for _, table := range tables {
		var count int
		err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
		if err != nil {
			t.Errorf("table %s should exist: %v", table, err)
		}
	}
}

func TestUserRepository_CRUD(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:           uuid.New().String(),
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		Role:         models.RoleOperator,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := store.Users().Create(ctx, user)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Get by ID
	got, err := store.Users().GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if got == nil {
		t.Fatal("user should exist")
	}
	if got.Username != user.Username {
		t.Errorf("username = %v, want %v", got.Username, user.Username)
	}

	// Get by username
	got, err = store.Users().GetByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("get user by username: %v", err)
	}
	if got == nil {
		t.Fatal("user should exist")
	}

	// Get by email
	got, err = store.Users().GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if got == nil {
		t.Fatal("user should exist")
	}

	// Update
	user.Username = "updated-user"
	user.UpdatedAt = time.Now()
	err = store.Users().Update(ctx, user)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}

	got, _ = store.Users().GetByID(ctx, user.ID)
	if got.Username != "updated-user" {
		t.Errorf("username = %v, want updated-user", got.Username)
	}

	// List
	users, err := store.Users().List(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("users count = %d, want 1", len(users))
	}

	// Count
	count, err := store.Users().Count(ctx)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Delete
	err = store.Users().Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	got, _ = store.Users().GetByID(ctx, user.ID)
	if got != nil {
		t.Error("user should be deleted")
	}
}

func TestProjectRepository_CRUD(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create project
	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        "test-project",
		Description: "Test description",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := store.Projects().Create(ctx, project)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Get by ID
	got, err := store.Projects().GetByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("get project by id: %v", err)
	}
	if got == nil {
		t.Fatal("project should exist")
	}
	if got.Name != project.Name {
		t.Errorf("name = %v, want %v", got.Name, project.Name)
	}

	// Get by name
	got, err = store.Projects().GetByName(ctx, project.Name)
	if err != nil {
		t.Fatalf("get project by name: %v", err)
	}
	if got == nil {
		t.Fatal("project should exist")
	}

	// Update
	project.Description = "Updated description"
	project.UpdatedAt = time.Now()
	err = store.Projects().Update(ctx, project)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	// List
	projects, err := store.Projects().List(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("projects count = %d, want 1", len(projects))
	}

	// Delete
	err = store.Projects().Delete(ctx, project.ID)
	if err != nil {
		t.Fatalf("delete project: %v", err)
	}

	got, _ = store.Projects().GetByID(ctx, project.ID)
	if got != nil {
		t.Error("project should be deleted")
	}
}

func TestProjectRepository_UserAssociation(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:           uuid.New().String(),
		Username:     "projectuser",
		Email:        "project@example.com",
		PasswordHash: "hash",
		Role:         models.RoleViewer,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	store.Users().Create(ctx, user)

	// Create project
	project := &models.Project{
		ID:        uuid.New().String(),
		Name:      "user-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Projects().Create(ctx, project)

	// Add user to project
	err := store.Projects().AddUser(ctx, project.ID, user.ID, models.RoleOperator)
	if err != nil {
		t.Fatalf("add user to project: %v", err)
	}

	// Get users in project
	users, err := store.Projects().GetUsers(ctx, project.ID)
	if err != nil {
		t.Fatalf("get project users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("users count = %d, want 1", len(users))
	}

	// Get projects for user
	projects, err := store.Projects().GetProjectsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("projects count = %d, want 1", len(projects))
	}

	// Remove user from project
	err = store.Projects().RemoveUser(ctx, project.ID, user.ID)
	if err != nil {
		t.Fatalf("remove user from project: %v", err)
	}

	users, _ = store.Projects().GetUsers(ctx, project.ID)
	if len(users) != 0 {
		t.Error("user should be removed from project")
	}
}

func TestAlertRepository_CRUD(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create alert
	alert := &models.AlertRule{
		ID:          uuid.New().String(),
		Name:        "test-alert",
		Description: "Test alert",
		Type:        models.AlertTypePattern,
		Condition:   `{"pattern": "ERROR"}`,
		Severity:    models.SeverityHigh,
		Window:      5 * time.Minute,
		Cooldown:    10 * time.Minute,
		Notify:      []string{"email", "slack"},
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := store.Alerts().Create(ctx, alert)
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}

	// Get by ID
	got, err := store.Alerts().GetByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get alert by id: %v", err)
	}
	if got == nil {
		t.Fatal("alert should exist")
	}
	if got.Name != alert.Name {
		t.Errorf("name = %v, want %v", got.Name, alert.Name)
	}
	if len(got.Notify) != 2 {
		t.Errorf("notify count = %d, want 2", len(got.Notify))
	}

	// Update
	alert.Description = "Updated description"
	alert.UpdatedAt = time.Now()
	err = store.Alerts().Update(ctx, alert)
	if err != nil {
		t.Fatalf("update alert: %v", err)
	}

	// List enabled
	enabled, err := store.Alerts().ListEnabled(ctx)
	if err != nil {
		t.Fatalf("list enabled alerts: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("enabled alerts count = %d, want 1", len(enabled))
	}

	// Set enabled = false
	err = store.Alerts().SetEnabled(ctx, alert.ID, false)
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}

	enabled, _ = store.Alerts().ListEnabled(ctx)
	if len(enabled) != 0 {
		t.Error("alert should be disabled")
	}

	// Delete
	err = store.Alerts().Delete(ctx, alert.ID)
	if err != nil {
		t.Fatalf("delete alert: %v", err)
	}

	got, _ = store.Alerts().GetByID(ctx, alert.ID)
	if got != nil {
		t.Error("alert should be deleted")
	}
}

func TestConnectionRepository_CRUD(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create connection
	conn := &models.Connection{
		ID:        uuid.New().String(),
		Name:      "test-connection",
		Type:      models.ConnectionTypeSSH,
		Host:      "example.com",
		Port:      22,
		User:      "admin",
		Status:    models.ConnectionStatusUnknown,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := store.Connections().Create(ctx, conn)
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	// Get by ID
	got, err := store.Connections().GetByID(ctx, conn.ID)
	if err != nil {
		t.Fatalf("get connection by id: %v", err)
	}
	if got == nil {
		t.Fatal("connection should exist")
	}
	if got.Name != conn.Name {
		t.Errorf("name = %v, want %v", got.Name, conn.Name)
	}

	// Get by name
	got, err = store.Connections().GetByName(ctx, conn.Name)
	if err != nil {
		t.Fatalf("get connection by name: %v", err)
	}
	if got == nil {
		t.Fatal("connection should exist")
	}

	// Update status
	now := time.Now()
	err = store.Connections().UpdateStatus(ctx, conn.ID, models.ConnectionStatusConnected, now)
	if err != nil {
		t.Fatalf("update connection status: %v", err)
	}

	got, _ = store.Connections().GetByID(ctx, conn.ID)
	if got.Status != models.ConnectionStatusConnected {
		t.Errorf("status = %v, want connected", got.Status)
	}

	// List
	conns, err := store.Connections().List(ctx)
	if err != nil {
		t.Fatalf("list connections: %v", err)
	}
	if len(conns) != 1 {
		t.Errorf("connections count = %d, want 1", len(conns))
	}

	// Delete
	err = store.Connections().Delete(ctx, conn.ID)
	if err != nil {
		t.Fatalf("delete connection: %v", err)
	}

	got, _ = store.Connections().GetByID(ctx, conn.ID)
	if got != nil {
		t.Error("connection should be deleted")
	}
}

func TestConnectionRepository_EncryptCredentials(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	repo := store.connections

	// Encrypt
	plaintext := []byte("secret-password")
	encrypted, err := repo.EncryptCredentials(plaintext)
	if err != nil {
		t.Fatalf("encrypt credentials: %v", err)
	}

	if len(encrypted) == 0 {
		t.Fatal("encrypted data should not be empty")
	}

	// Decrypt
	decrypted, err := repo.DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("decrypt credentials: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %v, want %v", string(decrypted), string(plaintext))
	}
}

func TestEnsureAdminUser(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// First call should create admin
	err := store.EnsureAdminUser()
	if err != nil {
		t.Fatalf("ensure admin user: %v", err)
	}

	// Verify admin exists
	admin, err := store.Users().GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if admin == nil {
		t.Fatal("admin user should exist")
	}
	if admin.Role != models.RoleAdmin {
		t.Errorf("admin role = %v, want admin", admin.Role)
	}

	// Second call should not create duplicate
	count1, _ := store.Users().Count(ctx)
	err = store.EnsureAdminUser()
	if err != nil {
		t.Fatalf("second ensure admin user: %v", err)
	}
	count2, _ := store.Users().Count(ctx)
	if count1 != count2 {
		t.Errorf("user count changed from %d to %d", count1, count2)
	}
}

func TestForeignKeyConstraints(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create project
	project := &models.Project{
		ID:        uuid.New().String(),
		Name:      "fk-test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Projects().Create(ctx, project)

	// Create alert with project reference
	alert := &models.AlertRule{
		ID:        uuid.New().String(),
		Name:      "fk-test-alert",
		Type:      models.AlertTypePattern,
		Condition: `{}`,
		Severity:  models.SeverityMedium,
		Notify:    []string{},
		Enabled:   true,
		ProjectID: project.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Alerts().Create(ctx, alert)

	// Delete project - should set alert's project_id to NULL
	store.Projects().Delete(ctx, project.ID)

	// Verify alert still exists but project_id is empty
	got, _ := store.Alerts().GetByID(ctx, alert.ID)
	if got == nil {
		t.Fatal("alert should still exist")
	}
	if got.ProjectID != "" {
		t.Errorf("project_id should be empty after project deletion, got %v", got.ProjectID)
	}
}
