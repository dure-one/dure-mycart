package handlers

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dure-one/dure-mycart/internal/testutil"
)

func TestProbeWebSocketEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		timeout    time.Duration
		wantResult bool
	}{
		{
			name:       "failed probe with invalid URL",
			url:        "ws://invalid-host-that-does-not-exist:9999/xmpp",
			timeout:    100 * time.Millisecond,
			wantResult: false,
		},
		{
			name:       "failed probe with timeout",
			url:        "ws://10.255.255.1:9999/xmpp", // Non-routable IP
			timeout:    50 * time.Millisecond,
			wantResult: false,
		},
		{
			name:       "failed probe with empty URL",
			url:        "",
			timeout:    100 * time.Millisecond,
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := probeWebSocketEndpoint(tt.url, tt.timeout)
			if result != tt.wantResult {
				t.Errorf("probeWebSocketEndpoint() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestGetWebSocketURL(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		wantURL string
	}{
		{
			name:    "returns env var when set",
			envVar:  "wss://test.example.com/xmpp",
			wantURL: "wss://test.example.com/xmpp",
		},
		{
			name:    "returns default when env var empty",
			envVar:  "",
			wantURL: "wss://localhost:5280/xmpp-websocket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XMPP_WEBSOCKET_URL", tt.envVar)
			url := getWebSocketURL()
			if url != tt.wantURL {
				t.Errorf("getWebSocketURL() = %v, want %v", url, tt.wantURL)
			}
		})
	}
}

func TestHostMetaXML(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/.well-known/host-meta", HostMetaXML)

	tests := []struct {
		name         string
		envURL       string
		wantStatus   int
		wantContains []string
	}{
		{
			name:       "returns empty XRD when no WebSocket configured",
			envURL:     "",
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<?xml version='1.0' encoding='utf-8'?>",
				"<XRD xmlns='http://docs.oasis-open.org/ns/xri/xrd-1.0'>",
			},
		},
		{
			name:       "returns empty XRD when WebSocket unreachable",
			envURL:     "ws://invalid:9999/xmpp",
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<?xml version='1.0' encoding='utf-8'?>",
				"<XRD xmlns='http://docs.oasis-open.org/ns/xri/xrd-1.0'>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XMPP_WEBSOCKET_URL", tt.envURL)

			resp := testutil.DoRequest(t, app, http.MethodGet, "/.well-known/host-meta", "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus)

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/xrd+xml") {
				t.Errorf("Content-Type = %s, want application/xrd+xml", contentType)
			}

			cors := resp.Header.Get("Access-Control-Allow-Origin")
			if cors != "*" {
				t.Errorf("Access-Control-Allow-Origin = %s, want *", cors)
			}
		})
	}
}

func TestHostMetaJSON(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/.well-known/host-meta.json", HostMetaJSON)

	tests := []struct {
		name         string
		envURL       string
		wantStatus   int
		wantContains []string
	}{
		{
			name:       "returns empty links when no WebSocket configured",
			envURL:     "",
			wantStatus: http.StatusOK,
			wantContains: []string{
				`"links":[]`,
			},
		},
		{
			name:       "returns empty links when WebSocket unreachable",
			envURL:     "ws://invalid:9999/xmpp",
			wantStatus: http.StatusOK,
			wantContains: []string{
				`"links":[]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XMPP_WEBSOCKET_URL", tt.envURL)

			resp := testutil.DoRequest(t, app, http.MethodGet, "/.well-known/host-meta.json", "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus)

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/jrd+json") && !strings.Contains(contentType, "application/json") {
				t.Errorf("Content-Type = %s, want application/jrd+json or application/json", contentType)
			}

			cors := resp.Header.Get("Access-Control-Allow-Origin")
			if cors != "*" {
				t.Errorf("Access-Control-Allow-Origin = %s, want *", cors)
			}
		})
	}
}
