// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/migrations"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	cfg := config.DatabaseConfig{
		Driver:       "sqlite",
		DSN:          filepath.Join(t.TempDir(), "test.db"),
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}
	database, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.ApplyMigrations(ctx, migrations.FS()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func TestDBService_CreateAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc := auth.NewDBService(newTestDB(t))

	u, err := svc.CreateUser(ctx, "berk@relteco.com", "berk", "supersecret", true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !u.IsAdmin {
		t.Errorf("expected admin")
	}

	got, err := svc.AuthenticateBasic(ctx, "berk", "supersecret")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got %q, want %q", got.ID, u.ID)
	}

	_, err = svc.AuthenticateBasic(ctx, "berk", "wrongpass")
	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Errorf("wrong password: got %v, want ErrUnauthorized", err)
	}

	_, err = svc.AuthenticateBasic(ctx, "nobody", "supersecret")
	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Errorf("unknown user: got %v, want ErrUnauthorized", err)
	}
}

func TestDBService_TokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc := auth.NewDBService(newTestDB(t))

	u, err := svc.CreateUser(ctx, "berk@relteco.com", "berk", "supersecret", false)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	plain, tok, err := svc.IssueToken(ctx, u.ID, "ci-token", []string{"repo:read", "repo:write"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if plain == "" || tok.ID == "" {
		t.Fatalf("empty token data")
	}
	if tok.ExpiresAt == nil {
		t.Fatalf("expected non-nil expiry")
	}

	gotUser, gotTok, err := svc.AuthenticateToken(ctx, plain)
	if err != nil {
		t.Fatalf("authenticate token: %v", err)
	}
	if gotUser.ID != u.ID || gotTok.ID != tok.ID {
		t.Errorf("mismatch: got user=%q tok=%q, want user=%q tok=%q",
			gotUser.ID, gotTok.ID, u.ID, tok.ID)
	}
	if len(gotTok.Scopes) != 2 {
		t.Errorf("scopes = %v, want 2", gotTok.Scopes)
	}

	if err := svc.RevokeToken(ctx, tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, _, err := svc.AuthenticateToken(ctx, plain); !errors.Is(err, auth.ErrUnauthorized) {
		t.Errorf("revoked token: got %v, want ErrUnauthorized", err)
	}
}
