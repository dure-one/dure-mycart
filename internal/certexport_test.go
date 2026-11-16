package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dure-one/dure-mycart/pkg/logging"
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

// generateTestCertWithExpiry generates a PEM certificate that expires in the given duration
func generateTestCertWithExpiry(t *testing.T, expiresIn time.Duration) []byte {
	t.Helper()

	// Generate ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		NotBefore:             now,
		NotAfter:              now.Add(expiresIn),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"test.example.com"},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM
}

// TestCheckCertificateExpiry tests certificate expiry validation
func TestCheckCertificateExpiry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expiresIn      time.Duration
		renewThreshold time.Duration
		wantRenew      bool
	}{
		{
			name:           "cert with 90 days left",
			expiresIn:      90 * 24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      false,
		},
		{
			name:           "cert with 8 days left",
			expiresIn:      8 * 24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      false,
		},
		{
			name:           "cert with 7 days left - boundary",
			expiresIn:      7 * 24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      false, // Equal to threshold, still safe
		},
		{
			name:           "cert with 6 days left - RENEW",
			expiresIn:      6 * 24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      true,
		},
		{
			name:           "cert with 1 day left - RENEW",
			expiresIn:      24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      true,
		},
		{
			name:           "expired cert - RENEW",
			expiresIn:      -24 * time.Hour,
			renewThreshold: 7 * 24 * time.Hour,
			wantRenew:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			certPEM := generateTestCertWithExpiry(t, tt.expiresIn)

			daysLeft, shouldRenew, err := checkCertificateExpiry(certPEM, tt.renewThreshold)
			if err != nil {
				t.Fatalf("checkCertificateExpiry failed: %v", err)
			}

			if shouldRenew != tt.wantRenew {
				t.Errorf("shouldRenew = %v, want %v (daysLeft=%d)", shouldRenew, tt.wantRenew, daysLeft)
			}
		})
	}
}

// TestCheckCertificateExpiry_InvalidPEM tests error handling for invalid PEM
func TestCheckCertificateExpiry_InvalidPEM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pemData []byte
	}{
		{
			name:    "empty data",
			pemData: []byte{},
		},
		{
			name:    "not PEM",
			pemData: []byte("not a PEM certificate"),
		},
		{
			name:    "PEM but not certificate",
			pemData: []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAK\n-----END RSA PRIVATE KEY-----\n"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := checkCertificateExpiry(tt.pemData, 7*24*time.Hour)
			if err == nil {
				t.Error("expected error for invalid PEM, got nil")
			}
		})
	}
}
