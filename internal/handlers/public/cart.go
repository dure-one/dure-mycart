package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/shurco/mycart/internal/mailer"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/webhook"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/security"
	"github.com/shurco/mycart/pkg/webutil"
)

// sendPaymentWebhook sends a payment webhook notification.
// If blockOnError is true, returns error on webhook failure (for API endpoints).
// If blockOnError is false, logs error but doesn't block (for user-facing pages).
func sendPaymentWebhook(event webhook.Event, paymentSystem litepay.PaymentSystem, paymentStatus litepay.Status, cartID string, log *logging.Log, blockOnError bool) error {
	hook := &webhook.Payment{
		Event:     event,
		TimeStamp: time.Now().Unix(),
		Data: webhook.Data{
			PaymentSystem: paymentSystem,
			PaymentStatus: paymentStatus,
			CartID:        cartID,
		},
	}

	if err := webhook.SendPaymentHook(hook); err != nil {
		log.ErrorStack(err)
		if blockOnError {
			return err
		}
	}
	return nil
}

// PaymentList returns a list of available payment systems.
//
// @Summary      List payment providers
// @Description  Get active/inactive status of all payment providers
// @Tags         Cart
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Payment provider statuses"
// @Failure      400 {object} webutil.HTTPResponse "Bad request"
// @Router       /api/cart/payment [get]
func PaymentList(c fiber.Ctx) error {
	log := logging.New()
	paymentList, err := store.PaymentList(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	// Get current store currency for PortOne filtering
	currencySetting, err := store.GetSettingByKey(c.Context(), "currency")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}
	mainCurrency := currencySetting["currency"].Value.(string)

	// Filter PortOne based on supported currencies
	if paymentList["portone"] {
		portoneSettings, err := store.GetSettingByGroupTyped[models.Portone](c.Context())
		if err == nil && portoneSettings != nil && len(portoneSettings.SupportedCurrencies) > 0 {
			supported := false
			for _, curr := range portoneSettings.SupportedCurrencies {
				if curr == mainCurrency {
					supported = true
					break
				}
			}
			if !supported {
				paymentList["portone"] = false
			}
		}
	}

	return webutil.Response(c, fiber.StatusOK, "Payment list", paymentList)
}

// CreateCart creates a cart record in the database for PortOne payment flow.
// Unlike traditional providers that create cart during /cart/payment,
// PortOne needs cart_id upfront to pass to browser SDK.
//
// @Summary      Create cart
// @Description  Create a cart record and return its ID for browser-based payment (PortOne)
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Param        request body models.CartPayment true "Cart creation request"
// @Success      200 {object} webutil.HTTPResponse "Cart created"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/cart/create [post]
func CreateCart(c fiber.Ctx) error {
	log := logging.New()
	payment := new(models.CartPayment)

	if err := c.Bind().Body(payment); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	setting, err := store.GetSettingByKey(c.Context(), "currency")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}
	currency := setting["currency"].Value.(string)

	// Validate cart items before processing
	validationResult, err := store.ValidateCartItems(c.Context(), payment.Products, currency)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if !validationResult.Valid {
		return webutil.Response(c, fiber.StatusConflict, "Cart validation failed", map[string]any{
			"validation_errors": validationResult.Errors,
			"corrected_cart":    validationResult.CorrectedItems,
		})
	}

	// Calculate total amount using validated prices (includes variant surcharges)
	var amountTotal int
	for _, correctedItem := range validationResult.CorrectedItems {
		amountTotal += correctedItem.UnitPrice * correctedItem.Quantity
	}

	// Generate cart ID
	cartID := security.RandomString()

	// Create cart record
	if err := store.AddCart(c.Context(), &models.Cart{
		Core: models.Core{
			ID: cartID,
		},
		Email:         payment.Email,
		Cart:          payment.Products,
		AmountTotal:   amountTotal,
		Currency:      currency,
		PaymentStatus: litepay.NEW,
		PaymentSystem: payment.Provider,
	}); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Send webhook for cart initiation
	hook := &webhook.Payment{
		Event:     webhook.PAYMENT_INITIATION,
		TimeStamp: time.Now().Unix(),
		Data: webhook.Data{
			PaymentSystem: payment.Provider,
			PaymentStatus: litepay.NEW,
			CartID:        cartID,
			TotalAmount:   amountTotal,
			Currency:      currency,
		},
	}
	if err := webhook.SendPaymentHook(hook); err != nil {
		log.ErrorStack(err)
		// Don't fail cart creation if webhook fails
	}

	return webutil.Response(c, fiber.StatusOK, "Cart created", map[string]interface{}{
		"cart_id":      cartID,
		"amount_total": amountTotal,
		"currency":     currency,
	})
}

// GetCart returns cart information by cart_id.
//
// @Summary      Get cart (public)
// @Description  Get cart details including product items by cart ID
// @Tags         Cart
// @Produce      json
// @Param        cart_id path string true "Cart ID"
// @Success      200 {object} webutil.HTTPResponse "Cart details"
// @Failure      400 {object} webutil.HTTPResponse "Missing cart_id"
// @Failure      404 {object} webutil.HTTPResponse "Cart not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/cart/{cart_id} [get]
func GetCart(c fiber.Ctx) error {
	log := logging.New()
	cartID := c.Params("cart_id")

	if cartID == "" {
		return webutil.StatusBadRequest(c, "cart_id is required")
	}

	cart, err := store.Cart(c.Context(), cartID)
	if err != nil {
		log.ErrorStack(err)
		if errors.Is(err, errors.ErrProductNotFound) {
			return webutil.StatusNotFound(c)
		}
		return webutil.StatusInternalServerError(c)
	}

	// Load full product information for cart items
	// Pass cartID to include digital products purchased in this cart
	var cartItems []map[string]any
	if len(cart.Cart) > 0 {
		products, err := store.ListProducts(c.Context(), false, 0, 0, cartID, cart.Cart...)
		if err != nil {
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}
		cartItems = store.BuildCartItems(cart, products)
	}

	return webutil.Response(c, fiber.StatusOK, "Cart", map[string]any{
		"id":             cart.ID,
		"email":          cart.Email,
		"amount_total":   cart.AmountTotal,
		"currency":       cart.Currency,
		"payment_status": cart.PaymentStatus,
		"payment_system": cart.PaymentSystem,
		"items":          cartItems,
	})
}

// initStripePayment initializes Stripe payment session
func initStripePayment(ctx context.Context, pay litepay.Cfg, cart litepay.Cart) (string, error) {
	setting, err := store.GetSettingByGroupTyped[models.Stripe](ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Stripe settings: %w", err)
	}

	if !setting.Active {
		return "", nil // Provider inactive, will use fallback URL
	}

	session := pay.Stripe(setting.SecretKey)
	response, err := session.Pay(cart)
	if err != nil {
		return "", fmt.Errorf("Stripe payment failed: %w", err)
	}

	return response.URL, nil
}

// initPaypalPayment initializes PayPal payment session
func initPaypalPayment(ctx context.Context, pay litepay.Cfg, cart litepay.Cart) (string, error) {
	setting, err := store.GetSettingByGroupTyped[models.Paypal](ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get PayPal settings: %w", err)
	}

	if !setting.Active {
		return "", nil // Provider inactive, will use fallback URL
	}

	session := pay.Paypal(setting.ClientID, setting.SecretKey)
	response, err := session.Pay(cart)
	if err != nil {
		return "", fmt.Errorf("PayPal payment failed: %w", err)
	}

	return response.URL, nil
}

// initSpectrocoinPayment initializes Spectrocoin payment session
func initSpectrocoinPayment(ctx context.Context, pay litepay.Cfg, cart litepay.Cart) (string, error) {
	setting, err := store.GetSettingByGroupTyped[models.Spectrocoin](ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Spectrocoin settings: %w", err)
	}

	if !setting.Active {
		return "", nil // Provider inactive, will use fallback URL
	}

	session := pay.Spectrocoin(setting.MerchantID, setting.ProjectID, setting.PrivateKey)
	response, err := session.Pay(cart)
	if err != nil {
		return "", fmt.Errorf("Spectrocoin payment failed: %w", err)
	}

	return response.URL, nil
}

// initCoinbasePayment initializes Coinbase payment session
func initCoinbasePayment(ctx context.Context, pay litepay.Cfg, cart litepay.Cart) (string, error) {
	setting, err := store.GetSettingByGroupTyped[models.Coinbase](ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Coinbase settings: %w", err)
	}

	if !setting.Active {
		return "", nil // Provider inactive, will use fallback URL
	}

	session := pay.Coinbase(setting.ApiKey)
	response, err := session.Pay(cart)
	if err != nil {
		return "", fmt.Errorf("Coinbase payment failed: %w", err)
	}

	return response.URL, nil
}

// initDummyPayment initializes dummy payment session (free carts only)
func initDummyPayment(ctx context.Context, pay litepay.Cfg, cart litepay.Cart) (string, error) {
	session := pay.Dummy()
	response, err := session.Pay(cart)
	if err != nil {
		return "", fmt.Errorf("dummy payment failed: %w", err)
	}

	return response.URL, nil
}

// dispatchPaymentInit routes payment initialization to the appropriate provider
func dispatchPaymentInit(ctx context.Context, paymentSystem litepay.PaymentSystem, pay litepay.Cfg, cart litepay.Cart, fallbackURL string) (string, error) {
	var paymentURL string
	var err error

	switch paymentSystem {
	case litepay.STRIPE:
		paymentURL, err = initStripePayment(ctx, pay, cart)
	case litepay.PAYPAL:
		paymentURL, err = initPaypalPayment(ctx, pay, cart)
	case litepay.SPECTROCOIN:
		paymentURL, err = initSpectrocoinPayment(ctx, pay, cart)
	case litepay.COINBASE:
		paymentURL, err = initCoinbasePayment(ctx, pay, cart)
	case litepay.DUMMY:
		paymentURL, err = initDummyPayment(ctx, pay, cart)
	default:
		return "", fmt.Errorf("unsupported payment system: %s", paymentSystem)
	}

	if err != nil {
		return "", err
	}

	// If provider is inactive (paymentURL is empty), use fallback
	if paymentURL == "" {
		return fallbackURL, nil
	}

	return paymentURL, nil
}

// finalizePaymentInit saves cart, sends prepayment email, and triggers webhook
func finalizePaymentInit(ctx context.Context, cart litepay.Cart, payment *models.CartPayment, paymentSystem litepay.PaymentSystem, amountTotal int, items []litepay.Item, paymentURL string) error {
	log := logging.New()

	// Save cart
	if err := store.AddCart(ctx, &models.Cart{
		Core: models.Core{
			ID: cart.ID,
		},
		Email:         payment.Email,
		Cart:          payment.Products,
		AmountTotal:   amountTotal,
		Currency:      cart.Currency,
		PaymentStatus: litepay.NEW,
		PaymentSystem: paymentSystem,
	}); err != nil {
		return fmt.Errorf("failed to save cart: %w", err)
	}

	// Send prepayment email
	if err := mailer.SendPrepaymentLetter(payment.Email, fmt.Sprintf("%.2f %s", float64(amountTotal)/100, cart.Currency), paymentURL); err != nil {
		return fmt.Errorf("failed to send prepayment email: %w", err)
	}

	// Send payment initiation webhook
	hook := &webhook.Payment{
		Event:     webhook.PAYMENT_INITIATION,
		TimeStamp: time.Now().Unix(),
		Data: webhook.Data{
			PaymentSystem: paymentSystem,
			PaymentStatus: litepay.NEW,
			CartID:        cart.ID,
			TotalAmount:   amountTotal,
			Currency:      cart.Currency,
			CartItems:     items,
		},
	}
	if err := webhook.SendPaymentHook(hook); err != nil {
		return fmt.Errorf("failed to send payment webhook: %w", err)
	}

	log.Info().Msgf("Payment initialized: cart=%s, system=%s, amount=%d", cart.ID, paymentSystem, amountTotal)
	return nil
}

// Payment initiates a payment process for a cart.
//
// @Summary      Initiate payment
// @Description  Create a payment session and return a redirect URL for the selected provider
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Param        request body models.CartPayment true "Payment request"
// @Success      200 {object} webutil.HTTPResponse "Payment URL"
// @Failure      400 {object} webutil.HTTPResponse "Validation error or dummy provider for paid cart"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /cart/payment [post]
func Payment(c fiber.Ctx) error {
	log := logging.New()
	payment := new(models.CartPayment)

	if err := c.Bind().Body(payment); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	setting, err := store.GetSettingByKey(c.Context(), "domain", "currency")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}
	domain := setting["domain"].Value.(string)
	currency := setting["currency"].Value.(string)

	products, err := store.ListProducts(c.Context(), false, 0, 0, "", payment.Products...)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Validate cart items before processing
	validationResult, err := store.ValidateCartItems(c.Context(), payment.Products, currency)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if !validationResult.Valid {
		return webutil.Response(c, fiber.StatusConflict, "Cart validation failed", map[string]any{
			"validation_errors": validationResult.Errors,
			"corrected_cart":    validationResult.CorrectedItems,
		})
	}

	// Use request scheme (http/https) for URLs
	protocol := c.Scheme()

	// Build product map for quick lookup
	productMap := make(map[string]*models.Product)
	for i := range products.Products {
		productMap[products.Products[i].ID] = &products.Products[i]
	}

	// Build cart items using validated prices
	items := make([]litepay.Item, 0, len(validationResult.CorrectedItems))
	var amountTotal int
	for _, correctedItem := range validationResult.CorrectedItems {
		product, exists := productMap[correctedItem.ProductID]
		if !exists {
			continue
		}

		images := []string{}
		for _, image := range product.Images {
			path := fmt.Sprintf("%s://%s/uploads/%s_md.%s", protocol, domain, image.Name, image.Ext)
			images = append(images, path)
		}

		items = append(items, litepay.Item{
			PriceData: litepay.Price{
				UnitAmount: correctedItem.UnitPrice, // Uses validated price with variant surcharge
				Product: litepay.Product{
					Name:        product.Name,
					Description: product.Description,
					Images:      images,
				},
			},
			Quantity: correctedItem.Quantity,
		})

		amountTotal += correctedItem.UnitPrice * correctedItem.Quantity
	}

	cart := litepay.Cart{
		ID:       security.RandomString(),
		Currency: currency,
		Items:    items,
	}

	// Validate dummy provider usage: only allowed for free carts (amountTotal = 0)
	paymentSystem := payment.Provider
	if paymentSystem == litepay.DUMMY && amountTotal > 0 {
		log.Error().Msg("Attempt to use dummy provider for paid cart")
		return webutil.StatusBadRequest(c, "Dummy payment provider can only be used for free items")
	}

	// Initialize payment provider
	callbackURL := fmt.Sprintf("%s://%s/cart/payment/callback", protocol, domain)
	successURL := fmt.Sprintf("%s://%s/cart/payment/success", protocol, domain)
	cancelURL := fmt.Sprintf("%s://%s/cart/payment/cancel", protocol, domain)
	pay := litepay.New(callbackURL, successURL, cancelURL)
	fallbackURL := fmt.Sprintf("%s://%s/cart", protocol, domain)

	paymentURL, err := dispatchPaymentInit(c.Context(), paymentSystem, pay, cart, fallbackURL)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Finalize payment (save cart, send email, trigger webhook)
	if err := finalizePaymentInit(c.Context(), cart, payment, paymentSystem, amountTotal, items, paymentURL); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Payment url", map[string]string{"url": paymentURL})
}

// PaymentCallback handles payment callback from payment providers.
//
// @Summary      Payment callback
// @Description  Webhook endpoint for payment providers to report status changes
// @Tags         Cart
// @Accept       json
// @Produce      plain
// @Param        cart_id        query string true "Cart ID"
// @Param        payment_system query string true "Payment system"
// @Success      200 {string} string "*ok*"
// @Failure      400 {object} webutil.HTTPResponse "Bad request"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /cart/payment/callback [post]
func PaymentCallback(c fiber.Ctx) error {
	log := logging.New()
	payment := &litepay.Payment{
		CartID:        c.Query("cart_id"),
		PaymentSystem: litepay.PaymentSystem(c.Query("payment_system")),
	}

	switch payment.PaymentSystem {
	// case litepay.STRIPE:
	//	return webutil.Response(c, fiber.StatusOK, "Callback", payment)
	case litepay.SPECTROCOIN:
		response := new(litepay.CallbackSpectrocoin)
		if err := c.Bind().Body(response); err != nil {
			log.ErrorStack(err)
			return webutil.StatusBadRequest(c, err.Error())
		}
		// Verify signature before trusting any callback data
		if err := litepay.VerifySpectrocoinCallback(response); err != nil {
			log.Error().Err(err).Msg("Invalid SpectroCoin callback signature")
			return webutil.StatusBadRequest(c, "Invalid signature")
		}
		payment.Status = litepay.StatusPayment(litepay.SPECTROCOIN, string(rune(response.Status)))
		payment.MerchantID = response.MerchantApiID
		payment.Coin = &litepay.Coin{
			AmountTotal: response.ReceiveAmount,
			Currency:    response.ReceiveCurrency,
		}
	default:
		return webutil.StatusBadRequest(c, "Unsupported payment system for callbacks")
	}

	err := store.UpdateCart(c.Context(), &models.Cart{
		Core: models.Core{
			ID: payment.CartID,
		},
		PaymentID:     payment.MerchantID,
		PaymentStatus: payment.Status,
		PaymentSystem: payment.PaymentSystem,
	})
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// send email
	if payment.Status == litepay.PAID {
		if err := mailer.SendCartLetter(payment.CartID); err != nil {
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}
	}

	// send hook
	if err := sendPaymentWebhook(webhook.PAYMENT_CALLBACK, payment.PaymentSystem, payment.Status, payment.CartID, log, true); err != nil {
		return webutil.StatusInternalServerError(c)
	}

	return c.Status(fiber.StatusOK).SendString("*ok*")
}

// processStripePayment handles Stripe payment verification
func processStripePayment(ctx context.Context, session string, cartInfo *models.Cart, payment *litepay.Payment) error {
	log := logging.New()

	// Validate session against cart's payment ID (cross-cart replay defense)
	if cartInfo.PaymentID != "" && (session == "" || session != cartInfo.PaymentID) {
		return fmt.Errorf("invalid or mismatched session")
	}

	setting, err := store.GetSettingByGroupTyped[models.Stripe](ctx)
	if err != nil {
		return fmt.Errorf("failed to get Stripe settings: %w", err)
	}

	if !setting.Active {
		return fmt.Errorf("Stripe provider is not active")
	}

	response, err := litepay.New("", "", "").Stripe(setting.SecretKey).Checkout(payment, session)
	if err != nil {
		return fmt.Errorf("Stripe checkout failed: %w", err)
	}

	payment.MerchantID = response.MerchantID
	payment.Status = response.Status
	log.Info().Msgf("Stripe payment processed: %s", payment.Status)
	return nil
}

// processPaypalPayment handles PayPal payment verification
func processPaypalPayment(ctx context.Context, token string, cartInfo *models.Cart, payment *litepay.Payment) error {
	log := logging.New()

	// Validate token against cart's payment ID (cross-cart replay defense)
	if cartInfo.PaymentID != "" && (token == "" || token != cartInfo.PaymentID) {
		return fmt.Errorf("invalid or mismatched token")
	}

	setting, err := store.GetSettingByGroupTyped[models.Paypal](ctx)
	if err != nil {
		return fmt.Errorf("failed to get PayPal settings: %w", err)
	}

	if !setting.Active {
		return fmt.Errorf("PayPal provider is not active")
	}

	response, err := litepay.New("", "", "").Paypal(setting.ClientID, setting.SecretKey).Checkout(payment, token)
	if err != nil {
		return fmt.Errorf("PayPal checkout failed: %w", err)
	}

	payment.MerchantID = response.MerchantID
	payment.Status = response.Status
	log.Info().Msgf("PayPal payment processed: %s", payment.Status)
	return nil
}

// processCoinbasePayment handles Coinbase payment verification
func processCoinbasePayment(ctx context.Context, chargeID string, cartInfo *models.Cart, payment *litepay.Payment) error {
	log := logging.New()

	// Validate chargeID against cart's payment ID (cross-cart replay defense)
	if cartInfo.PaymentID != "" && (chargeID == "" || chargeID != cartInfo.PaymentID) {
		return fmt.Errorf("invalid or mismatched charge_id")
	}

	setting, err := store.GetSettingByGroupTyped[models.Coinbase](ctx)
	if err != nil {
		return fmt.Errorf("failed to get Coinbase settings: %w", err)
	}

	if !setting.Active {
		return fmt.Errorf("Coinbase provider is not active")
	}

	response, err := litepay.New("", "", "").Coinbase(setting.ApiKey).Checkout(payment, chargeID)
	if err != nil {
		return fmt.Errorf("Coinbase checkout failed: %w", err)
	}

	payment.MerchantID = response.MerchantID
	payment.Status = response.Status
	log.Info().Msgf("Coinbase payment processed: %s", payment.Status)
	return nil
}

// processDummyPayment handles dummy payment provider (free carts only)
func processDummyPayment(ctx context.Context, payment *litepay.Payment) error {
	log := logging.New()

	response, err := litepay.New("", "", "").Dummy().Checkout(payment, "")
	if err != nil {
		return fmt.Errorf("dummy checkout failed: %w", err)
	}

	payment.MerchantID = response.MerchantID
	payment.Status = response.Status
	log.Info().Msgf("Dummy payment processed: %s", payment.Status)
	return nil
}

// validatePaymentRequest validates payment request parameters and cart state
func validatePaymentRequest(c fiber.Ctx, payment *litepay.Payment, cartInfo *models.Cart) error {
	log := logging.New()

	// Validate cart_id presence
	if c.Query("cart_id") == "" {
		return fmt.Errorf("missing cart_id")
	}

	// Validate payment object
	if err := payment.Validate(); err != nil {
		return fmt.Errorf("invalid payment: %w", err)
	}

	// Validate dummy provider usage: only allowed for free carts
	if payment.PaymentSystem == litepay.DUMMY && cartInfo.AmountTotal > 0 {
		log.Error().Msgf("Attempt to use dummy provider for paid cart (cart_id: %s, amount: %d)", payment.CartID, cartInfo.AmountTotal)
		return fmt.Errorf("dummy payment provider can only be used for free items")
	}

	// Check if already paid
	if cartInfo.PaymentStatus == "paid" {
		return fmt.Errorf("cart already paid")
	}

	return nil
}

// dispatchPaymentProvider routes payment to the appropriate provider handler
func dispatchPaymentProvider(ctx context.Context, c fiber.Ctx, payment *litepay.Payment, cartInfo *models.Cart) error {
	switch payment.PaymentSystem {
	case litepay.STRIPE:
		return processStripePayment(ctx, c.Query("session"), cartInfo, payment)
	case litepay.PAYPAL:
		return processPaypalPayment(ctx, c.Query("token"), cartInfo, payment)
	case litepay.COINBASE:
		return processCoinbasePayment(ctx, c.Query("charge_id"), cartInfo, payment)
	case litepay.DUMMY:
		return processDummyPayment(ctx, payment)
	case litepay.SPECTROCOIN:
		// Spectrocoin payment processing handled in callback
		return nil
	default:
		return fmt.Errorf("unsupported payment system: %s", payment.PaymentSystem)
	}
}

// completePaymentProcessing updates cart, sends email, and triggers webhook
func completePaymentProcessing(ctx context.Context, payment *litepay.Payment) error {
	log := logging.New()

	// Update cart with payment information
	err := store.UpdateCart(ctx, &models.Cart{
		Core: models.Core{
			ID: payment.CartID,
		},
		PaymentID:     payment.MerchantID,
		PaymentStatus: payment.Status,
		PaymentSystem: payment.PaymentSystem,
	})
	if err != nil {
		return fmt.Errorf("failed to update cart: %w", err)
	}

	// Send email if payment is complete
	if payment.Status == litepay.PAID {
		if err := mailer.SendCartLetter(payment.CartID); err != nil {
			return fmt.Errorf("failed to send cart email: %w", err)
		}
	}

	// Send webhook (don't block on error)
	sendPaymentWebhook(webhook.PAYMENT_SUCCESS, payment.PaymentSystem, payment.Status, payment.CartID, log, false)

	return nil
}

// PaymentSuccess handles successful payment redirects.
//
// @Summary      Payment success
// @Description  Handle successful payment redirect, verify with provider, and update cart
// @Tags         Cart
// @Produce      json
// @Param        cart_id        query string true  "Cart ID"
// @Param        payment_system query string true  "Payment system"
// @Param        session        query string false "Stripe session ID"
// @Param        token          query string false "PayPal token"
// @Param        charge_id      query string false "Coinbase charge ID"
// @Success      200 "Passes to SPA handler"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /cart/payment/success [get]
func PaymentSuccess(c fiber.Ctx) error {
	// Only process GET requests
	if c.Method() != fiber.MethodGet {
		return c.Next()
	}

	log := logging.New()
	payment := &litepay.Payment{
		CartID:        c.Query("cart_id"),
		PaymentSystem: litepay.PaymentSystem(c.Query("payment_system")),
	}

	cartInfo, err := store.Cart(c.Context(), c.Query("cart_id"))
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Validate payment request
	if err := validatePaymentRequest(c, payment, cartInfo); err != nil {
		log.ErrorStack(err)
		switch err.Error() {
		case "cart already paid":
			return c.Next() // Pass to SPA
		case "missing cart_id", "dummy payment provider can only be used for free items":
			return webutil.StatusBadRequest(c, err.Error())
		default:
			if strings.Contains(err.Error(), "invalid payment") {
				return c.Redirect().To("/")
			}
			return webutil.StatusBadRequest(c, err.Error())
		}
	}

	// Process payment with provider
	if err := dispatchPaymentProvider(c.Context(), c, payment, cartInfo); err != nil {
		log.ErrorStack(err)
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "invalid or mismatched"):
			return webutil.StatusBadRequest(c, errMsg)
		case strings.Contains(errMsg, "provider is not active"):
			return webutil.StatusNotFound(c)
		default:
			return webutil.StatusInternalServerError(c)
		}
	}

	// Complete payment processing (update cart, send email, trigger webhook)
	if err := completePaymentProcessing(c.Context(), payment); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// After processing payment, pass control to SPA handler
	// The SPA will display the success page with cart information
	return c.Next()
}

// PaymentCancel handles canceled payment redirects.
//
// @Summary      Payment cancel
// @Description  Handle canceled payment, update cart status, and redirect to SPA
// @Tags         Cart
// @Produce      json
// @Param        cart_id        query string false "Cart ID"
// @Param        payment_system query string false "Payment system"
// @Success      302 "Redirect to cancel page"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /cart/payment/cancel [get]
func PaymentCancel(c fiber.Ctx) error {
	// Only process GET requests
	if c.Method() != fiber.MethodGet {
		return c.Next()
	}

	// If error parameter is present, pass to SPA for error display
	if c.Query("error") != "" {
		return c.Next()
	}

	log := logging.New()
	payment := &litepay.Payment{
		CartID:        c.Query("cart_id"),
		PaymentSystem: litepay.PaymentSystem(c.Query("payment_system")),
	}

	// Validate cancel token
	cancelToken := c.Query("cancel_token")
	if cancelToken == "" {
		return c.Redirect().To("/cart/payment/cancel?error=invalid_token")
	}

	// Get JWT secret for token verification
	settingJWT, err := store.GetSettingByGroupTyped[models.JWT](c.Context())
	if err != nil {
		log.ErrorStack(err)
		return c.Redirect().To("/cart/payment/cancel?error=server_error")
	}

	// Parse and verify the cancel token
	token, err := jwt.Parse(cancelToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(settingJWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return c.Redirect().To("/cart/payment/cancel?error=invalid_token")
	}

	// Extract claims and verify the token is for this cart
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Redirect().To("/cart/payment/cancel?error=invalid_token")
	}

	tokenCartID, ok := claims["id"].(string)
	if !ok || tokenCartID != payment.CartID {
		return c.Redirect().To("/cart/payment/cancel?error=invalid_token")
	}

	err = store.UpdateCart(c.Context(), &models.Cart{
		Core: models.Core{
			ID: payment.CartID,
		},
		PaymentStatus: litepay.CANCELED,
		PaymentSystem: payment.PaymentSystem,
	})
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// send hook (don't block process on webhook error)
	sendPaymentWebhook(webhook.PAYMENT_CANCEL, payment.PaymentSystem, litepay.CANCELED, payment.CartID, log, false)

	// Redirect to SPA cancel page with query parameters
	redirectURL := "/cart/payment/cancel"
	if payment.CartID != "" {
		redirectURL += "?cart_id=" + payment.CartID
		if string(payment.PaymentSystem) != "" {
			redirectURL += "&payment_system=" + string(payment.PaymentSystem)
		}
	}
	return c.Redirect().To(redirectURL)
}
