package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// exportCertificatesToPEM exports autocert certificates to PEM format for xmpp-proxy.
// In dev mode (MYCART_DEV_MODE=true), generates self-signed certificates immediately.
// In production, monitors the autocert cache and writes fullchain.pem and privkey.pem
// when certificates are obtained.
func exportCertificatesToPEM(manager *autocert.Manager, domain string) {
	certDir := "./lc_certs"

	// Check if dev mode is enabled
	devMode := strings.ToLower(os.Getenv("MYCART_DEV_MODE")) == "true"

	// Debug: Write to file to verify this code runs
	os.WriteFile("/logs/certexport-debug.log", []byte(fmt.Sprintf("exportCertificatesToPEM called. devMode=%v, MYCART_DEV_MODE=%s\n", devMode, os.Getenv("MYCART_DEV_MODE"))), 0644)

	if devMode {
		// Dev mode: Generate self-signed certificate immediately
		log.Info().Msg("Dev mode enabled - generating self-signed certificate")
		if err := generateSelfSignedCert(domain, certDir); err != nil {
			log.Err(err).Msg("Failed to generate self-signed certificate")
			return
		}
		log.Info().Msgf("✓ Self-signed certificate generated for %s (dev mode)", domain)
		return
	}

	// Production mode: Monitor autocert and export when ready
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		exported := false

		for range ticker.C {
			// Skip if already exported successfully
			if exported {
				continue
			}

			// Try to get the certificate from autocert
			hello := &tls.ClientHelloInfo{
				ServerName: domain,
			}

			cert, err := manager.GetCertificate(hello)
			if err != nil {
				// Certificate not ready yet, will retry on next tick
				continue
			}

			// Export to PEM files
			if err := writePEMFiles(cert, certDir); err != nil {
				log.Err(err).Msg("Failed to export PEM files")
				continue
			}

			log.Info().Msgf("✓ Exported SSL certificates to PEM format for xmpp-proxy")
			exported = true
		}
	}()
}

// generateSelfSignedCert creates a self-signed certificate for local development.
func generateSelfSignedCert(domain, certDir string) error {
	// Ensure cert directory exists
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"mycart (self-signed)"},
			CommonName:   domain,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write fullchain.pem
	fullchainPath := filepath.Join(certDir, "fullchain.pem")
	certFile, err := os.OpenFile(fullchainPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create fullchain.pem: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Write privkey.pem
	privkeyPath := filepath.Join(certDir, "privkey.pem")
	keyFile, err := os.OpenFile(privkeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create privkey.pem: %w", err)
	}
	defer keyFile.Close()

	privKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyBytes,
	}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	log.Info().Msgf("Generated self-signed certificate: %s", fullchainPath)
	log.Info().Msgf("Generated private key: %s", privkeyPath)
	log.Info().Msg("⚠️  WARNING: Self-signed certificate is for development only - not trusted by browsers/clients")

	return nil
}

// writePEMFiles writes certificate and private key to PEM files.
func writePEMFiles(cert *tls.Certificate, certDir string) error {
	if len(cert.Certificate) == 0 {
		return fmt.Errorf("no certificates in chain")
	}

	// Write fullchain.pem (all certificates in chain)
	fullchainPath := filepath.Join(certDir, "fullchain.pem")
	f, err := os.OpenFile(fullchainPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create fullchain.pem: %w", err)
	}
	defer f.Close()

	for _, certDER := range cert.Certificate {
		certParsed, err := x509.ParseCertificate(certDER)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		if err := pem.Encode(f, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certParsed.Raw,
		}); err != nil {
			return fmt.Errorf("failed to encode certificate: %w", err)
		}
	}

	// Write privkey.pem (private key)
	if cert.PrivateKey == nil {
		return fmt.Errorf("no private key in certificate")
	}

	privkeyPath := filepath.Join(certDir, "privkey.pem")
	privFile, err := os.OpenFile(privkeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create privkey.pem: %w", err)
	}
	defer privFile.Close()

	// Marshal private key to DER
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(privFile, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	return nil
}
