package db

import (
	"context"
	"database/sql"
)

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
	UpsertSessionFunc func(ctx context.Context, arg UpsertSessionParams) error

	// Page operations
	GetPageByIDFunc         func(ctx context.Context, id string) (Page, error)
	GetPageBySlugFunc       func(ctx context.Context, slug string) (Page, error)
	ListPagesFunc           func(ctx context.Context, limit, offset int32) ([]Page, error)
	CountPagesFunc          func(ctx context.Context) (int64, error)
	CreatePageFunc          func(ctx context.Context, params CreatePageParams) (Page, error)
	UpdatePageFunc          func(ctx context.Context, params UpdatePageParams) error
	UpdatePageContentFunc   func(ctx context.Context, id string, content sql.NullString) error
	UpdatePageActiveFunc    func(ctx context.Context, id string) error
	DeletePageFunc          func(ctx context.Context, id string) error
	PageExistsFunc          func(ctx context.Context, slug string) (bool, error)
	GetPageSeoFunc          func(ctx context.Context, id string) ([]byte, error)
	UpdatePageSeoFunc       func(ctx context.Context, seo []byte, id string) error

	// Product operations
	GetProductByIDFunc     func(ctx context.Context, id string) (Product, error)
	GetProductBySlugFunc   func(ctx context.Context, slug string) (Product, error)
	CreateProductFunc      func(ctx context.Context, params CreateProductParams) (Product, error)
	UpdateProductFunc      func(ctx context.Context, params UpdateProductParams) error
	UpdateProductFullFunc  func(ctx context.Context, params UpdateProductFullParams) error
	UpdateProductActiveFunc func(ctx context.Context, id string) error
	ProductHasSoldDigitalDataFunc func(ctx context.Context, productID string) (bool, error)
	SoftDeleteProductFunc  func(ctx context.Context, id string) error
	DeleteProductFunc      func(ctx context.Context, id string) error
	CheckSlugExistsFunc    func(ctx context.Context, slug string, excludeID string) (int64, error)
	ListProductsPrivateFunc func(ctx context.Context, params ListProductsPrivateParams) ([]ProductListRow, error)
	ListProductsPublicFunc  func(ctx context.Context, params ListProductsPublicParams) ([]ProductListRow, error)
	CountProductsFunc       func(ctx context.Context) (int64, error)
	GetProductDetailByIDFunc func(ctx context.Context, id string) (ProductDetail, error)
	GetProductDetailBySlugFunc func(ctx context.Context, slug string) (ProductDetail, error)

	// Product image operations
	GetProductImageFunc    func(ctx context.Context, id string) (ProductImage, error)
	ListProductImagesFunc  func(ctx context.Context, productID string) ([]ProductImage, error)
	CreateProductImageFunc func(ctx context.Context, params CreateProductImageParams) (ProductImage, error)
	DeleteProductImageFunc func(ctx context.Context, id string) error
	DeleteProductImagesFunc func(ctx context.Context, productID string) error

	// Product variant operations
	ListProductVariantsByProductFunc func(ctx context.Context, productID string) ([]ProductVariant, error)
	CreateProductVariantFunc         func(ctx context.Context, params CreateProductVariantParams) (ProductVariant, error)
	UpdateProductVariantFunc         func(ctx context.Context, params UpdateProductVariantParams) error
	DeleteProductVariantFunc         func(ctx context.Context, id string) error

	// Product option operations
	ListProductOptionsByProductFunc     func(ctx context.Context, productID string) ([]ProductOption, error)
	CreateProductOptionFunc             func(ctx context.Context, params CreateProductOptionParams) (ProductOption, error)
	DeleteProductOptionFunc             func(ctx context.Context, id string) error
	CreateProductOptionValueFunc        func(ctx context.Context, params CreateProductOptionValueParams) (ProductOptionValue, error)
	ListProductOptionValuesByOptionFunc func(ctx context.Context, optionID string) ([]ProductOptionValue, error)
	DeleteProductOptionValueFunc        func(ctx context.Context, id string) error

	// Digital file operations
	GetDigitalFileFunc     func(ctx context.Context, id string) (DigitalFile, error)
	ListDigitalFilesFunc   func(ctx context.Context, productID string) ([]DigitalFile, error)
	CreateDigitalFileFunc  func(ctx context.Context, params CreateDigitalFileParams) (DigitalFile, error)
	DeleteDigitalFileFunc  func(ctx context.Context, id string) error
	DeleteDigitalFilesFunc func(ctx context.Context, productID string) error

	// Digital data operations
	GetDigitalDataFunc                    func(ctx context.Context, id string) (DigitalData, error)
	GetDigitalDataByProductFunc           func(ctx context.Context, productID string) (DigitalData, error)
	ListDigitalDataByCartFunc             func(ctx context.Context, cartID string) ([]DigitalData, error)
	ListUnassignedDigitalDataByProductFunc func(ctx context.Context, productID string) ([]DigitalData, error)
	CreateDigitalDataFunc                  func(ctx context.Context, params CreateDigitalDataParams) (DigitalData, error)
	UpdateDigitalDataFunc       func(ctx context.Context, params UpdateDigitalDataParams) error
	DeleteDigitalDataFunc       func(ctx context.Context, id string) error
	DeleteDigitalDataByProductFunc func(ctx context.Context, productID string) error

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
	GetOldCartFunc               func(ctx context.Context, id string) (OldCartRow, error)
	ListOldCartsFunc             func(ctx context.Context, limit, offset int32) ([]OldCartRow, error)
	CountOldCartsFunc            func(ctx context.Context) (int64, error)
	CreateOldCartFunc            func(ctx context.Context, arg CreateOldCartParams) (OldCartRow, error)
	UpdateOldCartFunc            func(ctx context.Context, arg UpdateOldCartParams) error
	UpdateCartPaymentIDFunc      func(ctx context.Context, paymentID sql.NullString, id string) error
	UpdateCartPaymentStatusFunc  func(ctx context.Context, paymentStatus sql.NullString, id string) error
	UpdateCartPaymentFieldsFunc  func(ctx context.Context, paymentID, paymentStatus sql.NullString, id string) error
	GetCartByStatusAndIDFunc     func(ctx context.Context, paymentStatus sql.NullString, id string) (string, string, error) // returns email, cart
	DeleteOldCartFunc            func(ctx context.Context, id string) error
	GetPaymentSettingsFunc       func(ctx context.Context) ([]PaymentSettingRow, error)
)

// Additional function pointers can be added here as needed during migration
// Pattern: declare var, then assign in init.go's initPostgres/initSQLite functions
