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
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// DB connection for transaction-based operations
// Set by app initialization
var sqlDB *sql.DB
var dbType string

// InitStore initializes the store package with database connection
// Deprecated: Use InitStoreWithType to support multiple database backends
func InitStore(database *sql.DB) {
	sqlDB = database
	dbType = "sqlite" // default for backward compatibility
}

// InitStoreWithType initializes the store package with database connection and type
func InitStoreWithType(database *sql.DB, databaseType string) {
	sqlDB = database
	dbType = databaseType
}

// GroupFieldMap generates a map of fields based on the type of settings.
func GroupFieldMap(settings any) map[string]any {
	switch s := settings.(type) {
	case *models.Main:
		return map[string]any{
			"site_name": &s.SiteName,
			"domain":    &s.Domain,
		}
	case *models.Auth:
		return map[string]any{
			"email": &s.Email,
		}
	case *models.JWT:
		return map[string]any{
			"jwt_secret":              &s.Secret,
			"jwt_secret_expire_hours": &s.ExpireHours,
		}
	case *models.Social:
		return map[string]any{
			"social_facebook":  &s.Facebook,
			"social_instagram": &s.Instagram,
			"social_twitter":   &s.Twitter,
			"social_dribbble":  &s.Dribbble,
			"social_github":    &s.Github,
			"social_youtube":   &s.Youtube,
			"social_other":     &s.Other,
		}
	case *models.Payment:
		return map[string]any{
			"currency":       &s.Currency,
			"truncation":     &s.Truncation,
			"number_format":  &s.NumberFormat,
			"symbol_display": &s.SymbolDisplay,
		}
	case *models.Stripe:
		return map[string]any{
			"stripe_secret_key": &s.SecretKey,
			"stripe_active":     &s.Active,
		}
	case *models.Paypal:
		return map[string]any{
			"paypal_client_id":  &s.ClientID,
			"paypal_secret_key": &s.SecretKey,
			"paypal_active":     &s.Active,
		}
	case *models.Spectrocoin:
		return map[string]any{
			"spectrocoin_merchant_id": &s.MerchantID,
			"spectrocoin_project_id":  &s.ProjectID,
			"spectrocoin_private_key": &s.PrivateKey,
			"spectrocoin_active":      &s.Active,
		}
	case *models.Coinbase:
		return map[string]any{
			"coinbase_api_key": &s.ApiKey,
			"coinbase_active":  &s.Active,
		}
	case *models.Portone:
		return map[string]any{
			"portone_store_id":    &s.StoreID,
			"portone_channel_key": &s.ChannelKey,
			"portone_api_secret":  &s.ApiSecret,
			"portone_active":      &s.Active,
		}
	case *models.Dummy:
		return map[string]any{
			"dummy_active": &s.Active,
		}
	case *models.Webhook:
		return map[string]any{
			"webhook_url": &s.Url,
		}
	case *models.Mail:
		return map[string]any{
			"mail_sender_name":  &s.SenderName,
			"mail_sender_email": &s.SenderEmail,
			"smtp_host":         &s.SMTP.Host,
			"smtp_port":         &s.SMTP.Port,
			"smtp_username":     &s.SMTP.Username,
			"smtp_password":     &s.SMTP.Password,
			"smtp_encryption":   &s.SMTP.Encryption,
		}
	default:
		return nil
	}
}

// GetSettingByGroup retrieves settings based on the provided `settings` struct, populating it with values from the database.
func GetSettingByGroup(ctx context.Context, settings any) (any, error) {
	fieldMap := GroupFieldMap(settings)
	if fieldMap == nil {
		return nil, errors.ErrSettingNotFound
	}

	keys := make([]any, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}

	// Build placeholders based on database type
	placeholders := buildPlaceholders(len(keys))
	query := fmt.Sprintf("SELECT key, value FROM setting WHERE key IN (%s)", placeholders)
	rows, err := sqlDB.QueryContext(ctx, query, keys...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		if fieldPtr, ok := fieldMap[key]; ok {
			if err := parseSettingValue(value, fieldPtr); err != nil {
				return nil, err
			}
		}
	}

	return settings, nil
}

// buildPlaceholders generates database-specific parameter placeholders
func buildPlaceholders(count int) string {
	if count == 0 {
		return ""
	}

	// PostgreSQL uses $1, $2, $3... while SQLite uses ?, ?, ?
	if dbType == "postgres" || dbType == "postgresql" {
		placeholders := make([]string, count)
		for i := 0; i < count; i++ {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		return strings.Join(placeholders, ", ")
	}

	// SQLite and default
	return strings.Repeat("?, ", count-1) + "?"
}

// GetSettingByGroupTyped is a generic function that retrieves a setting from the database.
func GetSettingByGroupTyped[T any](ctx context.Context) (*T, error) {
	setting, err := GetSettingByGroup(ctx, new(T))
	if err != nil {
		return nil, err
	}
	return setting.(*T), nil
}

// UpdateSettingByGroup updates the settings in the database using a transaction.
// It takes a context and a settings object of any type as arguments.
// Creates new settings if they don't exist, updates existing ones otherwise.
func UpdateSettingByGroup(ctx context.Context, settings any) error {
	fieldMap := GroupFieldMap(settings)

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Build upsert query with database-specific placeholders
	upsertQuery := buildUpsertQuery()
	upsertStmt, err := tx.PrepareContext(ctx, upsertQuery)
	if err != nil {
		return err
	}
	defer func() { _ = upsertStmt.Close() }()

	for key, valuePtr := range fieldMap {
		value, ok, err := serializeSettingValue(valuePtr)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		newID := security.RandomString()
		if _, err = upsertStmt.ExecContext(ctx, newID, key, value); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// buildUpsertQuery generates database-specific UPSERT statement
func buildUpsertQuery() string {
	if dbType == "postgres" || dbType == "postgresql" {
		return `
			INSERT INTO setting (id, key, value) VALUES ($1, $2, $3)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`
	}
	// SQLite and default
	return `
		INSERT INTO setting (id, key, value) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
}

// UpdatePassword updates the current user's password in the database.
func UpdatePassword(ctx context.Context, password *models.Password) error {
	var passwordHash string
	query := `SELECT value FROM setting WHERE key = 'password'`
	if err := sqlDB.QueryRowContext(ctx, query).Scan(&passwordHash); err != nil {
		return errors.ErrUserNotFound
	}
	compareUserPassword := security.ComparePasswords(passwordHash, password.Old)
	if !compareUserPassword {
		return errors.ErrWrongPassword
	}

	// Use database-specific placeholder
	placeholder := "?"
	if dbType == "postgres" || dbType == "postgresql" {
		placeholder = "$1"
	}
	query = fmt.Sprintf("UPDATE setting SET value = %s WHERE key = 'password'", placeholder)
	_, err := sqlDB.ExecContext(ctx, query, security.GeneratePassword(password.New))
	return err
}

// GetSettingByKey retrieves settings by key using the function pointer.
// Returns a map of settings or an error if not found.
func GetSettingByKey(ctx context.Context, key ...string) (map[string]models.SettingName, error) {
	if len(key) == 0 {
		return nil, errors.ErrSettingNotFound
	}

	settings := map[string]models.SettingName{}

	for _, k := range key {
		setting, err := db.GetSettingByKeyFunc(ctx, k)
		if err != nil {
			if err == sql.ErrNoRows {
				continue // Key not found, skip
			}
			return nil, err
		}

		// Convert to models.SettingName
		value := ""
		if setting.Value.Valid {
			value = setting.Value.String
		}
		settings[k] = models.SettingName{
			ID:    setting.ID,
			Key:   k,
			Value: value,
		}
	}

	return settings, nil
}

// UpdateSettingByKey updates the value of a setting using the function pointer.
func UpdateSettingByKey(ctx context.Context, setting *models.SettingName) error {
	// Convert setting.Value to string
	valueStr := fmt.Sprint(setting.Value)
	value := sql.NullString{String: valueStr, Valid: valueStr != ""}

	params := db.UpdateSettingParams{
		Value: value,
		Key:   setting.Key,
	}
	return db.UpdateSettingFunc(ctx, params)
}

// parseSettingValue parses a string value into the appropriate type
func parseSettingValue(value string, fieldPtr any) error {
	switch ptr := fieldPtr.(type) {
	case *string:
		*ptr = value
	case *bool:
		bValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		*ptr = bValue
	case *int:
		if value == "" {
			*ptr = 0
		} else {
			iValue, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			*ptr = iValue
		}
	case **models.TruncationSettings:
		return unmarshalJSONToPointer(value, ptr)
	case **models.NumberFormatSettings:
		return unmarshalJSONToPointer(value, ptr)
	case **models.SymbolDisplaySettings:
		return unmarshalJSONToPointer(value, ptr)
	}
	return nil
}

// serializeSettingValue converts a field pointer to its string representation
func serializeSettingValue(valuePtr any) (string, bool, error) {
	switch v := valuePtr.(type) {
	case *string:
		return *v, true, nil
	case *bool:
		return strconv.FormatBool(*v), true, nil
	case *int:
		return strconv.Itoa(*v), true, nil
	case **models.TruncationSettings:
		value, err := marshalJSONFromPointer(*v)
		return value, true, err
	case **models.NumberFormatSettings:
		value, err := marshalJSONFromPointer(*v)
		return value, true, err
	case **models.SymbolDisplaySettings:
		value, err := marshalJSONFromPointer(*v)
		return value, true, err
	default:
		return "", false, nil
	}
}

// unmarshalJSONToPointer unmarshals JSON value into a pointer of type T
func unmarshalJSONToPointer[T any](value string, ptr **T) error {
	if value == "" {
		return nil
	}
	var t T
	if err := json.Unmarshal([]byte(value), &t); err != nil {
		return err
	}
	*ptr = &t
	return nil
}

// marshalJSONFromPointer marshals a pointer of type T to JSON string
func marshalJSONFromPointer[T any](ptr *T) (string, error) {
	if ptr == nil {
		return "", nil
	}
	jsonBytes, err := json.Marshal(ptr)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
