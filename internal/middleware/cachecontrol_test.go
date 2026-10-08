package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestCacheControl(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		responseStatus int
		setCookie      bool
		want           string
		wantVary       string
	}{
		{
			name:           "immutable hashed asset",
			path:           "/_app/immutable/chunks/main.abc123.js",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "public, max-age=31536000, s-maxage=31536000, immutable",
			wantVary:       "Accept-Encoding",
		},
		{
			name:           "HTML shell",
			path:           "/",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "no-cache",
			wantVary:       "Accept-Encoding",
		},
		{
			name:           "product page HTML",
			path:           "/products/foo",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "no-cache",
			wantVary:       "Accept-Encoding",
		},
		{
			name:           "catalog API",
			path:           "/api/products",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "public, max-age=60, s-maxage=300, stale-while-revalidate=86400",
			wantVary:       "Accept-Encoding, Accept",
		},
		{
			name:           "single product API",
			path:           "/api/products/foo-bar",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "public, max-age=60, s-maxage=300, stale-while-revalidate=86400",
			wantVary:       "Accept-Encoding, Accept",
		},
		{
			name:           "cart API",
			path:           "/api/cart",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "no-store, private",
			wantVary:       "Cookie, Authorization",
		},
		{
			name:           "auth endpoint",
			path:           "/api/auth/signin",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "no-store, private",
			wantVary:       "Cookie, Authorization",
		},
		{
			name:           "admin API",
			path:           "/api/private/products",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "no-store, private",
			wantVary:       "Cookie, Authorization",
		},
		{
			name:           "product image",
			path:           "/products/foo.png",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "public, max-age=86400, s-maxage=604800, stale-while-revalidate=86400",
			wantVary:       "Accept-Encoding",
		},
		{
			name:           "robots.txt",
			path:           "/robots.txt",
			method:         fiber.MethodGet,
			responseStatus: 200,
			want:           "public, max-age=3600, s-maxage=86400",
			wantVary:       "Accept-Encoding",
		},
		{
			name:           "non-GET request",
			path:           "/api/products",
			method:         fiber.MethodPost,
			responseStatus: 201,
			want:           "",
			wantVary:       "",
		},
		{
			name:           "error response forces no-store",
			path:           "/api/products",
			method:         fiber.MethodGet,
			responseStatus: 500,
			want:           "no-store",
			wantVary:       "",
		},
		{
			name:           "Set-Cookie forces no-store private",
			path:           "/api/products",
			method:         fiber.MethodGet,
			responseStatus: 200,
			setCookie:      true,
			want:           "no-store, private",
			wantVary:       "Cookie, Authorization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()

			app.Use(CacheControl())

			app.All("*", func(c fiber.Ctx) error {
				if tt.setCookie {
					c.Cookie(&fiber.Cookie{Name: "test", Value: "value"})
				}
				return c.Status(tt.responseStatus).SendString("ok")
			})

			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			_, _ = io.ReadAll(resp.Body)

			got := resp.Header.Get("Cache-Control")
			if got != tt.want {
				t.Errorf("Cache-Control = %q, want %q", got, tt.want)
			}

			if tt.wantVary != "" {
				gotVary := resp.Header.Get("Vary")
				if gotVary != tt.wantVary {
					t.Errorf("Vary = %q, want %q", gotVary, tt.wantVary)
				}
			}
		})
	}
}

func TestCacheControlOverride(t *testing.T) {
	app := fiber.New()

	app.Use(CacheControl())

	app.Get("/custom", func(c fiber.Ctx) error {
		// Handler sets custom cache control before middleware runs
		c.Locals("cache_override", "max-age=999")
		c.Set(fiber.HeaderCacheControl, "max-age=999")
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/custom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	got := resp.Header.Get("Cache-Control")
	want := "max-age=999"
	if got != want {
		t.Errorf("Cache-Control = %q, want %q (override should be respected)", got, want)
	}
}
