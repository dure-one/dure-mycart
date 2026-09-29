package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/testutil"
)

func TestMessageThreads(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/responder/messages", MessageThreads)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"list all threads", "", http.StatusOK},
		{"filter by channel", "?channel=sms", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/responder/messages"+tt.query, "", cookie)
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var res struct {
				Result struct {
					Threads []map[string]any `json:"threads"`
					Total   int              `json:"total"`
				} `json:"result"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&res)
		})
	}
}

func TestMessagesByCustomer(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/responder/messages/:customer_id", MessagesByCustomer)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/responder/messages/nonexistent123", "", cookie)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var res struct {
		Result struct {
			Messages []map[string]any `json:"messages"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
}

func TestCreateMessage(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/responder/messages", CreateMessage)

	payload := `{
		"contact_address": "+12125551234",
		"contact_type": "sms",
		"content": "Test message",
		"direction": "outbound",
		"channel": "sms"
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/_/responder/messages", payload, cookie)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestMarkMessageRead(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/responder/messages/:message_id/read", MarkMessageRead)

	resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/responder/messages/msg123456789012/read", "", cookie)
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusInternalServerError)
}

func TestLinkContact(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/responder/messages/link-contact", LinkContact)

	payload := `{
		"contact_id": "contact12345678",
		"new_customer_id": "customer1234567"
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/_/responder/messages/link-contact", payload, cookie)
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusInternalServerError)
}
