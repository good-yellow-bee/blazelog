// Package security provides mTLS certificate management for BlazeLog.
package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultCAValidDays is the default CA certificate validity (10 years).
	DefaultCAValidDays = 3650
	// DefaultCertValidDays is the default certificate validity (1 year).
	DefaultCertValidDays = 365
	// KeySize is the RSA key size in bits.
	KeySize = 4096
)

// GenerateCA generates a new CA certificate and private key.
// The certificate and key are written to outputDir as ca.crt and ca.key.
func GenerateCA(outputDir string, validDays int) error {
	if validDays <= 0 {
		validDays = DefaultCAValidDays
	}

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return fmt.Errorf("generate private key: %w", err)
	}

	// Create CA certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"BlazeLog"},
			CommonName:   "BlazeLog CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validDays),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Self-sign the CA certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Write CA certificate
	certPath := filepath.Join(outputDir, "ca.crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode certificate: %w", err)
	}

	// Write CA private key with restrictive permissions
	keyPath := filepath.Join(outputDir, "ca.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer keyFile.Close()

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("encode private key: %w", err)
	}

	return nil
}

// LoadCA loads a CA certificate and private key from the given directory.
func LoadCA(caDir string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Load CA certificate
	certPath := filepath.Join(caDir, "ca.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("invalid CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	// Load CA private key
	keyPath := filepath.Join(caDir, "ca.key")
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA private key: %w", err)
	}

	block, _ = pem.Decode(keyPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, nil, fmt.Errorf("invalid CA private key PEM")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA private key: %w", err)
	}

	return caCert, caKey, nil
}
