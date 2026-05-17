// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Tokens are opaque random strings. We never store the raw token; only its
// SHA-256 hash. The first few characters of the hex are kept as a "prefix"
// so the UI can render "pkr_abc123…" without revealing the full token.

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrBadToken is returned by VerifyToken when the candidate does not match.
var ErrBadToken = errors.New("auth: token mismatch")

const (
	// tokenPayloadBytes is the entropy of a token (256 bits).
	tokenPayloadBytes = 32
	// tokenPrefix is the display prefix so users can tell a Packrune token at
	// a glance and so leak-scanners can recognize them.
	tokenPrefix = "pkr_"
)

// NewToken returns (plaintext, hash, displayPrefix). The plaintext is what
// the user sees once and never again; the hash is what goes to the database;
// the displayPrefix is what shows up in the UI ("pkr_ab12cd34…").
func NewToken() (plain, hash, displayPrefix string, err error) {
	var b [tokenPayloadBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", "", err
	}
	payload := hex.EncodeToString(b[:])
	plain = tokenPrefix + payload

	h := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(h[:])
	displayPrefix = plain[:len(tokenPrefix)+8]
	return plain, hash, displayPrefix, nil
}

// HashToken returns the database-stored hash of a plaintext token. Use when
// looking up an inbound token by hash.
func HashToken(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

// VerifyToken compares a candidate plaintext against a stored hash in
// constant time. Returns nil on match, ErrBadToken on mismatch.
func VerifyToken(storedHash, candidate string) error {
	candHash := HashToken(candidate)
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(candHash)) != 1 {
		return ErrBadToken
	}
	return nil
}

// LooksLikeToken returns true if s has the Packrune token shape. Used by
// middleware to decide whether to attempt token auth on a request.
func LooksLikeToken(s string) bool {
	return strings.HasPrefix(s, tokenPrefix) && len(s) == len(tokenPrefix)+2*tokenPayloadBytes
}
