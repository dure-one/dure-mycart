package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
)

// Init initializes function pointers based on database type
// Call once at application startup after database connection
func Init(sqlDB *sql.DB, dbType string) error {
	if sqlDB == nil {
		return fmt.Errorf("database connection is nil")
	}

	switch dbType {
	case "postgres", "postgresql":
		initPostgres(sqlDB)
	case "sqlite", "sqlite3":
		initSQLite(sqlDB)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	return nil
}

// initPostgres assigns PostgreSQL sqlc implementations to function pointers
func initPostgres(sqlDB *sql.DB) {
	q := postgres.New(sqlDB)

	// Wrap methods that return postgres-specific types
	GetSettingByKeyFunc = func(ctx context.Context, key string) (Setting, error) {
		pgSetting, err := q.GetSettingByKey(ctx, key)
		if err != nil {
			return Setting{}, err
		}
		return FromPostgresSetting(pgSetting), nil
	}

	DeleteSettingFunc = q.DeleteSetting

	ListSettingsFunc = func(ctx context.Context) ([]Setting, error) {
		pgSettings, err := q.ListSettings(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]Setting, len(pgSettings))
		for i, s := range pgSettings {
			result[i] = FromPostgresSetting(s)
		}
		return result, nil
	}

	GetSessionFunc = func(ctx context.Context, key string) (Session, error) {
		pgSession, err := q.GetSession(ctx, key)
		if err != nil {
			return Session{}, err
		}
		return FromPostgresSession(pgSession), nil
	}

	DeleteSessionFunc = q.DeleteSession

	// Wrap methods that need parameter type conversion
	CreateSettingFunc = func(ctx context.Context, arg CreateSettingParams) (Setting, error) {
		pgParams := postgres.CreateSettingParams{
			ID:    arg.ID,
			Key:   arg.Key,
			Value: arg.Value,
		}
		pgSetting, err := q.CreateSetting(ctx, pgParams)
		if err != nil {
			return Setting{}, err
		}
		return Setting{
			ID:    pgSetting.ID,
			Key:   pgSetting.Key,
			Value: pgSetting.Value,
		}, nil
	}

	UpdateSettingFunc = func(ctx context.Context, arg UpdateSettingParams) error {
		pgParams := postgres.UpdateSettingParams{
			Value: arg.Value,
			Key:   arg.Key,
		}
		return q.UpdateSetting(ctx, pgParams)
	}

	CreateSessionFunc = func(ctx context.Context, arg CreateSessionParams) error {
		pgParams := postgres.CreateSessionParams{
			Key:   arg.Key,
			Value: arg.Value,
			Expires: sql.NullInt32{
				Int32: int32(arg.Expires.Int64),
				Valid: arg.Expires.Valid,
			},
		}
		return q.CreateSession(ctx, pgParams)
	}

	UpdateSessionFunc = func(ctx context.Context, arg UpdateSessionParams) error {
		pgParams := postgres.UpdateSessionParams{
			Value: arg.Value,
			Expires: sql.NullInt32{
				Int32: int32(arg.Expires.Int64),
				Valid: arg.Expires.Valid,
			},
			Key: arg.Key,
		}
		return q.UpdateSession(ctx, pgParams)
	}

	// Page operations
	GetPageBySlugFunc = func(ctx context.Context, slug string) (Page, error) {
		pgPage, err := q.GetPageBySlug(ctx, slug)
		if err != nil {
			return Page{}, err
		}
		return FromPostgresPageRow(pgPage), nil
	}

	ListPagesFunc = func(ctx context.Context, limit, offset int32) ([]Page, error) {
		pgPages, err := q.ListPages(ctx, postgres.ListPagesParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		pages := make([]Page, len(pgPages))
		for i, p := range pgPages {
			pages[i] = FromPostgresPageRow(p)
		}
		return pages, nil
	}

	CreatePageFunc = func(ctx context.Context, params CreatePageParams) (Page, error) {
		pgPage, err := q.CreatePage(ctx, postgres.CreatePageParams{
			ID:       params.ID,
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
		})
		if err != nil {
			return Page{}, err
		}
		return FromPostgresPageRow(pgPage), nil
	}

	UpdatePageFunc = func(ctx context.Context, params UpdatePageParams) error {
		return q.UpdatePage(ctx, postgres.UpdatePageParams{
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
			ID:       params.ID,
		})
	}

	DeletePageFunc = func(ctx context.Context, id string) error {
		return q.DeletePage(ctx, id)
	}

	// Product operations
	GetProductByIDFunc = func(ctx context.Context, id string) (Product, error) {
		pgProduct, err := q.GetProductByID(ctx, id)
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	GetProductBySlugFunc = func(ctx context.Context, slug string) (Product, error) {
		pgProduct, err := q.GetProductBySlug(ctx, slug)
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	CreateProductFunc = func(ctx context.Context, params CreateProductParams) (Product, error) {
		pgProduct, err := q.CreateProduct(ctx, postgres.CreateProductParams{
			ID:        params.ID,
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  params.Metadata,
			Attribute: params.Attribute,
			Digital:   params.Digital,
			Active:    params.Active,
		})
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	UpdateProductFunc = func(ctx context.Context, params UpdateProductParams) error {
		return q.UpdateProduct(ctx, postgres.UpdateProductParams{
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  params.Metadata,
			Attribute: params.Attribute,
			Digital:   params.Digital,
			Active:    params.Active,
			ID:        params.ID,
		})
	}

	DeleteProductFunc = func(ctx context.Context, id string) error {
		return q.DeleteProduct(ctx, id)
	}

	// Auth operations
	GetUserByEmailFunc = func(ctx context.Context, email string) (User, error) {
		pgUser, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			return User{}, err
		}
		return FromPostgresUser(pgUser), nil
	}

	CreateUserFunc = func(ctx context.Context, arg CreateUserParams) error {
		return q.CreateUser(ctx, postgres.CreateUserParams{
			ID:        arg.ID,
			Email:     arg.Email,
			Password:  arg.Password,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	UpdateUserPasswordFunc = func(ctx context.Context, arg UpdateUserPasswordParams) error {
		return q.UpdateUserPassword(ctx, postgres.UpdateUserPasswordParams{
			Password:  arg.Password,
			UpdatedAt: arg.UpdatedAt,
			Email:     arg.Email,
		})
	}
}

// initSQLite assigns SQLite sqlc implementations to function pointers
func initSQLite(sqlDB *sql.DB) {
	q := sqlite.New(sqlDB)

	// Wrap methods that return sqlite-specific types
	GetSettingByKeyFunc = func(ctx context.Context, key string) (Setting, error) {
		sqliteSetting, err := q.GetSettingByKey(ctx, key)
		if err != nil {
			return Setting{}, err
		}
		return FromSQLiteSetting(sqliteSetting), nil
	}

	DeleteSettingFunc = q.DeleteSetting

	ListSettingsFunc = func(ctx context.Context) ([]Setting, error) {
		sqliteSettings, err := q.ListSettings(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]Setting, len(sqliteSettings))
		for i, s := range sqliteSettings {
			result[i] = FromSQLiteSetting(s)
		}
		return result, nil
	}

	GetSessionFunc = func(ctx context.Context, key string) (Session, error) {
		sqliteSession, err := q.GetSession(ctx, key)
		if err != nil {
			return Session{}, err
		}
		return FromSQLiteSession(sqliteSession), nil
	}

	DeleteSessionFunc = q.DeleteSession

	// Wrap methods that need parameter type conversion
	CreateSettingFunc = func(ctx context.Context, arg CreateSettingParams) (Setting, error) {
		sqliteParams := sqlite.CreateSettingParams{
			ID:    arg.ID,
			Key:   arg.Key,
			Value: arg.Value,
		}
		sqliteSetting, err := q.CreateSetting(ctx, sqliteParams)
		if err != nil {
			return Setting{}, err
		}
		return Setting{
			ID:    sqliteSetting.ID,
			Key:   sqliteSetting.Key,
			Value: sqliteSetting.Value,
		}, nil
	}

	UpdateSettingFunc = func(ctx context.Context, arg UpdateSettingParams) error {
		sqliteParams := sqlite.UpdateSettingParams{
			Value: arg.Value,
			Key:   arg.Key,
		}
		return q.UpdateSetting(ctx, sqliteParams)
	}

	CreateSessionFunc = func(ctx context.Context, arg CreateSessionParams) error {
		sqliteParams := sqlite.CreateSessionParams{
			Key:     arg.Key,
			Value:   arg.Value,
			Expires: arg.Expires, // sqlite uses Int64, matches our unified type
		}
		return q.CreateSession(ctx, sqliteParams)
	}

	UpdateSessionFunc = func(ctx context.Context, arg UpdateSessionParams) error {
		sqliteParams := sqlite.UpdateSessionParams{
			Value:   arg.Value,
			Expires: arg.Expires, // sqlite uses Int64, matches our unified type
			Key:     arg.Key,
		}
		return q.UpdateSession(ctx, sqliteParams)
	}

	// Page operations
	GetPageBySlugFunc = func(ctx context.Context, slug string) (Page, error) {
		sqlitePage, err := q.GetPageBySlug(ctx, slug)
		if err != nil {
			return Page{}, err
		}
		return FromSQLitePageRow(sqlitePage), nil
	}

	ListPagesFunc = func(ctx context.Context, limit, offset int32) ([]Page, error) {
		sqlitePages, err := q.ListPages(ctx, sqlite.ListPagesParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}
		pages := make([]Page, len(sqlitePages))
		for i, p := range sqlitePages {
			pages[i] = FromSQLitePageRow(p)
		}
		return pages, nil
	}

	CreatePageFunc = func(ctx context.Context, params CreatePageParams) (Page, error) {
		sqlitePage, err := q.CreatePage(ctx, sqlite.CreatePageParams{
			ID:       params.ID,
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
		})
		if err != nil {
			return Page{}, err
		}
		return FromSQLitePageRow(sqlitePage), nil
	}

	UpdatePageFunc = func(ctx context.Context, params UpdatePageParams) error {
		return q.UpdatePage(ctx, sqlite.UpdatePageParams{
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
			ID:       params.ID,
		})
	}

	DeletePageFunc = func(ctx context.Context, id string) error {
		return q.DeletePage(ctx, id)
	}

	// Product operations
	GetProductByIDFunc = func(ctx context.Context, id string) (Product, error) {
		sqliteProduct, err := q.GetProductByID(ctx, id)
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	GetProductBySlugFunc = func(ctx context.Context, slug string) (Product, error) {
		sqliteProduct, err := q.GetProductBySlug(ctx, slug)
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	CreateProductFunc = func(ctx context.Context, params CreateProductParams) (Product, error) {
		sqliteProduct, err := q.CreateProduct(ctx, sqlite.CreateProductParams{
			ID:        params.ID,
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  params.Metadata,
			Attribute: params.Attribute,
			Digital:   params.Digital,
			Active:    params.Active,
		})
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	UpdateProductFunc = func(ctx context.Context, params UpdateProductParams) error {
		return q.UpdateProduct(ctx, sqlite.UpdateProductParams{
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  params.Metadata,
			Attribute: params.Attribute,
			Digital:   params.Digital,
			Active:    params.Active,
			ID:        params.ID,
		})
	}

	DeleteProductFunc = func(ctx context.Context, id string) error {
		return q.DeleteProduct(ctx, id)
	}

	// Auth operations
	GetUserByEmailFunc = func(ctx context.Context, email string) (User, error) {
		sqliteUser, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			return User{}, err
		}
		return FromSQLiteUser(sqliteUser), nil
	}

	CreateUserFunc = func(ctx context.Context, arg CreateUserParams) error {
		return q.CreateUser(ctx, sqlite.CreateUserParams{
			ID:        arg.ID,
			Email:     arg.Email,
			Password:  arg.Password,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	UpdateUserPasswordFunc = func(ctx context.Context, arg UpdateUserPasswordParams) error {
		return q.UpdateUserPassword(ctx, sqlite.UpdateUserPasswordParams{
			Password:  arg.Password,
			UpdatedAt: arg.UpdatedAt,
			Email:     arg.Email,
		})
	}
}
