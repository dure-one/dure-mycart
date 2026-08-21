package handlers

import (
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/testutil"
)

// Protected setting keys must never be writable through the generic
// key/value PATCH fallback: flipping `installed` re-opens the unauthenticated
// install endpoint, rotating `jwt_secret` enables token forgery.
func TestUpdateSetting_ProtectedKeysRejected(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/:setting_key", UpdateSetting)

	tests := []struct {
		name string
		key  string
		body string
	}{
		{"installed flag", "installed", `{"value":"false"}`},
		{"jwt secret", "jwt_secret", `{"value":"attacker-secret"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodPatch,
				"/api/_/settings/"+tt.key, tt.body, cookie)
			testutil.AssertStatus(t, resp, http.StatusBadRequest)
		})
	}
}
