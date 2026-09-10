package db

import (
	"database/sql"
	"fmt"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
)

// Unified types that abstract postgres/sqlite differences
// Key difference: postgres uses sql.NullInt32 for integers, sqlite uses sql.NullInt64

// Setting is the unified type for database settings
// Compatible with both postgres.Setting and sqlite.Setting
type Setting struct {
	ID    string
	Key   string
	Value sql.NullString
}

// Session is the unified type for sessions
// Uses Int64 for Expires to accommodate both postgres (Int32) and sqlite (Int64)
type Session struct {
	Key     string
	Value   sql.NullString
	Expires sql.NullInt64 // Unified as Int64 (postgres Int32 converts to this)
}

// CreateSettingParams for CreateSetting operation
type CreateSettingParams struct {
	ID    string
	Key   string
	Value sql.NullString
}

// UpdateSettingParams for UpdateSetting operation
type UpdateSettingParams struct {
	Value sql.NullString
	Key   string
}

// CreateSessionParams for CreateSession operation
type CreateSessionParams struct {
	Key     string
	Value   sql.NullString
	Expires sql.NullInt64 // Unified as Int64
}

// UpdateSessionParams for UpdateSession operation
type UpdateSessionParams struct {
	Value   sql.NullString
	Expires sql.NullInt64 // Unified as Int64
	Key     string
}

// UpsertSessionParams for UpsertSession operation
type UpsertSessionParams struct {
	Key     string
	Value   string
	Expires int64
}

// ToPostgresSetting converts unified Setting to postgres.Setting
func ToPostgresSetting(s Setting) postgres.Setting {
	return postgres.Setting{
		ID:    s.ID,
		Key:   s.Key,
		Value: s.Value,
	}
}

// ToSQLiteSetting converts unified Setting to sqlite.Setting
func ToSQLiteSetting(s Setting) sqlite.Setting {
	return sqlite.Setting{
		ID:    s.ID,
		Key:   s.Key,
		Value: s.Value,
	}
}

// FromPostgresSetting converts postgres.Setting to unified Setting
func FromPostgresSetting(s postgres.Setting) Setting {
	return Setting{
		ID:    s.ID,
		Key:   s.Key,
		Value: s.Value,
	}
}

// FromSQLiteSetting converts sqlite.Setting to unified Setting
func FromSQLiteSetting(s sqlite.Setting) Setting {
	return Setting{
		ID:    s.ID,
		Key:   s.Key,
		Value: s.Value,
	}
}

// FromPostgresSession converts postgres.Session to unified Session
func FromPostgresSession(s postgres.Session) Session {
	return Session{
		Key:   s.Key,
		Value: s.Value,
		Expires: sql.NullInt64{
			Int64: int64(s.Expires.Int32),
			Valid: s.Expires.Valid,
		},
	}
}

// FromSQLiteSession converts sqlite.Session to unified Session
func FromSQLiteSession(s sqlite.Session) Session {
	return Session{
		Key:     s.Key,
		Value:   s.Value,
		Expires: s.Expires,
	}
}

// Page is the unified type for CMS pages
type Page struct {
	ID       string
	Name     string
	Slug     string
	Content  sql.NullString
	Position string
	Active   bool
	Created  sql.NullTime
	Updated  sql.NullTime
}

// CreatePageParams for CreatePage operation
type CreatePageParams struct {
	ID       string
	Name     string
	Slug     string
	Content  sql.NullString
	Position string
	Active   bool
}

// UpdatePageParams for UpdatePage operation
type UpdatePageParams struct {
	Name     string
	Slug     string
	Content  sql.NullString
	Position string
	Active   bool
	ID       string
}

// FromPostgresPageRow converts postgres Row types to unified Page
func FromPostgresPageRow(p interface{}) Page {
	// Handle different row types from postgres queries
	switch v := p.(type) {
	case postgres.GetPageBySlugRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	case postgres.ListPagesRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	case postgres.CreatePageRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	default:
		return Page{}
	}
}

// FromSQLitePageRow converts sqlite Row types to unified Page
func FromSQLitePageRow(p interface{}) Page {
	switch v := p.(type) {
	case sqlite.GetPageBySlugRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	case sqlite.ListPagesRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	case sqlite.CreatePageRow:
		return Page{
			ID:       v.ID,
			Name:     v.Name,
			Slug:     v.Slug,
			Content:  v.Content,
			Position: v.Position,
			Active:   v.Active,
			Created:  v.Created,
			Updated:  v.Updated,
		}
	default:
		return Page{}
	}
}

// Product is the unified type for products
type Product struct {
	ID        string
	Name      string
	Desc      string
	Slug      string
	Amount    string
	Metadata  []byte // json.RawMessage
	Attribute []byte // json.RawMessage
	Digital   sql.NullString
	Active    bool
	Deleted   bool
	Created   sql.NullTime
	Updated   sql.NullTime
}

// CreateProductParams for CreateProduct operation
type CreateProductParams struct {
	ID        string
	Name      string
	Desc      string
	Slug      string
	Amount    string
	Metadata  []byte
	Attribute []byte
	Digital   sql.NullString
	Active    bool
}

// UpdateProductParams for UpdateProduct operation
type UpdateProductParams struct {
	Name      string
	Desc      string
	Slug      string
	Amount    string
	Metadata  []byte
	Attribute []byte
	Digital   sql.NullString
	Active    bool
	ID        string
}

// FromPostgresProductRow converts postgres Row types to unified Product
func FromPostgresProductRow(p interface{}) Product {
	switch v := p.(type) {
	case postgres.GetProductByIDRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    v.Amount,
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	case postgres.GetProductBySlugRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    v.Amount,
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	case postgres.CreateProductRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    v.Amount,
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	default:
		return Product{}
	}
}

// convertAmount converts SQLite's interface{} Amount to string
func convertAmount(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%.0f", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// convertPriceToFloat converts string price to sql.NullFloat64 for SQLite
func convertPriceToFloat(price sql.NullString) sql.NullFloat64 {
	if !price.Valid {
		return sql.NullFloat64{Valid: false}
	}
	var f float64
	if _, err := fmt.Sscanf(price.String, "%f", &f); err == nil {
		return sql.NullFloat64{Float64: f, Valid: true}
	}
	return sql.NullFloat64{Valid: false}
}

// convertFloatToPrice converts sql.NullFloat64 to string price
func convertFloatToPrice(price sql.NullFloat64) sql.NullString {
	if !price.Valid {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: fmt.Sprintf("%.2f", price.Float64), Valid: true}
}

// FromSQLiteProductRow converts sqlite Row types to unified Product
func FromSQLiteProductRow(p interface{}) Product {
	switch v := p.(type) {
	case sqlite.GetProductByIDRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    convertAmount(v.Amount),
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	case sqlite.GetProductBySlugRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    convertAmount(v.Amount),
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	case sqlite.CreateProductRow:
		return Product{
			ID:        v.ID,
			Name:      v.Name,
			Desc:      v.Desc,
			Slug:      v.Slug,
			Amount:    convertAmount(v.Amount),
			Metadata:  v.Metadata,
			Attribute: v.Attribute,
			Digital:   v.Digital,
			Active:    v.Active,
			Deleted:   v.Deleted,
			Created:   v.Created,
			Updated:   v.Updated,
		}
	default:
		return Product{}
	}
}

// User is the unified type for user authentication
type User struct {
	ID        string
	Email     string
	Password  string
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// CreateUserParams for CreateUser operation
type CreateUserParams struct {
	ID        string
	Email     string
	Password  string
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// UpdateUserPasswordParams for UpdateUserPassword operation
type UpdateUserPasswordParams struct {
	Password  string
	UpdatedAt sql.NullTime
	Email     string
}

// FromPostgresUser converts postgres.User to unified User
func FromPostgresUser(u postgres.User) User {
	return User{
		ID:        u.ID,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// FromSQLiteUser converts sqlite.User to unified User
func FromSQLiteUser(u sqlite.User) User {
	return User{
		ID:        u.ID,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// Cart is the unified type for shopping carts
type Cart struct {
	ID        string
	SessionID string
	Status    string
	Total     string
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// CreateCartParams for CreateCart operation
type CreateCartParams struct {
	ID        string
	SessionID string
	Status    string
	Total     string
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// UpdateCartParams for UpdateCart operation
type UpdateCartParams struct {
	Status    string
	Total     string
	UpdatedAt sql.NullTime
	ID        string
}

// FromPostgresNewCart converts postgres.NewCart to unified Cart
func FromPostgresNewCart(c postgres.NewCart) Cart {
	return Cart{
		ID:        c.ID,
		SessionID: c.SessionID,
		Status:    c.Status,
		Total:     c.Total,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// FromSQLiteNewCart converts sqlite.NewCart to unified Cart
func FromSQLiteNewCart(c sqlite.NewCart) Cart {
	return Cart{
		ID:        c.ID,
		SessionID: c.SessionID,
		Status:    c.Status,
		Total:     c.Total,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// CartItem is the unified type for cart line items
type CartItem struct {
	ID        string
	CartID    string
	ProductID string
	Quantity  int64
	Price     string
	CreatedAt sql.NullTime
}

// CreateCartItemParams for CreateCartItem operation
type CreateCartItemParams struct {
	ID        string
	CartID    string
	ProductID string
	Quantity  int64
	Price     string
	CreatedAt sql.NullTime
}

// UpdateCartItemParams for UpdateCartItem operation
type UpdateCartItemParams struct {
	Quantity int64
	Price    string
	ID       string
}

// FromPostgresCartItem converts postgres.CartItem to unified CartItem
func FromPostgresCartItem(ci postgres.CartItem) CartItem {
	return CartItem{
		ID:        ci.ID,
		CartID:    ci.CartID,
		ProductID: ci.ProductID,
		Quantity:  int64(ci.Quantity), // Convert int32 to int64
		Price:     ci.Price,
		CreatedAt: ci.CreatedAt,
	}
}

// FromSQLiteCartItem converts sqlite.CartItem to unified CartItem
func FromSQLiteCartItem(ci sqlite.CartItem) CartItem {
	return CartItem{
		ID:        ci.ID,
		CartID:    ci.CartID,
		ProductID: ci.ProductID,
		Quantity:  ci.Quantity,
		Price:     ci.Price,
		CreatedAt: ci.CreatedAt,
	}
}

// Legacy cart types (old JSON schema - used until full migration to new_carts)

// OldCartRow is the unified type for cart table rows (with JSON cart field)
type OldCartRow struct {
	ID            string
	Email         sql.NullString
	AmountTotal   string
	Currency      string
	PaymentID     sql.NullString
	PaymentStatus sql.NullString
	Cart          string // JSON string
	PaymentSystem string
	Created       sql.NullTime
	Updated       sql.NullTime
}

// CreateOldCartParams for CreateCart operation on old cart table
type CreateOldCartParams struct {
	ID            string
	Email         sql.NullString
	AmountTotal   string
	Currency      string
	PaymentID     sql.NullString
	PaymentStatus sql.NullString
	Cart          []byte // JSON bytes
	PaymentSystem string
}

// UpdateOldCartParams for UpdateCart operation on old cart table
type UpdateOldCartParams struct {
	Email         sql.NullString
	AmountTotal   string
	Currency      string
	PaymentID     sql.NullString
	PaymentStatus sql.NullString
	Cart          []byte // JSON bytes
	PaymentSystem string
	ID            string
}

// PaymentSettingRow represents a payment setting row
type PaymentSettingRow struct {
	Key   string
	Value sql.NullString
}

// FromPostgresGetCartRow converts postgres.GetCartRow to unified OldCartRow
func FromPostgresGetCartRow(c postgres.GetCartRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   c.AmountTotal,
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// FromPostgresListCartsRow converts postgres.ListCartsRow to unified OldCartRow
func FromPostgresListCartsRow(c postgres.ListCartsRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   c.AmountTotal,
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// FromPostgresCreateCartRow converts postgres.CreateCartRow to unified OldCartRow
func FromPostgresCreateCartRow(c postgres.CreateCartRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   c.AmountTotal,
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// FromSQLiteGetCartRow converts sqlite.GetCartRow to unified OldCartRow
func FromSQLiteGetCartRow(c sqlite.GetCartRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   fmt.Sprintf("%v", c.AmountTotal), // Convert interface{} to string
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// FromSQLiteListCartsRow converts sqlite.ListCartsRow to unified OldCartRow
func FromSQLiteListCartsRow(c sqlite.ListCartsRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   fmt.Sprintf("%v", c.AmountTotal), // Convert interface{} to string
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// FromSQLiteCreateCartRow converts sqlite.CreateCartRow to unified OldCartRow
func FromSQLiteCreateCartRow(c sqlite.CreateCartRow) OldCartRow {
	return OldCartRow{
		ID:            c.ID,
		Email:         c.Email,
		AmountTotal:   fmt.Sprintf("%v", c.AmountTotal), // Convert interface{} to string
		Currency:      c.Currency,
		PaymentID:     c.PaymentID,
		PaymentStatus: c.PaymentStatus,
		Cart:          c.Cart,
		PaymentSystem: c.PaymentSystem,
		Created:       c.Created,
		Updated:       c.Updated,
	}
}

// Additional Product types

// UpdateProductFullParams for UpdateProductFull operation
type UpdateProductFullParams struct {
	Name        string
	Brief       string
	Desc        string
	Slug        string
	Amount      string
	Quantity    sql.NullInt64
	Sku         sql.NullString
	HasVariants sql.NullBool
	Metadata    []byte
	Attribute   []byte
	Seo         []byte
	ID          string
}

// ListProductsPrivateParams for listing products (admin view)
type ListProductsPrivateParams struct {
	Limit  int32
	Offset int32
}

// ListProductsPublicParams for listing active products (public view)
type ListProductsPublicParams struct {
	Limit  int32
	Offset int32
}

// ProductListRow represents a product in list view
type ProductListRow struct {
	ID        string
	Name      string
	Brief     string
	Desc      string
	Slug      string
	Amount    string
	Quantity  sql.NullInt64
	Sku       sql.NullString
	Metadata  []byte
	Attribute []byte
	Digital   sql.NullString
	Active    bool
	Deleted   bool
	Created   sql.NullTime
	Updated   sql.NullTime
}

// ProductDetail represents full product details with all fields
type ProductDetail struct {
	ID          string
	Name        string
	Brief       string
	Desc        string
	Slug        string
	Amount      string
	Quantity    sql.NullInt64
	Sku         sql.NullString
	HasVariants bool
	Metadata    []byte
	Attribute   []byte
	Seo         []byte
	Digital     sql.NullString
	Active      bool
	Deleted     bool
	Created     sql.NullTime
	Updated     sql.NullTime
}

// ProductImage types

// ProductImage represents a product image
type ProductImage struct {
	ID        string
	ProductID string
	Name      string
	Ext       string
	OrigName  string
}

// CreateProductImageParams for creating a product image
type CreateProductImageParams struct {
	ID        string
	ProductID string
	Name      string
	Ext       string
	OrigName  string
}

// ProductVariant types

// ProductVariant represents a product variant
type ProductVariant struct {
	ID             string
	ProductID      string
	Sku            sql.NullString
	PriceSurcharge sql.NullString // postgres uses string, sqlite uses float64
	Quantity       sql.NullInt64
	OptionValues   string
	Active         sql.NullBool
	Deleted        sql.NullBool
	Created        sql.NullTime
	Updated        sql.NullTime
}

// CreateProductVariantParams for creating a product variant
type CreateProductVariantParams struct {
	ID             string
	ProductID      string
	Sku            sql.NullString
	PriceSurcharge sql.NullString
	Quantity       sql.NullInt64
	OptionValues   string
}

// UpdateProductVariantParams for updating a product variant
type UpdateProductVariantParams struct {
	Sku            sql.NullString
	PriceSurcharge sql.NullString
	Quantity       sql.NullInt64
	OptionValues   string
	ID             string
}

// DigitalFile types

// DigitalFile represents a digital file for a product
type DigitalFile struct {
	ID        string
	ProductID string
	Name      string
	Ext       string
	OrigName  string
}

// CreateDigitalFileParams for creating a digital file
type CreateDigitalFileParams struct {
	ID        string
	ProductID string
	Name      string
	Ext       string
	OrigName  string
}

// DigitalData types

// DigitalData represents digital content data
type DigitalData struct {
	ID        string
	ProductID string
	Content   string
	CartID    sql.NullString
}

// CreateDigitalDataParams for creating digital data
type CreateDigitalDataParams struct {
	ID        string
	ProductID string
	Content   string
	CartID    sql.NullString
}

// UpdateDigitalDataParams for updating digital data
type UpdateDigitalDataParams struct {
	Content string
	ID      string
}
