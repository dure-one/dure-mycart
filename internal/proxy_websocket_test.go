package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/websocket"
	"github.com/shurco/mycart/pkg/logging"
)

// TestProxyWebSocketUpgrade tests that the reverse proxy correctly handles WebSocket upgrade requests
func TestProxyWebSocketUpgrade(t *testing.T) {
	// RED Phase: This test should FAIL because WebSocket proxying is not implemented yet

	// Initialize logger (required by SetupProxyRoutes)
	log = logging.New()

	// Create a backend WebSocket server that echoes messages
	backendUpgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := backendUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("backend upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(messageType, message); err != nil {
				break
			}
		}
	}))
	defer backend.Close()

	// Set up Fiber app with proxy configuration pointing to our backend
	t.Setenv("REVERSE_PROXY_BINDINGS", "/ws->"+backend.URL+"/ws")

	app := fiber.New()
	if err := SetupProxyRoutes(app); err != nil {
		t.Fatalf("SetupProxyRoutes failed: %v", err)
	}

	// Start the Fiber server on a test port
	testPort := "18765"
	go func() {
		if err := app.Listen("127.0.0.1:" + testPort); err != nil {
			t.Logf("Fiber server error: %v", err)
		}
	}()
	defer app.Shutdown()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Connect to the proxy WebSocket
	proxyWS := "ws://127.0.0.1:" + testPort + "/ws"

	// Try to connect to the proxy via WebSocket
	clientDialer := websocket.DefaultDialer
	clientDialer.HandshakeTimeout = 2 * time.Second

	conn, resp, err := clientDialer.Dial(proxyWS, nil)
	if err != nil {
		t.Fatalf("WebSocket dial through proxy failed: %v (status: %v)", err, resp)
	}
	defer conn.Close()

	// Send a test message
	testMessage := "hello websocket"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(testMessage)); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// Read the echoed response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if messageType != websocket.TextMessage {
		t.Errorf("expected TextMessage, got %v", messageType)
	}

	if string(message) != testMessage {
		t.Errorf("expected echo %q, got %q", testMessage, string(message))
	}
}
