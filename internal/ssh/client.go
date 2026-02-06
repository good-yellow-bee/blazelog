// Package ssh provides SSH client functionality for remote log collection.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/security"
	"golang.org/x/crypto/ssh"
)

// shellQuote returns a shell-safe quoted string using single quotes.
// Single quotes prevent all shell expansion ($, `, \, etc.).
// Any embedded single quotes are escaped via the pattern: 'text'"'"'more'.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// ClientConfig holds SSH client configuration.
type ClientConfig struct {
	// Host is the SSH server address (host:port).
	Host string
	// User is the SSH username.
	User string
	// KeyFile is the path to the private key file.
	// If the file has .enc suffix, it will be decrypted using MasterPassword.
	KeyFile string
	// KeyPassphrase is the optional passphrase for encrypted keys.
	KeyPassphrase string
	// MasterPassword is the password for decrypting .enc key files.
	MasterPassword string
	// Password is used for password authentication (not recommended).
	Password string
	// Timeout is the connection timeout.
	Timeout time.Duration
	// KeepAliveInterval is the interval for keep-alive probes.
	KeepAliveInterval time.Duration

	// JumpHost is an optional bastion/jump host for proxying connections.
	JumpHost *ClientConfig
	// HostKeyCallback is the callback for host key verification.
	// If nil and InsecureIgnoreHostKey is false, connection will fail.
	HostKeyCallback ssh.HostKeyCallback
	// InsecureIgnoreHostKey skips host key verification (NOT RECOMMENDED).
	// Only use for development/testing. A warning will be logged.
	InsecureIgnoreHostKey bool
	// AuditLogger is the optional audit logger for SSH operations.
	AuditLogger AuditLogger
}

// Client is an SSH client for remote file operations.
type Client struct {
	config     *ClientConfig
	sshClient  *ssh.Client
	jumpClient *Client // connection to jump host
	mu         sync.Mutex
	connected  bool
	lastError  error
	closeCh    chan struct{}
	closeOnce  sync.Once
}

// NewClient creates a new SSH client with the given configuration.
func NewClient(cfg *ClientConfig) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.KeepAliveInterval == 0 {
		cfg.KeepAliveInterval = 30 * time.Second
	}
	if cfg.AuditLogger == nil {
		cfg.AuditLogger = NopAuditLogger{}
	}

	return &Client{
		config:  cfg,
		closeCh: make(chan struct{}),
	}
}

// Connect establishes an SSH connection to the remote server.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	authMethods, err := c.buildAuthMethods()
	if err != nil {
		return fmt.Errorf("build auth methods: %w", err)
	}

	hostKeyCallback := c.config.HostKeyCallback
	if hostKeyCallback == nil {
		if c.config.InsecureIgnoreHostKey {
			log.Printf("WARNING: SSH host key verification disabled for %s - vulnerable to MITM attacks", c.config.Host)
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		} else {
			return fmt.Errorf("host key verification required: set HostKeyCallback or enable InsecureIgnoreHostKey (not recommended)")
		}
	}

	sshConfig := &ssh.ClientConfig{
		User:            c.config.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.config.Timeout,
	}

	// Determine jump host info for audit logging
	jumpHostStr := ""
	if c.config.JumpHost != nil {
		jumpHostStr = c.config.JumpHost.Host
	}

	var conn net.Conn

	// Connect through jump host if configured
	if c.config.JumpHost != nil {
		jumpClient := NewClient(c.config.JumpHost)
		if err := jumpClient.Connect(ctx); err != nil {
			return fmt.Errorf("connect to jump host %s: %w", c.config.JumpHost.Host, err)
		}
		c.jumpClient = jumpClient

		// Dial target through jump host
		conn, err = jumpClient.sshClient.Dial("tcp", c.config.Host)
		if err != nil {
			jumpClient.Close()
			c.jumpClient = nil
			return fmt.Errorf("dial %s through jump host: %w", c.config.Host, err)
		}
	} else {
		// Direct connection
		dialer := net.Dialer{Timeout: c.config.Timeout}
		conn, err = dialer.DialContext(ctx, "tcp", c.config.Host)
		if err != nil {
			return fmt.Errorf("dial %s: %w", c.config.Host, err)
		}
	}

	// Log connection attempt
	c.config.AuditLogger.LogConnect(c.config.Host, c.config.User, jumpHostStr)

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, c.config.Host, sshConfig)
	if err != nil {
		conn.Close()
		if c.jumpClient != nil {
			c.jumpClient.Close()
			c.jumpClient = nil
		}
		return fmt.Errorf("ssh handshake: %w", err)
	}

	c.sshClient = ssh.NewClient(sshConn, chans, reqs)
	c.connected = true
	c.lastError = nil

	// Start keep-alive routine
	go c.keepAlive()

	return nil
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})

	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error

	if c.connected && c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil {
			firstErr = err
		}
		c.config.AuditLogger.LogDisconnect(c.config.Host, firstErr)
	}

	// Close jump host connection
	if c.jumpClient != nil {
		if err := c.jumpClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.jumpClient = nil
	}

	c.connected = false
	c.sshClient = nil
	return firstErr
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// LastError returns the last connection error.
func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// Host returns the host address.
func (c *Client) Host() string {
	return c.config.Host
}

// ReadFile reads a remote file and returns its contents.
func (c *Client) ReadFile(ctx context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	start := time.Now()
	session, err := client.NewSession()
	if err != nil {
		c.handleError(err)
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Use cat to read file contents (shellQuote prevents command injection)
	cmd := fmt.Sprintf("cat %s", shellQuote(path))
	output, err := session.Output(cmd)
	duration := time.Since(start)

	c.config.AuditLogger.LogCommand(c.config.Host, cmd, err == nil, duration)

	if err != nil {
		c.config.AuditLogger.LogFileOp(c.config.Host, "read", path, 0, err)
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	c.config.AuditLogger.LogFileOp(c.config.Host, "read", path, int64(len(output)), nil)
	return output, nil
}

// ReadFileRange reads a portion of a remote file starting at offset.
func (c *Client) ReadFileRange(ctx context.Context, path string, offset, limit int64) ([]byte, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	start := time.Now()
	session, err := client.NewSession()
	if err != nil {
		c.handleError(err)
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Use tail with byte offset to read from specific position (shellQuote prevents command injection)
	cmd := fmt.Sprintf("tail -c +%d %s | head -c %d", offset+1, shellQuote(path), limit)
	output, err := session.Output(cmd)
	duration := time.Since(start)

	c.config.AuditLogger.LogCommand(c.config.Host, cmd, err == nil, duration)

	if err != nil {
		c.config.AuditLogger.LogFileOp(c.config.Host, "read_range", path, 0, err)
		return nil, fmt.Errorf("read file range %s: %w", path, err)
	}

	c.config.AuditLogger.LogFileOp(c.config.Host, "read_range", path, int64(len(output)), nil)
	return output, nil
}

// FileInfo returns information about a remote file.
func (c *Client) FileInfo(ctx context.Context, path string) (*RemoteFileInfo, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	start := time.Now()
	session, err := client.NewSession()
	if err != nil {
		c.handleError(err)
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Use stat to get file info (size, mtime, inode) - shellQuote prevents command injection
	cmd := fmt.Sprintf("stat -c '%%s %%Y %%i' %s 2>/dev/null || stat -f '%%z %%m %%i' %s", shellQuote(path), shellQuote(path))
	output, err := session.Output(cmd)
	duration := time.Since(start)

	c.config.AuditLogger.LogCommand(c.config.Host, "stat", err == nil, duration)

	if err != nil {
		c.config.AuditLogger.LogFileOp(c.config.Host, "stat", path, 0, err)
		return nil, fmt.Errorf("stat file %s: %w", path, err)
	}

	info := &RemoteFileInfo{Path: path}
	_, err = fmt.Sscanf(string(output), "%d %d %d", &info.Size, &info.ModTime, &info.Inode)
	if err != nil {
		return nil, fmt.Errorf("parse stat output: %w", err)
	}

	c.config.AuditLogger.LogFileOp(c.config.Host, "stat", path, 0, nil)
	return info, nil
}

// ListFiles lists files matching a glob pattern.
func (c *Client) ListFiles(ctx context.Context, pattern string) ([]string, error) {
	// Validate pattern to prevent command injection
	// Only allow safe glob characters: alphanumeric, /, ., -, _, *, ?, [, ]
	if err := validateGlobPattern(pattern); err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	start := time.Now()
	session, err := client.NewSession()
	if err != nil {
		c.handleError(err)
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Use ls with glob pattern - pattern is validated above to be safe
	// We use sh -c with single quotes and escape any single quotes in pattern
	escapedPattern := strings.ReplaceAll(pattern, "'", "'\"'\"'")
	cmd := fmt.Sprintf("sh -c 'ls -1 %s 2>/dev/null || true'", escapedPattern)
	output, err := session.Output(cmd)
	duration := time.Since(start)

	c.config.AuditLogger.LogCommand(c.config.Host, cmd, err == nil, duration)

	if err != nil {
		c.config.AuditLogger.LogFileOp(c.config.Host, "list", pattern, 0, err)
		return nil, fmt.Errorf("list files %s: %w", pattern, err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	// Parse output into file list
	var files []string
	for _, line := range splitLines(output) {
		if line != "" {
			files = append(files, line)
		}
	}

	c.config.AuditLogger.LogFileOp(c.config.Host, "list", pattern, int64(len(files)), nil)
	return files, nil
}

// NewSession creates a new SSH session for command execution.
func (c *Client) NewSession() (*ssh.Session, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	return client.NewSession()
}

// StreamFile opens a stream to read a remote file continuously.
func (c *Client) StreamFile(ctx context.Context, path string, offset int64) (io.ReadCloser, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := c.sshClient
	c.mu.Unlock()

	session, err := client.NewSession()
	if err != nil {
		c.handleError(err)
		return nil, fmt.Errorf("create session: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("get stdout pipe: %w", err)
	}

	// Use tail -f to stream file with optional offset (shellQuote prevents command injection)
	var cmd string
	if offset > 0 {
		cmd = fmt.Sprintf("tail -c +%d -f %s", offset+1, shellQuote(path))
	} else {
		cmd = fmt.Sprintf("tail -f %s", shellQuote(path))
	}

	if err := session.Start(cmd); err != nil {
		session.Close()
		return nil, fmt.Errorf("start tail: %w", err)
	}

	c.config.AuditLogger.LogFileOp(c.config.Host, "stream", path, 0, nil)

	return &sessionReader{
		session: session,
		reader:  stdout,
	}, nil
}

func (c *Client) buildAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try key authentication first
	if c.config.KeyFile != "" {
		var keyData []byte
		var err error

		// Check if key file is encrypted (.enc suffix)
		if security.IsEncryptedFile(c.config.KeyFile) {
			if c.config.MasterPassword == "" {
				return nil, fmt.Errorf("master password required for encrypted key file")
			}
			keyData, err = security.ReadEncryptedFile(c.config.KeyFile, []byte(c.config.MasterPassword))
		} else {
			keyData, err = os.ReadFile(c.config.KeyFile)
		}
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}

		var signer ssh.Signer
		if c.config.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(c.config.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}

		methods = append(methods, ssh.PublicKeys(signer))
	}

	// Fallback to password authentication
	if c.config.Password != "" {
		methods = append(methods, ssh.Password(c.config.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication methods configured")
	}

	return methods, nil
}

func (c *Client) keepAlive() {
	ticker := time.NewTicker(c.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			if !c.connected || c.sshClient == nil {
				c.mu.Unlock()
				return
			}
			client := c.sshClient
			c.mu.Unlock()

			// Send keep-alive request
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				c.handleError(err)
			}
		}
	}
}

func (c *Client) handleError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastError = err
	// Mark as disconnected on connection errors
	if isConnectionError(err) {
		c.connected = false
	}
}

// RemoteFileInfo contains information about a remote file.
type RemoteFileInfo struct {
	Path    string
	Size    int64
	ModTime int64
	Inode   int64
}

// sessionReader wraps a session and reader for streaming.
type sessionReader struct {
	session *ssh.Session
	reader  io.Reader
}

func (r *sessionReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *sessionReader) Close() error {
	// Signal the session to stop (sends SIGTERM to remote process)
	r.session.Signal(ssh.SIGTERM)
	return r.session.Close()
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common connection errors
	if errors.Is(err, io.EOF) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// Check for SSH-specific errors indicating connection loss
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") {
		return true
	}
	return false
}

// validateGlobPattern checks that a glob pattern contains only safe characters.
// This prevents command injection when the pattern is used in shell commands.
// Allowed characters: alphanumeric, /, ., -, _, *, ?, [, ], space
func validateGlobPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}

	for i, r := range pattern {
		if !isAllowedGlobChar(r) {
			return fmt.Errorf("invalid character %q at position %d", r, i)
		}
	}

	// Check for dangerous patterns
	if strings.Contains(pattern, "..") {
		return fmt.Errorf("path traversal not allowed")
	}

	return nil
}

// isAllowedGlobChar returns true if the rune is allowed in glob patterns.
func isAllowedGlobChar(r rune) bool {
	// Alphanumeric
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
		return true
	}
	// Safe special characters for glob patterns
	switch r {
	case '/', '.', '-', '_', '*', '?', '[', ']', ' ':
		return true
	}
	return false
}
