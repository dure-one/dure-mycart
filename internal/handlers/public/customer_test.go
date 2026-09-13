package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/middleware"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/jwtutil"
)

// cabinetApp wires the cabinet the way routes.ApiPublicRoutes does, minus the
// rate limiter: the limiter is shared and per-IP, and a test that signs in
// repeatedly would trip it. The gate and the auth middleware are part of what
// is under test, so they are mounted here rather than left out.
func cabinetApp() *fiber.App {
	app := fiber.New()

	customer := app.Group("/api/customer", middleware.AccountEnabled())
	customer.Post("/signup", CustomerSignUp)
	customer.Post("/signin", CustomerSignIn)
	customer.Post("/signout", middleware.CustomerJWTProtected(), CustomerSignOut)
	customer.Get("/me", middleware.CustomerJWTProtected(), CustomerMe)
	customer.Get("/purchases", middleware.CustomerJWTProtected(), CustomerPurchases)
	customer.Get("/purchases/:file_id<len(15)>/download", middleware.CustomerJWTProtected(), CustomerDownload)

	return app
}

// enableCabinet turns the cabinet on for the test.
func enableCabinet(t *testing.T) {
	t.Helper()

	if err := queries.DB().UpdateSettingByGroup(t.Context(), &models.Account{Enabled: true, ExpireHours: 24}); err != nil {
		t.Fatalf("enable cabinet: %v", err)
	}
}

// cabinetCookie returns the session cookie a sign-in response set.
func cabinetCookie(t *testing.T, resp *http.Response) string {
	t.Helper()

	for _, c := range resp.Cookies() {
		if c.Name == middleware.CookieCustomerToken && c.Value != "" {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatalf("response carries no %s cookie", middleware.CookieCustomerToken)
	return ""
}

// TestCustomerCabinetDisabled is the off switch: a shop that does not want a
// cabinet must not present one, and must not admit that it could.
func TestCustomerCabinetDisabled(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	app := cabinetApp()

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/customer/signup", `{"email":"buyer@example.com","password":"Passw0rd!"}`},
		{http.MethodPost, "/api/customer/signin", `{"email":"buyer@example.com","password":"Passw0rd!"}`},
		{http.MethodPost, "/api/customer/signout", ""},
		{http.MethodGet, "/api/customer/me", ""},
		{http.MethodGet, "/api/customer/purchases", ""},
		{http.MethodGet, "/api/customer/purchases/" + cabinetFileID + "/download", ""},
	}

	for _, r := range requests {
		resp := testutil.DoRequest(t, app, r.method, r.path, r.body, "")
		testutil.AssertStatus(t, resp, http.StatusNotFound)
	}
}

func TestCustomerSignUpAndIn(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	enableCabinet(t)
	app := cabinetApp()

	t.Run("rejects a malformed payload", func(t *testing.T) {
		for _, body := range []string{
			`{"email":"not-an-email","password":"Passw0rd!"}`,
			`{"email":"buyer@example.com","password":"short"}`,
			`{"email":"","password":""}`,
		} {
			resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signup", body, "")
			testutil.AssertStatus(t, resp, http.StatusBadRequest)
		}
	})

	t.Run("creates the account", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signup",
			`{"email":"buyer@example.com","password":"Passw0rd!","name":"Ada"}`, "")
		testutil.AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("refuses a duplicate address", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signup",
			`{"email":"BUYER@example.com","password":"Passw0rd!"}`, "")
		testutil.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("refuses a wrong password without saying why", func(t *testing.T) {
		status, body := readResponse(t, testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signin",
			`{"email":"buyer@example.com","password":"Wrong123!"}`, ""))
		testutil.AssertStatusCode(t, status, http.StatusBadRequest)

		// An unknown address must be answered with the same status and the same
		// words, so the endpoint cannot be used to discover who has an account.
		unknownStatus, unknownBody := readResponse(t, testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signin",
			`{"email":"nobody@example.com","password":"Wrong123!"}`, ""))
		testutil.AssertStatusCode(t, unknownStatus, http.StatusBadRequest)

		if got, want := messageOf(t, unknownBody), messageOf(t, body); got != want {
			t.Errorf("unknown address answered %q, wrong password answered %q", got, want)
		}
	})

	var cookie string

	t.Run("signs in and sets the cabinet cookie", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signin",
			`{"email":"buyer@example.com","password":"Passw0rd!"}`, "")
		testutil.AssertStatus(t, resp, http.StatusOK)

		cookie = cabinetCookie(t, resp)
		if want := middleware.CookieCustomerToken + "="; cookie[:len(want)] != want {
			t.Errorf("cookie = %q", cookie)
		}
	})

	t.Run("serves the signed-in customer", func(t *testing.T) {
		status, got := readResponse(t, testutil.DoRequest(t, app, http.MethodGet, "/api/customer/me", "", cookie))
		testutil.AssertStatusCode(t, status, http.StatusOK)

		// The response carries what the cabinet needs and nothing more: the
		// account model holds a password hash, and it must not reach the wire.
		if !strings.Contains(got, "buyer@example.com") || strings.Contains(got, "password") {
			t.Errorf("unexpected body: %s", got)
		}
	})

	t.Run("refuses a request with no session", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet, "/api/customer/me", "", "")
		testutil.AssertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("signs out and invalidates the session", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signout", "", cookie)
		testutil.AssertStatus(t, resp, http.StatusNoContent)

		// The row is gone, so the token that was just retired — and any copy of
		// it — must stop working before its TTL would have expired.
		resp = testutil.DoRequest(t, app, http.MethodGet, "/api/customer/me", "", cookie)
		testutil.AssertStatus(t, resp, http.StatusUnauthorized)
	})
}

// TestCustomerCabinetRejectsAnAdminSession proves the wiring end to end: the
// admin cookie is a perfectly valid token for the admin surface, backed by a
// real session row, and it must still not open the cabinet.
func TestCustomerCabinetRejectsAnAdminSession(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	enableCabinet(t)
	app := cabinetApp()

	sessionID := uuid.NewString()
	expires := time.Now().Add(time.Hour).Unix()
	if err := queries.DB().AddSession(t.Context(), sessionID,
		queries.SessionValue(queries.SessionRoleAdmin, ""), expires); err != nil {
		t.Fatalf("add admin session: %v", err)
	}

	token, err := jwtutil.GenerateNewToken(testutil.FixtureJWTSecret, sessionID, expires, nil)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/customer/me", "",
		middleware.CookieCustomerToken+"="+token)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

// cabinetFileID is a digital file the fixtures give the paid buyer
// (secret_image_1.png of the first product in their order).
const cabinetFileID = "QLYUrC7p3XuXRFC"

// signUpAndIn creates an account and returns its cabinet cookie. The two calls
// are kept together because every test below needs both and the cabinet never
// signs a new account in by itself.
func signUpAndIn(t *testing.T, app *fiber.App, email, password string) string {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signup", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK)

	resp = testutil.DoRequest(t, app, http.MethodPost, "/api/customer/signin", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK)

	return cabinetCookie(t, resp)
}

// digitalFileName returns the stored name of a digital file, which is the part
// of its path on disk.
func digitalFileName(t *testing.T, fileID string) string {
	t.Helper()

	var name string
	err := queries.DB().ProductQueries.DB.QueryRowContext(t.Context(),
		`SELECT name FROM digital_file WHERE id = ?`, fileID).Scan(&name)
	if err != nil {
		t.Fatalf("load digital file name: %v", err)
	}
	return name
}

// writeDigitalFile puts a file's bytes where the download endpoint looks for
// them. The test suite runs in a temporary directory, so this leaves nothing
// behind in the tree.
func writeDigitalFile(t *testing.T, fileID, ext string, payload []byte) {
	t.Helper()

	if err := os.MkdirAll(dirDigitals, 0o775); err != nil {
		t.Fatalf("mkdir digitals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirDigitals, digitalFileName(t, fileID)+"."+ext), payload, 0o644); err != nil {
		t.Fatalf("write digital file: %v", err)
	}
}

// cabinetPurchases is the part of the purchases response the tests read.
type cabinetPurchases struct {
	Success bool `json:"success"`
	Result  struct {
		Purchases []struct {
			ID          string `json:"id"`
			Created     int64  `json:"created"`
			AmountTotal int    `json:"amount_total"`
			Currency    string `json:"currency"`
			Items       []struct {
				ProductID string `json:"product_id"`
				Name      string `json:"name"`
				Slug      string `json:"slug"`
				Quantity  int    `json:"quantity"`
				Digital   string `json:"digital"`
				Files     []struct {
					ID       string `json:"id"`
					OrigName string `json:"orig_name"`
				} `json:"files"`
				Codes []string `json:"codes"`
			} `json:"items"`
		} `json:"purchases"`
	} `json:"result"`
}

// TestCustomerPurchasesEndpoint is the cabinet's own list: what a buyer sees
// after signing in, and what a buyer who has bought nothing sees.
func TestCustomerPurchasesEndpoint(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	enableCabinet(t)
	app := cabinetApp()

	t.Run("refuses an anonymous request", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet, "/api/customer/purchases", "", "")
		testutil.AssertStatus(t, resp, http.StatusUnauthorized)
	})

	// The fixtures leave a paid order against this address, with two products
	// sold in the two ways that deliver something: files to download, and a key
	// claimed for the cart.
	cookie := signUpAndIn(t, app, "user@gmail.com", "Passw0rd!")

	t.Run("lists the paid order and what it hands over", func(t *testing.T) {
		status, body := readResponse(t, testutil.DoRequest(t, app, http.MethodGet,
			"/api/customer/purchases", "", cookie))
		testutil.AssertStatusCode(t, status, http.StatusOK)

		var got cabinetPurchases
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("decode %q: %v", body, err)
		}

		if len(got.Result.Purchases) != 1 {
			t.Fatalf("purchases = %d, want 1: %s", len(got.Result.Purchases), body)
		}
		purchase := got.Result.Purchases[0]
		if purchase.ID != "iodz4ibf5h5zmov" || purchase.AmountTotal != 6300 || purchase.Currency != "USD" {
			t.Errorf("purchase = %+v", purchase)
		}
		if purchase.Created == 0 {
			t.Error("purchase carries no created timestamp")
		}

		if len(purchase.Items) != 3 {
			t.Fatalf("items = %d, want 3: %+v", len(purchase.Items), purchase.Items)
		}

		byProduct := map[string]int{}
		for i, item := range purchase.Items {
			byProduct[item.ProductID] = i
			if item.Name == "" || item.Slug == "" || item.Quantity != 1 {
				t.Errorf("item = %+v", item)
			}
		}

		files := purchase.Items[byProduct["fv6c9s9cqzf36sc"]]
		if files.Digital != "file" {
			t.Errorf("digital = %q, want file", files.Digital)
		}
		wantFiles := []string{"secret_image_1.png", "secret_image_2.png"}
		if len(files.Files) != len(wantFiles) {
			t.Fatalf("files = %+v, want %v", files.Files, wantFiles)
		}
		for i, name := range wantFiles {
			if files.Files[i].OrigName != name || files.Files[i].ID == "" {
				t.Errorf("file %d = %+v, want %q", i, files.Files[i], name)
			}
		}

		keyed := purchase.Items[byProduct["xrtb1b919t2nuj9"]]
		if keyed.Digital != "data" {
			t.Errorf("digital = %q, want data", keyed.Digital)
		}
		// The key this order claimed, and only it: the product holds four more,
		// still unclaimed, and printing them here would hand this buyer the
		// shop's stock.
		if len(keyed.Codes) != 1 || keyed.Codes[0] != "ff0b48d1-0a75-4d67-a0ac-e6243cfd6cec" {
			t.Errorf("codes = %+v, want the one claimed key", keyed.Codes)
		}

		// An order delivered by an external API has nothing to hand over.
		external := purchase.Items[byProduct["7mweb67t8xv9pzx"]]
		if len(external.Files) != 0 || len(external.Codes) != 0 {
			t.Errorf("api item carries deliverables: %+v", external)
		}

		// The list is the buyer's own, so it does not need to name them — and
		// the address they typed at checkout is not part of it.
		if strings.Contains(body, "user@gmail.com") {
			t.Errorf("the response carries the buyer's address: %s", body)
		}
	})

	t.Run("shows an empty list to a buyer with no orders", func(t *testing.T) {
		other := signUpAndIn(t, app, "nobody@example.com", "Passw0rd!")

		status, body := readResponse(t, testutil.DoRequest(t, app, http.MethodGet,
			"/api/customer/purchases", "", other))
		testutil.AssertStatusCode(t, status, http.StatusOK)

		var got cabinetPurchases
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("decode %q: %v", body, err)
		}
		// Another address's paid order is in the same table; an empty list is
		// what says it was not swept up into this one.
		if len(got.Result.Purchases) != 0 {
			t.Errorf("purchases = %+v, want none", got.Result.Purchases)
		}
	})
}

// TestCustomerDownload covers the download endpoint: what a buyer who paid may
// take away, and the ways a request may fail to be entitled to it.
func TestCustomerDownload(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	enableCabinet(t)
	app := cabinetApp()

	payload := []byte("%PDF-1.4 a guide the buyer paid for")
	writeDigitalFile(t, cabinetFileID, "png", payload)

	// A file of a product this buyer never bought, so the endpoint has one it
	// must refuse even though the file exists.
	unbought, err := queries.DB().AddDigitalFile(t.Context(), "k4pkxqhn4p0xhoc",
		"99999999-9999-4999-8999-999999999999", "pdf", "somebody-elses.pdf")
	if err != nil {
		t.Fatalf("add unbought digital file: %v", err)
	}
	writeDigitalFile(t, unbought.ID, "pdf", []byte("not yours"))

	cookie := signUpAndIn(t, app, "user@gmail.com", "Passw0rd!")
	stranger := signUpAndIn(t, app, "stranger@example.com", "Passw0rd!")

	t.Run("streams a file the buyer paid for", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/api/customer/purchases/"+cabinetFileID+"/download", "", cookie)

		// Read before asserting: AssertStatus closes the body.
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		_ = resp.Body.Close()
		testutil.AssertStatus(t, resp, http.StatusOK)

		if !bytes.Equal(body, payload) {
			t.Errorf("body = %q, want the stored bytes", body)
		}
		if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "secret_image_1.png") {
			t.Errorf("Content-Disposition = %q, want the upload name", cd)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q", ct)
		}
		if nosniff := resp.Header.Get("X-Content-Type-Options"); nosniff != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q, want nosniff", nosniff)
		}
	})

	// Every refusal below has to look the same from outside. A response that
	// told "no such file" from "not yours" would turn the endpoint into a way
	// to walk the shop's catalogue of guides by id.
	refusals := []struct {
		name   string
		fileID string
		cookie string
		status int
	}{
		{"signed out", cabinetFileID, "", http.StatusUnauthorized},
		{"signed in but bought nothing", cabinetFileID, stranger, http.StatusNotFound},
		{"a file of a product the buyer never bought", unbought.ID, cookie, http.StatusNotFound},
		{"no such file", "abcdefghijklmno", cookie, http.StatusNotFound},
	}

	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet,
				"/api/customer/purchases/"+tt.fileID+"/download", "", tt.cookie)
			testutil.AssertStatus(t, resp, tt.status)
		})
	}

	t.Run("serves nothing when the file is missing on disk", func(t *testing.T) {
		// A file of a product the buyer did pay for, whose bytes were never
		// written: the row is theirs and the file is still unreachable, and the
		// buyer can tell nothing from the answer.
		orphan, err := queries.DB().AddDigitalFile(t.Context(), "fv6c9s9cqzf36sc",
			"88888888-8888-4888-8888-888888888888", "pdf", "gone.pdf")
		if err != nil {
			t.Fatalf("add orphan file: %v", err)
		}

		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/api/customer/purchases/"+orphan.ID+"/download", "", cookie)
		testutil.AssertStatus(t, resp, http.StatusNotFound)
	})
}

// readResponse consumes a response and returns its status and body. Reading
// before asserting keeps the two orderings from mattering: testutil.AssertStatus
// closes the body.
func readResponse(t *testing.T, resp *http.Response) (int, string) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// messageOf returns the `message` field of an HTTPResponse, for comparing the
// wording of two failures.
func messageOf(t *testing.T, body string) string {
	t.Helper()

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
	return payload.Message
}
