package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/testutil"
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

func TestWorkflows(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/responder/workflows", Workflows)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"list all workflows", "", http.StatusOK},
		{"filter by enabled", "?enabled=true", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/responder/workflows"+tt.query, "", cookie)
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var res struct {
				Result struct {
					Workflows []map[string]any `json:"workflows"`
					Total     int              `json:"total"`
				} `json:"result"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&res)
		})
	}
}

func TestWorkflowCRUD(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/responder/workflows", CreateWorkflow)
	app.Get("/api/_/responder/workflows/:workflow_id", GetWorkflow)
	app.Patch("/api/_/responder/workflows/:workflow_id", UpdateWorkflow)
	app.Delete("/api/_/responder/workflows/:workflow_id", DeleteWorkflow)

	// Create
	createPayload := `{
		"name": "Test Workflow",
		"description": "Test description",
		"content": "graph TD\nA --> B",
		"enabled": true,
		"tags": "[\"test\"]",
		"version": "1.0",
		"author": "admin"
	}`

	createResp := testutil.DoRequest(t, app, http.MethodPost, "/api/_/responder/workflows", createPayload, cookie)

	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create: status = %d", createResp.StatusCode)
	}

	var createRes struct {
		Result models.Workflow `json:"result"`
	}
	_ = json.NewDecoder(createResp.Body).Decode(&createRes)
	_ = createResp.Body.Close()

	workflowID := createRes.Result.ID
	if workflowID == "" {
		t.Fatal("create returned empty id")
	}

	// Get
	getResp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/responder/workflows/"+workflowID, "", cookie)
	testutil.AssertStatus(t, getResp, http.StatusOK)

	// Update
	updatePayload := `{
		"name": "Updated Workflow",
		"content": "graph TD\nA --> C",
		"enabled": false
	}`
	updateResp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/responder/workflows/"+workflowID, updatePayload, cookie)
	testutil.AssertStatus(t, updateResp, http.StatusOK)

	// Delete
	deleteResp := testutil.DoRequest(t, app, http.MethodDelete, "/api/_/responder/workflows/"+workflowID, "", cookie)
	testutil.AssertStatus(t, deleteResp, http.StatusOK)
}

func TestCrontabJobs(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/settings/crontab", CrontabJobs)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/settings/crontab", "", cookie)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var res struct {
		Result struct {
			Jobs []map[string]any `json:"jobs"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
}

func TestUpdateCrontabJob(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/crontab/:job_id", UpdateCrontabJobSettings)

	payload := `{
		"enabled": true,
		"interval": "15min"
	}`

	resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/settings/crontab/job123456789012", payload, cookie)
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusNotFound, http.StatusBadRequest)
}

func TestResponderSettings(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/settings/responder", GetResponderSettings)
	app.Patch("/api/_/settings/responder", UpdateResponderSettings)

	// Get settings
	getResp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/settings/responder", "", cookie)
	testutil.AssertStatus(t, getResp, http.StatusOK)

	// Update settings
	updatePayload := `{
		"xmpp_jid": "bot@example.com",
		"xmpp_password": "secret",
		"xmpp_server": "example.com",
		"xmpp_port": 5222
	}`
	updateResp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/settings/responder", updatePayload, cookie)
	testutil.AssertStatus(t, updateResp, http.StatusOK)
}

func TestXMPPConnection(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/settings/responder/test-connection", XMPPConnectionTest)

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/_/settings/responder/test-connection", "", cookie)
	testutil.AssertStatus(t, resp, http.StatusOK)
}
