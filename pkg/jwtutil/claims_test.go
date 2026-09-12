package jwtutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// signClaims mints a token with exactly the claims given, signed with the test
// secret. GenerateNewToken always writes both "id" and "expires", so a token
// missing one of them can only be built by hand — which is the point: the
// parser has to reject it rather than return a half-filled metadata struct.
func signClaims(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}
	return signed
}

func TestExtractTokenMetadata_RejectsIncompleteClaims(t *testing.T) {
	t.Parallel()

	const validID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantSub string
	}{
		{
			name:    "no id",
			claims:  jwt.MapClaims{"expires": float64(4102444800)},
			wantSub: "missing id",
		},
		{
			name:    "id is not a string",
			claims:  jwt.MapClaims{"id": 42, "expires": float64(4102444800)},
			wantSub: "missing id",
		},
		{
			name:    "id is not a uuid",
			claims:  jwt.MapClaims{"id": "test-user-id", "expires": float64(4102444800)},
			wantSub: "invalid UUID",
		},
		{
			name:    "no expires",
			claims:  jwt.MapClaims{"id": validID},
			wantSub: "missing expires",
		},
		{
			name:    "expires is not a number",
			claims:  jwt.MapClaims{"id": validID, "expires": "tomorrow"},
			wantSub: "missing expires",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta, err := parseWithCookie(t, signClaims(t, tt.claims))
			if err == nil {
				t.Fatalf("token with %s was accepted, metadata = %+v", tt.name, meta)
			}
			if meta != nil {
				t.Errorf("metadata returned alongside the error: %+v", meta)
			}
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("error %v is not ErrInvalidToken", err)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not mention %q", err, tt.wantSub)
			}
		})
	}
}

// The cookie may be absent, empty or plain garbage; all three must come back as
// ErrInvalidToken rather than a panic or a nil-metadata success.
func TestExtractTokenMetadata_RejectsAnUnusableCookie(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "not-a-jwt", "a.b.c"} {
		meta, err := parseWithCookie(t, raw)
		if err == nil {
			t.Errorf("cookie %q was accepted, metadata = %+v", raw, meta)
		}
	}
}
