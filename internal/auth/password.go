// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package auth handles user identity, password hashing, and access tokens.
// Everything in this package treats user input as hostile and constant-time
// compares anything that touches a secret.

package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrBadPassword is returned by Verify when the candidate does not match.
var ErrBadPassword = errors.New("auth: password mismatch")

// passwordCost is the bcrypt cost factor. 12 is a 2026-appropriate default
// (~250ms on a modern CPU); raise if hardware moves on.
const passwordCost = 12

// HashPassword returns a bcrypt hash suitable for storing in the users
// table. The returned string includes the algorithm prefix, cost, and salt.
func HashPassword(plain string) (string, error) {
	if len(plain) < 8 {
		return "", errors.New("auth: password must be at least 8 characters")
	}
	if len(plain) > 72 {
		// bcrypt silently truncates above 72 bytes; we refuse instead so
		// callers do not develop a false sense of strength.
		return "", errors.New("auth: password must be 72 characters or fewer")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), passwordCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// VerifyPassword checks plain against hash. Returns nil on match,
// ErrBadPassword on mismatch, or another error if the hash is malformed.
func VerifyPassword(hash, plain string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrBadPassword
	}
	if err != nil {
		return fmt.Errorf("auth: verify password: %w", err)
	}
	return nil
}
