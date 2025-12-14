package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// ServerTLSConfig holds server TLS configuration.
type ServerTLSConfig struct {
	CertFile     string // Server certificate file
	KeyFile      string // Server private key file
	ClientCAFile string // CA certificate for verifying client certs
}

// ClientTLSConfig holds client TLS configuration.
type ClientTLSConfig struct {
	CertFile           string // Client certificate file
	KeyFile            string // Client private key file
	CAFile             string // CA certificate for verifying server cert
	InsecureSkipVerify bool   // Skip server certificate verification (dev only)
}

// LoadServerTLS loads TLS credentials for the gRPC server.
// Requires client certificate verification (mTLS).
func LoadServerTLS(cfg *ServerTLSConfig) (credentials.TransportCredentials, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read client CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add client CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// LoadClientTLS loads TLS credentials for the gRPC client.
// Verifies server certificate and presents client certificate for mTLS.
func LoadClientTLS(cfg *ClientTLSConfig) (credentials.TransportCredentials, error) {
	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS13,
	}

	// Load CA certificate for server verification (unless skipping)
	if !cfg.InsecureSkipVerify {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}

	return credentials.NewTLS(tlsConfig), nil
}
