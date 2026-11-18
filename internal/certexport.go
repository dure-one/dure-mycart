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
func exportCertificatesToPEM(manager *autocert.Manager, domains []string) {
	if len(domains) == 0 {
		logger().Warn().Msg("No domains provided for certificate export")
		return
	}

	certDir := "./lc_certs"
	domain := domains[0] // Primary domain for cache lookup

	// Check if dev mode is enabled
	devMode := strings.ToLower(os.Getenv("MYCART_DEV_MODE")) == "true"

	// Debug: Write to file to verify this code runs
	os.WriteFile("/logs/certexport-debug.log", []byte(fmt.Sprintf("exportCertificatesToPEM called. devMode=%v, MYCART_DEV_MODE=%s, domains=%v\n", devMode, os.Getenv("MYCART_DEV_MODE"), domains)), 0644)

	if devMode {
		// Dev mode: Generate self-signed certificate with all domains as SANs
		logger().Info().Msgf("Dev mode enabled - generating self-signed certificate for %v", domains)
		if err := generateSelfSignedCert(domains, certDir); err != nil {
			logger().Err(err).Msg("Failed to generate self-signed certificate")
			return
		}
		logger().Info().Msgf("✓ Self-signed certificate generated for %v (dev mode)", domains)
		return
	}

	// Production mode: Proactively request certificate, then monitor and export

	// Log provider info
	provider := os.Getenv("ACME_PROVIDER")
	if provider == "" {
		provider = "letsencrypt"
	}
	logger().Info().Msgf("Certificate provider: %s, domains: %v", provider, domains)

	// Import existing PEM files to autocert cache (if present)
	// This allows reusing manually placed certs, pre-populating cache, or recovering from corruption
	// Autocert will check expiry and only renew if <30 days remaining
	imported, err := importPEMToCache(domains, certDir)
	if err != nil {
		logger().Warn().Err(err).Msg("Failed to import existing certificates, will request new cert")
	}

	// Proactively request certificate at startup (non-blocking) only if no valid cert imported
	// This triggers ACME challenge immediately instead of waiting for first HTTPS request
	// GetCertificate for primary domain → autocert issues ONE cert with all HostWhitelist domains as SANs
	if !imported {
		go func() {
			logger().Info().Msgf("Requesting SSL certificate for %v", domains)
			hello := &tls.ClientHelloInfo{
				ServerName: domains[0], // Primary domain triggers cert request with all SANs
			}
			_, err := manager.GetCertificate(hello)
			if err != nil {
				logger().Warn().Err(err).Msgf("Initial certificate request failed for %s, will retry", domains[0])
			} else {
				logger().Info().Msgf("Certificate request initiated for %s", domains[0])
			}
		}()
	} else {
		logger().Info().Msg("Skipping proactive cert request - valid certificate already imported")
	}

	// Monitor autocert cache and export PEM files when certificate is ready
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
				ServerName: domain, // Check primary domain
			}

			cert, err := manager.GetCertificate(hello)
			if err != nil {
				// Certificate not ready yet, will retry on next tick
				continue
			}

			// Export to PEM files
			if err := writePEMFiles(cert, certDir); err != nil {
				logger().Err(err).Msg("Failed to export PEM files")
				continue
			}

			logger().Info().Msgf("✓ Exported SSL certificates to PEM format for xmpp-proxy")
			exported = true
		}
	}()
}

// generateSelfSignedCert creates a self-signed certificate for local development.
func generateSelfSignedCert(domains []string, certDir string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no domains provided")
	}

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
			CommonName:   domains[0], // Primary domain as CN
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              domains, // All domains as SANs
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

	logger().Info().Msgf("Generated self-signed certificate: %s", fullchainPath)
	logger().Info().Msgf("Generated private key: %s", privkeyPath)
	logger().Info().Msg("⚠️  WARNING: Self-signed certificate is for development only - not trusted by browsers/clients")

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

// checkCertificateExpiry parses PEM certificate and checks expiry.
// Returns: daysLeft, shouldRenew, error
// shouldRenew=true when certificate expires within renewThreshold.
func checkCertificateExpiry(certPEM []byte, renewThreshold time.Duration) (int, bool, error) {
	// Decode PEM
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return 0, false, fmt.Errorf("failed to decode PEM certificate")
	}

	// Parse X.509 certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Calculate time until expiry
	timeUntilExpiry := time.Until(cert.NotAfter)
	daysLeft := int(timeUntilExpiry.Hours() / 24)

	// Should renew if within threshold
	shouldRenew := timeUntilExpiry < renewThreshold

	return daysLeft, shouldRenew, nil
}

// importPEMToCache imports PEM files back into autocert cache format.
// Combines privkey.pem + fullchain.pem → certDir/domain (autocert cache format).
// This allows pre-populating cache from existing certs or recovering from corruption.
//
// Returns (imported, error):
//   - (true, nil): Successfully imported valid certificate
//   - (false, nil): No files to import, or cert needs renewal
//   - (false, err): Import failed with error
func importPEMToCache(domains []string, certDir string) (bool, error) {
	if len(domains) == 0 {
		return false, fmt.Errorf("no domains provided")
	}

	primaryDomain := domains[0]
	privkeyPath := filepath.Join(certDir, "privkey.pem")
	fullchainPath := filepath.Join(certDir, "fullchain.pem")

	// Check if PEM files exist
	if _, err := os.Stat(privkeyPath); os.IsNotExist(err) {
		logger().Debug().Msg("No existing PEM certificate files found - will request new certificate")
		return false, nil // No PEM files to import
	}
	if _, err := os.Stat(fullchainPath); os.IsNotExist(err) {
		logger().Warn().Msg("Private key found but fullchain.pem missing - will request new certificate")
		return false, nil
	}

	// Read fullchain to check expiry
	fullchain, err := os.ReadFile(fullchainPath)
	if err != nil {
		return false, fmt.Errorf("failed to read fullchain.pem: %w", err)
	}

	// Check certificate expiry (7-day safety threshold)
	renewThreshold := 7 * 24 * time.Hour
	daysLeft, shouldRenew, err := checkCertificateExpiry(fullchain, renewThreshold)
	if err != nil {
		logger().Warn().Err(err).Msg("Failed to check certificate expiry, skipping import")
		return false, nil // Don't import invalid cert
	}

	if shouldRenew {
		logger().Warn().Msgf("Certificate expires in %d days (<7 day threshold) - forcing renewal", daysLeft)
		return false, nil // Skip import, autocert will request new cert
	}

	logger().Info().Msgf("Certificate valid for %d more days - importing to cache", daysLeft)

	// Read privkey
	privkey, err := os.ReadFile(privkeyPath)
	if err != nil {
		return false, fmt.Errorf("failed to read privkey.pem: %w", err)
	}

	// Combine: privkey first, then fullchain (autocert DirCache format)
	combined := append(privkey, fullchain...)

	// Write to autocert cache file
	cachePath := filepath.Join(certDir, primaryDomain)
	if err := os.WriteFile(cachePath, combined, 0600); err != nil {
		return false, fmt.Errorf("failed to write cache file: %w", err)
	}

	logger().Info().Msgf("✓ Imported PEM files to autocert cache: %s", primaryDomain)
	return true, nil
}
