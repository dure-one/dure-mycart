package db

import "context"

// Function pointer variables for database operations
// Initialized once at startup by Init() based on database type (postgres or sqlite)
// Provides zero-overhead database abstraction layer

var (
	// Settings operations
	GetSettingByKeyFunc func(ctx context.Context, key string) (Setting, error)
	CreateSettingFunc   func(ctx context.Context, arg CreateSettingParams) (Setting, error)
	UpdateSettingFunc   func(ctx context.Context, arg UpdateSettingParams) error
	DeleteSettingFunc   func(ctx context.Context, key string) error
	ListSettingsFunc    func(ctx context.Context) ([]Setting, error)

	// Session operations
	GetSessionFunc    func(ctx context.Context, key string) (Session, error)
	CreateSessionFunc func(ctx context.Context, arg CreateSessionParams) error
	UpdateSessionFunc func(ctx context.Context, arg UpdateSessionParams) error
	DeleteSessionFunc func(ctx context.Context, key string) error

	// Page operations
	GetPageBySlugFunc func(ctx context.Context, slug string) (Page, error)
	ListPagesFunc     func(ctx context.Context, limit, offset int32) ([]Page, error)
	CreatePageFunc    func(ctx context.Context, params CreatePageParams) (Page, error)
	UpdatePageFunc    func(ctx context.Context, params UpdatePageParams) error
	DeletePageFunc    func(ctx context.Context, id string) error

	// Product operations
	GetProductByIDFunc   func(ctx context.Context, id string) (Product, error)
	GetProductBySlugFunc func(ctx context.Context, slug string) (Product, error)
	CreateProductFunc    func(ctx context.Context, params CreateProductParams) (Product, error)
	UpdateProductFunc    func(ctx context.Context, params UpdateProductParams) error
	DeleteProductFunc    func(ctx context.Context, id string) error

	// Auth operations
	GetUserByEmailFunc     func(ctx context.Context, email string) (User, error)
	CreateUserFunc         func(ctx context.Context, arg CreateUserParams) error
	UpdateUserPasswordFunc func(ctx context.Context, arg UpdateUserPasswordParams) error

	// Cart operations (new normalized schema)
	CreateNewCartFunc           func(ctx context.Context, arg CreateCartParams) error
	GetNewCartByIDFunc          func(ctx context.Context, id string) (Cart, error)
	GetNewCartBySessionIDFunc   func(ctx context.Context, sessionID string) (Cart, error)
	UpdateNewCartFunc           func(ctx context.Context, arg UpdateCartParams) error
	DeleteNewCartFunc           func(ctx context.Context, id string) error
	CreateCartItemFunc          func(ctx context.Context, arg CreateCartItemParams) error
	GetCartItemFunc             func(ctx context.Context, id string) (CartItem, error)
	ListCartItemsFunc           func(ctx context.Context, cartID string) ([]CartItem, error)
	UpdateCartItemFunc          func(ctx context.Context, arg UpdateCartItemParams) error
	DeleteCartItemFunc          func(ctx context.Context, id string) error
	DeleteCartItemsByCartIDFunc func(ctx context.Context, cartID string) error

	// Legacy cart operations (old JSON schema - to be migrated)
	GetOldCartFunc          func(ctx context.Context, id string) (OldCartRow, error)
	ListOldCartsFunc        func(ctx context.Context, limit, offset int32) ([]OldCartRow, error)
	CountOldCartsFunc       func(ctx context.Context) (int64, error)
	CreateOldCartFunc       func(ctx context.Context, arg CreateOldCartParams) (OldCartRow, error)
	UpdateOldCartFunc       func(ctx context.Context, arg UpdateOldCartParams) error
	DeleteOldCartFunc       func(ctx context.Context, id string) error
	GetPaymentSettingsFunc  func(ctx context.Context) ([]PaymentSettingRow, error)
)

// Additional function pointers can be added here as needed during migration
// Pattern: declare var, then assign in init.go's initPostgres/initSQLite functions
