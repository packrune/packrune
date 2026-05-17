// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Login/logout + /me. Sessions are bearer tokens delivered as an HttpOnly
// cookie. They're regular token rows in the database — same schema, same
// revoke path; the cookie wrapping is purely a transport detail.

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/packrune/packrune/internal/audit"
	"github.com/packrune/packrune/internal/auth"
)

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	u, err := a.Auth.AuthenticateBasic(r.Context(), req.Username, req.Password)
	if err != nil {
		if a.AuditWriter != nil {
			_ = a.AuditWriter.Write(r.Context(), audit.Event{
				Action: "login", Result: audit.ResultDeny, RemoteAddr: r.RemoteAddr,
				Metadata: map[string]any{"username": req.Username},
			})
		}
		if errors.Is(err, auth.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	plain, tok, err := a.Auth.IssueToken(r.Context(), u.ID, "session", []string{"session"}, 24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue session: "+err.Error())
		return
	}
	if a.AuditWriter != nil {
		_ = a.AuditWriter.Write(r.Context(), audit.Event{
			UserID: u.ID, Action: "login", Result: audit.ResultAllow,
			RemoteAddr: r.RemoteAddr,
		})
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    plain,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
		// Secure must be set in production behind TLS. We leave it off here
		// so local-dev (plain HTTP) still works; reverse proxies / browsers
		// upgrade as appropriate.
	})

	writeJSON(w, http.StatusOK, userView(u, &tok))
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookieName)
	if err == nil && c.Value != "" {
		// Resolve the token to its DB ID so we can revoke it (so the
		// plaintext value cannot be reused even if it was logged).
		if _, tok, err := a.Auth.AuthenticateToken(r.Context(), c.Value); err == nil {
			_ = a.Auth.RevokeToken(r.Context(), tok.ID)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	writeJSON(w, http.StatusOK, userView(u, nil))
}

// userView is the JSON shape of /api/me. Token field, when present, is the
// metadata of the *currently authenticating* token (handy for showing "you
// logged in 2 hours ago").
type userViewModel struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name"`
	IsAdmin     bool           `json:"is_admin"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	Token       *tokenViewItem `json:"token,omitempty"`
}

type tokenViewItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

func userView(u auth.User, t *auth.Token) userViewModel {
	v := userViewModel{
		ID:          u.ID,
		Email:       u.Email,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		IsAdmin:     u.IsAdmin,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
	}
	if t != nil {
		v.Token = &tokenViewItem{ID: t.ID, Name: t.Name, Prefix: t.Prefix, CreatedAt: t.CreatedAt}
	}
	return v
}
