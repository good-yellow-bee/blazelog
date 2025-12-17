package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

func setupTestDB(t *testing.T) (*storage.SQLiteStorage, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use a 32-byte master key for encryption tests
	masterKey := []byte("test-master-key-32-bytes-long!!!")

	store := storage.NewSQLiteStorage(dbPath, masterKey)
	if err := store.Open(); err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestResolveConnection_ByID(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a project first (required for connection)
	project := &models.Project{
		ID:        "proj-1",
		Name:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Projects().Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Create a connection
	conn := &models.Connection{
		ID:        "conn-1",
		Name:      "test-ssh",
		Type:      models.ConnectionTypeSSH,
		Host:      "example.com",
		Port:      22,
		User:      "testuser",
		Status:    models.ConnectionStatusUnknown,
		ProjectID: project.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Connections().Create(ctx, conn); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	// Test resolving by ID
	found, err := resolveConnection(ctx, store.Connections(), "", "conn-1")
	if err != nil {
		t.Fatalf("resolveConnection: %v", err)
	}
	if found.ID != "conn-1" {
		t.Errorf("ID = %v, want conn-1", found.ID)
	}
	if found.Name != "test-ssh" {
		t.Errorf("Name = %v, want test-ssh", found.Name)
	}
}

func TestResolveConnection_ByName(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a project first
	project := &models.Project{
		ID:        "proj-1",
		Name:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Projects().Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Create a connection
	conn := &models.Connection{
		ID:        "conn-2",
		Name:      "prod-web",
		Type:      models.ConnectionTypeSSH,
		Host:      "web.example.com",
		Port:      22,
		User:      "deploy",
		Status:    models.ConnectionStatusUnknown,
		ProjectID: project.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Connections().Create(ctx, conn); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	// Test resolving by name
	found, err := resolveConnection(ctx, store.Connections(), "prod-web", "")
	if err != nil {
		t.Fatalf("resolveConnection: %v", err)
	}
	if found.ID != "conn-2" {
		t.Errorf("ID = %v, want conn-2", found.ID)
	}
	if found.Host != "web.example.com" {
		t.Errorf("Host = %v, want web.example.com", found.Host)
	}
}

func TestResolveConnection_NotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to resolve non-existent connection
	_, err := resolveConnection(ctx, store.Connections(), "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for non-existent connection")
	}
}

func TestResolveConnection_NoIdentifier(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to resolve without name or ID
	_, err := resolveConnection(ctx, store.Connections(), "", "")
	if err == nil {
		t.Fatal("expected error when both name and id are empty")
	}
}

func TestCredentialEncryption(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Test encryption/decryption through the repository
	plaintext := []byte(`{"user":"testuser","password":"secret123"}`)

	encrypted, err := store.Connections().EncryptCredentials(plaintext)
	if err != nil {
		t.Fatalf("EncryptCredentials: %v", err)
	}

	// Encrypted should be different from plaintext
	if string(encrypted) == string(plaintext) {
		t.Error("encrypted data should differ from plaintext")
	}

	decrypted, err := store.Connections().DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("DecryptCredentials: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %v, want %v", string(decrypted), string(plaintext))
	}
}

func TestSSHCommandFlags(t *testing.T) {
	// Test that commands have required flags defined
	tests := []struct {
		cmd   string
		flags []string
	}{
		{"list", []string{"project", "project-id", "db"}},
		{"create", []string{"name", "host", "port", "user", "key-file", "project", "project-id", "db"}},
		{"show", []string{"name", "id", "db"}},
		{"delete", []string{"name", "id", "force", "db"}},
		{"test", []string{"name", "id", "db"}},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			var cmd *cobra.Command
			switch tt.cmd {
			case "list":
				cmd = sshListCmd
			case "create":
				cmd = sshCreateCmd
			case "show":
				cmd = sshShowCmd
			case "delete":
				cmd = sshDeleteCmd
			case "test":
				cmd = sshTestCmd
			}

			for _, flagName := range tt.flags {
				if cmd.Flags().Lookup(flagName) == nil {
					t.Errorf("command %s missing flag: %s", tt.cmd, flagName)
				}
			}
		})
	}
}
