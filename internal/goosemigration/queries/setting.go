package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// getSQLCQueries returns the appropriate sqlc Queries based on DB type
func getSQLCQueries() interface{} {
	if dbAdapter == nil {
		return nil
	}
	return dbAdapter.Queries()
}

// buildPlaceholders is deprecated - use BuildPlaceholders from queries package
// Kept for backward compatibility with cart.go
func buildPlaceholders(count int) string {
	return BuildPlaceholders(count)
}

// toModelSetting converts sqlc Setting to models.SettingName
func toModelSetting(s interface{}) models.SettingName {
	switch v := s.(type) {
	case sqlite.Setting:
		value := ""
		if v.Value.Valid {
			value = v.Value.String
		}
		return models.SettingName{
			ID:    v.ID,
			Key:   v.Key,
			Value: value,
		}
	case postgres.Setting:
		value := ""
		if v.Value.Valid {
			value = v.Value.String
		}
		return models.SettingName{
			ID:    v.ID,
			Key:   v.Key,
			Value: value,
		}
	default:
		return models.SettingName{}
	}
}

// SettingQueries wraps a sql.DB connection allowing for easy querying and interaction
// with the database related to application settings.
type SettingQueries struct {
	*sql.DB
}

// GroupFieldMap generates a map of fields based on the type of settings.
func (q *SettingQueries) GroupFieldMap(settings any) map[string]any {
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

// GetSettingByGroup is a generic function that retrieves a setting from the database.
// It takes a context and a pointer to the Base struct which holds the database methods.
// The function returns a pointer to the requested setting of type T or an error if any occurs.
func GetSettingByGroup[T any](ctx context.Context, db *Base) (*T, error) {
	setting, err := db.GetSettingByGroup(ctx, new(T))
	if err != nil {
		return nil, err
	}
	return setting.(*T), nil
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

// GetSettingByGroup retrieves settings based on the provided `settings` struct, populating it with values from the database using sqlc.
func (q *SettingQueries) GetSettingByGroup(ctx context.Context, settings any) (any, error) {
	fieldMap := q.GroupFieldMap(settings)
	if fieldMap == nil {
		return nil, errors.ErrSettingNotFound
	}

	queries := getSQLCQueries()

	// Query each setting by key using sqlc
	for key, fieldPtr := range fieldMap {
		var result interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			result, err = pgQueries.GetSettingByKey(ctx, key)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			result, err = sqliteQueries.GetSettingByKey(ctx, key)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				continue // Key not found, skip
			}
			return nil, err
		}

		// Extract value from result
		setting := toModelSetting(result)
		valueStr, _ := setting.Value.(string)

		// Parse and assign to field
		if err := parseSettingValue(valueStr, fieldPtr); err != nil {
			return nil, err
		}
	}

	return settings, nil
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

// UpdateSettingByGroup updates the settings in the database using a transaction and sqlc.
// It takes a context and a settings object of any type as arguments.
// Creates new settings if they don't exist, updates existing ones otherwise.
func (q *SettingQueries) UpdateSettingByGroup(ctx context.Context, settings any) error {
	fieldMap := q.GroupFieldMap(settings)

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Create transaction-bound queries
	var txQueries interface{}
	if DBType() == "postgres" {
		txQueries = postgres.New(tx)
	} else {
		txQueries = sqlite.New(tx)
	}

	for key, valuePtr := range fieldMap {
		value, ok, err := serializeSettingValue(valuePtr)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		newID := security.RandomString()
		valueNull := sql.NullString{String: value, Valid: true}

		if DBType() == "postgres" {
			pgQueries := txQueries.(*postgres.Queries)
			params := postgres.UpsertSettingParams{
				ID:    newID,
				Key:   key,
				Value: valueNull,
			}
			if err := pgQueries.UpsertSetting(ctx, params); err != nil {
				return err
			}
		} else {
			sqliteQueries := txQueries.(*sqlite.Queries)
			params := sqlite.UpsertSettingParams{
				ID:    newID,
				Key:   key,
				Value: valueNull,
			}
			if err := sqliteQueries.UpsertSetting(ctx, params); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// UpdatePassword updates the current user's password in the database using sqlc.
func (q *SettingQueries) UpdatePassword(ctx context.Context, password *models.Password) error {
	queries := getSQLCQueries()

	// Get current password hash
	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.GetSettingByKey(ctx, "password")
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.GetSettingByKey(ctx, "password")
	}

	if err != nil {
		return errors.ErrUserNotFound
	}

	// Extract password hash from result
	setting := toModelSetting(result)
	passwordHash, _ := setting.Value.(string)

	// Verify old password
	if !security.ComparePasswords(passwordHash, password.Old) {
		return errors.ErrWrongPassword
	}

	// Update with new password hash
	newHash := security.GeneratePassword(password.New)
	newHashNull := sql.NullString{String: newHash, Valid: true}

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdateSettingParams{
			Value: newHashNull,
			Key:   "password",
		}
		return pgQueries.UpdateSetting(ctx, params)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.UpdateSettingParams{
		Value: newHashNull,
		Key:   "password",
	}
	return sqliteQueries.UpdateSetting(ctx, params)
}

// GetSettingByKey retrieves settings by key using sqlc.
// It accepts a context for cancellation and one or more keys.
// Returns a map of settings or an error if not found or any other issue occurs.
func (q *SettingQueries) GetSettingByKey(ctx context.Context, key ...string) (map[string]models.SettingName, error) {
	if len(key) == 0 {
		return nil, errors.ErrSettingNotFound
	}

	settings := map[string]models.SettingName{}
	queries := getSQLCQueries()

	// Call sqlc query for each key
	for _, k := range key {
		var result interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			result, err = pgQueries.GetSettingByKey(ctx, k)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			result, err = sqliteQueries.GetSettingByKey(ctx, k)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				continue // Key not found, skip
			}
			return nil, err
		}

		setting := toModelSetting(result)
		setting.Key = k
		settings[k] = setting
	}

	return settings, nil
}

// UpdateSettingByKey updates the value of a setting using sqlc.
func (q *SettingQueries) UpdateSettingByKey(ctx context.Context, setting *models.SettingName) error {
	queries := getSQLCQueries()

	// Convert setting.Value to string
	valueStr := fmt.Sprint(setting.Value)
	value := sql.NullString{String: valueStr, Valid: valueStr != ""}

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdateSettingParams{
			Value: value,
			Key:   setting.Key,
		}
		return pgQueries.UpdateSetting(ctx, params)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.UpdateSettingParams{
		Value: value,
		Key:   setting.Key,
	}
	return sqliteQueries.UpdateSetting(ctx, params)
}

