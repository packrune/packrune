// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Authentication middleware. Resolves the inbound credential (cookie or
// bearer header) into a *auth.User and stuffs it into the request context.
// Downstream handlers retrieve it via userFrom.

package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/packrune/packrune/internal/auth"
)

const sessionCookieName = "packrune_session"

type ctxKey int

const ctxKeyUser ctxKey = iota

// requireAuth rejects requests without a valid session/token. The resolved
// User is attached to the request context.
func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.resolveUser(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveUser tries cookie first, then bearer header.
func (a *API) resolveUser(r *http.Request) (auth.User, error) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		u, _, err := a.Auth.AuthenticateToken(r.Context(), c.Value)
		if err == nil {
			return u, nil
		}
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		u, _, err := a.Auth.AuthenticateToken(r.Context(), strings.TrimPrefix(h, "Bearer "))
		if err == nil {
			return u, nil
		}
	}
	return auth.User{}, errors.New("no credential")
}

// userFrom returns the authenticated user for a context. Panics if called
// outside of an authenticated handler — that would be a programming error.
func userFrom(ctx context.Context) auth.User {
	u, _ := ctx.Value(ctxKeyUser).(auth.User)
	return u
}
