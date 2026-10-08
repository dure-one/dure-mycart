package middleware

import (
	"fmt"
	"strings"
)

// Policy defines cache-control directives for a resource.
type Policy struct {
	MaxAge     int  // Browser cache TTL in seconds
	SMaxAge    int  // CDN cache TTL in seconds
	SWR        int  // stale-while-revalidate in seconds
	Public     bool // public directive
	Private    bool // private directive
	Immutable  bool // immutable directive
	NoCache    bool // no-cache directive
	NoStore    bool // no-store directive
	Vary       []string
}

// Header renders the Cache-Control header value.
func (p Policy) Header() string {
	if p.NoStore {
		parts := []string{"no-store"}
		if p.Private {
			parts = append(parts, "private")
		}
		return strings.Join(parts, ", ")
	}

	if p.NoCache {
		return "no-cache"
	}

	var parts []string

	// Private and Public are mutually exclusive per RFC 7234
	if p.Private {
		parts = append(parts, "private")
	} else if p.Public {
		parts = append(parts, "public")
	}

	if p.MaxAge > 0 {
		parts = append(parts, fmt.Sprintf("max-age=%d", p.MaxAge))
	}
	if p.SMaxAge > 0 {
		parts = append(parts, fmt.Sprintf("s-maxage=%d", p.SMaxAge))
	}
	if p.SWR > 0 {
		parts = append(parts, fmt.Sprintf("stale-while-revalidate=%d", p.SWR))
	}
	if p.Immutable {
		parts = append(parts, "immutable")
	}

	return strings.Join(parts, ", ")
}

// VaryHeader returns the Vary header value.
func (p Policy) VaryHeader() string {
	if len(p.Vary) == 0 {
		return ""
	}
	return strings.Join(p.Vary, ", ")
}

// policyRule maps a path prefix to a policy.
type policyRule struct {
	prefix string
	policy Policy
}

// isImagePath checks if path ends with image extension
func isImagePath(path string) bool {
	return strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") ||
		strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".webp")
}

var defaultRules = []policyRule{
	// Hashed static assets - immutable forever (site)
	{
		prefix: "/_app/immutable/",
		policy: Policy{
			MaxAge:    31536000,
			SMaxAge:   31536000,
			Public:    true,
			Immutable: true,
			Vary:      []string{"Accept-Encoding"},
		},
	},
	// Hashed static assets - immutable forever (admin)
	{
		prefix: "/_/_app/immutable/",
		policy: Policy{
			MaxAge:    31536000,
			SMaxAge:   31536000,
			Public:    true,
			Immutable: true,
			Vary:      []string{"Accept-Encoding"},
		},
	},
	// Private/admin API - never cache
	{
		prefix: "/api/private/",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Cart API - never cache
	{
		prefix: "/api/cart",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Orders API - never cache
	{
		prefix: "/api/orders",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Checkout API - never cache
	{
		prefix: "/api/checkout",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Inventory API - never cache
	{
		prefix: "/api/inventory",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Auth endpoints - never cache
	{
		prefix: "/api/auth/",
		policy: Policy{
			NoStore: true,
			Private: true,
			Vary:    []string{"Cookie", "Authorization"},
		},
	},
	// Catalog API - short browser cache, longer CDN cache
	{
		prefix: "/api/products",
		policy: Policy{
			MaxAge:  60,
			SMaxAge: 300,
			SWR:     86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding", "Accept"},
		},
	},
	// Categories API
	{
		prefix: "/api/categories",
		policy: Policy{
			MaxAge:  60,
			SMaxAge: 300,
			SWR:     86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding", "Accept"},
		},
	},
	// robots.txt, sitemap.xml, favicon
	{
		prefix: "/robots.txt",
		policy: Policy{
			MaxAge:  3600,
			SMaxAge: 86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding"},
		},
	},
	{
		prefix: "/sitemap.xml",
		policy: Policy{
			MaxAge:  3600,
			SMaxAge: 86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding"},
		},
	},
	{
		prefix: "/favicon.ico",
		policy: Policy{
			MaxAge:  3600,
			SMaxAge: 86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding"},
		},
	},
	// HTML shells - must revalidate
	{
		prefix: "/",
		policy: Policy{
			NoCache: true,
			Vary:    []string{"Accept-Encoding"},
		},
	},
}

// match returns the policy for a given path (first match wins).
func match(path string) Policy {
	// Special case: product images under /products/*.{png,jpg,...}
	if strings.HasPrefix(path, "/products/") && isImagePath(path) {
		return Policy{
			MaxAge:  86400,
			SMaxAge: 604800,
			SWR:     86400,
			Public:  true,
			Vary:    []string{"Accept-Encoding"},
		}
	}

	for _, rule := range defaultRules {
		if strings.HasPrefix(path, rule.prefix) {
			return rule.policy
		}
	}
	// Default: no cache for safety
	return Policy{NoCache: true, Vary: []string{"Accept-Encoding"}}
}
