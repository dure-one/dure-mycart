package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// TestAuthLimiter_BlocksAfterMax verifies the shared limiter rejects requests
// beyond its configured burst for the same IP. The limiter is process-wide,
// so this test uses a dedicated route on a dedicated app; other tests never
// touch AuthLimiter.
func TestAuthLimiter_BlocksAfterMax(t *testing.T) {
	app := fiber.New()
	app.Post("/limited", AuthLimiter(), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	statuses := make([]int, 0, 15)
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest(http.MethodPost, "/limited", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_ = resp.Body.Close()
		statuses = append(statuses, resp.StatusCode)
	}

	okCount, tooManyCount := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			tooManyCount++
		}
	}

	if okCount == 0 {
		t.Fatal("limiter blocked everything — first requests should pass")
	}
	if tooManyCount == 0 {
		t.Fatalf("expected some 429 after burst, got statuses %v", statuses)
	}
	if okCount+tooManyCount != len(statuses) {
		t.Errorf("unexpected statuses in %v", statuses)
	}
}
