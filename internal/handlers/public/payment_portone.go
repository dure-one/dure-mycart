package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

// portoneAPIURL can be overridden for testing
var portoneAPIURL = "https://api.portone.io"

// GetPortoneConfig returns public PortOne configuration (store_id, channel_key)
// API secret is NOT exposed to frontend for security
//
// @Summary      Get PortOne config
// @Description  Get public PortOne configuration for browser SDK
// @Tags         Cart
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "PortOne public config"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/cart/portone-config [get]
func GetPortoneConfig(c fiber.Ctx) error {
	log := logging.New()

	settings, err := store.GetSettingByGroupTyped[models.Portone](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Only expose store_id, channel_key, and debug_enabled, NOT api_secret
	config := map[string]interface{}{
		"store_id":      settings.StoreID,
		"channel_key":   settings.ChannelKey,
		"debug_enabled": settings.DebugEnabled,
	}

	return webutil.Response(c, fiber.StatusOK, "PortOne config", config)
}

// callPortoneAPI makes authenticated HTTP request to PortOne API
func callPortoneAPI(endpoint string, apiSecret string) (*http.Response, error) {
	url := portoneAPIURL + endpoint

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "PortOne "+apiSecret)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call portone api: %w", err)
	}

	return resp, nil
}

// validatePaymentAmount verifies payment amount matches cart total
// System stores amounts as (value * 100) for all currencies.
// Zero-decimal currencies (KRW, JPY, etc.) are sent to PortOne divided by 100.
func validatePaymentAmount(paymentTotal int, cartTotal int, currency string, log *logging.Log) error {
	// For zero-decimal currencies, divide stored amount by 100 to match PortOne's expected format
	zeroDecimalCurrencies := map[string]bool{
		"KRW": true, // Korean Won
		"JPY": true, // Japanese Yen
		"VND": true, // Vietnamese Dong
		"CLP": true, // Chilean Peso
	}

	expectedAmount := cartTotal
	if zeroDecimalCurrencies[currency] {
		expectedAmount = cartTotal / 100
	}

	if paymentTotal != expectedAmount {
		log.Error().Msgf("Amount mismatch: expected %d, got %d", expectedAmount, paymentTotal)
		return fmt.Errorf("amount mismatch")
	}
	return nil
}

// validatePaymentCurrency verifies payment currency matches cart currency
func validatePaymentCurrency(paymentCurrency, cartCurrency string, log *logging.Log) error {
	if paymentCurrency != cartCurrency {
		log.Error().Msgf("Currency mismatch: expected %s, got %s", cartCurrency, paymentCurrency)
		return fmt.Errorf("currency mismatch")
	}
	return nil
}

// validateCartID verifies cart_id in payment custom data
func validateCartID(customDataJSON, expectedCartID string, log *logging.Log) error {
	var customData struct {
		CartID string `json:"cart_id"`
	}
	if err := json.Unmarshal([]byte(customDataJSON), &customData); err != nil {
		return fmt.Errorf("failed to parse custom data: %w", err)
	}
	if customData.CartID != expectedCartID {
		log.Error().Msgf("Cart ID mismatch: expected %s, got %s", expectedCartID, customData.CartID)
		return fmt.Errorf("cart ID mismatch")
	}
	return nil
}

// CompletePortonePayment verifies payment after browser completes PortOne payment flow
//
// @Summary      Complete PortOne payment
// @Description  Verify payment with PortOne API and update cart status
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Param        request body object{payment_id=string,cart_id=string} true "Payment completion request"
// @Success      200 {object} webutil.HTTPResponse "Payment verified"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/payment/portone/complete [post]
// portonePaymentDetails represents payment details from API
type portonePaymentDetails struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Currency string `json:"currency"`
	Amount   struct {
		Total int `json:"total"`
	} `json:"amount"`
	CustomData string `json:"customData"`
}

// ErrPaymentNotFound is returned when payment is not found in PortOne API
var ErrPaymentNotFound = fmt.Errorf("payment not found")

// fetchAndParsePortonePayment fetches payment from API and parses response
func fetchAndParsePortonePayment(paymentID, apiSecret string, log *logging.Log) (*portonePaymentDetails, error) {
	resp, err := callPortoneAPI("/payments/"+paymentID, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPaymentNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().Msgf("PortOne API error: %d %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	log.Debug().Msgf("PortOne API response: %s", string(bodyBytes))

	var payment portonePaymentDetails
	if err := json.Unmarshal(bodyBytes, &payment); err != nil {
		return nil, fmt.Errorf("decode payment: %w", err)
	}

	log.Debug().Msgf("Parsed payment: ID=%s, Status=%s, Total=%d, Currency=%s",
		payment.ID, payment.Status, payment.Amount.Total, payment.Currency)

	return &payment, nil
}

// verifyPaymentComplete validates payment status and details
func verifyPaymentComplete(payment *portonePaymentDetails, cart *models.Cart, cartID string, log *logging.Log) error {
	if payment.Status != "PAID" && payment.Status != "VIRTUAL_ACCOUNT_ISSUED" {
		return fmt.Errorf("payment not completed: %s", payment.Status)
	}

	if err := validatePaymentAmount(payment.Amount.Total, cart.AmountTotal, cart.Currency, log); err != nil {
		return err
	}

	if err := validatePaymentCurrency(payment.Currency, cart.Currency, log); err != nil {
		return err
	}

	return validateCartID(payment.CustomData, cartID, log)
}

func CompletePortonePayment(c fiber.Ctx) error {
	log := logging.New()

	var request struct {
		PaymentID string `json:"payment_id"`
		CartID    string `json:"cart_id"`
	}

	if err := c.Bind().JSON(&request); err != nil {
		return webutil.StatusBadRequest(c, "Invalid request")
	}

	if request.PaymentID == "" || request.CartID == "" {
		return webutil.StatusBadRequest(c, "Missing payment_id or cart_id")
	}

	// Load cart
	cart, err := store.Cart(c.Context(), request.CartID)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "Cart not found")
	}

	// Load PortOne settings
	settings, err := store.GetSettingByGroupTyped[models.Portone](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Fetch and parse payment details
	payment, err := fetchAndParsePortonePayment(request.PaymentID, settings.ApiSecret, log)
	if err != nil {
		log.ErrorStack(err)
		if err == ErrPaymentNotFound {
			return webutil.StatusBadRequest(c, "Payment not found")
		}
		return webutil.StatusInternalServerError(c)
	}

	// Verify payment is complete and valid
	if err := verifyPaymentComplete(payment, cart, request.CartID, log); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	// Update cart status
	cart.PaymentID = request.PaymentID
	cart.PaymentStatus = "paid"
	cart.PaymentSystem = "portone"
	if err := store.UpdateCart(c.Context(), cart); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Payment verified", map[string]interface{}{
		"status":  payment.Status,
		"cart_id": cart.ID,
	})
}

// verifyWebhookSignature verifies webhook signature using HMAC-SHA256
func verifyWebhookSignature(body []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// PortoneWebhook handles PortOne webhook notifications
//
// @Summary      PortOne webhook
// @Description  Handle PortOne webhook notifications for payment events
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Param        PortOne-Signature header string true "Webhook signature"
// @Success      200 {string} string "OK"
// @Failure      401 {string} string "Unauthorized"
// @Router       /api/payment/portone/webhook [post]
// portoneWebhookPayment represents payment data from webhook
type portoneWebhookPayment struct {
	Status     string `json:"status"`
	Currency   string `json:"currency"`
	CustomData string `json:"customData"`
	Amount     struct {
		Total int `json:"total"`
	} `json:"amount"`
}

// fetchPortonePaymentData fetches and parses payment data from PortOne API
func fetchPortonePaymentData(paymentID, apiSecret string, log *logging.Log) (*portoneWebhookPayment, string, error) {
	resp, err := callPortoneAPI("/payments/"+paymentID, apiSecret)
	if err != nil {
		return nil, "", fmt.Errorf("call API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var payment portoneWebhookPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, "", fmt.Errorf("decode payment: %w", err)
	}

	var customData struct {
		CartID string `json:"cart_id"`
	}
	if err := json.Unmarshal([]byte(payment.CustomData), &customData); err != nil {
		return nil, "", fmt.Errorf("decode custom data: %w", err)
	}

	return &payment, customData.CartID, nil
}

// verifyWebhookPaymentAmount verifies payment amount matches cart total
func verifyWebhookPaymentAmount(payment *portoneWebhookPayment, cart *models.Cart) bool {
	zeroDecimalCurrencies := map[string]bool{
		"KRW": true, "JPY": true, "VND": true, "CLP": true,
	}
	expectedAmount := cart.AmountTotal
	if zeroDecimalCurrencies[cart.Currency] {
		expectedAmount = cart.AmountTotal / 100
	}
	return payment.Amount.Total == expectedAmount && payment.Currency == cart.Currency
}

func PortoneWebhook(c fiber.Ctx) error {
	log := logging.New()

	// Read raw body for signature verification
	body := c.Body()

	// Load PortOne settings
	settings, err := store.GetSettingByGroupTyped[models.Portone](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// Verify webhook signature
	signature := c.Get("PortOne-Signature")
	if signature == "" {
		log.Error().Msg("Missing webhook signature")
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	if !verifyWebhookSignature(body, signature, settings.ApiSecret) {
		log.Error().Msg("Invalid webhook signature")
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	// Parse webhook payload
	var webhook struct {
		Type string `json:"type"`
		Data struct {
			PaymentID string `json:"paymentId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &webhook); err != nil {
		log.ErrorStack(err)
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Extract payment_id
	paymentID := webhook.Data.PaymentID
	if paymentID == "" {
		log.Error().Msg("Missing payment_id in webhook")
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Fetch payment data from PortOne API
	payment, cartID, err := fetchPortonePaymentData(paymentID, settings.ApiSecret, log)
	if err != nil {
		log.ErrorStack(err)
		return c.SendStatus(fiber.StatusOK) // Return 200 to prevent retry storms
	}

	// Load cart
	cart, err := store.Cart(c.Context(), cartID)
	if err != nil {
		log.ErrorStack(err)
		return c.SendStatus(fiber.StatusOK)
	}

	// Verify amount and currency
	if !verifyWebhookPaymentAmount(payment, cart) {
		log.Error().Msgf("Amount/currency mismatch in webhook: expected %d %s, got %d %s",
			cart.AmountTotal, cart.Currency, payment.Amount.Total, payment.Currency)
		return c.SendStatus(fiber.StatusOK)
	}

	// Update cart status
	if payment.Status == "PAID" || payment.Status == "VIRTUAL_ACCOUNT_ISSUED" {
		cart.PaymentID = paymentID
		cart.PaymentStatus = "paid"
		cart.PaymentSystem = "portone"
		if err := store.UpdateCart(c.Context(), cart); err != nil {
			log.ErrorStack(err)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
