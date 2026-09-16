// +build sqlc

package queries_sqlc

import (
	"context"
	"fmt"
	"time"

	"github.com/shurco/mycart/internal/database"
	sqlitegen "github.com/shurco/mycart/internal/queries_sqlc/sqlc/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

type sqliteBackend struct {
	conn    *database.Conn
	queries *sqlitegen.Queries
}

func newSQLiteBackend(conn *database.Conn) backend {
	return &sqliteBackend{
		conn:    conn,
		queries: sqlitegen.New(conn.Raw()),
	}
}

func (s *sqliteBackend) GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	rows, err := s.queries.GetPasswordByEmail(ctx)
	if err != nil {
		return "", err
	}

	var foundEmail, foundPassword string
	for _, row := range rows {
		switch row.Key {
		case "email":
			if row.Value != nil && *row.Value != email {
				return "", errors.ErrUserEmailNotFound
			}
			foundEmail = "matched"
		case "password":
			if row.Value == nil || *row.Value == "" {
				return "", errors.ErrUserPasswordNotFound
			}
			foundPassword = *row.Value
		}
	}

	if foundEmail == "" || foundPassword == "" {
		return "", errors.ErrUserNotFound
	}

	return foundPassword, nil
}

func (s *sqliteBackend) GetSession(ctx context.Context, key string) (string, error) {
	expires := time.Now().Unix()
	session, err := s.queries.GetSession(ctx, sqlitegen.GetSessionParams{
		Key:     key,
		Expires: &expires,
	})
	if err != nil {
		return "", err
	}

	if session.Value == nil {
		return "", nil
	}
	return *session.Value, nil
}

func (s *sqliteBackend) AddSession(ctx context.Context, key, value string, expires int64) error {
	now := time.Now().Unix()

	// Cleanup expired sessions (except current key)
	if err := s.queries.CleanupExpiredSessions(ctx, sqlitegen.CleanupExpiredSessionsParams{
		Expires: &now,
		Key:     key,
	}); err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}

	// Upsert the session
	if err := s.queries.UpsertSession(ctx, sqlitegen.UpsertSessionParams{
		Key:     key,
		Value:   &value,
		Expires: &expires,
	}); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	return nil
}

func (s *sqliteBackend) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	return s.queries.UpdateSession(ctx, sqlitegen.UpdateSessionParams{
		Value:   &value,
		Expires: &expires,
		Key:     key,
	})
}

func (s *sqliteBackend) DeleteSession(ctx context.Context, key string) error {
	return s.queries.DeleteSession(ctx, key)
}

func (s *sqliteBackend) IsInstalled(ctx context.Context) (bool, error) {
	value, err := s.queries.GetSettingValue(ctx, "installed")
	if err != nil {
		return false, err
	}

	if value == nil || *value == "" {
		return false, nil
	}

	// Parse boolean value
	return *value == "true" || *value == "1", nil
}

func (s *sqliteBackend) Install(ctx context.Context, settings map[string]string) error {
	// Begin transaction using database/sql
	tx, err := s.conn.Raw().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create queries with transaction
	txQueries := s.queries.WithTx(tx)

	// Update each setting
	for key, value := range settings {
		if err := txQueries.UpdateSettingValue(ctx, sqlitegen.UpdateSettingValueParams{
			Value: &value,
			Key:   key,
		}); err != nil {
			return fmt.Errorf("update setting %s: %w", key, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *sqliteBackend) PageExists(ctx context.Context, slug string) (bool, error) {
	return s.queries.PageExists(ctx, slug)
}

func (s *sqliteBackend) GetPageBySlug(ctx context.Context, slug string) (*PageRow, error) {
	row, err := s.queries.GetPageBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &PageRow{
		ID:       row.ID,
		Name:     row.Name,
		Slug:     row.Slug,
		Position: row.Position,
		Content:  row.Content,
		Active:   row.Active,
		Created:  timeToUnix(row.Created),
		Updated:  timeToUnixPtr(row.Updated),
	}, nil
}

func (s *sqliteBackend) GetPageByID(ctx context.Context, id string) (*PageRow, error) {
	row, err := s.queries.GetPageByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &PageRow{
		ID:       row.ID,
		Name:     row.Name,
		Slug:     row.Slug,
		Position: row.Position,
		Content:  row.Content,
		Active:   row.Active,
		Created:  timeToUnix(row.Created),
		Updated:  timeToUnixPtr(row.Updated),
	}, nil
}

func (s *sqliteBackend) InsertPage(ctx context.Context, id, name, slug, position string, content *string, active bool) (*PageRow, error) {
	row, err := s.queries.InsertPage(ctx, sqlitegen.InsertPageParams{
		ID:       id,
		Name:     name,
		Slug:     slug,
		Position: position,
		Content:  content,
		Active:   active,
	})
	if err != nil {
		return nil, err
	}
	return &PageRow{
		ID:       row.ID,
		Name:     row.Name,
		Slug:     row.Slug,
		Position: row.Position,
		Content:  row.Content,
		Active:   row.Active,
		Created:  timeToUnix(row.Created),
		Updated:  timeToUnixPtr(row.Updated),
	}, nil
}

func (s *sqliteBackend) UpdatePage(ctx context.Context, name, slug, position string, content *string, active bool, id string) error {
	return s.queries.UpdatePage(ctx, sqlitegen.UpdatePageParams{
		Name:     name,
		Slug:     slug,
		Position: position,
		Content:  content,
		Active:   active,
		ID:       id,
	})
}

func (s *sqliteBackend) DeletePage(ctx context.Context, id string) error {
	return s.queries.DeletePage(ctx, id)
}

func (s *sqliteBackend) UpdatePageActive(ctx context.Context, id string) error {
	return s.queries.UpdatePageActive(ctx, id)
}

// timeToUnix converts *time.Time to int64 Unix timestamp
func timeToUnix(t *time.Time) int64 {
	if t != nil {
		return t.Unix()
	}
	return 0
}

// timeToUnixPtr converts *time.Time to *int64 Unix timestamp
func timeToUnixPtr(t *time.Time) *int64 {
	if t != nil {
		unix := t.Unix()
		return &unix
	}
	return nil
}
