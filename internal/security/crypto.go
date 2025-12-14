package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// SaltSize is the size of the salt in bytes.
	SaltSize = 16
	// NonceSize is the size of the GCM nonce in bytes.
	NonceSize = 12
	// KeySizeAES is the AES-256 key size in bytes.
	KeySizeAES = 32
	// PBKDF2Iterations is the number of PBKDF2 iterations.
	PBKDF2Iterations = 100000
	// EncryptedFileSuffix indicates an encrypted file.
	EncryptedFileSuffix = ".enc"
)

// EncryptedData holds the components needed to decrypt data.
type EncryptedData struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// GenerateSalt generates a cryptographically secure random salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// DeriveKey derives an AES-256 key from a password and salt using PBKDF2.
func DeriveKey(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, PBKDF2Iterations, KeySizeAES, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM with a key derived from password.
func Encrypt(plaintext, password []byte) (*EncryptedData, error) {
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}

	key := DeriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// Decrypt decrypts data using AES-256-GCM with a key derived from password.
func Decrypt(data *EncryptedData, password []byte) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("encrypted data is nil")
	}

	if len(data.Salt) != SaltSize {
		return nil, fmt.Errorf("invalid salt size: got %d, want %d", len(data.Salt), SaltSize)
	}

	if len(data.Nonce) != NonceSize {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(data.Nonce), NonceSize)
	}

	key := DeriveKey(password, data.Salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, data.Nonce, data.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// IsEncryptedFile returns true if the path has the encrypted file suffix.
func IsEncryptedFile(path string) bool {
	return strings.HasSuffix(path, EncryptedFileSuffix)
}

// ReadEncryptedFile reads and decrypts a file if it has .enc suffix.
// For non-encrypted files, it returns the contents directly.
func ReadEncryptedFile(path string, password []byte) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if !IsEncryptedFile(path) {
		return content, nil
	}

	if len(password) == 0 {
		return nil, fmt.Errorf("password required for encrypted file")
	}

	var data EncryptedData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("parse encrypted file: %w", err)
	}

	return Decrypt(&data, password)
}

// WriteEncryptedFile encrypts and writes data to a file with .enc suffix.
// The file is written with restrictive permissions (0600).
func WriteEncryptedFile(path string, plaintext, password []byte) error {
	if !IsEncryptedFile(path) {
		path = path + EncryptedFileSuffix
	}

	if len(password) == 0 {
		return fmt.Errorf("password required for encryption")
	}

	data, err := Encrypt(plaintext, password)
	if err != nil {
		return err
	}

	content, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal encrypted data: %w", err)
	}

	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
