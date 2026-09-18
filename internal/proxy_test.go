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
	setLogger(logging.New())

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

// TestParseProxyBindings tests parsing of REVERSE_PROXY_BINDINGS environment variable
func TestParseProxyBindings(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		want    []ProxyBinding
		wantErr bool
	}{
		{
			name:   "valid single binding",
			envVar: "/api->http://backend:8080",
			want: []ProxyBinding{
				{Path: "/api", Target: "http://backend:8080"},
			},
			wantErr: false,
		},
		{
			name:   "valid multiple bindings",
			envVar: "/api->http://backend:8080,/ws->https://websocket:9090",
			want: []ProxyBinding{
				{Path: "/api", Target: "http://backend:8080"},
				{Path: "/ws", Target: "https://websocket:9090"},
			},
			wantErr: false,
		},
		{
			name:    "empty env var",
			envVar:  "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "invalid format - no arrow",
			envVar:  "/api http://backend:8080",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid format - too many arrows",
			envVar:  "/api->http://backend:8080->extra",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid path - missing leading slash",
			envVar:  "api->http://backend:8080",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid target - not http/https",
			envVar:  "/api->ftp://backend:8080",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid target - missing protocol",
			envVar:  "/api->backend:8080",
			want:    nil,
			wantErr: true,
		},
		{
			name:   "whitespace trimming",
			envVar: " /api -> http://backend:8080 , /ws -> https://websocket:9090 ",
			want: []ProxyBinding{
				{Path: "/api", Target: "http://backend:8080"},
				{Path: "/ws", Target: "https://websocket:9090"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			t.Setenv("REVERSE_PROXY_BINDINGS", tt.envVar)

			// Act
			got, err := ParseProxyBindings()

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProxyBindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseProxyBindings() got %d bindings, want %d", len(got), len(tt.want))
					return
				}

				for i := range got {
					if got[i].Path != tt.want[i].Path || got[i].Target != tt.want[i].Target {
						t.Errorf("ParseProxyBindings()[%d] = %+v, want %+v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

// TestHTTPProxyIntegration tests HTTP reverse proxy functionality
func TestHTTPProxyIntegration(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	// Arrange - Create a backend HTTP server
	backendResponse := `{"status":"ok","message":"Hello from backend"}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(backendResponse))
	}))
	defer backend.Close()

	// Set up Fiber app with proxy configuration
	t.Setenv("REVERSE_PROXY_BINDINGS", "/api->"+backend.URL)

	app := fiber.New()
	if err := SetupProxyRoutes(app); err != nil {
		t.Fatalf("SetupProxyRoutes failed: %v", err)
	}

	// Act - Make HTTP request through proxy
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body := make([]byte, len(backendResponse))
	n, _ := resp.Body.Read(body)
	if string(body[:n]) != backendResponse {
		t.Errorf("expected body %q, got %q", backendResponse, string(body[:n]))
	}
}

// TestHTTPProxyWithQueryParams tests that query parameters are forwarded correctly
func TestHTTPProxyWithQueryParams(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	// Arrange - Create a backend that echoes query params
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.RawQuery))
	}))
	defer backend.Close()

	// Set up Fiber app with proxy configuration
	t.Setenv("REVERSE_PROXY_BINDINGS", "/api->"+backend.URL)

	app := fiber.New()
	if err := SetupProxyRoutes(app); err != nil {
		t.Fatalf("SetupProxyRoutes failed: %v", err)
	}

	// Act - Make request with query parameters
	req := httptest.NewRequest("GET", "/api/endpoint?foo=bar&baz=qux", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer resp.Body.Close()

	// Assert - Query params should be forwarded
	expectedQuery := "foo=bar&baz=qux"
	body := make([]byte, 100)
	n, _ := resp.Body.Read(body)
	if string(body[:n]) != expectedQuery {
		t.Errorf("expected query params %q, got %q", expectedQuery, string(body[:n]))
	}
}

// TestMIMETypeFix tests that Content-Type is fixed for static assets
func TestMIMETypeFix(t *testing.T) {
	// Initialize logger
	setLogger(logging.New())

	tests := []struct {
		name         string
		path         string
		backendType  string
		expectedType string
	}{
		{
			name:         "fix CSS content type",
			path:         "/static/style.css",
			backendType:  "text/plain",
			expectedType: "text/css",
		},
		{
			name:         "fix JavaScript content type",
			path:         "/static/app.js",
			backendType:  "text/plain",
			expectedType: "application/javascript",
		},
		{
			name:         "fix JSON content type",
			path:         "/api/data.json",
			backendType:  "text/plain",
			expectedType: "application/json",
		},
		{
			name:         "preserve correct content type",
			path:         "/api/data.json",
			backendType:  "application/json",
			expectedType: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange - Create a backend that returns wrong Content-Type
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.backendType)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("test content"))
			}))
			defer backend.Close()

			// Set up Fiber app with proxy configuration
			t.Setenv("REVERSE_PROXY_BINDINGS", "/static->"+backend.URL+",/api->"+backend.URL)

			app := fiber.New()
			if err := SetupProxyRoutes(app); err != nil {
				t.Fatalf("SetupProxyRoutes failed: %v", err)
			}

			// Act
			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer resp.Body.Close()

			// Assert
			contentType := resp.Header.Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("expected Content-Type %q, got %q", tt.expectedType, contentType)
			}
		})
	}
}
