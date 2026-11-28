package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dure-one/dure-mycart/internal/middleware"
	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/queries"
	"github.com/dure-one/dure-mycart/pkg/errors"
	"github.com/dure-one/dure-mycart/pkg/jwtutil"
	"github.com/dure-one/dure-mycart/pkg/logging"
	"github.com/dure-one/dure-mycart/pkg/security"
	"github.com/dure-one/dure-mycart/pkg/webutil"
)

// wrongCustomerCredentials is the single answer to every failed sign-in, so
// that a wrong password, an unknown address and a deactivated account cannot be
// told apart from the outside.
const wrongCustomerCredentials = "wrong email or password"

// CustomerSignUp creates a storefront account.
//
// It does not sign the new account in. Email verification is not part of this
// feature, so issuing a session here would turn a mistyped address into a live
// account that nobody can ever reach; making the buyer sign in once keeps that
// address in front of them.
//
// @Summary      Create a storefront account
// @Description  Register a customer account for the cabinet. Available only while the cabinet is enabled.
// @Tags         Account
// @Accept       json
// @Produce      json
// @Param        request body models.CustomerSignUp true "Account details"
// @Success      200 {object} webutil.HTTPResponse "Account created"
// @Failure      400 {object} webutil.HTTPResponse "Validation error or address already registered"
// @Failure      404 {object} webutil.HTTPResponse "The cabinet is disabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/customer/signup [post]
func CustomerSignUp(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	request := new(models.CustomerSignUp)
	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}
	if err := request.Validate(); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	customer, err := db.CreateCustomer(c.Context(), request.Email, request.Password, request.Name)
	if err != nil {
		if errors.Is(err, errors.ErrCustomerEmailTaken) {
			return webutil.StatusBadRequest(c, "this email is already registered")
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.StatusOK(c, "Customer", map[string]any{
		"id":    customer.ID,
		"email": customer.Email,
	})
}

// CustomerSignIn authenticates a customer and issues the cabinet session.
//
// @Summary      Sign in to the cabinet
// @Description  Authenticate a customer and set the session cookie
// @Tags         Account
// @Accept       json
// @Produce      json
// @Param        request body models.SignIn true "Credentials"
// @Success      200 {object} webutil.HTTPResponse "Signed in"
// @Failure      400 {object} webutil.HTTPResponse "Wrong credentials or validation error"
// @Failure      404 {object} webutil.HTTPResponse "The cabinet is disabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/customer/signin [post]
func CustomerSignIn(c fiber.Ctx) error {
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

	customer, err := db.CustomerByEmail(c.Context(), request.Email)
	if err != nil {
		if !errors.Is(err, errors.ErrCustomerNotFound) {
			// The address is registered and the lookup is what failed. Telling
			// the buyer their password is wrong would hide a broken database
			// from whoever is watching, and from them.
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}
		// Unknown address: run the same bcrypt comparison against a dummy hash
		// and return the same answer as a wrong password.
		security.ComparePasswords(security.DummyPasswordHash, request.Password)
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, wrongCustomerCredentials)
	}

	if !security.ComparePasswords(customer.Password, request.Password) {
		return webutil.StatusBadRequest(c, wrongCustomerCredentials)
	}

	// Checked after the password so that deactivating an account cannot be used
	// to probe which addresses exist, and reported with the same words for the
	// same reason.
	if !customer.Active {
		return webutil.StatusBadRequest(c, wrongCustomerCredentials)
	}

	return issueCustomerSession(c, customer.ID)
}

// CustomerSignOut invalidates the cabinet session.
//
// @Summary      Sign out of the cabinet
// @Description  Delete the current session row and clear the cabinet cookie
// @Tags         Account
// @Security     BearerAuth
// @Success      204 "Session invalidated"
// @Failure      401 {object} webutil.HTTPResponse "Not signed in"
// @Failure      404 {object} webutil.HTTPResponse "The cabinet is disabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/customer/signout [post]
func CustomerSignOut(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	sessionID := middleware.CustomerSessionID(c)

	// Delete session (revokes access token)
	if err := db.DeleteSession(c.Context(), sessionID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Delete all refresh tokens for this session
	if err := db.DeleteRefreshTokensByUser(c.Context(), sessionID, "customer"); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	c.Cookie(&fiber.Cookie{
		Name:     middleware.CookieCustomerToken,
		Expires:  time.Now().Add(-(time.Hour * 2)),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax",
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// CustomerMe returns the signed-in customer.
//
// @Summary      Current customer
// @Description  Return the account behind the cabinet session
// @Tags         Account
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Customer"
// @Failure      401 {object} webutil.HTTPResponse "Not signed in"
// @Failure      404 {object} webutil.HTTPResponse "The cabinet is disabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/customer/me [get]
func CustomerMe(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	customer, err := db.CustomerByID(c.Context(), middleware.CustomerID(c))
	if err != nil {
		if errors.Is(err, errors.ErrCustomerNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// An account deactivated between the session being issued and this request
	// is treated as gone. Its session stays valid until it expires — the row is
	// not swept here — but nothing is served from it.
	if !customer.Active {
		return webutil.StatusNotFound(c)
	}

	// Built field by field rather than marshalled from the model: the model
	// carries the password hash, and a response that reaches into it is one
	// field tag away from publishing it.
	return webutil.StatusOK(c, "Customer", map[string]any{
		"id":    customer.ID,
		"email": customer.Email,
		"name":  customer.Name,
	})
}

// issueCustomerSession mints a cabinet session for customerID with token pair.
func issueCustomerSession(c fiber.Ctx, customerID string) error {
	db := queries.DB()
	log := logging.New()

	secret, err := db.AccountSecret(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	sessionID := uuid.NewString()

	// Generate access (15 min) and refresh (7 days) tokens
	accessToken, refreshToken, accessExp, refreshExp, err := jwtutil.GenerateTokenPair(
		secret, sessionID, 15, 10080)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Add session record (for access token revocation)
	if err := db.AddSession(c.Context(),
		sessionID, queries.SessionValue(queries.SessionRoleCustomer, customerID), accessExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Store refresh token
	refreshID := sessionID + ":r:" + time.Now().Format("20060102150405")
	if err := db.AddRefreshToken(c.Context(), refreshID, sessionID, "customer", refreshExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Set cookie for browser clients
	c.Cookie(&fiber.Cookie{
		Name:     middleware.CookieCustomerToken,
		Value:    accessToken,
		Expires:  time.Unix(accessExp, 0),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax",
	})

	// Return tokens in body for API clients
	return webutil.StatusOK(c, "Signed in", map[string]any{
		"id":                  customerID,
		"access_token":        accessToken,
		"refresh_token":       refreshToken,
		"expires_at":          accessExp,
		"refresh_expires_at":  refreshExp,
	})
}

// CustomerRefresh issues a new token pair from a valid customer refresh token.
//
// @Summary      Refresh customer access token
// @Description  Exchange a refresh token for a new access + refresh token pair
// @Tags         Account
// @Accept       json
// @Produce      json
// @Param        request body object{refresh_token=string} true "Refresh token"
// @Success      200 {object} webutil.HTTPResponse "New token pair"
// @Failure      400 {object} webutil.HTTPResponse "Invalid or expired refresh token"
// @Failure      404 {object} webutil.HTTPResponse "The cabinet is disabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/customer/refresh [post]
func CustomerRefresh(c fiber.Ctx) error {
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

	secret, err := db.AccountSecret(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Parse and validate refresh token
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		if err := jwtutil.ValidateHeader(t); err != nil {
			return nil, err
		}
		return []byte(secret), nil
	}, jwtutil.ParserOptions()...)
	if err != nil || !token.Valid {
		return webutil.StatusBadRequest(c, "invalid or expired refresh token")
	}

	// Validate token type is "refresh"
	if err := jwtutil.ValidateTokenType(claims, jwtutil.TokenTypeRefresh); err != nil {
		return webutil.StatusBadRequest(c, "not a refresh token")
	}

	refreshID, err := jwtutil.SessionID(claims)
	if err != nil {
		return webutil.StatusBadRequest(c, "invalid refresh token")
	}

	// Check refresh token exists in DB and is not expired
	sessionID, role, err := db.GetRefreshToken(c.Context(), refreshID)
	if err != nil {
		return webutil.StatusBadRequest(c, "refresh token not found or expired")
	}
	if role != "customer" {
		return webutil.StatusBadRequest(c, "invalid refresh token role")
	}

	// Get customer ID from session
	sessionValue, err := db.GetSession(c.Context(), sessionID)
	if err != nil {
		return webutil.StatusBadRequest(c, "session not found")
	}
	_, customerID := queries.ParseSessionValue(sessionValue)

	// Delete old refresh token (rotation)
	if err := db.DeleteRefreshToken(c.Context(), refreshID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Generate new token pair
	newAccessToken, newRefreshToken, accessExp, refreshExp, err := jwtutil.GenerateTokenPair(
		secret, sessionID, 15, 10080)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Update session expiry
	if err := db.UpdateSession(c.Context(), sessionID, queries.SessionValue(queries.SessionRoleCustomer, customerID), accessExp); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Store new refresh token
	newRefreshID := sessionID + ":r:" + time.Now().Format("20060102150405")
	if err := db.AddRefreshToken(c.Context(), newRefreshID, sessionID, "customer", refreshExp); err != nil {
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
