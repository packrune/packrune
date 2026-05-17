// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Database-backed implementation of Service. Keeps SQL local to its caller —
// this file is the only place in the auth package that knows about tables.

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/packrune/packrune/internal/db"
)

// DBService implements Service against a SQL database (SQLite or Postgres).
type DBService struct {
	db *db.DB
}

// NewDBService constructs a DBService backed by the given database.
func NewDBService(database *db.DB) *DBService {
	return &DBService{db: database}
}

// CreateUser inserts a new user with a bcrypt-hashed password.
func (s *DBService) CreateUser(ctx context.Context, email, username, password string, isAdmin bool) (User, error) {
	email = strings.TrimSpace(email)
	username = strings.TrimSpace(username)
	if email == "" || username == "" {
		return User{}, errors.New("auth: email and username required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	id := uuid.NewString()
	now := time.Now().UTC()
	q := s.db.Rebind(`INSERT INTO users
		(id, email, username, display_name, password_hash, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, id, email, username, username, hash, boolToInt(isAdmin), now, now); err != nil {
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}
	return User{
		ID:          id,
		Email:       email,
		Username:    username,
		DisplayName: username,
		IsAdmin:     isAdmin,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// AuthenticateBasic verifies a username-or-email + password against the users
// table. Returns ErrUnauthorized for both "user not found" and "wrong
// password" to avoid leaking which.
func (s *DBService) AuthenticateBasic(ctx context.Context, identifier, password string) (User, error) {
	var (
		u        User
		hash     string
		isAdmin  int
		isActive int
	)
	q := s.db.Rebind(`SELECT id, email, username, display_name, password_hash,
		is_admin, is_active, created_at, updated_at FROM users
		WHERE username = ? OR email = ? LIMIT 1`)
	err := s.db.QueryRowContext(ctx, q, identifier, identifier).Scan(
		&u.ID, &u.Email, &u.Username, &u.DisplayName, &hash,
		&isAdmin, &isActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: query user: %w", err)
	}
	if isActive == 0 {
		return User{}, ErrUnauthorized
	}
	if err := VerifyPassword(hash, password); err != nil {
		return User{}, ErrUnauthorized
	}
	u.IsAdmin = isAdmin != 0
	u.IsActive = true
	return u, nil
}

// IssueToken creates a new opaque token bound to userID.
func (s *DBService) IssueToken(ctx context.Context, userID, name string, scopes []string, ttl time.Duration) (string, Token, error) {
	plain, hash, displayPrefix, err := NewToken()
	if err != nil {
		return "", Token{}, fmt.Errorf("auth: gen token: %w", err)
	}
	id := uuid.NewString()
	now := time.Now().UTC()

	var expiresAt sql.NullTime
	if ttl > 0 {
		expiresAt = sql.NullTime{Time: now.Add(ttl), Valid: true}
	}
	scopesStr := strings.Join(scopes, ",")

	q := s.db.Rebind(`INSERT INTO tokens
		(id, user_id, name, hash, prefix, scopes, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, id, userID, name, hash, displayPrefix, scopesStr, expiresAt, now); err != nil {
		return "", Token{}, fmt.Errorf("auth: insert token: %w", err)
	}

	t := Token{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Prefix:    displayPrefix,
		Scopes:    scopes,
		CreatedAt: now,
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return plain, t, nil
}

// AuthenticateToken resolves an inbound plaintext token to the (User, Token)
// pair it represents. Updates last_used_at on success.
func (s *DBService) AuthenticateToken(ctx context.Context, plain string) (User, Token, error) {
	if !LooksLikeToken(plain) {
		return User{}, Token{}, ErrUnauthorized
	}
	hash := HashToken(plain)

	var (
		t         Token
		u         User
		scopesStr string
		isAdmin   int
		isActive  int
		lastUsed  sql.NullTime
		expires   sql.NullTime
	)
	q := s.db.Rebind(`SELECT
		t.id, t.user_id, t.name, t.prefix, t.scopes, t.last_used_at, t.expires_at, t.created_at,
		u.id, u.email, u.username, u.display_name, u.is_admin, u.is_active, u.created_at, u.updated_at
		FROM tokens t INNER JOIN users u ON u.id = t.user_id
		WHERE t.hash = ? LIMIT 1`)
	err := s.db.QueryRowContext(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.Name, &t.Prefix, &scopesStr, &lastUsed, &expires, &t.CreatedAt,
		&u.ID, &u.Email, &u.Username, &u.DisplayName, &isAdmin, &isActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Token{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, Token{}, fmt.Errorf("auth: token lookup: %w", err)
	}
	if isActive == 0 {
		return User{}, Token{}, ErrUnauthorized
	}
	if expires.Valid {
		if time.Now().After(expires.Time) {
			return User{}, Token{}, ErrUnauthorized
		}
		t.ExpiresAt = &expires.Time
	}
	if lastUsed.Valid {
		t.LastUsedAt = &lastUsed.Time
	}
	if scopesStr != "" {
		t.Scopes = strings.Split(scopesStr, ",")
	}
	u.IsAdmin = isAdmin != 0
	u.IsActive = true

	// Best-effort last_used_at refresh. We don't fail auth on this.
	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, s.db.Rebind(`UPDATE tokens SET last_used_at = ? WHERE id = ?`), now, t.ID)

	return u, t, nil
}

// RevokeToken deletes a token by ID.
func (s *DBService) RevokeToken(ctx context.Context, tokenID string) error {
	q := s.db.Rebind(`DELETE FROM tokens WHERE id = ?`)
	if _, err := s.db.ExecContext(ctx, q, tokenID); err != nil {
		return fmt.Errorf("auth: revoke token: %w", err)
	}
	return nil
}

// ListTokens returns every token for a user, ordered by created_at desc.
// Plaintext values are NEVER returned — only the metadata.
func (s *DBService) ListTokens(ctx context.Context, userID string) ([]Token, error) {
	q := s.db.Rebind(`SELECT id, user_id, name, prefix, scopes, last_used_at, expires_at, created_at
		FROM tokens WHERE user_id = ? ORDER BY created_at DESC`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list tokens: %w", err)
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		var scopesStr string
		var lastUsed sql.NullTime
		var expires sql.NullTime
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &scopesStr, &lastUsed, &expires, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan token: %w", err)
		}
		if scopesStr != "" {
			t.Scopes = strings.Split(scopesStr, ",")
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Time
		}
		if expires.Valid {
			t.ExpiresAt = &expires.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListUsers returns every user, ordered by created_at desc. Admin-only at
// the API layer.
func (s *DBService) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, username, display_name, is_admin, is_active, created_at, updated_at
		FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("auth: list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var isAdmin, isActive int
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &isAdmin, &isActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan user: %w", err)
		}
		u.IsAdmin = isAdmin != 0
		u.IsActive = isActive != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

// DeactivateUser sets is_active=0 without deleting the row, preserving
// referential integrity for tokens / audit log.
func (s *DBService) DeactivateUser(ctx context.Context, userID string) error {
	q := s.db.Rebind(`UPDATE users SET is_active = 0, updated_at = ? WHERE id = ?`)
	if _, err := s.db.ExecContext(ctx, q, time.Now().UTC(), userID); err != nil {
		return fmt.Errorf("auth: deactivate user: %w", err)
	}
	return nil
}

// SetPasswordByUsername rehashes and stores a new password for the user with
// the given username (or email). Returns ErrNotFound if no such user.
func (s *DBService) SetPasswordByUsername(ctx context.Context, identifier, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	q := s.db.Rebind(`UPDATE users SET password_hash = ?, updated_at = ?
		WHERE username = ? OR email = ?`)
	res, err := s.db.ExecContext(ctx, q, hash, time.Now().UTC(), identifier, identifier)
	if err != nil {
		return fmt.Errorf("auth: set password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// Force re-login: revoke every existing session token for this user.
	_, _ = s.db.ExecContext(ctx,
		s.db.Rebind(`DELETE FROM tokens WHERE user_id IN (SELECT id FROM users WHERE username = ? OR email = ?)`),
		identifier, identifier)
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
