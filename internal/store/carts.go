package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/litepay"
)

// Carts retrieves a paginated list of carts.
func Carts(ctx context.Context, limit, offset int) ([]*models.Cart, int, error) {
	// Handle limit=0 case
	if limit == 0 {
		limit = 999999
	}

	// List carts using function pointer
	results, err := db.ListOldCartsFunc(ctx, int32(limit), int32(offset))
	if err != nil {
		return nil, 0, err
	}

	// Convert to models.Cart
	carts := make([]*models.Cart, len(results))
	for i, row := range results {
		amountTotal, _ := strconv.Atoi(row.AmountTotal)
		cart := &models.Cart{
			Core: models.Core{
				ID: row.ID,
			},
			Email:         row.Email.String,
			AmountTotal:   amountTotal,
			Currency:      row.Currency,
			PaymentID:     row.PaymentID.String,
			PaymentStatus: litepay.Status(row.PaymentStatus.String),
			PaymentSystem: litepay.PaymentSystem(row.PaymentSystem),
		}
		if row.Created.Valid {
			cart.Created = row.Created.Time.Unix()
		}
		if row.Updated.Valid {
			cart.Updated = row.Updated.Time.Unix()
		}
		carts[i] = cart
	}

	// Count total records
	total, err := db.CountOldCartsFunc(ctx)
	if err != nil {
		return nil, 0, err
	}

	return carts, int(total), nil
}

// Cart retrieves a cart by ID with full details.
func Cart(ctx context.Context, cartID string) (*models.Cart, error) {
	result, err := db.GetOldCartFunc(ctx, cartID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, err
	}

	amountTotal, _ := strconv.Atoi(result.AmountTotal)
	cart := &models.Cart{
		Core: models.Core{
			ID: result.ID,
		},
		Email:         result.Email.String,
		AmountTotal:   amountTotal,
		Currency:      result.Currency,
		PaymentID:     result.PaymentID.String,
		PaymentStatus: litepay.Status(result.PaymentStatus.String),
		PaymentSystem: litepay.PaymentSystem(result.PaymentSystem),
	}
	if result.Created.Valid {
		cart.Created = result.Created.Time.Unix()
	}
	if result.Updated.Valid {
		cart.Updated = result.Updated.Time.Unix()
	}

	// Unmarshal cart products from JSON
	if len(result.Cart) > 0 {
		if err := json.Unmarshal([]byte(result.Cart), &cart.Cart); err != nil {
			return nil, err
		}
	}

	return cart, nil
}

// BuildCartItems builds cart items from cart and products data.
// This is a utility function for combining cart and product information.
func BuildCartItems(cart *models.Cart, products *models.Products) []map[string]any {
	if len(cart.Cart) == 0 || len(products.Products) == 0 {
		return nil
	}

	productMap := make(map[string]*models.Product, len(products.Products))
	for i := range products.Products {
		productMap[products.Products[i].ID] = &products.Products[i]
	}

	cartItems := make([]map[string]any, 0, len(cart.Cart))
	for _, cartItem := range cart.Cart {
		product, ok := productMap[cartItem.ProductID]
		if !ok {
			continue
		}

		// Generate unique ID: use product_id + variant_id when variant exists
		itemID := product.ID
		if cartItem.VariantID != nil && *cartItem.VariantID != "" {
			itemID = product.ID + "_" + *cartItem.VariantID
		}

		item := map[string]any{
			"id":       itemID,
			"name":     product.Name,
			"slug":     product.Slug,
			"amount":   product.Amount,
			"quantity": cartItem.Quantity,
		}

		if len(product.Images) > 0 {
			item["image"] = product.Images[0]
		}

		// Include variant information if present
		if cartItem.VariantID != nil && *cartItem.VariantID != "" {
			item["variant_id"] = *cartItem.VariantID

			// Find the matching variant details
			for _, variant := range product.Variants {
				if variant.ID == *cartItem.VariantID {
					// Include variant option values (e.g. {"Size": "Medium", "Color": "Black"})
					if len(variant.OptionValues) > 0 {
						item["variant_options"] = variant.OptionValues
					}
					// Include SKU if available
					if variant.SKU != "" {
						item["variant_sku"] = variant.SKU
					}
					// Include price surcharge for accurate unit price calculation
					item["variant_price_surcharge"] = variant.PriceSurcharge
					break
				}
			}
		}

		cartItems = append(cartItems, item)
	}

	return cartItems
}

// AddCart creates a new cart.
func AddCart(ctx context.Context, cart *models.Cart) error {
	byteCart, err := json.Marshal(cart.Cart)
	if err != nil {
		return err
	}

	params := db.CreateOldCartParams{
		ID:            cart.ID,
		Email:         sql.NullString{String: cart.Email, Valid: cart.Email != ""},
		AmountTotal:   strconv.Itoa(cart.AmountTotal),
		Currency:      cart.Currency,
		PaymentID:     sql.NullString{String: cart.PaymentID, Valid: cart.PaymentID != ""},
		PaymentStatus: sql.NullString{String: string(cart.PaymentStatus), Valid: cart.PaymentStatus != ""},
		Cart:          byteCart,
		PaymentSystem: string(cart.PaymentSystem),
	}

	_, err = db.CreateOldCartFunc(ctx, params)
	return err
}

// UpdateCart updates an existing cart.
func UpdateCart(ctx context.Context, cart *models.Cart) error {
	// Partial update: only payment_id and/or payment_status
	hasPaymentID := cart.PaymentID != ""
	hasPaymentStatus := cart.PaymentStatus != ""

	if hasPaymentID && hasPaymentStatus {
		// Update both fields
		return db.UpdateCartPaymentFieldsFunc(ctx,
			sql.NullString{String: cart.PaymentID, Valid: true},
			sql.NullString{String: string(cart.PaymentStatus), Valid: true},
			cart.ID,
		)
	} else if hasPaymentID {
		// Update only payment_id
		return db.UpdateCartPaymentIDFunc(ctx,
			sql.NullString{String: cart.PaymentID, Valid: true},
			cart.ID,
		)
	} else if hasPaymentStatus {
		// Update only payment_status
		return db.UpdateCartPaymentStatusFunc(ctx,
			sql.NullString{String: string(cart.PaymentStatus), Valid: true},
			cart.ID,
		)
	}

	return nil // No updates needed
}

// PaymentList retrieves available payment methods.
func PaymentList(ctx context.Context) (map[string]bool, error) {
	payments := map[string]bool{}

	results, err := db.GetPaymentSettingsFunc(ctx)
	if err != nil {
		return nil, err
	}

	for _, row := range results {
		vBool, err := strconv.ParseBool(row.Value.String)
		if err != nil {
			return nil, err
		}
		name := strings.ReplaceAll(row.Key, "_active", "")
		payments[name] = vBool
	}

	// Dummy provider is always active (no database check needed)
	payments["dummy"] = true

	return payments, nil
}

// ValidateCartItems validates cart items against product inventory and pricing.
func ValidateCartItems(ctx context.Context, products []models.CartProduct, currency string) (*models.CartValidationResult, error) {
	result := &models.CartValidationResult{
		Valid:          true,
		Errors:         nil,
		CorrectedItems: make([]models.CorrectedCartItem, len(products)),
	}

	if len(products) == 0 {
		return result, nil
	}

	// Extract product IDs for database query
	productIDs := make([]models.CartProduct, len(products))
	for i, p := range products {
		productIDs[i] = models.CartProduct{ProductID: p.ProductID}
	}

	// Fetch current product data
	// Use private=true to include inactive parent products (variant active status is checked separately)
	productList, err := ListProducts(ctx, true, 0, 0, "", productIDs...)
	if err != nil {
		return nil, err
	}

	// Build product map for quick lookup
	productMap := make(map[string]*models.Product, len(productList.Products))
	for i := range productList.Products {
		productMap[productList.Products[i].ID] = &productList.Products[i]
	}

	// Validate each cart item
	for i, requested := range products {
		validationError, correctedItem := validateCartItem(i, requested, productMap)

		result.CorrectedItems[i] = correctedItem

		if validationError != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *validationError)
		}
	}

	return result, nil
}

// CartLetterPayment builds an email message for payment notification
func CartLetterPayment(ctx context.Context, email, amountPayment, paymentURL string) (*models.MessageMail, error) {
	siteNameSetting, err := db.GetSettingByKeyFunc(ctx, "site_name")
	if err != nil {
		return nil, fmt.Errorf("get site_name: %w", err)
	}

	letterSetting, err := db.GetSettingByKeyFunc(ctx, "mail_letter_payment")
	if err != nil {
		return nil, fmt.Errorf("get mail_letter_payment: %w", err)
	}

	letterTemplate := models.Letter{}
	if letterSetting.Value.Valid && letterSetting.Value.String != "" {
		if err := json.Unmarshal([]byte(letterSetting.Value.String), &letterTemplate); err != nil {
			return nil, fmt.Errorf("unmarshal letter template: %w", err)
		}
	}

	mail := &models.MessageMail{
		To:     email,
		Letter: letterTemplate,
		Data: map[string]string{
			"Payment_URL":    paymentURL,
			"Site_Name":      siteNameSetting.Value.String,
			"Amount_Payment": amountPayment,
		},
	}

	return mail, nil
}

// CartLetterPurchase builds an email message for purchase confirmation
func CartLetterPurchase(ctx context.Context, cartID string) (*models.MessageMail, error) {
	mail := &models.MessageMail{}

	// Fetch cart with PAID status using sqlc query
	email, cartJSON, err := db.GetCartByStatusAndIDFunc(ctx,
		sql.NullString{String: string(litepay.PAID), Valid: true},
		cartID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, fmt.Errorf("query cart: %w", err)
	}
	mail.To = email

	// Unmarshal cart products
	products := []models.CartProduct{}
	if cartJSON != "" {
		if err := json.Unmarshal([]byte(cartJSON), &products); err != nil {
			return nil, fmt.Errorf("unmarshal cart: %w", err)
		}
	}

	// Get letter template
	letterSetting, err := db.GetSettingByKeyFunc(ctx, "mail_letter_purchase")
	if err != nil {
		return nil, fmt.Errorf("get mail_letter_purchase: %w", err)
	}

	letterTemplate := models.Letter{}
	if letterSetting.Value.Valid && letterSetting.Value.String != "" {
		if err := json.Unmarshal([]byte(letterSetting.Value.String), &letterTemplate); err != nil {
			return nil, fmt.Errorf("unmarshal letter template: %w", err)
		}
	}

	mail.Letter = letterTemplate
	mail.Data = map[string]string{
		"Cart_ID": cartID,
	}

	return mail, nil
}

// validateCartItem validates a single cart item against current product data
func validateCartItem(
	index int,
	requested models.CartProduct,
	productMap map[string]*models.Product,
) (*models.CartValidationError, models.CorrectedCartItem) {
	product, exists := productMap[requested.ProductID]

	// Check if product exists
	if !exists {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "product_not_found",
			RequestedQty:       requested.Quantity,
			AvailableQty:       0,
			RequestedUnitPrice: 0,
			CurrentUnitPrice:   0,
			RequestedTotal:     0,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  0,
			UnitPrice: 0,
			Available: false,
		}
	}

	// Handle variant validation
	// For variant products, the variant's active status is checked, not the parent's
	if requested.VariantID != nil && *requested.VariantID != "" {
		return validateVariantItem(index, requested, product)
	}

	// Check if product requires a variant but none was provided
	if product.HasVariants {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          nil,
			ErrorType:          "variant_required",
			RequestedQty:       requested.Quantity,
			AvailableQty:       0,
			RequestedUnitPrice: requested.UnitPrice,
			CurrentUnitPrice:   product.Amount,
			RequestedTotal:     0,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: nil,
			Quantity:  0,
			UnitPrice: product.Amount,
			Available: false,
		}
	}

	// Check if non-variant product is active
	if !product.Active {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "product_inactive",
			RequestedQty:       requested.Quantity,
			AvailableQty:       0,
			RequestedUnitPrice: product.Amount,
			CurrentUnitPrice:   product.Amount,
			RequestedTotal:     0,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  0,
			UnitPrice: product.Amount,
			Available: false,
		}
	}

	// Validate non-variant product
	return validateNonVariantItem(index, requested, product)
}

// validateVariantItem validates a cart item with a variant
func validateVariantItem(
	index int,
	requested models.CartProduct,
	product *models.Product,
) (*models.CartValidationError, models.CorrectedCartItem) {
	var variant *models.ProductVariant

	// Find the variant
	for i := range product.Variants {
		if product.Variants[i].ID == *requested.VariantID {
			variant = &product.Variants[i]
			break
		}
	}

	// Variant not found
	if variant == nil {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "product_not_found",
			RequestedQty:       requested.Quantity,
			AvailableQty:       0,
			RequestedUnitPrice: product.Amount,
			CurrentUnitPrice:   product.Amount,
			RequestedTotal:     0,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  0,
			UnitPrice: product.Amount,
			Available: false,
		}
	}

	// Variant inactive
	if !variant.Active {
		currentUnitPrice := product.Amount + variant.PriceSurcharge
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "product_inactive",
			RequestedQty:       requested.Quantity,
			AvailableQty:       0,
			RequestedUnitPrice: currentUnitPrice,
			CurrentUnitPrice:   currentUnitPrice,
			RequestedTotal:     0,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  0,
			UnitPrice: currentUnitPrice,
			Available: false,
		}
	}

	currentUnitPrice := product.Amount + variant.PriceSurcharge

	// Check if unit price changed (when client sends unit_price)
	if requested.UnitPrice > 0 && requested.UnitPrice != currentUnitPrice {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "price_changed",
			RequestedQty:       requested.Quantity,
			AvailableQty:       variant.Quantity,
			RequestedUnitPrice: requested.UnitPrice,
			CurrentUnitPrice:   currentUnitPrice,
			RequestedTotal:     requested.Quantity * requested.UnitPrice,
			CurrentTotal:       requested.Quantity * currentUnitPrice,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  requested.Quantity,
			UnitPrice: currentUnitPrice,
			Available: true,
		}
	}

	// Check quantity availability
	if requested.Quantity > variant.Quantity {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          requested.VariantID,
			ErrorType:          "quantity_unavailable",
			RequestedQty:       requested.Quantity,
			AvailableQty:       variant.Quantity,
			RequestedUnitPrice: currentUnitPrice,
			CurrentUnitPrice:   currentUnitPrice,
			RequestedTotal:     requested.Quantity * currentUnitPrice,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: requested.VariantID,
			Quantity:  0,
			UnitPrice: currentUnitPrice,
			Available: false,
		}
	}

	// All validation passed
	return nil, models.CorrectedCartItem{
		ProductID: requested.ProductID,
		VariantID: requested.VariantID,
		Quantity:  requested.Quantity,
		UnitPrice: currentUnitPrice,
		Available: true,
	}
}

// validateNonVariantItem validates a cart item without a variant
func validateNonVariantItem(
	index int,
	requested models.CartProduct,
	product *models.Product,
) (*models.CartValidationError, models.CorrectedCartItem) {
	currentUnitPrice := product.Amount

	// Check if unit price changed (when client sends unit_price)
	if requested.UnitPrice > 0 && requested.UnitPrice != currentUnitPrice {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          nil,
			ErrorType:          "price_changed",
			RequestedQty:       requested.Quantity,
			AvailableQty:       product.Quantity,
			RequestedUnitPrice: requested.UnitPrice,
			CurrentUnitPrice:   currentUnitPrice,
			RequestedTotal:     requested.Quantity * requested.UnitPrice,
			CurrentTotal:       requested.Quantity * currentUnitPrice,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: nil,
			Quantity:  requested.Quantity,
			UnitPrice: currentUnitPrice,
			Available: true,
		}
	}

	// Check quantity availability
	if requested.Quantity > product.Quantity {
		return &models.CartValidationError{
			ItemIndex:          index,
			ProductID:          requested.ProductID,
			VariantID:          nil,
			ErrorType:          "quantity_unavailable",
			RequestedQty:       requested.Quantity,
			AvailableQty:       product.Quantity,
			RequestedUnitPrice: currentUnitPrice,
			CurrentUnitPrice:   currentUnitPrice,
			RequestedTotal:     requested.Quantity * currentUnitPrice,
			CurrentTotal:       0,
		}, models.CorrectedCartItem{
			ProductID: requested.ProductID,
			VariantID: nil,
			Quantity:  0,
			UnitPrice: currentUnitPrice,
			Available: false,
		}
	}

	// All validation passed
	return nil, models.CorrectedCartItem{
		ProductID: requested.ProductID,
		VariantID: nil,
		Quantity:  requested.Quantity,
		UnitPrice: currentUnitPrice,
		Available: true,
	}
}
