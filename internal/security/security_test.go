package security

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	tmpDir := t.TempDir()

	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Check ca.crt exists
	certPath := filepath.Join(tmpDir, "ca.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read ca.crt: %v", err)
	}

	// Check ca.key exists
	keyPath := filepath.Join(tmpDir, "ca.key")
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read ca.key: %v", err)
	}

	// Verify certificate is valid
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if !cert.IsCA {
		t.Error("certificate is not a CA")
	}

	if cert.Subject.CommonName != "BlazeLog CA" {
		t.Errorf("unexpected CN: got %s, want BlazeLog CA", cert.Subject.CommonName)
	}

	// Verify key is valid
	block, _ = pem.Decode(keyPEM)
	if block == nil {
		t.Fatal("failed to decode key PEM")
	}

	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// Check key file permissions
	keyInfo, _ := os.Stat(keyPath)
	if keyInfo.Mode().Perm() != 0600 {
		t.Errorf("ca.key permissions: got %o, want 0600", keyInfo.Mode().Perm())
	}
}

func TestLoadCA(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA first
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Load CA
	cert, key, err := LoadCA(tmpDir)
	if err != nil {
		t.Fatalf("LoadCA failed: %v", err)
	}

	if !cert.IsCA {
		t.Error("loaded certificate is not a CA")
	}

	if key == nil {
		t.Error("loaded key is nil")
	}
}

func TestGenerateServerCert(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA first
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Generate server cert
	hosts := []string{"blazelog.local", "192.168.1.100"}
	err = GenerateServerCert(tmpDir, "server", tmpDir, 365, hosts)
	if err != nil {
		t.Fatalf("GenerateServerCert failed: %v", err)
	}

	// Check server.crt exists
	certPath := filepath.Join(tmpDir, "server.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read server.crt: %v", err)
	}

	// Verify certificate
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if cert.Subject.CommonName != "server" {
		t.Errorf("unexpected CN: got %s, want server", cert.Subject.CommonName)
	}

	// Check SANs include localhost
	if !containsString(cert.DNSNames, "localhost") {
		t.Error("DNS names should include localhost")
	}
	if !containsString(cert.DNSNames, "blazelog.local") {
		t.Error("DNS names should include blazelog.local")
	}

	// Check ExtKeyUsage
	if len(cert.ExtKeyUsage) == 0 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Error("server cert should have ServerAuth extended key usage")
	}
}

func TestGenerateAgentCert(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA first
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Generate agent cert
	err = GenerateAgentCert(tmpDir, "agent1", tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateAgentCert failed: %v", err)
	}

	// Check agent1.crt exists
	certPath := filepath.Join(tmpDir, "agent1.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read agent1.crt: %v", err)
	}

	// Verify certificate
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if cert.Subject.CommonName != "agent1" {
		t.Errorf("unexpected CN: got %s, want agent1", cert.Subject.CommonName)
	}

	// Check ExtKeyUsage
	if len(cert.ExtKeyUsage) == 0 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Error("agent cert should have ClientAuth extended key usage")
	}
}

func TestLoadServerTLS(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA and server cert
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	err = GenerateServerCert(tmpDir, "server", tmpDir, 365, nil)
	if err != nil {
		t.Fatalf("GenerateServerCert failed: %v", err)
	}

	// Load server TLS
	cfg := &ServerTLSConfig{
		CertFile:     filepath.Join(tmpDir, "server.crt"),
		KeyFile:      filepath.Join(tmpDir, "server.key"),
		ClientCAFile: filepath.Join(tmpDir, "ca.crt"),
	}

	creds, err := LoadServerTLS(cfg)
	if err != nil {
		t.Fatalf("LoadServerTLS failed: %v", err)
	}

	if creds == nil {
		t.Error("credentials should not be nil")
	}
}

func TestLoadClientTLS(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA and agent cert
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	err = GenerateAgentCert(tmpDir, "agent1", tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateAgentCert failed: %v", err)
	}

	// Load client TLS
	cfg := &ClientTLSConfig{
		CertFile: filepath.Join(tmpDir, "agent1.crt"),
		KeyFile:  filepath.Join(tmpDir, "agent1.key"),
		CAFile:   filepath.Join(tmpDir, "ca.crt"),
	}

	creds, err := LoadClientTLS(cfg)
	if err != nil {
		t.Fatalf("LoadClientTLS failed: %v", err)
	}

	if creds == nil {
		t.Error("credentials should not be nil")
	}
}

func TestLoadClientTLS_InsecureSkipVerify(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate CA and agent cert
	err := GenerateCA(tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	err = GenerateAgentCert(tmpDir, "agent1", tmpDir, 365)
	if err != nil {
		t.Fatalf("GenerateAgentCert failed: %v", err)
	}

	// Load client TLS with insecure skip verify
	cfg := &ClientTLSConfig{
		CertFile:           filepath.Join(tmpDir, "agent1.crt"),
		KeyFile:            filepath.Join(tmpDir, "agent1.key"),
		InsecureSkipVerify: true,
	}

	creds, err := LoadClientTLS(cfg)
	if err != nil {
		t.Fatalf("LoadClientTLS failed: %v", err)
	}

	if creds == nil {
		t.Error("credentials should not be nil")
	}
}

func TestLoadServerTLS_MissingCert(t *testing.T) {
	cfg := &ServerTLSConfig{
		CertFile:     "/nonexistent/server.crt",
		KeyFile:      "/nonexistent/server.key",
		ClientCAFile: "/nonexistent/ca.crt",
	}

	_, err := LoadServerTLS(cfg)
	if err == nil {
		t.Error("LoadServerTLS should fail with missing cert")
	}
}

func TestLoadClientTLS_MissingCert(t *testing.T) {
	cfg := &ClientTLSConfig{
		CertFile: "/nonexistent/agent.crt",
		KeyFile:  "/nonexistent/agent.key",
		CAFile:   "/nonexistent/ca.crt",
	}

	_, err := LoadClientTLS(cfg)
	if err == nil {
		t.Error("LoadClientTLS should fail with missing cert")
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
