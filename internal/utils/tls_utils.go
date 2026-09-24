package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// SetupTLS configures TLS with certificate pinning.
func SetupTLS(certPath string) (*tls.Config, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certPEM) {
		return nil, fmt.Errorf("failed to parse certificate")
	}

	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: false, // Verify certificate
	}, nil
}
