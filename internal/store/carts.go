package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/litepay"
)

// CartQueries is a struct that holds a dialect-aware database handle.
// Calls on DB are automatically rebound for the configured dialect.
type CartQueries struct {
	DB *database.Conn
}

// PaymentList retrieves the status of different payment methods from the database.
func (q *CartQueries) PaymentList(ctx context.Context) (map[string]bool, error) {
	payments := map[string]bool{}
	keys := []any{
		"stripe_active", "paypal_active", "spectrocoin_active", "coinbase_active", "portone_active",
	}

	query := fmt.Sprintf("SELECT key, value FROM setting WHERE key IN (%s)", strings.Repeat("?, ", len(keys)-1)+"?")
	rows, err := q.DB.QueryContext(ctx, query, keys...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}

		vBool, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		name := strings.ReplaceAll(key, "_active", "")
		payments[name] = vBool
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Dummy provider is always active (no database check needed)
	payments["dummy"] = true

	return payments, nil
}

// Carts retrieves a list of carts from the database.
func (q *CartQueries) Carts(ctx context.Context, limit, offset int) ([]*models.Cart, int, error) {
	carts := []*models.Cart{}

	query := fmt.Sprintf(`
	SELECT
		id,
		email,
		amount_total,
		currency,
		payment_id,
		payment_status,
		payment_system,
		%s,
		%s
	FROM cart
	ORDER BY created DESC
`, q.DB.Dialect().Epoch("created"), q.DB.Dialect().Epoch("updated"))

	// Add pagination
	var params []any
	if limit > 0 {
		query += " LIMIT ?"
		params = append(params, limit)
		if offset > 0 {
			query += " OFFSET ?"
			params = append(params, offset)
		}
	}

	rows, err := q.DB.QueryContext(ctx, query, params...)
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

// Cart retrieves a cart from the database using the provided cartId.
func (q *CartQueries) Cart(ctx context.Context, cartId string) (*models.Cart, error) {
	query := fmt.Sprintf(`
	SELECT 
    id, 
    email, 
    cart,
    amount_total,
    currency,
    payment_id,
    payment_status,
    payment_system,
    %s,
    %s
	FROM cart
	WHERE id = ?
	`, q.DB.Dialect().Epoch("created"), q.DB.Dialect().Epoch("updated"))

	var email, paymentID, cartJSON sql.NullString
	var created, updated sql.NullInt64
	cart := &models.Cart{}

	err := q.DB.QueryRowContext(ctx, query, cartId).
		Scan(
			&cart.ID,
			&email,
			&cartJSON,
			&cart.AmountTotal,
			&cart.Currency,
			&paymentID,
			&cart.PaymentStatus,
			&cart.PaymentSystem,
			&created,
			&updated,
		)
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

// AddCart inserts a new cart into the database.
func (q *CartQueries) AddCart(ctx context.Context, cart *models.Cart) error {
	byteCart, err := json.Marshal(cart.Cart)
	if err != nil {
		return err
	}

	query := `INSERT INTO cart (id, email, cart, amount_total, currency, payment_id, payment_status, payment_system) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = q.DB.ExecContext(ctx, query, cart.ID, cart.Email, string(byteCart), cart.AmountTotal, cart.Currency, cart.PaymentID, cart.PaymentStatus, cart.PaymentSystem)
	return err
}

// UpdateCart updates the cart details in the database.
func (q *CartQueries) UpdateCart(ctx context.Context, cart *models.Cart) error {
	var (
		args []any
		sql  strings.Builder
	)

	sql.WriteString("UPDATE cart SET ")

	if cart.PaymentID != "" {
		sql.WriteString("payment_id = ?, ")
		args = append(args, cart.PaymentID)
	}

	if cart.PaymentStatus != "" {
		sql.WriteString("payment_status = ?, ")
		args = append(args, cart.PaymentStatus)
	}

	sql.WriteString("updated = CURRENT_TIMESTAMP WHERE id = ?")
	args = append(args, cart.ID)

	_, err := q.DB.ExecContext(ctx, sql.String(), args...)
	return err
}

// CartLetterPayment is ...
func (q *CartQueries) CartLetterPayment(ctx context.Context, email, amountPayment, paymentURL string) (*models.MessageMail, error) {
	mailLetter, err := NewBase(q.DB).GetSettingByKey(ctx, "site_name", "mail_letter_payment")
	if err != nil {
		return nil, err
	}
	letterTemplate := models.Letter{}
	if err := json.Unmarshal([]byte(mailLetter["mail_letter_payment"].Value.(string)), &letterTemplate); err != nil {
		return nil, err
	}

	mail := &models.MessageMail{
		To:     email,
		Letter: letterTemplate,
		Data: map[string]string{
			"Payment_URL":    paymentURL,
			"Site_Name":      mailLetter["site_name"].Value.(string),
			"Amount_Payment": amountPayment,
		},
	}

	return mail, nil
}

// CartLetterPurchase is ...
func (q *CartQueries) CartLetterPurchase(ctx context.Context, cartID string) (*models.MessageMail, error) {
	mail := &models.MessageMail{}

	// Fetch the email, cart information, and 'email' setting in one query.
	var cartJSON string
	err := q.DB.QueryRowContext(ctx, `
        SELECT email, cart
        FROM cart
        WHERE payment_status = ? AND id = ?
    `, litepay.PAID, cartID).Scan(&mail.To, &cartJSON)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	// Unmarshal the products from the cart JSON.
	products := []models.CartProduct{}
	if err := json.Unmarshal([]byte(cartJSON), &products); err != nil {
		return nil, err
	}

	// Begin a transaction.
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	keys := []models.Data{}
	files := []models.File{}
	for _, cart := range products {
		var digitalType string
		err := tx.QueryRowContext(ctx, `SELECT digital FROM product WHERE id = ?`, cart.ProductID).Scan(&digitalType)
		if err != nil {
			if stderrors.Is(err, sql.ErrNoRows) {
				return nil, errors.ErrPageNotFound
			}
			return nil, err
		}

		switch digitalType {
		case "file":
			productFiles, err := scanDigitalFiles(ctx, tx, cart.ProductID)
			if err != nil {
				return nil, err
			}
			files = append(files, productFiles...)
		case "data":
			key, err := claimDigitalData(ctx, tx, cartID, cart.ProductID)
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Construct the purchases information.
	var purchases strings.Builder
	count := 1
	if len(keys) > 0 {
		purchases.WriteString("Keys:\n")
		for _, key := range keys {
			purchases.WriteString(fmt.Sprintf("%v: %s\n", count, key.Content))
			count++
		}
	}
	if len(files) > 0 {
		purchases.WriteString("Files:\n")
		for _, file := range files {
			purchases.WriteString(fmt.Sprintf("%v: %s\n", count, file.OrigName))
			count++
		}
	}

	// Fetch the 'mail_letter_purchase' setting value.
	mailLetter, err := NewBase(q.DB).GetSettingByKey(ctx, "email", "mail_letter_purchase")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(mailLetter["mail_letter_purchase"].Value.(string)), &mail.Letter); err != nil {
		return nil, err
	}

	mail.Data = map[string]string{
		"Purchases":   purchases.String(),
		"Admin_Email": mailLetter["email"].Value.(string),
	}
	mail.Files = files

	return mail, nil
}

// claimDigitalData hands one unassigned digital_data row to cartID and returns
// it. Already owning a row makes the call idempotent, so a retried purchase
// mail keeps its keys.
//
// The row is claimed by an UPDATE guarded on `cart_id IS NULL`, and the claim
// only counts when it reports having changed a row. Reading a candidate and
// then updating it unconditionally — as this used to do — lets two concurrent
// purchases pick the same row and hands the same key to both buyers. SQLite hid
// the race behind `_txlock=immediate`; PostgreSQL does not.
func claimDigitalData(ctx context.Context, tx *database.Tx, cartID, productID string) (models.Data, error) {
	// Bounded so that a pathological amount of contention fails loudly instead
	// of spinning; each lost round is another buyer taking a key, so the loop
	// makes progress and this is a safety net, not the normal exit.
	const maxAttempts = 100

	for range maxAttempts {
		key := models.Data{}
		err := tx.QueryRowContext(ctx,
			`SELECT id, content FROM digital_data WHERE cart_id = ? AND product_id = ?`,
			cartID, productID).Scan(&key.ID, &key.Content)
		if err == nil {
			return key, nil
		}
		if !stderrors.Is(err, sql.ErrNoRows) {
			return models.Data{}, err
		}

		var candidate models.Data
		err = tx.QueryRowContext(ctx,
			`SELECT id, content FROM digital_data WHERE cart_id IS NULL AND product_id = ? ORDER BY id LIMIT 1`,
			productID).Scan(&candidate.ID, &candidate.Content)
		if err != nil {
			if stderrors.Is(err, sql.ErrNoRows) {
				return models.Data{}, errors.ErrPageNotFound
			}
			return models.Data{}, err
		}

		result, err := tx.ExecContext(ctx,
			`UPDATE digital_data SET cart_id = ? WHERE id = ? AND cart_id IS NULL`,
			cartID, candidate.ID)
		if err != nil {
			return models.Data{}, err
		}

		if claimed, err := result.RowsAffected(); err == nil && claimed == 1 {
			return candidate, nil
		}
	}

	return models.Data{}, fmt.Errorf("claim digital data for product %s: too much contention", productID)
}

// scanDigitalFiles loads every digital_file row for a product within the given transaction.
// Extracted from CartLetterPurchase to avoid a defer-in-loop leak when a cart contains
// many "file" products (each iteration used to accumulate an open sql.Rows until the
// enclosing function returned).
func scanDigitalFiles(ctx context.Context, tx *database.Tx, productID string) ([]models.File, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name, ext, orig_name FROM digital_file WHERE product_id = ?`, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var files []models.File
	for rows.Next() {
		f := models.File{}
		if err := rows.Scan(&f.ID, &f.Name, &f.Ext, &f.OrigName); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
