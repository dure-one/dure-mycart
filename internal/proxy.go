package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
)

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

		// Add /* suffix for wildcard matching if not present
		routePath := path
		if !strings.HasSuffix(routePath, "*") {
			routePath = strings.TrimSuffix(routePath, "/") + "/*"
		}

		log.Info().
			Str("path", routePath).
			Str("target", target).
			Msg("Registering reverse proxy")

		app.All(routePath, func(c fiber.Ctx) error {
			// Get the path after the proxy prefix
			remainingPath := strings.TrimPrefix(c.Path(), strings.TrimSuffix(path, "*"))
			proxyURL := target + remainingPath

			// Forward query parameters
			if len(c.Queries()) > 0 {
				proxyURL += "?" + string(c.Request().URI().QueryString())
			}

			return proxy.Do(c, proxyURL)
		})
	}

	return nil
}
