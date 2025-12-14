package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/security"
)

// Integration tests for SSH security features

func TestHostKeyVerification_TOFU_FirstConnect(t *testing.T) {
	tmpDir := t.TempDir()
	knownHostsPath := filepath.Join(tmpDir, "known_hosts")

	store, err := NewFileHostKeyStore(knownHostsPath)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	buf := newTestWriteCloser()
	audit := NewJSONAuditLoggerWriter(buf)

	callback := NewHostKeyCallback(store, PolicyTOFU, audit)

	// Generate test key
	key := generateTestKey(t)

	// First connection - should accept and store
	err = callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}

	// Verify key was stored in file
	stored, err := store.Get("example.com:22")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored == nil {
		t.Error("key should be stored after TOFU")
	}

	// Verify audit log contains host_key_accepted with is_new=true
	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if event.Event != "host_key_accepted" {
		t.Errorf("event type: got %q, want %q", event.Event, "host_key_accepted")
	}
	if event.IsNew == nil || !*event.IsNew {
		t.Error("is_new should be true for first connection")
	}
}

func TestHostKeyVerification_TOFU_KnownHost(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	// Pre-add key
	store.Add("example.com:22", key)

	buf := newTestWriteCloser()
	audit := NewJSONAuditLoggerWriter(buf)

	callback := NewHostKeyCallback(store, PolicyTOFU, audit)

	// Connection with known key - should succeed
	err := callback("example.com", &mockAddr{port: 22}, key)
	if err != nil {
		t.Fatalf("connection with known key should succeed: %v", err)
	}

	// Verify audit log contains host_key_accepted with is_new=false
	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if event.IsNew == nil || *event.IsNew {
		t.Error("is_new should be false for known host")
	}
}

func TestHostKeyVerification_Mismatch_Rejected(t *testing.T) {
	store := NewMemoryHostKeyStore()
	originalKey := generateTestKey(t)
	differentKey := generateTestKey(t)

	// Pre-add original key
	store.Add("example.com:22", originalKey)

	buf := newTestWriteCloser()
	audit := NewJSONAuditLoggerWriter(buf)

	callback := NewHostKeyCallback(store, PolicyTOFU, audit)

	// Connection with different key - should fail
	err := callback("example.com", &mockAddr{port: 22}, differentKey)
	if err == nil {
		t.Fatal("connection with different key should fail")
	}

	if !strings.Contains(err.Error(), "host key mismatch") {
		t.Errorf("error should mention host key mismatch: %v", err)
	}

	// Verify audit log contains host_key_rejected
	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if event.Event != "host_key_rejected" {
		t.Errorf("event type: got %q, want %q", event.Event, "host_key_rejected")
	}
	if event.Expected == "" || event.Actual == "" {
		t.Error("expected and actual fingerprints should be logged")
	}
}

func TestHostKeyVerification_StrictMode_UnknownRejected(t *testing.T) {
	store := NewMemoryHostKeyStore()
	key := generateTestKey(t)

	buf := newTestWriteCloser()
	audit := NewJSONAuditLoggerWriter(buf)

	callback := NewHostKeyCallback(store, PolicyStrict, audit)

	// Connection to unknown host in strict mode - should fail
	err := callback("example.com", &mockAddr{port: 22}, key)
	if err != ErrUnknownHost {
		t.Errorf("expected ErrUnknownHost, got %v", err)
	}

	// Verify audit log contains host_key_rejected
	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if event.Event != "host_key_rejected" {
		t.Errorf("event type: got %q, want %q", event.Event, "host_key_rejected")
	}
}

func TestEncryptedKeyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a fake SSH key content for testing encryption/decryption
	fakeKeyContent := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBIq/Jz1rBj3qQw1z0Q3Q==
-----END OPENSSH PRIVATE KEY-----`)

	// Encrypt and save the key
	encryptedPath := filepath.Join(tmpDir, "test.key.enc")
	masterPassword := []byte("master-secret")

	err := security.WriteEncryptedFile(encryptedPath, fakeKeyContent, masterPassword)
	if err != nil {
		t.Fatalf("WriteEncryptedFile: %v", err)
	}

	// Verify the file is encrypted (not readable as plain text)
	content, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if bytes.Contains(content, []byte("PRIVATE KEY")) {
		t.Error("encrypted file should not contain plain text key markers")
	}

	// Verify we can read it back with correct password
	decrypted, err := security.ReadEncryptedFile(encryptedPath, masterPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile: %v", err)
	}
	if !bytes.Equal(decrypted, fakeKeyContent) {
		t.Error("decrypted content doesn't match original")
	}

	// Verify wrong password fails
	_, err = security.ReadEncryptedFile(encryptedPath, []byte("wrong-password"))
	if err == nil {
		t.Error("ReadEncryptedFile should fail with wrong password")
	}

	// Test IsEncryptedFile detection
	if !security.IsEncryptedFile(encryptedPath) {
		t.Error("IsEncryptedFile should return true for .enc files")
	}
}

func TestClientConfig_EncryptedKeyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple encrypted "key" file (not a real SSH key, just for testing the flow)
	encryptedPath := filepath.Join(tmpDir, "test.key.enc")
	masterPassword := []byte("master-secret")
	fakeKey := []byte("fake-key-content")

	err := security.WriteEncryptedFile(encryptedPath, fakeKey, masterPassword)
	if err != nil {
		t.Fatalf("WriteEncryptedFile: %v", err)
	}

	// Test that ClientConfig with MasterPassword is required for .enc files
	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: encryptedPath,
		// MasterPassword not set
	}

	client := NewClient(cfg)
	err = client.Connect(context.Background())
	if err == nil {
		t.Error("Connect should fail without MasterPassword for encrypted key")
	}
	if !strings.Contains(err.Error(), "master password required") {
		t.Errorf("error should mention master password: %v", err)
	}
}

func TestConnectionPool_Integration(t *testing.T) {
	pool := NewPool(PoolConfig{
		MaxPerHost:  3,
		IdleTimeout: 10 * time.Second,
	})
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: "/nonexistent/key", // Will fail to connect
	}

	// Attempt to get connection (will fail due to missing key)
	_, err := pool.Get(context.Background(), cfg)
	if err == nil {
		t.Error("Get should fail with missing key file")
	}

	// Pool should still be usable
	stats := pool.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("expected 0 connections after failed attempt, got %d", stats.TotalConnections)
	}
}

func TestAuditLogging_AllEvents(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	// Log various events
	logger.LogConnect("host:22", "user", "jump:22")
	logger.LogHostKeyAccepted("host:22", "SHA256:abc", true)
	logger.LogCommand("host:22", "ls -la", true, 100*time.Millisecond)
	logger.LogFileOp("host:22", "read", "/var/log/test.log", 1024, nil)
	logger.LogDisconnect("host:22", nil)

	// Parse all events
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 events, got %d", len(lines))
	}

	expectedEvents := []string{"connect", "host_key_accepted", "command", "file_op", "disconnect"}
	for i, line := range lines {
		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d: unmarshal failed: %v", i, err)
			continue
		}
		if event.Event != expectedEvents[i] {
			t.Errorf("line %d: expected event %q, got %q", i, expectedEvents[i], event.Event)
		}
		if event.Timestamp == "" {
			t.Errorf("line %d: timestamp should be set", i)
		}
	}
}

func TestFileHostKeyStore_PersistenceAndRemove(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "known_hosts")

	// Create store and add keys
	store1, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	if err := store1.Add("host1.com:22", key1); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store1.Add("host2.com:22", key2); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Create new store instance and verify persistence
	store2, err := NewFileHostKeyStore(path)
	if err != nil {
		t.Fatalf("NewFileHostKeyStore: %v", err)
	}

	got1, _ := store2.Get("host1.com:22")
	if got1 == nil || Fingerprint(got1) != Fingerprint(key1) {
		t.Error("key1 not persisted correctly")
	}

	got2, _ := store2.Get("host2.com:22")
	if got2 == nil || Fingerprint(got2) != Fingerprint(key2) {
		t.Error("key2 not persisted correctly")
	}

	// Test remove
	if err := store2.Remove("host1.com:22"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify removal persisted
	store3, _ := NewFileHostKeyStore(path)
	got1, _ = store3.Get("host1.com:22")
	if got1 != nil {
		t.Error("host1 should be removed")
	}
	got2, _ = store3.Get("host2.com:22")
	if got2 == nil {
		t.Error("host2 should still exist")
	}
}

func TestCryptoRoundTrip(t *testing.T) {
	passwords := []string{
		"simple",
		"complex-p@ssw0rd!",
		"unicode-密码-пароль",
	}

	data := []string{
		"short",
		strings.Repeat("long data ", 1000),
		"special\x00chars\nand\ttabs",
	}

	for _, password := range passwords {
		for _, plaintext := range data {
			t.Run(password+"/"+plaintext[:min(10, len(plaintext))], func(t *testing.T) {
				encrypted, err := security.Encrypt([]byte(plaintext), []byte(password))
				if err != nil {
					t.Fatalf("Encrypt: %v", err)
				}

				decrypted, err := security.Decrypt(encrypted, []byte(password))
				if err != nil {
					t.Fatalf("Decrypt: %v", err)
				}

				if string(decrypted) != plaintext {
					t.Error("round trip failed")
				}
			})
		}
	}
}
