package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/wenlng/go-captcha/v2/rotate"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

// captchaStore holds active captcha tokens with expiration
var captchaStore = make(map[string]captchaToken)

type captchaToken struct {
	angle     float64
	expiresAt time.Time
}

type verifiedToken struct {
	verified  bool
	expiresAt time.Time
}

// verifiedStore holds verified tokens that grant access to seller info
var verifiedStore = make(map[string]verifiedToken)

// GenerateCaptcha generates a rotate captcha image.
//
// @Summary      Generate captcha
// @Description  Generate rotate captcha for seller info access
// @Tags         Public
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Captcha data"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sellerinfo/captcha [get]
func GenerateCaptcha(c fiber.Ctx) error {
	log := logging.New()

	// Create rotate captcha builder and instance
	builder := rotate.NewBuilder()
	captcha := builder.Make()

	// Generate captcha data
	captData, err := captcha.Generate()
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Get images and angle
	masterImage := captData.GetMasterImage()
	thumbImage := captData.GetThumbImage()
	block := captData.GetData()

	// Convert images to base64
	masterBase64, err := masterImage.ToBase64Data()
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	thumbBase64, err := thumbImage.ToBase64Data()
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store token with angle (expires in 5 minutes)
	captchaStore[token] = captchaToken{
		angle:     float64(block.Angle),
		expiresAt: time.Now().Add(5 * time.Minute),
	}

	// Clean up expired tokens
	go cleanupExpiredCaptchas()

	return webutil.Response(c, fiber.StatusOK, "Captcha generated", map[string]any{
		"token":        token,
		"master_image": masterBase64,
		"thumb_image":  thumbBase64,
	})
}

// VerifyCaptcha verifies the rotate captcha answer.
//
// @Summary      Verify captcha
// @Description  Verify rotate captcha answer and issue access token
// @Tags         Public
// @Accept       json
// @Produce      json
// @Param        request body object true "Captcha verification data"
// @Success      200 {object} webutil.HTTPResponse "Verification result"
// @Failure      400 {object} webutil.HTTPResponse "Invalid request"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sellerinfo/verify [post]
func VerifyCaptcha(c fiber.Ctx) error {
	log := logging.New()

	var req struct {
		Token string `json:"token"`
		Angle int    `json:"angle"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "Invalid request")
	}

	// Check if token exists and not expired
	stored, exists := captchaStore[req.Token]
	if !exists {
		return webutil.StatusBadRequest(c, "Invalid or expired captcha token")
	}

	if time.Now().After(stored.expiresAt) {
		delete(captchaStore, req.Token)
		return webutil.StatusBadRequest(c, "Captcha expired")
	}

	// Verify angle using go-captcha's validation (5 degree padding)
	if !rotate.Validate(req.Angle, int(stored.angle), 5) {
		return webutil.Response(c, fiber.StatusOK, "Verification failed", map[string]any{
			"verified": false,
		})
	}

	// Generate access token
	accessTokenBytes := make([]byte, 32)
	if _, err := rand.Read(accessTokenBytes); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}
	accessToken := base64.URLEncoding.EncodeToString(accessTokenBytes)

	// Store verified token (expires in 10 minutes)
	verifiedStore[accessToken] = verifiedToken{
		verified:  true,
		expiresAt: time.Now().Add(10 * time.Minute),
	}

	// Clean up captcha token
	delete(captchaStore, req.Token)

	// Clean up expired verified tokens
	go cleanupExpiredVerified()

	return webutil.Response(c, fiber.StatusOK, "Verification successful", map[string]any{
		"verified":     true,
		"access_token": accessToken,
	})
}

// GetSellerInfo returns seller information after captcha verification.
//
// @Summary      Get seller info
// @Description  Get Korean seller information (requires captcha verification)
// @Tags         Public
// @Produce      json
// @Param        X-Access-Token header string true "Access token from captcha verification"
// @Success      200 {object} webutil.HTTPResponse "Seller information"
// @Failure      401 {object} webutil.HTTPResponse "Unauthorized"
// @Failure      404 {object} webutil.HTTPResponse "Seller info not enabled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/sellerinfo [get]
func GetSellerInfo(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	// Verify access token
	accessToken := c.Get("X-Access-Token")
	if accessToken == "" {
		return webutil.StatusBadRequest(c, "Access token required")
	}

	verified, exists := verifiedStore[accessToken]
	if !exists || !verified.verified {
		return webutil.Response(c, fiber.StatusUnauthorized, "Invalid or expired access token", nil)
	}

	if time.Now().After(verified.expiresAt) {
		delete(verifiedStore, accessToken)
		return webutil.Response(c, fiber.StatusUnauthorized, "Access token expired", nil)
	}

	// Load Dureone settings
	dureone, err := queries.GetSettingByGroup[models.Dureone](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Check if enabled
	if !dureone.Enabled {
		return webutil.StatusNotFound(c)
	}

	// Return seller info (excluding the enabled flag)
	return webutil.Response(c, fiber.StatusOK, "Seller info", map[string]string{
		"business_name":       dureone.BusinessName,
		"representative":      dureone.Representative,
		"customer_service":    dureone.CustomerService,
		"business_reg_number": dureone.BusinessRegNumber,
		"business_address":    dureone.BusinessAddress,
		"ecommerce_license":   dureone.EcommerceLicense,
		"email":               dureone.Email,
	})
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func cleanupExpiredCaptchas() {
	now := time.Now()
	for token, data := range captchaStore {
		if now.After(data.expiresAt) {
			delete(captchaStore, token)
		}
	}
}

func cleanupExpiredVerified() {
	now := time.Now()
	for token, data := range verifiedStore {
		if now.After(data.expiresAt) {
			delete(verifiedStore, token)
		}
	}
}
