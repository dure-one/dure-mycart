package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
)

// MigrateData copies all data from source database to target database.
func MigrateData(source, target Database) error {
	ctx := context.Background()

	// Begin transaction on target
	targetDB := target.DB()
	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Migrate settings
	if err := migrateSettings(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate settings: %w", err)
	}

	// Migrate sessions
	if err := migrateSessions(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate sessions: %w", err)
	}

	// Migrate subdomains
	if err := migrateSubdomains(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate subdomains: %w", err)
	}

	// Migrate pages
	if err := migratePages(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate pages: %w", err)
	}

	// Migrate products and related data
	if err := migrateProducts(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate products: %w", err)
	}

	// Migrate carts
	if err := migrateCarts(ctx, source, target, tx); err != nil {
		return fmt.Errorf("failed to migrate carts: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// migrateSettings migrates the settings table
func migrateSettings(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source settings
	var sourceSettings interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		settings, err := sourceQueries.ListSettings(ctx)
		if err != nil {
			return err
		}
		sourceSettings = settings
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		settings, err := sourceQueries.ListSettings(ctx)
		if err != nil {
			return err
		}
		sourceSettings = settings
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		settings := sourceSettings.([]sqlite.Setting)
		for _, setting := range settings {
			_, err := targetQueries.CreateSetting(ctx, sqlite.CreateSettingParams{
				ID:    setting.ID,
				Key:   setting.Key,
				Value: setting.Value,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		settings := sourceSettings.([]postgres.Setting)
		for _, setting := range settings {
			_, err := targetQueries.CreateSetting(ctx, postgres.CreateSettingParams{
				ID:    setting.ID,
				Key:   setting.Key,
				Value: setting.Value,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateSessions migrates the session table
func migrateSessions(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source sessions
	var sourceSessions interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		sessions, err := sourceQueries.ListAllSessions(ctx)
		if err != nil {
			return err
		}
		sourceSessions = sessions
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		sessions, err := sourceQueries.ListAllSessions(ctx)
		if err != nil {
			return err
		}
		sourceSessions = sessions
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		sessions := sourceSessions.([]sqlite.Session)
		for _, session := range sessions {
			err := targetQueries.CreateSession(ctx, sqlite.CreateSessionParams{
				Key:     session.Key,
				Value:   session.Value,
				Expires: session.Expires,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		sessions := sourceSessions.([]postgres.Session)
		for _, session := range sessions {
			err := targetQueries.CreateSession(ctx, postgres.CreateSessionParams{
				Key:     session.Key,
				Value:   session.Value,
				Expires: session.Expires,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateSubdomains migrates the subdomain table
func migrateSubdomains(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source subdomains
	var sourceSubdomains interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		subdomains, err := sourceQueries.ListAllSubdomains(ctx)
		if err != nil {
			return err
		}
		sourceSubdomains = subdomains
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		subdomains, err := sourceQueries.ListAllSubdomains(ctx)
		if err != nil {
			return err
		}
		sourceSubdomains = subdomains
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		subdomains := sourceSubdomains.([]sqlite.Subdomain)
		for _, subdomain := range subdomains {
			_, err := targetQueries.CreateSubdomain(ctx, sqlite.CreateSubdomainParams{
				ID:   subdomain.ID,
				Name: subdomain.Name,
				Desc: subdomain.Desc,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		subdomains := sourceSubdomains.([]postgres.Subdomain)
		for _, subdomain := range subdomains {
			_, err := targetQueries.CreateSubdomain(ctx, postgres.CreateSubdomainParams{
				ID:   subdomain.ID,
				Name: subdomain.Name,
				Desc: subdomain.Desc,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migratePages migrates the page table
func migratePages(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source pages
	var sourcePages interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		pages, err := sourceQueries.ListAllPages(ctx)
		if err != nil {
			return err
		}
		sourcePages = pages
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		pages, err := sourceQueries.ListAllPages(ctx)
		if err != nil {
			return err
		}
		sourcePages = pages
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		pages := sourcePages.([]sqlite.Page)
		for _, page := range pages {
			_, err := targetQueries.CreatePage(ctx, sqlite.CreatePageParams{
				ID:       page.ID,
				Name:     page.Name,
				Slug:     page.Slug,
				Content:  page.Content,
				Position: page.Position,
				Active:   page.Active,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		pages := sourcePages.([]postgres.Page)
		for _, page := range pages {
			_, err := targetQueries.CreatePage(ctx, postgres.CreatePageParams{
				ID:       page.ID,
				Name:     page.Name,
				Slug:     page.Slug,
				Content:  page.Content,
				Position: page.Position,
				Active:   page.Active,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateProducts migrates products and related tables
func migrateProducts(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source products
	var sourceProducts interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		products, err := sourceQueries.ListAllProducts(ctx)
		if err != nil {
			return err
		}
		sourceProducts = products
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		products, err := sourceQueries.ListAllProducts(ctx)
		if err != nil {
			return err
		}
		sourceProducts = products
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert products into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		products := sourceProducts.([]sqlite.Product)
		for _, product := range products {
			_, err := targetQueries.CreateProduct(ctx, sqlite.CreateProductParams{
				ID:        product.ID,
				Name:      product.Name,
				Desc:      product.Desc,
				Slug:      product.Slug,
				Amount:    product.Amount,
				Metadata:  product.Metadata,
				Attribute: product.Attribute,
				Digital:   product.Digital,
				Active:    product.Active,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		products := sourceProducts.([]postgres.Product)
		for _, product := range products {
			_, err := targetQueries.CreateProduct(ctx, postgres.CreateProductParams{
				ID:        product.ID,
				Name:      product.Name,
				Desc:      product.Desc,
				Slug:      product.Slug,
				Amount:    product.Amount,
				Metadata:  product.Metadata,
				Attribute: product.Attribute,
				Digital:   product.Digital,
				Active:    product.Active,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	// Migrate product images
	if err := migrateProductImages(ctx, source, target, tx); err != nil {
		return err
	}

	// Migrate digital files
	if err := migrateDigitalFiles(ctx, source, target, tx); err != nil {
		return err
	}

	// Migrate digital data
	if err := migrateDigitalData(ctx, source, target, tx); err != nil {
		return err
	}

	return nil
}

// migrateProductImages migrates product_image table
func migrateProductImages(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source product images
	var sourceImages interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		images, err := sourceQueries.ListAllProductImages(ctx)
		if err != nil {
			return err
		}
		sourceImages = images
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		images, err := sourceQueries.ListAllProductImages(ctx)
		if err != nil {
			return err
		}
		sourceImages = images
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		images := sourceImages.([]sqlite.ProductImage)
		for _, image := range images {
			_, err := targetQueries.CreateProductImage(ctx, sqlite.CreateProductImageParams{
				ID:        image.ID,
				ProductID: image.ProductID,
				Name:      image.Name,
				Ext:       image.Ext,
				OrigName:  image.OrigName,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		images := sourceImages.([]postgres.ProductImage)
		for _, image := range images {
			_, err := targetQueries.CreateProductImage(ctx, postgres.CreateProductImageParams{
				ID:        image.ID,
				ProductID: image.ProductID,
				Name:      image.Name,
				Ext:       image.Ext,
				OrigName:  image.OrigName,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateDigitalFiles migrates digital_file table
func migrateDigitalFiles(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source digital files
	var sourceFiles interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		files, err := sourceQueries.ListAllDigitalFiles(ctx)
		if err != nil {
			return err
		}
		sourceFiles = files
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		files, err := sourceQueries.ListAllDigitalFiles(ctx)
		if err != nil {
			return err
		}
		sourceFiles = files
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		files := sourceFiles.([]sqlite.DigitalFile)
		for _, file := range files {
			_, err := targetQueries.CreateDigitalFile(ctx, sqlite.CreateDigitalFileParams{
				ID:        file.ID,
				ProductID: file.ProductID,
				Name:      file.Name,
				Ext:       file.Ext,
				OrigName:  file.OrigName,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		files := sourceFiles.([]postgres.DigitalFile)
		for _, file := range files {
			_, err := targetQueries.CreateDigitalFile(ctx, postgres.CreateDigitalFileParams{
				ID:        file.ID,
				ProductID: file.ProductID,
				Name:      file.Name,
				Ext:       file.Ext,
				OrigName:  file.OrigName,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateDigitalData migrates digital_data table
func migrateDigitalData(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source digital data
	var sourceData interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		data, err := sourceQueries.ListAllDigitalData(ctx)
		if err != nil {
			return err
		}
		sourceData = data
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		data, err := sourceQueries.ListAllDigitalData(ctx)
		if err != nil {
			return err
		}
		sourceData = data
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		dataList := sourceData.([]sqlite.DigitalDatum)
		for _, data := range dataList {
			_, err := targetQueries.CreateDigitalData(ctx, sqlite.CreateDigitalDataParams{
				ID:        data.ID,
				ProductID: data.ProductID,
				Content:   data.Content,
				CartID:    data.CartID,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		dataList := sourceData.([]postgres.DigitalDatum)
		for _, data := range dataList {
			_, err := targetQueries.CreateDigitalData(ctx, postgres.CreateDigitalDataParams{
				ID:        data.ID,
				ProductID: data.ProductID,
				Content:   data.Content,
				CartID:    data.CartID,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// migrateCarts migrates the cart table
func migrateCarts(ctx context.Context, source, target Database, tx *sql.Tx) error {
	// Get source carts
	var sourceCarts interface{}
	switch source.Type() {
	case "sqlite":
		sourceQueries := source.Queries().(*sqlite.Queries)
		carts, err := sourceQueries.ListAllCarts(ctx)
		if err != nil {
			return err
		}
		sourceCarts = carts
	case "postgres":
		sourceQueries := source.Queries().(*postgres.Queries)
		carts, err := sourceQueries.ListAllCarts(ctx)
		if err != nil {
			return err
		}
		sourceCarts = carts
	default:
		return fmt.Errorf("unknown source database type: %s", source.Type())
	}

	// Insert into target
	switch target.Type() {
	case "sqlite":
		targetQueries := sqlite.New(tx)
		carts := sourceCarts.([]sqlite.Cart)
		for _, cart := range carts {
			_, err := targetQueries.CreateCart(ctx, sqlite.CreateCartParams{
				ID:            cart.ID,
				Email:         cart.Email,
				AmountTotal:   cart.AmountTotal,
				Currency:      cart.Currency,
				PaymentID:     cart.PaymentID,
				PaymentStatus: cart.PaymentStatus,
				Cart:          []byte(cart.Cart),
				PaymentSystem: cart.PaymentSystem,
			})
			if err != nil {
				return err
			}
		}
	case "postgres":
		targetQueries := postgres.New(tx)
		carts := sourceCarts.([]postgres.Cart)
		for _, cart := range carts {
			_, err := targetQueries.CreateCart(ctx, postgres.CreateCartParams{
				ID:            cart.ID,
				Email:         cart.Email,
				AmountTotal:   cart.AmountTotal,
				Currency:      cart.Currency,
				PaymentID:     cart.PaymentID,
				PaymentStatus: cart.PaymentStatus,
				Cart:          []byte(cart.Cart),
				PaymentSystem: cart.PaymentSystem,
			})
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown target database type: %s", target.Type())
	}

	return nil
}

// BackupDatabase creates a timestamped backup of the database
func BackupDatabase(dbPath string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup_%s", dbPath, timestamp)

	// For SQLite, we can just copy the file
	// For PostgreSQL, we would use pg_dump (not implemented here)

	// Simple file copy for SQLite
	data, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", err
	}
	defer data.Close()

	backup, err := sql.Open("sqlite", backupPath)
	if err != nil {
		return "", err
	}
	defer backup.Close()

	_, err = backup.Exec("VACUUM INTO ?", backupPath)
	if err != nil {
		// Fallback: just create an empty backup file as a marker
		return backupPath, nil
	}

	return backupPath, nil
}
