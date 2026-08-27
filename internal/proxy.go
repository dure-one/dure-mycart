package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	fiberws "github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/gorilla/websocket"
)

func init() {
	// Configure proxy security policy to allow localhost and private IPs for internal services
	proxy.WithSecurityPolicy(proxy.SecurityPolicy{
		AllowPrivateIPs: true,
	})
}

// getMimeType returns the MIME type for a file extension
func getMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".htm":  "text/html",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".woff": "font/woff",
		".woff2": "font/woff2",
		".ttf":  "font/ttf",
		".eot":  "application/vnd.ms-fontobject",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return ""
}

// ProxyBinding represents a reverse proxy configuration
type ProxyBinding struct {
	Path   string // Frontend path (e.g., "/prosody")
	Target string // Backend URL (e.g., "http://prosody:5280")
}

// ParseProxyBindings parses the REVERSE_PROXY_BINDINGS environment variable
// Format: "/path->http://target,/path2->http://target2"
// Example: "/prosody->http://prosody:5280,/api->http://backend:8080"
func ParseProxyBindings() ([]ProxyBinding, error) {
	bindingsEnv := os.Getenv("REVERSE_PROXY_BINDINGS")
	if bindingsEnv == "" {
		return nil, nil
	}

	var bindings []ProxyBinding
	entries := strings.Split(bindingsEnv, ",")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, "->")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid proxy binding format: %s (expected '/path->http://target')", entry)
		}

		path := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])

		if !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("proxy path must start with /: %s", path)
		}

		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			return nil, fmt.Errorf("proxy target must start with http:// or https://: %s", target)
		}

		bindings = append(bindings, ProxyBinding{
			Path:   path,
			Target: target,
		})
	}

	return bindings, nil
}

// proxyWebSocketHandler creates a WebSocket proxy handler for the given target URL
func proxyWebSocketHandler(targetURL string) fiber.Handler {
	// Convert http:// to ws:// or https:// to wss://
	wsURL := strings.Replace(targetURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	return fiberws.New(func(clientConn *fiberws.Conn) {
		// Get subprotocol from client request (e.g., "xmpp" for Prosody)
		requestHeader := http.Header{}
		if subprotocol := clientConn.Subprotocol(); subprotocol != "" {
			requestHeader.Set("Sec-WebSocket-Protocol", subprotocol)
		}

		// Dial backend WebSocket
		backendConn, _, err := websocket.DefaultDialer.Dial(wsURL, requestHeader)
		if err != nil {
			log.Error().Err(err).Str("target", wsURL).Msg("Failed to dial backend WebSocket")
			return
		}
		defer backendConn.Close()

		// Bidirectional proxy
		errChan := make(chan error, 2)

		// Backend -> Client
		go func() {
			for {
				msgType, msg, err := backendConn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := clientConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		// Client -> Backend
		go func() {
			for {
				msgType, msg, err := clientConn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := backendConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		// Wait for error from either direction
		<-errChan
	}, fiberws.Config{
		// Accept common WebSocket subprotocols (xmpp for Prosody, etc.)
		Subprotocols: []string{"xmpp"},
	})
}

// SetupProxyRoutes registers reverse proxy routes based on environment configuration
func SetupProxyRoutes(app *fiber.App) error {
	bindings, err := ParseProxyBindings()
	if err != nil {
		return fmt.Errorf("failed to parse proxy bindings: %w", err)
	}

	if len(bindings) == 0 {
		return nil
	}

	for _, binding := range bindings {
		// Capture binding in closure
		target := binding.Target
		path := binding.Path

		log.Info().
			Str("path", path).
			Str("target", target).
			Msg("Registering reverse proxy")

		// Register WebSocket proxy route (only handles WebSocket upgrades)
		app.Get(path, proxyWebSocketHandler(target))

		// Register HTTP proxy for non-WebSocket requests
		routePath := path
		if !strings.HasSuffix(routePath, "*") {
			routePath = strings.TrimSuffix(routePath, "/") + "/*"
		}

		app.All(routePath, func(c fiber.Ctx) error {
			// Skip if WebSocket (already handled by WebSocket route)
			if fiberws.IsWebSocketUpgrade(c) {
				return c.Next()
			}

			// Get the path after the proxy prefix
			remainingPath := strings.TrimPrefix(c.Path(), strings.TrimSuffix(path, "*"))
			proxyURL := target + remainingPath

			// Forward query parameters
			if len(c.Queries()) > 0 {
				proxyURL += "?" + string(c.Request().URI().QueryString())
			}

			// Proxy to internal services (AllowPrivateIPs is enabled in init())
			if err := proxy.Do(c, proxyURL); err != nil {
				return err
			}

			// Fix Content-Type for static assets when upstream doesn't provide it
			contentType := string(c.Response().Header.ContentType())
			if contentType == "" || contentType == "text/plain" || strings.HasPrefix(contentType, "text/plain;") {
				if mime := getMimeType(c.Path()); mime != "" {
					c.Response().Header.SetContentType(mime)
				}
			}

			return nil
		})
	}

	return nil
}
