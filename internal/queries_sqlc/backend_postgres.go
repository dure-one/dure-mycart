// +build sqlc

package queries_sqlc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurco/mycart/internal/database"
	pggen "github.com/shurco/mycart/internal/queries_sqlc/sqlc/postgres"
	"github.com/shurco/mycart/pkg/errors"
)

type postgresBackend struct {
	conn    *database.Conn
	pgxPool *pgxpool.Pool
	queries *pggen.Queries
}

func newPostgresBackend(conn *database.Conn) backend {
	// Get active database configuration
	cfg := database.Active()

	// Create native pgx pool
	pgxCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		panic(fmt.Sprintf("parse pgx config: %v", err))
	}

	pgxPool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		panic(fmt.Sprintf("create pgx pool: %v", err))
	}

	return &postgresBackend{
		conn:    conn,
		pgxPool: pgxPool,
		queries: pggen.New(pgxPool),
	}
}

func (p *postgresBackend) GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	rows, err := p.queries.GetPasswordByEmail(ctx)
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

func (p *postgresBackend) GetSession(ctx context.Context, key string) (string, error) {
	expires := int32(getCurrentTimestamp())
	session, err := p.queries.GetSession(ctx, pggen.GetSessionParams{
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

func (p *postgresBackend) AddSession(ctx context.Context, key, value string, expires int64) error {
	now := int32(getCurrentTimestamp())
	expiresInt := int32(expires)

	// Cleanup expired sessions (except current key)
	if err := p.queries.CleanupExpiredSessions(ctx, pggen.CleanupExpiredSessionsParams{
		Expires: &now,
		Key:     key,
	}); err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}

	// Upsert the session
	if err := p.queries.UpsertSession(ctx, pggen.UpsertSessionParams{
		Key:     key,
		Value:   &value,
		Expires: &expiresInt,
	}); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	return nil
}

func (p *postgresBackend) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	expiresInt := int32(expires)
	return p.queries.UpdateSession(ctx, pggen.UpdateSessionParams{
		Value:   &value,
		Expires: &expiresInt,
		Key:     key,
	})
}

func (p *postgresBackend) DeleteSession(ctx context.Context, key string) error {
	return p.queries.DeleteSession(ctx, key)
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

func (p *postgresBackend) IsInstalled(ctx context.Context) (bool, error) {
	value, err := p.queries.GetSettingValue(ctx, "installed")
	if err != nil {
		return false, err
	}

	if value == nil || *value == "" {
		return false, nil
	}

	// Parse boolean value
	return *value == "true" || *value == "1", nil
}

func (p *postgresBackend) Install(ctx context.Context, settings map[string]string) error {
	// Begin transaction
	tx, err := p.pgxPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create queries with transaction
	txQueries := p.queries.WithTx(tx)

	// Update each setting
	for key, value := range settings {
		if err := txQueries.UpdateSettingValue(ctx, pggen.UpdateSettingValueParams{
			Value: &value,
			Key:   key,
		}); err != nil {
			return fmt.Errorf("update setting %s: %w", key, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (p *postgresBackend) PageExists(ctx context.Context, slug string) (bool, error) {
	return p.queries.PageExists(ctx, slug)
}

func (p *postgresBackend) GetPageBySlug(ctx context.Context, slug string) (*PageRow, error) {
	row, err := p.queries.GetPageBySlug(ctx, slug)
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
		Created:  pgTimestampToUnix(row.Created),
		Updated:  pgTimestampToUnixPtr(row.Updated),
	}, nil
}

func (p *postgresBackend) GetPageByID(ctx context.Context, id string) (*PageRow, error) {
	row, err := p.queries.GetPageByID(ctx, id)
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
		Created:  pgTimestampToUnix(row.Created),
		Updated:  pgTimestampToUnixPtr(row.Updated),
	}, nil
}

func (p *postgresBackend) InsertPage(ctx context.Context, id, name, slug, position string, content *string, active bool) (*PageRow, error) {
	row, err := p.queries.InsertPage(ctx, pggen.InsertPageParams{
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
		Created:  pgTimestampToUnix(row.Created),
		Updated:  pgTimestampToUnixPtr(row.Updated),
	}, nil
}

func (p *postgresBackend) UpdatePage(ctx context.Context, name, slug, position string, content *string, active bool, id string) error {
	return p.queries.UpdatePage(ctx, pggen.UpdatePageParams{
		Name:     name,
		Slug:     slug,
		Position: position,
		Content:  content,
		Active:   active,
		ID:       id,
	})
}

func (p *postgresBackend) DeletePage(ctx context.Context, id string) error {
	return p.queries.DeletePage(ctx, id)
}

func (p *postgresBackend) UpdatePageActive(ctx context.Context, id string) error {
	return p.queries.UpdatePageActive(ctx, id)
}

// pgTimestampToUnix converts pgtype.Timestamp to Unix timestamp
func pgTimestampToUnix(ts pgtype.Timestamp) int64 {
	if ts.Valid {
		return ts.Time.Unix()
	}
	return 0
}

// pgTimestampToUnixPtr converts pgtype.Timestamp to Unix timestamp pointer
func pgTimestampToUnixPtr(ts pgtype.Timestamp) *int64 {
	if ts.Valid {
		unix := ts.Time.Unix()
		return &unix
	}
	return nil
}
