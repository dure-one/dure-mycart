package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/pkg/jwtutil"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/security"
	"github.com/shurco/mycart/pkg/webutil"
)

// SignIn authenticates a user and returns a JWT token.
//
// @Summary      Sign in
// @Description  Authenticate admin user with email and password, returns JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body models.SignIn true "Credentials"
// @Success      200 {object} webutil.HTTPResponse{result=string} "JWT token"
// @Failure      400 {object} webutil.HTTPResponse "Invalid credentials or validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sign/in [post]
// dummyPasswordHash is a bcrypt hash of an unrelated random password. It is
// compared against on unknown-email sign-in attempts so that both failure
// paths (unknown email / wrong password) burn comparable CPU time and return
// identical responses, preventing user enumeration via timing or status codes.
const dummyPasswordHash = "$2a$10$uo0yzV.HzLdx6hKO4F/4oOM0UQdmaLmZydgeRg9IDdL0lQjZZmoFq"

func SignIn(c fiber.Ctx) error {
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

	passwordHash, err := store.GetPasswordByEmail(c.Context(), request.Email)
	if err != nil {
		// Unknown email: run the same bcrypt comparison against a dummy hash
		// and return the same generic error as a wrong password.
		security.ComparePasswords(dummyPasswordHash, request.Password)
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "wrong user email address or password")
	}

	compareUserPassword := security.ComparePasswords(passwordHash, request.Password)
	if !compareUserPassword {
		return webutil.StatusBadRequest(c, "wrong user email address or password")
	}

	// Generate a new pair of access and refresh tokens.
	settingJWT, err := store.GetSettingByGroupTyped[models.JWT](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	userID := uuid.New()
	expires := time.Now().Add(time.Hour * time.Duration(settingJWT.ExpireHours)).Unix()
	token, err := jwtutil.GenerateNewToken(settingJWT.Secret, userID.String(), expires, nil)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Add session record
	if err := store.AddSession(c.Context(), userID.String(), "admin", expires); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Unix(expires, 0),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
	})

	return webutil.StatusOK(c, "Token", token)
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
	log := logging.New()

	settingJWT, err := store.GetSettingByGroupTyped[models.JWT](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	claims, err := jwtutil.ExtractTokenMetadata(c, settingJWT.Secret)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if err := store.DeleteSession(c.Context(), claims.ID); err != nil {
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
