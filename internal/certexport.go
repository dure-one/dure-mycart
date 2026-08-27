package app

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// exportCertificatesToPEM exports autocert certificates to PEM format for xmpp-proxy.
// This function monitors the autocert cache and writes fullchain.pem and privkey.pem
// when certificates are obtained.
func exportCertificatesToPEM(manager *autocert.Manager, domain string) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		certDir := "./lc_certs"
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
