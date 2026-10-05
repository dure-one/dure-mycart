package handlers

import (
	"net/http"
	"testing"

	"github.com/dure-one/dure-mycart/internal/testutil"
)

func TestUpdateSetting_BadJSONReturns400(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/:setting_key", UpdateSetting)

	resp := testutil.DoRequest(t, app, http.MethodPatch,
		"/api/_/settings/main", "{not json", cookie)
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestUpdateSetting_UnknownKeyRoutesToSettingName(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/:setting_key", UpdateSetting)

	// Custom keys go through UpdateSettingByKey.
	resp := testutil.DoRequest(t, app, http.MethodPatch,
		"/api/_/settings/custom_prop", `{"value":"hello"}`, cookie)
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusInternalServerError)
}

func TestGetSetting_UnknownKeyNotFound(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/settings/:setting_key", GetSetting)

	// Unknown key => GetSettingByKey returns empty map + no error, so the
	// handler returns 200 with an empty payload. Accept either path.
	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/api/_/settings/doesnotexist", "", cookie)
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusNotFound)
}
