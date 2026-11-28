//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dure-one/dure-mycart/internal/database"
	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/errors"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// SettingQueries holds a dialect-aware handle allowing for easy querying and interaction
// with the database related to application settings.
type SettingQueries struct {
	DB *database.Conn
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

// settingRows fetches key/value rows for keys on either engine.
func (q *SettingQueries) settingRows(ctx context.Context, keys []string) ([]pggen.GetSettingValuesRow, error) {
	return pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.GetSettingValuesRow, error) {
			list, err := json.Marshal(keys)
			if err != nil {
				return nil, err
			}
			return g.GetSettingValues(ctx, list)
		},
		func(g *sqlitegen.Queries) ([]pggen.GetSettingValuesRow, error) {
			r, err := g.GetSettingValues(ctx, keys)
			return convertAll(r, err, func(x sqlitegen.GetSettingValuesRow) pggen.GetSettingValuesRow { return pggen.GetSettingValuesRow(x) })
		})
}

// GetSettingByGroup retrieves settings based on the provided `settings` struct, populating it with values from the database.
func (q *SettingQueries) GetSettingByGroup(ctx context.Context, settings any) (any, error) {
	fieldMap := q.GroupFieldMap(settings)
	if fieldMap == nil {
		return nil, errors.ErrSettingNotFound
	}

	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}

	rows, err := q.settingRows(ctx, keys)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if fieldPtr, ok := fieldMap[row.Key]; ok {
			if err := parseSettingValue(row.Value.String, fieldPtr); err != nil {
				return nil, err
			}
		}
	}

	return settings, nil
}

// UpdateSettingByGroup updates the settings in the database using a transaction.
// Creates new settings if they don't exist, updates existing ones otherwise.
func (q *SettingQueries) UpdateSettingByGroup(ctx context.Context, settings any) error {
	fieldMap := q.GroupFieldMap(settings)

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for key, valuePtr := range fieldMap {
		value, ok, err := serializeSettingValue(valuePtr)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		id := security.RandomString()
		val := sql.NullString{String: value, Valid: true}
		if err := run(q.DB, tx,
			func(g *pggen.Queries) error {
				return g.UpsertSetting(ctx, pggen.UpsertSettingParams{ID: id, Key: key, Value: val})
			},
			func(g *sqlitegen.Queries) error {
				return g.UpsertSetting(ctx, sqlitegen.UpsertSettingParams{ID: id, Key: key, Value: val})
			}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdatePassword updates the current user's password in the database.
func (q *SettingQueries) UpdatePassword(ctx context.Context, password *models.Password) error {
	passwordHash, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (sql.NullString, error) { return g.GetPasswordHash(ctx) },
		func(g *sqlitegen.Queries) (sql.NullString, error) { return g.GetPasswordHash(ctx) })
	if err != nil {
		return errors.ErrUserNotFound
	}
	if !security.ComparePasswords(passwordHash.String, password.Old) {
		return errors.ErrWrongPassword
	}

	hash, err := security.HashPassword(password.New)
	if err != nil {
		return err
	}

	val := sql.NullString{String: hash, Valid: true}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.SetPasswordHash(ctx, val) },
		func(g *sqlitegen.Queries) error { return g.SetPasswordHash(ctx, val) })
}

// GetSettingByKey retrieves settings by key from the database.
// Returns a map of key to SettingName, or an error if no key is given or the query fails.
func (q *SettingQueries) GetSettingByKey(ctx context.Context, key ...string) (map[string]models.SettingName, error) {
	if len(key) == 0 {
		return nil, errors.ErrSettingNotFound
	}

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.Setting, error) {
			list, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			return g.GetSettingsWithID(ctx, list)
		},
		func(g *sqlitegen.Queries) ([]pggen.Setting, error) {
			r, err := g.GetSettingsWithID(ctx, key)
			return convertAll(r, err, func(x sqlitegen.Setting) pggen.Setting { return pggen.Setting(x) })
		})
	if err != nil {
		return nil, err
	}

	settings := map[string]models.SettingName{}
	for _, row := range rows {
		setting := models.SettingName{ID: row.ID}
		if row.Value.Valid {
			setting.Value = row.Value.String
		}
		settings[row.Key] = setting
	}
	return settings, nil
}

// UpdateSettingByKey updates the value of a setting in the database based on the provided key.
//
// A non-string value (a JSON number or bool from a request body) is stored as
// its fmt.Sprint text. The plain build hands it to the driver instead, which on
// SQLite stores a bool as "1"/"0".
func (q *SettingQueries) UpdateSettingByKey(ctx context.Context, setting *models.SettingName) error {
	var val sql.NullString
	switch v := setting.Value.(type) {
	case nil:
	case string:
		val = sql.NullString{String: v, Valid: true}
	default:
		val = sql.NullString{String: fmt.Sprint(v), Valid: true}
	}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.SetSettingValue(ctx, pggen.SetSettingValueParams{Value: val, Key: setting.Key})
		},
		func(g *sqlitegen.Queries) error {
			return g.SetSettingValue(ctx, sqlitegen.SetSettingValueParams{Value: val, Key: setting.Key})
		})
}

// The helpers below are copied unchanged from setting.go, which the sqlc build
// leaves out. TestSQLCPortedFilesUnchanged flags setting.go changing upstream.

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
