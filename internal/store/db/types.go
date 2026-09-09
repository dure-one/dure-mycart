package db

import (
	"database/sql"
	"fmt"
	"time"

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
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// CreateUserParams for CreateUser operation
type CreateUserParams struct {
	ID        string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// UpdateUserPasswordParams for UpdateUserPassword operation
type UpdateUserPasswordParams struct {
	Password  string
	UpdatedAt time.Time
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
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// CreateCartParams for CreateCart operation
type CreateCartParams struct {
	ID        string
	SessionID string
	Status    string
	Total     string
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// FromPostgresCart converts postgres.Cart to unified Cart
func FromPostgresCart(c postgres.Cart) Cart {
	return Cart{
		ID:        c.ID,
		SessionID: c.SessionID,
		Status:    c.Status,
		Total:     c.Total,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// FromSQLiteCart converts sqlite.Cart to unified Cart
func FromSQLiteCart(c sqlite.Cart) Cart {
	return Cart{
		ID:        c.ID,
		SessionID: c.SessionID,
		Status:    c.Status,
		Total:     convertAmount(c.Total),
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
	CreatedAt time.Time
}

// CreateCartItemParams for CreateCartItem operation
type CreateCartItemParams struct {
	ID        string
	CartID    string
	ProductID string
	Quantity  int64
	Price     string
	CreatedAt time.Time
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
		Quantity:  ci.Quantity,
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
		Price:     convertAmount(ci.Price),
		CreatedAt: ci.CreatedAt,
	}
}
