package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// GenerateServerCert generates a server certificate signed by the CA.
// The certificate includes localhost and the provided hosts in the SAN.
func GenerateServerCert(caDir, name, outputDir string, validDays int, hosts []string) error {
	return generateCert(caDir, name, outputDir, validDays, hosts, true)
}

// GenerateAgentCert generates an agent certificate signed by the CA.
// The certificate includes the local hostname in the SAN.
func GenerateAgentCert(caDir, name, outputDir string, validDays int) error {
	hostname, _ := os.Hostname()
	hosts := []string{hostname}
	return generateCert(caDir, name, outputDir, validDays, hosts, false)
}

func generateCert(caDir, name, outputDir string, validDays int, hosts []string, isServer bool) error {
	if validDays <= 0 {
		validDays = DefaultCertValidDays
	}

	// Load CA
	caCert, caKey, err := LoadCA(caDir)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}

	// Generate key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return fmt.Errorf("generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"BlazeLog"},
			CommonName:   name,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(0, 0, validDays),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	// Set extended key usage based on cert type
	if isServer {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		// Server certs always include localhost
		hosts = appendUnique(hosts, "localhost", "127.0.0.1", "::1")
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	// Add SANs (Subject Alternative Names)
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// Sign with CA
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Write certificate
	certPath := filepath.Join(outputDir, name+".crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode certificate: %w", err)
	}

	// Write private key
	keyPath := filepath.Join(outputDir, name+".key")
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

func appendUnique(slice []string, items ...string) []string {
	seen := make(map[string]bool)
	for _, s := range slice {
		seen[s] = true
	}
	for _, item := range items {
		if !seen[item] {
			slice = append(slice, item)
			seen[item] = true
		}
	}
	return slice
}
