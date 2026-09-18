package app

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/shurco/mycart/pkg/logging"
)

// TestGenerateSelfSignedCert_DevMode tests self-signed certificate generation in dev mode
func TestGenerateSelfSignedCert_DevMode(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	// Arrange
	domain := "localhost"
	certDir := t.TempDir()

	// Act
	err := generateSelfSignedCert(domain, certDir)

	// Assert
	if err != nil {
		t.Fatalf("generateSelfSignedCert failed: %v", err)
	}

	// Verify fullchain.pem exists
	fullchainPath := filepath.Join(certDir, "fullchain.pem")
	if _, err := os.Stat(fullchainPath); os.IsNotExist(err) {
		t.Errorf("fullchain.pem was not created")
	}

	// Verify privkey.pem exists
	privkeyPath := filepath.Join(certDir, "privkey.pem")
	if _, err := os.Stat(privkeyPath); os.IsNotExist(err) {
		t.Errorf("privkey.pem was not created")
	}

	// Verify fullchain.pem is valid PEM format
	certPEM, err := os.ReadFile(fullchainPath)
	if err != nil {
		t.Fatalf("failed to read fullchain.pem: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("failed to decode PEM block from fullchain.pem")
	}

	if block.Type != "CERTIFICATE" {
		t.Errorf("expected PEM type CERTIFICATE, got %s", block.Type)
	}

	// Verify certificate can be parsed
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Verify certificate fields
	if cert.Subject.CommonName != domain {
		t.Errorf("expected CommonName %s, got %s", domain, cert.Subject.CommonName)
	}

	if len(cert.DNSNames) == 0 || cert.DNSNames[0] != domain {
		t.Errorf("expected DNSNames to contain %s, got %v", domain, cert.DNSNames)
	}

	// Verify privkey.pem is valid PEM format
	keyPEM, err := os.ReadFile(privkeyPath)
	if err != nil {
		t.Fatalf("failed to read privkey.pem: %v", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatalf("failed to decode PEM block from privkey.pem")
	}

	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("expected PEM type EC PRIVATE KEY, got %s", keyBlock.Type)
	}

	// Verify private key can be parsed
	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse EC private key: %v", err)
	}
}

// TestGenerateSelfSignedCert_FilePermissions tests that cert files have correct permissions
func TestGenerateSelfSignedCert_FilePermissions(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	// Arrange
	domain := "test.example.com"
	certDir := t.TempDir()

	// Act
	err := generateSelfSignedCert(domain, certDir)
	if err != nil {
		t.Fatalf("generateSelfSignedCert failed: %v", err)
	}

	// Assert - fullchain.pem should be 0600
	fullchainPath := filepath.Join(certDir, "fullchain.pem")
	info, err := os.Stat(fullchainPath)
	if err != nil {
		t.Fatalf("failed to stat fullchain.pem: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("expected fullchain.pem permissions 0600, got %o", info.Mode().Perm())
	}

	// Assert - privkey.pem should be 0600
	privkeyPath := filepath.Join(certDir, "privkey.pem")
	info, err = os.Stat(privkeyPath)
	if err != nil {
		t.Fatalf("failed to stat privkey.pem: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("expected privkey.pem permissions 0600, got %o", info.Mode().Perm())
	}
}

// TestGenerateSelfSignedCert_InvalidDirectory tests error handling for invalid directory
func TestGenerateSelfSignedCert_InvalidDirectory(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	// Arrange - use a path that cannot be created (on most systems)
	domain := "test.example.com"
	certDir := "/dev/null/invalid/path"

	// Act
	err := generateSelfSignedCert(domain, certDir)

	// Assert - should return an error
	if err == nil {
		t.Error("expected error for invalid directory, got nil")
	}
}
