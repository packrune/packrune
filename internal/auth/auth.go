// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// High-level auth service skeleton. The concrete persistence wiring lands in
// Faz 1 continuation (db package); this file defines the surface area so
// other packages can depend on stable types right away.

package auth

import (
	"context"
	"errors"
	"time"
)

// Role is a coarse permission tier on a Repository.
type Role string

const (
	RoleRead  Role = "read"
	RoleWrite Role = "write"
	RoleAdmin Role = "admin"
)

// User is the canonical user record returned to callers. Password hashes do
// not appear here on purpose; only the auth package itself handles them.
type User struct {
	ID          string
	Email       string
	Username    string
	DisplayName string
	IsAdmin     bool
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Token is the metadata for an issued access token. The plaintext is shown
// once at creation; thereafter only the prefix and the hash are stored.
type Token struct {
	ID         string
	UserID     string
	Name       string
	Prefix     string
	Scopes     []string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

// Service is the public auth API. Implementations wire it up to the database
// in the db package; this interface is mocked in tests for higher layers.
type Service interface {
	// CreateUser provisions a user with a hashed password.
	CreateUser(ctx context.Context, email, username, password string, isAdmin bool) (User, error)

	// AuthenticateBasic verifies username+password.
	AuthenticateBasic(ctx context.Context, username, password string) (User, error)

	// IssueToken creates a new access token for a user. The returned plaintext
	// is shown to the user exactly once.
	IssueToken(ctx context.Context, userID, name string, scopes []string, ttl time.Duration) (plain string, t Token, err error)

	// AuthenticateToken resolves an inbound plaintext token to a User.
	AuthenticateToken(ctx context.Context, plain string) (User, Token, error)

	// RevokeToken removes a token by ID.
	RevokeToken(ctx context.Context, tokenID string) error
}

// ErrUnauthorized is returned by Service methods when a credential check
// fails. Higher layers map this to HTTP 401.
var ErrUnauthorized = errors.New("auth: unauthorized")

// ErrNotFound is returned when a user or token lookup misses.
var ErrNotFound = errors.New("auth: not found")
