package cdn

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSetSurrogateKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		wantSK   string
		wantCT   string
	}{
		{
			name:     "single key",
			keys:     []string{"products"},
			wantSK:   "products",
			wantCT:   "products",
		},
		{
			name:     "multiple keys",
			keys:     []string{"products", "product-foo"},
			wantSK:   "products product-foo",
			wantCT:   "products product-foo",
		},
		{
			name:     "no keys",
			keys:     []string{},
			wantSK:   "",
			wantCT:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c fiber.Ctx) error {
				SetSurrogateKeys(c, tt.keys...)
				return c.SendString("ok")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			_, _ = io.ReadAll(resp.Body)

			gotSK := resp.Header.Get("Surrogate-Key")
			gotCT := resp.Header.Get("Cache-Tag")

			if gotSK != tt.wantSK {
				t.Errorf("Surrogate-Key = %q, want %q", gotSK, tt.wantSK)
			}
			if gotCT != tt.wantCT {
				t.Errorf("Cache-Tag = %q, want %q", gotCT, tt.wantCT)
			}
		})
	}
}

func TestProductKeys(t *testing.T) {
	keys := ProductKeys("test-product")
	want := []string{"products", "product-test-product"}

	if len(keys) != len(want) {
		t.Fatalf("got %d keys, want %d", len(keys), len(want))
	}

	for i, key := range keys {
		if key != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, key, want[i])
		}
	}
}

func TestPurgeKeys(t *testing.T) {
	// Save original env
	origCF := os.Getenv("CLOUDFLARE_API_TOKEN")
	origFastly := os.Getenv("FASTLY_API_KEY")
	defer func() {
		os.Setenv("CLOUDFLARE_API_TOKEN", origCF)
		os.Setenv("FASTLY_API_KEY", origFastly)
	}()

	t.Run("no CDN configured - no-op", func(t *testing.T) {
		os.Unsetenv("CLOUDFLARE_API_TOKEN")
		os.Unsetenv("CLOUDFLARE_ZONE_ID")
		os.Unsetenv("FASTLY_API_KEY")
		os.Unsetenv("FASTLY_SERVICE_ID")

		err := PurgeKeys(context.Background(), "products")
		if err != nil {
			t.Errorf("PurgeKeys with no CDN should not error, got %v", err)
		}
	})

	t.Run("CDN configured - stub returns nil", func(t *testing.T) {
		os.Setenv("CLOUDFLARE_API_TOKEN", "test-token")

		err := PurgeKeys(context.Background(), "products")
		if err != nil {
			t.Errorf("PurgeKeys stub should not error, got %v", err)
		}
	})
}

func TestIsCDNConfigured(t *testing.T) {
	// Save original env
	origCF := os.Getenv("CLOUDFLARE_API_TOKEN")
	origZone := os.Getenv("CLOUDFLARE_ZONE_ID")
	origFastly := os.Getenv("FASTLY_API_KEY")
	origService := os.Getenv("FASTLY_SERVICE_ID")
	defer func() {
		os.Setenv("CLOUDFLARE_API_TOKEN", origCF)
		os.Setenv("CLOUDFLARE_ZONE_ID", origZone)
		os.Setenv("FASTLY_API_KEY", origFastly)
		os.Setenv("FASTLY_SERVICE_ID", origService)
	}()

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "no env vars",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "cloudflare token set",
			env:  map[string]string{"CLOUDFLARE_API_TOKEN": "test"},
			want: true,
		},
		{
			name: "cloudflare zone set",
			env:  map[string]string{"CLOUDFLARE_ZONE_ID": "test"},
			want: true,
		},
		{
			name: "fastly key set",
			env:  map[string]string{"FASTLY_API_KEY": "test"},
			want: true,
		},
		{
			name: "fastly service set",
			env:  map[string]string{"FASTLY_SERVICE_ID": "test"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all CDN env vars
			os.Unsetenv("CLOUDFLARE_API_TOKEN")
			os.Unsetenv("CLOUDFLARE_ZONE_ID")
			os.Unsetenv("FASTLY_API_KEY")
			os.Unsetenv("FASTLY_SERVICE_ID")

			// Set test env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			got := IsCDNConfigured()
			if got != tt.want {
				t.Errorf("IsCDNConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}
