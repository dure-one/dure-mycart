package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServeCertWaitPage(t *testing.T) {
	t.Parallel()

	startTime := time.Now().Add(-30 * time.Second) // 30 seconds ago
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	w := httptest.NewRecorder()

	serveCertWaitPage(w, req, startTime)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	if refresh := resp.Header.Get("Refresh"); refresh != "5" {
		t.Errorf("Refresh = %q, want 5", refresh)
	}

	body := w.Body.String()
	want := []string{
		"Certificate Acquisition",
		"Let's Encrypt",
		"example.com",
		"30s", // Should show ~30 seconds elapsed
	}
	for _, s := range want {
		if !strings.Contains(body, s) {
			t.Errorf("response missing %q", s)
		}
	}
}
