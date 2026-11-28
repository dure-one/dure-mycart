package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/queries"
	"github.com/dure-one/dure-mycart/pkg/jwtutil"
	"github.com/dure-one/dure-mycart/pkg/logging"
	"github.com/dure-one/dure-mycart/pkg/security"
	"github.com/dure-one/dure-mycart/pkg/webutil"
)

// SignIn authenticates a user and sets an HttpOnly JWT session cookie.
//
// @Summary      Sign in
// @Description  Authenticate an admin user with email and password and set the session cookie
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body models.SignIn true "Credentials"
// @Success      200 {object} webutil.HTTPResponse "Signed in"
// @Failure      400 {object} webutil.HTTPResponse "Invalid credentials or validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sign/in [post]
func SignIn(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()
	request := new(models.SignIn)

	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := request.Validate(); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	passwordHash, err := db.GetPasswordByEmail(c.Context(), request.Email)
	if err != nil {
		// Unknown email: run the same bcrypt comparison against a dummy hash
		// and return the same generic error as a wrong password.
		security.ComparePasswords(security.DummyPasswordHash, request.Password)
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "wrong user email address or password")
	}

	compareUserPassword := security.ComparePasswords(passwordHash, request.Password)
	if !compareUserPassword {
		return webutil.StatusBadRequest(c, "wrong user email address or password")
	}

	// Create a server-revocable first-party session with token pair.
	settingJWT, err := queries.GetSettingByGroup[models.JWT](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	sessionID := uuid.NewString()

	// Generate access (15 min) and refresh (7 days) tokens
	accessToken, refreshToken, accessExp, refreshExp, err := jwtutil.GenerateTokenPair(
		settingJWT.Secret, sessionID, 15, 10080)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Add session record (for access token revocation)
	if err := db.AddSession(c.Context(),
		sessionID, queries.SessionValue(queries.SessionRoleAdmin, ""), accessExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Store refresh token
	refreshID := sessionID + ":r:" + time.Now().Format("20060102150405")
	if err := db.AddRefreshToken(c.Context(), refreshID, sessionID, "admin", refreshExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Set cookie for browser clients (backward compatibility)
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    accessToken,
		Expires:  time.Unix(accessExp, 0),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
	})

	// Return tokens in body for API clients
	return webutil.StatusOK(c, "Signed in", map[string]any{
		"access_token":        accessToken,
		"refresh_token":       refreshToken,
		"expires_at":          accessExp,
		"refresh_expires_at":  refreshExp,
	})
}

// SignOut invalidates the user session and clears the authentication token.
//
// @Summary      Sign out
// @Description  Invalidate current session and clear token cookie
// @Tags         Auth
// @Security     BearerAuth
// @Success      204 "Session invalidated"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sign/out [post]
func SignOut(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	settingJWT, err := queries.GetSettingByGroup[models.JWT](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	claims, err := jwtutil.ExtractTokenMetadata(c, settingJWT.Secret)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Delete session (revokes access token)
	if err := db.DeleteSession(c.Context(), claims.ID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Delete all refresh tokens for this session
	if err := db.DeleteRefreshTokensByUser(c.Context(), claims.ID, "admin"); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Expires:  time.Now().Add(-(time.Hour * 2)),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// validateAdminRefreshToken parses and validates an admin refresh token.
func validateAdminRefreshToken(ctx context.Context, tokenString, secret string) (refreshID, sessionID string, err error) {
	db := queries.DB()

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		if err := jwtutil.ValidateHeader(t); err != nil {
			return nil, err
		}
		return []byte(secret), nil
	}, jwtutil.ParserOptions()...)
	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid or expired refresh token")
	}

	if err := jwtutil.ValidateTokenType(claims, jwtutil.TokenTypeRefresh); err != nil {
		return "", "", fmt.Errorf("not a refresh token")
	}

	refreshID, err = jwtutil.SessionID(claims)
	if err != nil {
		return "", "", err
	}

	sessionID, role, err := db.GetRefreshToken(ctx, refreshID)
	if err != nil {
		return "", "", fmt.Errorf("refresh token not found or expired")
	}
	if role != "admin" {
		return "", "", fmt.Errorf("invalid refresh token role")
	}

	return refreshID, sessionID, nil
}

// Refresh issues a new token pair from a valid refresh token.
//
// @Summary      Refresh access token
// @Description  Exchange a refresh token for a new access + refresh token pair
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body object{refresh_token=string} true "Refresh token"
// @Success      200 {object} webutil.HTTPResponse "New token pair"
// @Failure      400 {object} webutil.HTTPResponse "Invalid or expired refresh token"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sign/refresh [post]
func Refresh(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind().Body(&req); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "invalid request")
	}
	if req.RefreshToken == "" {
		return webutil.StatusBadRequest(c, "refresh_token required")
	}

	settingJWT, err := queries.GetSettingByGroup[models.JWT](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	refreshID, sessionID, err := validateAdminRefreshToken(c.Context(), req.RefreshToken, settingJWT.Secret)
	if err != nil {
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := db.DeleteRefreshToken(c.Context(), refreshID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	newAccessToken, newRefreshToken, accessExp, refreshExp, err := jwtutil.GenerateTokenPair(
		settingJWT.Secret, sessionID, 15, 10080)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if err := db.UpdateSession(c.Context(), sessionID, queries.SessionValue(queries.SessionRoleAdmin, ""), accessExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	newRefreshID := sessionID + ":r:" + time.Now().Format("20060102150405")
	if err := db.AddRefreshToken(c.Context(), newRefreshID, sessionID, "admin", refreshExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.StatusOK(c, "Token refreshed", map[string]any{
		"access_token":       newAccessToken,
		"refresh_token":      newRefreshToken,
		"expires_at":         accessExp,
		"refresh_expires_at": refreshExp,
	})
}
