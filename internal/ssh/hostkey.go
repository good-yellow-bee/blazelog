package ssh

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// HostKeyPolicy defines how unknown host keys are handled.
type HostKeyPolicy int

const (
	// PolicyStrict rejects connections to unknown hosts.
	PolicyStrict HostKeyPolicy = iota
	// PolicyTOFU (Trust On First Use) accepts unknown hosts and stores their keys.
	PolicyTOFU
	// PolicyWarn accepts all hosts but logs warnings (dev only).
	PolicyWarn
)

// String returns the string representation of the policy.
func (p HostKeyPolicy) String() string {
	switch p {
	case PolicyStrict:
		return "strict"
	case PolicyTOFU:
		return "tofu"
	case PolicyWarn:
		return "warn"
	default:
		return "unknown"
	}
}

// ParseHostKeyPolicy parses a policy string.
func ParseHostKeyPolicy(s string) (HostKeyPolicy, error) {
	switch strings.ToLower(s) {
	case "strict":
		return PolicyStrict, nil
	case "tofu":
		return PolicyTOFU, nil
	case "warn":
		return PolicyWarn, nil
	default:
		return PolicyStrict, fmt.Errorf("unknown host key policy: %s", s)
	}
}

// ErrHostKeyMismatch is returned when a host key doesn't match the stored key.
var ErrHostKeyMismatch = errors.New("host key mismatch (possible MITM attack)")

// ErrUnknownHost is returned when connecting to an unknown host in strict mode.
var ErrUnknownHost = errors.New("unknown host (not in known_hosts)")

// HostKeyStore manages known host keys.
type HostKeyStore interface {
	// Get returns the stored public key for a host.
	Get(host string) (ssh.PublicKey, error)
	// Add stores a public key for a host.
	Add(host string, key ssh.PublicKey) error
	// Remove removes a host from the store.
	Remove(host string) error
	// List returns all stored host keys.
	List() (map[string]ssh.PublicKey, error)
}

// FileHostKeyStore stores host keys in an OpenSSH-compatible known_hosts file.
type FileHostKeyStore struct {
	path string
	mu   sync.RWMutex
}

// NewFileHostKeyStore creates a new file-based host key store.
func NewFileHostKeyStore(path string) (*FileHostKeyStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create known_hosts directory: %w", err)
	}

	return &FileHostKeyStore{path: path}, nil
}

// Get returns the stored public key for a host.
func (s *FileHostKeyStore) Get(host string) (ssh.PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host = normalizeHost(host)

	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		//nolint:nilnil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		storedHost, key, err := parseKnownHostsLine(line)
		if err != nil {
			continue // Skip malformed lines
		}

		if storedHost == host {
			return key, nil
		}
	}

	return nil, scanner.Err()
}

// Add stores a public key for a host.
func (s *FileHostKeyStore) Add(host string, key ssh.PublicKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	host = normalizeHost(host)

	// Check if already exists
	existing, err := s.getUnlocked(host)
	if err != nil {
		return err
	}
	if existing != nil {
		// Key already stored - check if it matches
		if ssh.FingerprintSHA256(existing) == ssh.FingerprintSHA256(key) {
			return nil // Same key, nothing to do
		}
		return fmt.Errorf("host %s already has a different key stored", host)
	}

	// Append to file
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	line := formatKnownHostsLine(host, key)
	if _, err := file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}

	return nil
}

// Remove removes a host from the store.
func (s *FileHostKeyStore) Remove(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	host = normalizeHost(host)

	content, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read known_hosts: %w", err)
	}

	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		storedHost, _, err := parseKnownHostsLine(line)
		if err != nil || storedHost != host {
			newLines = append(newLines, line)
		}
	}

	if err := os.WriteFile(s.path, []byte(strings.Join(newLines, "\n")+"\n"), 0600); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}

	return nil
}

// List returns all stored host keys.
func (s *FileHostKeyStore) List() (map[string]ssh.PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ssh.PublicKey)

	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		host, key, err := parseKnownHostsLine(line)
		if err != nil {
			continue
		}
		result[host] = key
	}

	return result, scanner.Err()
}

func (s *FileHostKeyStore) getUnlocked(host string) (ssh.PublicKey, error) {
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		//nolint:nilnil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		storedHost, key, err := parseKnownHostsLine(line)
		if err != nil {
			continue
		}

		if storedHost == host {
			return key, nil
		}
	}

	return nil, scanner.Err()
}

// parseKnownHostsLine parses a line from known_hosts file.
// Format: hostname key-type base64-key [comment]
func parseKnownHostsLine(line string) (string, ssh.PublicKey, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", nil, fmt.Errorf("invalid known_hosts line")
	}

	host := fields[0]
	keyType := fields[1]
	keyData := fields[2]

	keyBytes, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return "", nil, fmt.Errorf("decode key: %w", err)
	}

	key, err := ssh.ParsePublicKey(keyBytes)
	if err != nil {
		return "", nil, fmt.Errorf("parse key: %w", err)
	}

	// Verify key type matches
	if key.Type() != keyType {
		return "", nil, fmt.Errorf("key type mismatch")
	}

	return host, key, nil
}

// formatKnownHostsLine formats a known_hosts line.
func formatKnownHostsLine(host string, key ssh.PublicKey) string {
	keyData := base64.StdEncoding.EncodeToString(key.Marshal())
	return fmt.Sprintf("%s %s %s", host, key.Type(), keyData)
}

// normalizeHost normalizes a host string for consistent storage.
func normalizeHost(host string) string {
	// Handle IPv6 addresses with brackets: [::1]:22 -> ::1:22
	if strings.HasPrefix(host, "[") {
		// Find the closing bracket
		if idx := strings.Index(host, "]"); idx != -1 {
			ipv6 := host[1:idx]
			port := ""
			if idx+1 < len(host) && host[idx+1] == ':' {
				port = host[idx+2:]
			}
			if port == "" {
				port = "22"
			}
			return strings.ToLower(ipv6 + ":" + port)
		}
	}

	// Ensure port is included (default to 22)
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	return strings.ToLower(host)
}

// Fingerprint returns the SHA256 fingerprint of a public key.
func Fingerprint(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
}

// FingerprintMD5 returns the MD5 fingerprint of a public key (legacy format).
func FingerprintMD5(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return fmt.Sprintf("%x", hash)
}

// NewHostKeyCallback creates an ssh.HostKeyCallback with the specified policy.
func NewHostKeyCallback(store HostKeyStore, policy HostKeyPolicy, audit AuditLogger) ssh.HostKeyCallback {
	if audit == nil {
		audit = NopAuditLogger{}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Use the hostname:port format
		host := hostname
		if addr, ok := remote.(*net.TCPAddr); ok && !strings.Contains(hostname, ":") {
			host = fmt.Sprintf("%s:%d", hostname, addr.Port)
		}
		host = normalizeHost(host)

		fingerprint := Fingerprint(key)

		storedKey, err := store.Get(host)
		if err != nil {
			return fmt.Errorf("check known_hosts: %w", err)
		}

		// Unknown host
		if storedKey == nil {
			switch policy {
			case PolicyStrict:
				audit.LogHostKeyRejected(host, "(none)", fingerprint)
				return ErrUnknownHost

			case PolicyTOFU:
				if err := store.Add(host, key); err != nil {
					return fmt.Errorf("store host key: %w", err)
				}
				audit.LogHostKeyAccepted(host, fingerprint, true)
				return nil

			case PolicyWarn:
				audit.LogHostKeyAccepted(host, fingerprint, true)
				return nil
			}
		}

		// Known host - verify key matches
		storedFingerprint := Fingerprint(storedKey)
		if storedFingerprint != fingerprint {
			audit.LogHostKeyRejected(host, storedFingerprint, fingerprint)
			return fmt.Errorf("%w: expected %s, got %s", ErrHostKeyMismatch, storedFingerprint, fingerprint)
		}

		audit.LogHostKeyAccepted(host, fingerprint, false)
		return nil
	}
}

// MemoryHostKeyStore is an in-memory host key store for testing.
type MemoryHostKeyStore struct {
	keys map[string]ssh.PublicKey
	mu   sync.RWMutex
}

// NewMemoryHostKeyStore creates a new in-memory host key store.
func NewMemoryHostKeyStore() *MemoryHostKeyStore {
	return &MemoryHostKeyStore{keys: make(map[string]ssh.PublicKey)}
}

func (s *MemoryHostKeyStore) Get(host string) (ssh.PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keys[normalizeHost(host)], nil
}

func (s *MemoryHostKeyStore) Add(host string, key ssh.PublicKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[normalizeHost(host)] = key
	return nil
}

func (s *MemoryHostKeyStore) Remove(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, normalizeHost(host))
	return nil
}

func (s *MemoryHostKeyStore) List() (map[string]ssh.PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]ssh.PublicKey, len(s.keys))
	for k, v := range s.keys {
		result[k] = v
	}
	return result, nil
}
