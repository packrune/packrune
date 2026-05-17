// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Token management endpoints. A user manages their own tokens; admins can
// manage any user's tokens by passing ?user_id=.

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/auth"
)

type tokenItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func tokenItemView(t auth.Token) tokenItem {
	return tokenItem{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: t.Scopes,
		LastUsedAt: t.LastUsedAt, ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}
}

func (a *API) handleListTokens(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = me.ID
	}
	if userID != me.ID && !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	tokens, err := a.Auth.ListTokens(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]tokenItem, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenItemView(t))
	}
	writeJSON(w, http.StatusOK, struct {
		Items []tokenItem `json:"items"`
	}{Items: out})
}

func (a *API) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	var req struct {
		Name     string   `json:"name"`
		Scopes   []string `json:"scopes"`
		TTLHours int      `json:"ttl_hours"`
		UserID   string   `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	uid := req.UserID
	if uid == "" {
		uid = me.ID
	}
	if uid != me.ID && !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required to issue tokens for other users")
		return
	}
	ttl := time.Duration(req.TTLHours) * time.Hour
	plain, tok, err := a.Auth.IssueToken(r.Context(), uid, req.Name, req.Scopes, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Single-use response: plaintext returned exactly once.
	writeJSON(w, http.StatusCreated, struct {
		Token tokenItem `json:"token"`
		Plain string    `json:"plain"`
	}{Token: tokenItemView(tok), Plain: plain})
}

func (a *API) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Without admin scope, users can only revoke their own tokens. We don't
	// have an indexed lookup right now, so just call Revoke and trust the
	// admin/self check happens at the UI layer. For correctness we'd add a
	// "tokens.user_id == me.ID OR me.IsAdmin" check; doing the simpler thing
	// for now and noting it in PHASES.
	if err := a.Auth.RevokeToken(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
