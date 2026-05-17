// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Minimal migration runner. We deliberately don't take a dependency on
// golang-migrate yet — a couple hundred lines of focused code is easier to
// audit than 200kB of third-party functionality we use 5% of. The
// schema_migrations table layout matches golang-migrate's so we can adopt it
// transparently in the future.

package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// ApplyMigrations runs every up-migration that has not yet been applied. Safe
// to call at every startup; it is a no-op once the schema is current. The
// caller supplies the fs.FS holding *.up.sql files (use migrations.FS()).
func (db *DB) ApplyMigrations(ctx context.Context, f fs.FS) error {
	if f == nil {
		return errors.New("db: ApplyMigrations: nil fs")
	}

	migs, err := loadMigrations(f)
	if err != nil {
		return err
	}
	if len(migs) == 0 {
		return errors.New("db: no migrations found")
	}

	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migs {
		if applied[m.Version] {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return fmt.Errorf("db: migrate %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

type migration struct {
	Version int64
	Name    string
	UpSQL   string
}

// loadMigrations reads files named "<version>_<name>.up.sql" from f and
// returns them sorted by version.
func loadMigrations(f fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(f, ".")
	if err != nil {
		return nil, fmt.Errorf("db: read migrations dir: %w", err)
	}

	var out []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		base := strings.TrimSuffix(name, ".up.sql")
		us := strings.IndexByte(base, '_')
		if us <= 0 {
			return nil, fmt.Errorf("db: migration %q does not start with <version>_", name)
		}
		v, err := strconv.ParseInt(base[:us], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("db: migration %q: parse version: %w", name, err)
		}
		body, err := fs.ReadFile(f, path.Clean(name))
		if err != nil {
			return nil, fmt.Errorf("db: read migration %q: %w", name, err)
		}
		out = append(out, migration{
			Version: v,
			Name:    base[us+1:],
			UpSQL:   string(body),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func ensureMigrationsTable(ctx context.Context, db *DB) error {
	// The CREATE TABLE for schema_migrations is also in 0001_initial.up.sql,
	// but we need the table before we can read which migrations are applied.
	// IF NOT EXISTS handles the bootstrap; 0001 then no-ops on the duplicate
	// CREATE because SQLite/Postgres both tolerate "CREATE TABLE IF NOT EXISTS"
	// — except 0001 uses plain CREATE TABLE.
	//
	// To avoid that conflict we use a slightly different bootstrap name and
	// the 0001 migration creates its own schema_migrations using plain CREATE.
	// Actually — to keep things simple, we just CREATE IF NOT EXISTS here and
	// rely on the same shape in 0001; the duplicate CREATE in 0001 will fail
	// only if we ran 0001 manually, which we don't.
	const ddl = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT NOT NULL PRIMARY KEY,
		dirty   BOOLEAN NOT NULL
	)`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("db: ensure schema_migrations: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations WHERE dirty = FALSE OR dirty = 0`)
	if err != nil {
		return nil, fmt.Errorf("db: read schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("db: scan schema_migrations: %w", err)
		}
		out[v] = true
	}
	return out, rows.Err()
}

func applyOne(ctx context.Context, db *DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// The body may contain a CREATE TABLE schema_migrations that already
	// exists from ensureMigrationsTable. Strip that one statement to keep the
	// migration idempotent on first apply.
	sqlBody := stripMigrationsTableCreate(m.UpSQL)

	if _, err := tx.ExecContext(ctx, sqlBody); err != nil {
		// Best-effort dirty mark; the tx is about to be rolled back anyway.
		_, _ = db.ExecContext(ctx, db.Rebind(`INSERT INTO schema_migrations (version, dirty) VALUES (?, ?) ON CONFLICT (version) DO UPDATE SET dirty = excluded.dirty`), m.Version, true)
		return fmt.Errorf("exec: %w", err)
	}
	if _, err := tx.ExecContext(ctx, db.Rebind(`INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)`), m.Version, false); err != nil {
		return fmt.Errorf("insert version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// stripMigrationsTableCreate removes a `CREATE TABLE schema_migrations (...)`
// statement from sql, since the bootstrap already created it. Other CREATE
// statements are untouched.
func stripMigrationsTableCreate(sql string) string {
	lower := strings.ToLower(sql)
	idx := strings.Index(lower, "create table schema_migrations")
	if idx < 0 {
		return sql
	}
	end := strings.Index(sql[idx:], ";")
	if end < 0 {
		return sql
	}
	return sql[:idx] + sql[idx+end+1:]
}

