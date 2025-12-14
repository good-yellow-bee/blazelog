package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func generateTestKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("create public key: %v", err)
	}
	return publicKey
}

func TestHostKeyPolicy_String(t *testing.T) {
	tests := []struct {
		policy   HostKeyPolicy
		expected string
	}{
		{PolicyStrict, "strict"},
		{PolicyTOFU, "tofu"},
		{PolicyWarn, "warn"},
		{HostKeyPolicy(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.policy.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseHostKeyPolicy(t *testing.T) {
	tests := []struct {
		input    string
		expected HostKeyPolicy
		wantErr  bool
	}{
		{"strict", PolicyStrict, false},
		{"STRICT", PolicyStrict, false},
		{"tofu", PolicyTOFU, false},
		{"TOFU", PolicyTOFU, false},
		{"warn", PolicyWarn, false},
		{"WARN", PolicyWarn, false},
		{"invalid", PolicyStrict, true},
		{"", PolicyStrict, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseHostKeyPolicy(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFileHostKeyStore_AddGet(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key := generateTestKey(t)
	host := "example.com:22"

	// Get non-existent
	got, err := store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent host")
	}

	// Add
	if err := store.Add(host, key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Get after add
	got, err = store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected key, got nil")
	}

	if Fingerprint(got) != Fingerprint(key) {
		t.Error("fingerprints don't match")
	}
}

func TestFileHostKeyStore_AddDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key := generateTestKey(t)
	host := "example.com:22"

	// Add same key twice - should be idempotent
	if err := store.Add(host, key); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add(host, key); err != nil {
		t.Fatalf("Add same key again: %v", err)
	}

	// Add different key for same host - should error
	key2 := generateTestKey(t)
	if err := store.Add(host, key2); err == nil {
		t.Error("expected error when adding different key for same host")
	}
}

func TestFileHostKeyStore_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key := generateTestKey(t)
	host := "example.com:22"

	if err := store.Add(host, key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := store.Remove(host); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	got, err := store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("expected nil after remove")
	}
}

func TestFileHostKeyStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	if err := store.Add("host1.com:22", key1); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("host2.com:22", key2); err != nil {
		t.Fatalf("Add: %v", err)
	}

	keys, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestFileHostKeyStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	key := generateTestKey(t)
	host := "example.com:22"

	// Create store and add key
	store1, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}
	if err := store1.Add(host, key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Create new store instance and verify key persisted
	store2, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}
	got, err := store2.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("key should persist")
	}
	if Fingerprint(got) != Fingerprint(key) {
		t.Error("fingerprints don't match")
	}
}

func TestFileHostKeyStore_NormalizeHost(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key := generateTestKey(t)

	// Add with uppercase
	if err := store.Add("EXAMPLE.COM:22", key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Get with lowercase
	got, err := store.Get("example.com:22")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("host normalization failed")
	}

	// Get with default port
	got, err = store.Get("example.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("default port handling failed")
	}
}

func TestMemoryHostKeyStore(t *testing.T) {
	store := NewMemoryHostKeyStore()

	key := generateTestKey(t)
	host := "example.com:22"

	// Get non-existent
	got, err := store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent host")
	}

	// Add
	if err := store.Add(host, key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Get after add
	got, err = store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected key")
	}

	// List
	keys, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	// Remove
	if err := store.Remove(host); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	got, err = store.Get(host)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("expected nil after remove")
	}
}

func TestNewHostKeyCallback_TOFU(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	callback := NewHostKeyCallback(store, PolicyTOFU, nil)

	// First connection - should accept and store
	err := callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}

	// Verify key was stored
	stored, _ := store.Get("example.com:22")
	if stored == nil {
		t.Error("key should be stored")
	}

	// Second connection with same key - should succeed
	err = callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("second connection should succeed: %v", err)
	}

	// Connection with different key - should fail
	differentKey := generateTestKey(t)
	err = callback("example.com", &mockAddr{port: 22}, differentKey)
	if err == nil {
		t.Error("connection with different key should fail")
	}
}

func TestNewHostKeyCallback_Strict(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	callback := NewHostKeyCallback(store, PolicyStrict, nil)

	// Unknown host - should reject
	err := callback("example.com", &mockAddr{port: 22}, key)
	if err != ErrUnknownHost {
		t.Errorf("expected ErrUnknownHost, got %v", err)
	}

	// Pre-add key to store
	store.Add("example.com:22", key)

	// Known host - should succeed
	err = callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("known host should succeed: %v", err)
	}
}

func TestNewHostKeyCallback_Warn(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	callback := NewHostKeyCallback(store, PolicyWarn, nil)

	// Unknown host - should accept
	err := callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("warn mode should accept unknown host: %v", err)
	}
}

func TestNewHostKeyCallback_WithAudit(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	callback := NewHostKeyCallback(store, PolicyTOFU, logger)

	// First connection - should log host_key_accepted with is_new=true
	_ = callback("example.com", &mockAddr{port: 22}, key)

	if buf.Len() == 0 {
		t.Error("expected audit log entry")
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com:22", "example.com:22"},
		{"example.com", "example.com:22"},
		{"EXAMPLE.COM:22", "example.com:22"},
		{"[::1]:22", "::1:22"},
		{"192.168.1.1:22", "192.168.1.1:22"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeHost(tt.input); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFileHostKeyStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "known_hosts")

	store, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key := generateTestKey(t)
	if err := store.Add("example.com:22", key); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	port int
}

func (a *mockAddr) Network() string { return "tcp" }
func (a *mockAddr) String() string  { return "" }
