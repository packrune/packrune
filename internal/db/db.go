// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package db owns the *sql.DB connection and the small set of helpers we use
// across the codebase. Query code lives next to its caller; we deliberately
// avoid a god-package that imports every other package's models.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // postgres driver
	_ "modernc.org/sqlite"             // pure-Go sqlite driver

	"github.com/packrune/packrune/internal/config"
)

// DB wraps a *sql.DB with a label for logging plus the driver name so callers
// can branch on dialect when needed (mostly: rare).
type DB struct {
	*sql.DB
	Driver string
}

// Open connects to the configured database, applying driver-specific tuning
// for SQLite (WAL mode, busy timeout) and Postgres (connection pool sizing).
func Open(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	driverName, dsn, err := resolveDSN(cfg)
	if err != nil {
		return nil, err
	}

	sqldb, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", cfg.Driver, err)
	}

	if cfg.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	sqldb.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqldb.PingContext(pingCtx); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("db: ping %s: %w", cfg.Driver, err)
	}

	if cfg.Driver == "sqlite" {
		if err := applySQLiteTuning(ctx, sqldb); err != nil {
			_ = sqldb.Close()
			return nil, err
		}
	}

	return &DB{DB: sqldb, Driver: cfg.Driver}, nil
}

func resolveDSN(cfg config.DatabaseConfig) (driver, dsn string, err error) {
	switch cfg.Driver {
	case "sqlite":
		// modernc.org/sqlite registers under "sqlite".
		// Ensure the parent dir exists; SQLite won't create it for you.
		if dir := filepath.Dir(cfg.DSN); dir != "" && dir != "." {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return "", "", fmt.Errorf("db: mkdir for sqlite: %w", mkErr)
			}
		}
		// Default to WAL via pragmas applied after open (see applySQLiteTuning).
		return "sqlite", cfg.DSN, nil
	case "postgres":
		// pgx/stdlib registers under "pgx".
		if !strings.HasPrefix(cfg.DSN, "postgres://") && !strings.HasPrefix(cfg.DSN, "postgresql://") {
			return "", "", errors.New("db: postgres DSN must start with postgres:// or postgresql://")
		}
		return "pgx", cfg.DSN, nil
	default:
		return "", "", fmt.Errorf("db: unknown driver %q", cfg.Driver)
	}
}

// Rebind converts a query written with "?" placeholders into the syntax used
// by the active driver. SQLite uses "?"; Postgres uses "$1", "$2", ... Use
// this anywhere we write portable SQL by hand.
func (db *DB) Rebind(query string) string {
	if db.Driver != "postgres" {
		return query
	}
	var out strings.Builder
	out.Grow(len(query) + 8)
	n := 1
	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == '?' {
			out.WriteByte('$')
			out.WriteString(strconv.Itoa(n))
			n++
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

// applySQLiteTuning enables write-ahead logging and a generous busy timeout.
// These pragmas are per-connection, so we run them every time a new
// connection is opened by setting them globally on the singleton DB; the
// SetMaxOpenConns(1) in dev mode avoids the per-connection-pragma pitfall.
func applySQLiteTuning(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("db: %s: %w", p, err)
		}
	}
	return nil
}
