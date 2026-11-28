package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/gofiber/fiber/v3"
)

// getProviderDisplayName returns human-readable ACME provider name.
func getProviderDisplayName(provider string) string {
	switch provider {
	case "letsencrypt":
		return "Let's Encrypt"
	case "gts":
		return "Google Trust Services"
	case "zerossl":
		return "ZeroSSL"
	default:
		return provider
	}
}

// getACMEStepMessage returns the message for each ACME acquisition step.
func getACMEStepMessage(step int) string {
	switch step {
	case 2:
		return "Waiting for HTTPS connection to trigger ACME challenge"
	case 3:
		return "ACME HTTP-01 challenge in progress"
	case 4:
		return "Waiting for CA validation"
	default:
		return "Acquiring certificate..."
	}
}

// serveCertWaitPage serves a status page while waiting for TLS certificate.
func serveCertWaitPage(w http.ResponseWriter, r *http.Request, startTime time.Time) {
	elapsed := time.Since(startTime)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Refresh", "5") // Auto-refresh every 5 seconds
	w.WriteHeader(http.StatusServiceUnavailable)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Certificate Acquisition in Progress</title>
	<style>
		body { font-family: system-ui, sans-serif; max-width: 600px; margin: 100px auto; padding: 20px; }
		.status { background: #fff3cd; border-left: 4px solid #ffc107; padding: 20px; margin: 20px 0; }
		.time { font-family: monospace; font-size: 1.2em; color: #333; }
		.spinner { display: inline-block; width: 20px; height: 20px; border: 3px solid #f3f3f3;
			border-top: 3px solid #ffc107; border-radius: 50%%; animation: spin 1s linear infinite; }
		@keyframes spin { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(360deg); } }
	</style>
</head>
<body>
	<h1>🔒 Certificate Acquisition in Progress</h1>
	<div class="status">
		<p><span class="spinner"></span> Acquiring TLS certificate from Let's Encrypt...</p>
		<p>Elapsed time: <span class="time">%s</span></p>
		<p><small>This page will automatically refresh and redirect once the certificate is ready.</small></p>
	</div>
	<p><small>Domain: %s | Server started: %s</small></p>
</body>
</html>`,
		elapsed.Round(time.Second),
		r.Host,
		startTime.Format("2006-01-02 15:04:05 MST"),
	)
}

// startBothServers starts both HTTP and HTTPS servers concurrently.
// HTTP server handles autocert HTTP-01 challenges, HTTPS serves the application.
func startBothServers(app *fiber.App, httpAddr, httpsAddr string) error {
	hostOnly := extractHostOnly(httpsAddr)
	startTime := time.Now()
	certReady := make(chan struct{})

	// Get ACME provider name for logs
	provider := os.Getenv("ACME_PROVIDER")
	if provider == "" {
		provider = "letsencrypt"
	}
	providerName := getProviderDisplayName(provider)

	// Check for existing certificate
	certPath := fmt.Sprintf("./lc_certs/%s", hostOnly)
	if _, err := os.Stat(certPath); err == nil {
		fmt.Fprintf(os.Stderr, "[ACME] Certificate found in cache for %s\n", hostOnly)
		close(certReady)
	} else {
		fmt.Fprintf(os.Stderr, "[ACME 1/5] Starting certificate acquisition for %s via %s\n", hostOnly, providerName)
	}

	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(hostOnly),
		Cache:      autocert.DirCache("./lc_certs"),
	}

	// Monitor cert readiness in background
	go func() {
		if _, err := os.Stat(certPath); err == nil {
			return // Already ready
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		step := 2
		for {
			select {
			case <-certReady:
				return
			case <-ticker.C:
				// Check if cert file exists
				if _, err := os.Stat(certPath); err == nil {
					fmt.Fprintf(os.Stderr, "[ACME 5/5] ✓ Certificate acquired (elapsed: %s)\n", time.Since(startTime).Round(time.Second))
					close(certReady)
					logger().Info().Msg("✓ TLS certificate ready")
					return
				}
				// Show progress
				if step <= 4 {
					msg := getACMEStepMessage(step)
					fmt.Fprintf(os.Stderr, "[ACME %d/5] %s\n", step, msg)
					step++
				}
			}
		}
	}()

	errCh := make(chan error, 2)

	// Start HTTP server for autocert HTTP-01 challenge
	go func() {
		logger().Info().Msgf("Starting HTTP server on %s for ACME challenges", httpAddr)

		// Serve status page until cert ready, then switch to ACME handler
		httpSrv := &http.Server{
			Addr: httpAddr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-certReady:
					// Cert ready: delegate to autocert
					manager.HTTPHandler(nil).ServeHTTP(w, r)
				default:
					// Cert not ready: show status page
					serveCertWaitPage(w, r, startTime)
				}
			}),
		}
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Start HTTPS server
	go func() {
		logger().Info().Msgf("Starting HTTPS server on %s", httpsAddr)
		cfgTLS := &tls.Config{
			GetCertificate: manager.GetCertificate,
			NextProtos: []string{
				"http/1.1",   // HTTP/1.1 (h2 removed: Fiber v3 via custom listener doesn't support HTTP/2)
				"acme-tls/1", // ACME TLS-ALPN-01 challenge
			},
		}

		ln, err := tls.Listen("tcp", httpsAddr, cfgTLS)
		if err != nil {
			errCh <- err
			return
		}

		if err := app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			errCh <- err
		}
	}()

	// Wait for either server to error
	return <-errCh
}

// startHTTPS starts the server with HTTPS support and automatic TLS.
func startHTTPS(app *fiber.App, mainAddr, httpsAddr string) error {
	hostOnly := extractHostOnly(mainAddr)
	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(hostOnly),
		Cache:      autocert.DirCache("./lc_certs"),
	}

	cfgTLS := &tls.Config{
		GetCertificate: manager.GetCertificate,
		NextProtos:     []string{"http/1.1", "acme-tls/1"},
	}

	listenAddr := DefaultHTTPSPort
	if httpsAddr != "" {
		listenAddr = httpsAddr
	}

	ln, err := tls.Listen("tcp", listenAddr, cfgTLS)
	if err != nil {
		logger().Err(err).Send()
		os.Exit(1)
	}

	if err := app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		logger().Err(err).Send()
		os.Exit(1)
	}

	return nil
}

// extractHostOnly extracts only the host from the address, removing the port.
func extractHostOnly(addr string) string {
	if !strings.Contains(addr, ":") {
		return addr
	}

	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}

	return addr
}

// startHTTP starts the HTTP server with graceful shutdown support.
func startHTTP(mainAddr string, app *fiber.App) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if DevMode() {
		return StartServer(ctx, mainAddr, app)
	}

	idleConnsClosed := make(chan struct{})

	go handleShutdown(ctx, app, idleConnsClosed)
	go func() {
		if err := StartServer(ctx, mainAddr, app); err != nil {
			logger().Err(err).Send()
		}
	}()

	<-idleConnsClosed
	return nil
}

// handleShutdown handles application shutdown signals.
func handleShutdown(ctx context.Context, app *fiber.App, idleConnsClosed chan struct{}) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	<-sigint

	if err := app.Shutdown(); err != nil {
		logger().Err(err).Send()
	}

	close(idleConnsClosed)
}

// StartServer starts the server and handles graceful shutdown.
func StartServer(ctx context.Context, addr string, a *fiber.App) error {
	// Buffered, so the listen goroutine can always report its error and exit.
	// Unbuffered it blocks on the send forever when the shutdown path below has
	// already returned — a goroutine leaked for the life of the process.
	errCh := make(chan error, 1)

	go func() {
		if err := a.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			logger().Err(err).Send()
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		err := errors.New("shutdown signal received, closing server")
		logger().Err(err).Send()
		return a.Shutdown()
	case err := <-errCh:
		return err
	}
}
