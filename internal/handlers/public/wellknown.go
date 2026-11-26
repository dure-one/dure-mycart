package handlers

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/websocket"
)

// probeCache holds WebSocket probe results with TTL
type probeCache struct {
	mu         sync.RWMutex
	lastProbe  time.Time
	lastResult bool
	wsURL      string
}

var cache = &probeCache{}

const cacheTTL = 60 * time.Second

// getWebSocketURL returns the WebSocket URL from env or default
func getWebSocketURL() string {
	if url := os.Getenv("XMPP_WEBSOCKET_URL"); url != "" {
		return url
	}
	return "wss://localhost:5280/xmpp-websocket"
}

// probeWebSocketEndpoint attempts to connect to WebSocket with xmpp subprotocol
func probeWebSocketEndpoint(url string, timeout time.Duration) bool {
	if url == "" {
		return false
	}

	// Set timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
		Subprotocols:     []string{"xmpp"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, resp, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Verify xmpp subprotocol was negotiated
	if resp != nil && resp.Header.Get("Sec-WebSocket-Protocol") != "xmpp" {
		return false
	}

	return true
}

// getCachedProbeResult returns cached probe result or performs new probe
func getCachedProbeResult(url string, timeout time.Duration) bool {
	cache.mu.RLock()
	// Cache hit: same URL and within TTL
	if cache.wsURL == url && time.Since(cache.lastProbe) < cacheTTL {
		result := cache.lastResult
		cache.mu.RUnlock()
		return result
	}
	cache.mu.RUnlock()

	// Cache miss: probe and update
	result := probeWebSocketEndpoint(url, timeout)

	cache.mu.Lock()
	cache.wsURL = url
	cache.lastProbe = time.Now()
	cache.lastResult = result
	cache.mu.Unlock()

	return result
}

// clearProbeCache clears the probe cache (for testing)
func clearProbeCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.wsURL = ""
	cache.lastProbe = time.Time{}
	cache.lastResult = false
}

// HostMetaXML serves XEP-0156 host-meta in XML format
//
// @Summary      XMPP WebSocket discovery (XML)
// @Description  Serves XEP-0156 host-meta for XMPP WebSocket endpoint discovery
// @Tags         Well-Known
// @Produce      application/xrd+xml
// @Success      200 {string} string "XRD XML"
// @Router       /.well-known/host-meta [get]
func HostMetaXML(c fiber.Ctx) error {
	c.Set("Content-Type", "application/xrd+xml; charset=utf-8")
	c.Set("Access-Control-Allow-Origin", "*")

	wsURL := getWebSocketURL()
	available := getCachedProbeResult(wsURL, 500*time.Millisecond)

	xrd := "<?xml version='1.0' encoding='utf-8'?>\n<XRD xmlns='http://docs.oasis-open.org/ns/xri/xrd-1.0'>\n"
	if available {
		xrd += fmt.Sprintf("  <Link rel=\"urn:xmpp:alt-connections:websocket\" href=\"%s\" />\n", wsURL)
	}
	xrd += "</XRD>"

	return c.SendString(xrd)
}

// HostMetaJSON serves XEP-0156 host-meta in JSON format
//
// @Summary      XMPP WebSocket discovery (JSON)
// @Description  Serves XEP-0156 host-meta for XMPP WebSocket endpoint discovery
// @Tags         Well-Known
// @Produce      application/jrd+json
// @Success      200 {object} map[string]any "JRD JSON"
// @Router       /.well-known/host-meta.json [get]
func HostMetaJSON(c fiber.Ctx) error {
	c.Set("Content-Type", "application/jrd+json; charset=utf-8")
	c.Set("Access-Control-Allow-Origin", "*")

	wsURL := getWebSocketURL()
	available := getCachedProbeResult(wsURL, 500*time.Millisecond)

	jrd := map[string]any{
		"links": []map[string]string{},
	}

	if available {
		jrd["links"] = []map[string]string{
			{
				"rel":  "urn:xmpp:alt-connections:websocket",
				"href": wsURL,
			},
		}
	}

	return c.JSON(jrd)
}
