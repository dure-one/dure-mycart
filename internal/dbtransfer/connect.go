package dbtransfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/shurco/mycart/internal/database"
)

// pgQuerier is the part of a pgx connection or transaction that table
// discovery and the loader need.
type pgQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// connectPostgres opens a connection of its own for a transfer.
//
// A transfer does not go through internal/database's wrapper: it needs the
// PostgreSQL protocol itself (COPY is not SQL), and it holds one connection for
// the whole operation rather than borrowing from the application's pool. It
// goes through the application's DSN normaliser, and pins the session to UTC
// the same way the application does — for a transfer the pin is stricter,
// because a dump is text and PostgreSQL renders a TIMESTAMP in the session's
// time zone. A dump taken in a shifted session would write every date wrong.
func connectPostgres(ctx context.Context, dsn string) (*pgx.Conn, error) {
	normalized, err := database.NormalizePostgresDSN(dsn)
	if err != nil {
		return nil, err
	}

	cfg, err := pgx.ParseConfig(normalized)
	if err != nil {
		return nil, fmt.Errorf("parse the postgres connection string: %w", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["timezone"] = "UTC"

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	var timezone string
	if err := conn.QueryRow(ctx, "SHOW timezone").Scan(&timezone); err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("check postgres timezone: %w", err)
	}
	if !strings.EqualFold(timezone, "UTC") {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("postgres session timezone is %q, must be \"UTC\": a transfer in a shifted "+
			"session would move every timestamp", timezone)
	}

	return conn, nil
}
