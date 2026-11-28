package jwtutil

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// TokenType identifies JWTs used as access tokens. It is deliberately not
	// the generic default "JWT" emitted by jwt-go.
	TokenType = "at+jwt"
	Issuer    = "mycart"
	Audience  = "mycart-api"
	ClientID  = "mycart"
)

type Setting struct {
	Secret            string
	SecretExpireHours int
}

const (
	// TokenTypeAccess is the type claim for access tokens (short-lived).
	TokenTypeAccess = "access"
	// TokenTypeRefresh is the type claim for refresh tokens (long-lived).
	TokenTypeRefresh = "refresh"
)

// GenerateNewToken generates an HS256 access token. id is the server-side
// session identifier: keeping it in jti makes revocation unambiguous, while
// sub identifies the authenticated session subject for this first-party API.
func GenerateNewToken(secret, id string, expires int64, credentials []string) (string, error) {
	return generateToken(secret, id, expires, TokenTypeAccess, credentials)
}

// GenerateTokenPair generates both access and refresh tokens.
// accessMinutes: lifespan of access token (e.g., 15 for 15 minutes).
// refreshMinutes: lifespan of refresh token (e.g., 10080 for 7 days).
// Returns: accessToken, refreshToken, accessExp, refreshExp, error.
func GenerateTokenPair(secret, sessionID string, accessMinutes, refreshMinutes int) (string, string, int64, int64, error) {
	now := time.Now().Unix()
	accessExp := now + int64(accessMinutes*60)
	refreshExp := now + int64(refreshMinutes*60)

	access, err := generateToken(secret, sessionID, accessExp, TokenTypeAccess, nil)
	if err != nil {
		return "", "", 0, 0, err
	}

	// Refresh token uses a unique jti (not the session ID) so it can be
	// independently revoked in the refresh_tokens table.
	refreshID := sessionID + ":r:" + time.Now().Format("20060102150405")
	refresh, err := generateToken(secret, refreshID, refreshExp, TokenTypeRefresh, nil)
	if err != nil {
		return "", "", 0, 0, err
	}

	return access, refresh, accessExp, refreshExp, nil
}

// generateToken is the internal token generator with a type claim.
func generateToken(secret, id string, expires int64, tokenType string, credentials []string) (string, error) {
	claims := jwt.MapClaims{
		"iss":       Issuer,
		"aud":       Audience,
		"sub":       id,
		"iat":       time.Now().Unix(),
		"exp":       expires,
		"jti":       id,
		"client_id": ClientID,
		"type":      tokenType,
	}
	if len(credentials) > 0 {
		claims["scope"] = strings.Join(credentials, " ")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = TokenType
	return token.SignedString([]byte(secret))
}
