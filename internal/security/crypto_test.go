package security

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	if len(salt1) != SaltSize {
		t.Errorf("salt size: got %d, want %d", len(salt1), SaltSize)
	}

	// Salts should be unique
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	if bytes.Equal(salt1, salt2) {
		t.Error("salts should be unique")
	}
}

func TestDeriveKey(t *testing.T) {
	password := []byte("test-password")
	salt := []byte("1234567890123456") // 16 bytes

	key := DeriveKey(password, salt)

	if len(key) != KeySizeAES {
		t.Errorf("key size: got %d, want %d", len(key), KeySizeAES)
	}

	// Same password + salt should produce same key
	key2 := DeriveKey(password, salt)
	if !bytes.Equal(key, key2) {
		t.Error("same inputs should produce same key")
	}

	// Different password should produce different key
	key3 := DeriveKey([]byte("different"), salt)
	if bytes.Equal(key, key3) {
		t.Error("different password should produce different key")
	}

	// Different salt should produce different key
	key4 := DeriveKey(password, []byte("6543210987654321"))
	if bytes.Equal(key, key4) {
		t.Error("different salt should produce different key")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		password  string
	}{
		{"short text", "hello", "password123"},
		{"empty text", "", "password123"},
		{"long text", string(make([]byte, 10000)), "password123"},
		{"unicode", "你好世界🌍", "пароль"},
		{"binary data", string([]byte{0, 1, 2, 255, 254, 253}), "pass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plaintext := []byte(tt.plaintext)
			password := []byte(tt.password)

			encrypted, err := Encrypt(plaintext, password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if encrypted == nil {
				t.Fatal("encrypted data is nil")
			}

			if len(encrypted.Salt) != SaltSize {
				t.Errorf("salt size: got %d, want %d", len(encrypted.Salt), SaltSize)
			}

			if len(encrypted.Nonce) != NonceSize {
				t.Errorf("nonce size: got %d, want %d", len(encrypted.Nonce), NonceSize)
			}

			decrypted, err := Decrypt(encrypted, password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("decrypted != plaintext")
			}
		})
	}
}

func TestDecrypt_WrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	password := []byte("correct-password")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, []byte("wrong-password"))
	if err == nil {
		t.Error("Decrypt should fail with wrong password")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	plaintext := []byte("secret data")
	password := []byte("password")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with ciphertext
	if len(encrypted.Ciphertext) > 0 {
		encrypted.Ciphertext[0] ^= 0xFF
	}

	_, err = Decrypt(encrypted, password)
	if err == nil {
		t.Error("Decrypt should fail with tampered ciphertext")
	}
}

func TestDecrypt_TamperedNonce(t *testing.T) {
	plaintext := []byte("secret data")
	password := []byte("password")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with nonce
	encrypted.Nonce[0] ^= 0xFF

	_, err = Decrypt(encrypted, password)
	if err == nil {
		t.Error("Decrypt should fail with tampered nonce")
	}
}

func TestDecrypt_InvalidSaltSize(t *testing.T) {
	data := &EncryptedData{
		Salt:       []byte("short"),
		Nonce:      make([]byte, NonceSize),
		Ciphertext: []byte("data"),
	}

	_, err := Decrypt(data, []byte("password"))
	if err == nil {
		t.Error("Decrypt should fail with invalid salt size")
	}
}

func TestDecrypt_InvalidNonceSize(t *testing.T) {
	data := &EncryptedData{
		Salt:       make([]byte, SaltSize),
		Nonce:      []byte("short"),
		Ciphertext: []byte("data"),
	}

	_, err := Decrypt(data, []byte("password"))
	if err == nil {
		t.Error("Decrypt should fail with invalid nonce size")
	}
}

func TestDecrypt_NilData(t *testing.T) {
	_, err := Decrypt(nil, []byte("password"))
	if err == nil {
		t.Error("Decrypt should fail with nil data")
	}
}

func TestIsEncryptedFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/key.enc", true},
		{"/path/to/key.key.enc", true},
		{"/path/to/key", false},
		{"/path/to/key.key", false},
		{"file.enc", true},
		{".enc", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsEncryptedFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsEncryptedFile(%q): got %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestReadWriteEncryptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.key.enc")
	plaintext := []byte("my-secret-ssh-key-content")
	password := []byte("master-password")

	// Write encrypted file
	err := WriteEncryptedFile(path, plaintext, password)
	if err != nil {
		t.Fatalf("WriteEncryptedFile failed: %v", err)
	}

	// Check file exists with correct permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions: got %o, want 0600", info.Mode().Perm())
	}

	// Read and decrypt
	decrypted, err := ReadEncryptedFile(path, password)
	if err != nil {
		t.Fatalf("ReadEncryptedFile failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted content mismatch")
	}
}

func TestReadEncryptedFile_PlainFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plain.key") // No .enc suffix
	content := []byte("plain-key-content")

	err := os.WriteFile(path, content, 0600)
	if err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// Should return content directly without decryption
	result, err := ReadEncryptedFile(path, nil)
	if err != nil {
		t.Fatalf("ReadEncryptedFile failed: %v", err)
	}

	if !bytes.Equal(result, content) {
		t.Errorf("content mismatch")
	}
}

func TestReadEncryptedFile_NoPassword(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.enc")

	// Write some encrypted-looking content
	err := os.WriteFile(path, []byte(`{"salt":"","nonce":"","ciphertext":""}`), 0600)
	if err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, err = ReadEncryptedFile(path, nil)
	if err == nil {
		t.Error("ReadEncryptedFile should fail without password for .enc file")
	}
}

func TestReadEncryptedFile_WrongPassword(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.enc")
	plaintext := []byte("secret")
	password := []byte("correct")

	err := WriteEncryptedFile(path, plaintext, password)
	if err != nil {
		t.Fatalf("WriteEncryptedFile failed: %v", err)
	}

	_, err = ReadEncryptedFile(path, []byte("wrong"))
	if err == nil {
		t.Error("ReadEncryptedFile should fail with wrong password")
	}
}

func TestWriteEncryptedFile_NoPassword(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.enc")

	err := WriteEncryptedFile(path, []byte("data"), nil)
	if err == nil {
		t.Error("WriteEncryptedFile should fail without password")
	}
}

func TestWriteEncryptedFile_AddsEncSuffix(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.key") // No .enc suffix

	err := WriteEncryptedFile(path, []byte("data"), []byte("password"))
	if err != nil {
		t.Fatalf("WriteEncryptedFile failed: %v", err)
	}

	// Should create file with .enc suffix
	expectedPath := path + ".enc"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("file should be created at %s", expectedPath)
	}
}
