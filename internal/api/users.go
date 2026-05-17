// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// User admin endpoints (admin-only).

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type userListItem struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	if !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	users, err := a.Auth.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]userListItem, 0, len(users))
	for _, u := range users {
		out = append(out, userListItem{
			ID: u.ID, Email: u.Email, Username: u.Username,
			DisplayName: u.DisplayName, IsAdmin: u.IsAdmin,
			IsActive: u.IsActive, CreatedAt: u.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, struct {
		Items []userListItem `json:"items"`
	}{Items: out})
}

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	if !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	u, err := a.Auth.CreateUser(r.Context(), req.Email, req.Username, req.Password, req.IsAdmin)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userListItem{
		ID: u.ID, Email: u.Email, Username: u.Username,
		DisplayName: u.DisplayName, IsAdmin: u.IsAdmin,
		IsActive: u.IsActive, CreatedAt: u.CreatedAt,
	})
}

func (a *API) handleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	if !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := a.Auth.DeactivateUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
