package cdn

import (
	"context"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
)

// SetSurrogateKeys sets CDN surrogate key headers for cache purging.
// Multiple keys can be provided for fine-grained cache invalidation.
func SetSurrogateKeys(c fiber.Ctx, keys ...string) {
	if len(keys) == 0 {
		return
	}

	// Fastly uses Surrogate-Key header
	// Cloudflare Enterprise uses Cache-Tag header
	// Set both for compatibility
	header := ""
	for i, key := range keys {
		if i > 0 {
			header += " "
		}
		header += key
	}

	c.Set("Surrogate-Key", header)
	c.Set("Cache-Tag", header)
}

// ProductKeys returns surrogate keys for a product.
func ProductKeys(slug string) []string {
	return []string{"products", fmt.Sprintf("product-%s", slug)}
}

// PurgeKeys purges CDN cache by surrogate keys.
// Returns nil if no CDN configured (no-op).
// ponytail: stub for CDN API integration when provisioned
func PurgeKeys(ctx context.Context, keys ...string) error {
	// Check if CDN is configured
	if !IsCDNConfigured() {
		return nil // No CDN, nothing to purge
	}

	// TODO: Implement actual CDN purge when CDN is provisioned
	// - Cloudflare: POST /zones/:zone_id/purge_cache with tags
	// - Fastly: POST /service/:service_id/purge with Surrogate-Key header

	return nil
}

// IsCDNConfigured checks if CDN credentials are configured.
func IsCDNConfigured() bool {
	return os.Getenv("CLOUDFLARE_API_TOKEN") != "" ||
		os.Getenv("CLOUDFLARE_ZONE_ID") != "" ||
		os.Getenv("FASTLY_API_KEY") != "" ||
		os.Getenv("FASTLY_SERVICE_ID") != ""
}
