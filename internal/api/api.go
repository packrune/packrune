// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package api hosts the JSON HTTP API consumed by the React frontend.
//
// Conventions:
//   - All responses are JSON encoded with snake_case field names.
//   - Authentication is by session cookie (issued at login) or bearer token.
//   - Errors come back as {"error": "..."} with the appropriate HTTP status.
//   - Endpoints under /api/auth/login and /api/system/version are public;
//     everything else requires an authenticated user.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/audit"
	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/webhook"
)

// API bundles the JSON API dependencies and constructs the chi sub-router.
type API struct {
	Logger      *slog.Logger
	Auth        *auth.DBService
	Store       *repo.Store
	Webhooks    *webhook.Service
	Audit       *audit.Reader
	AuditWriter *audit.Writer

	Version string
	Commit  string
	Date    string
}

// Router returns a chi.Router with all /api routes mounted.
func (a *API) Router() chi.Router {
	r := chi.NewRouter()

	r.Get("/system/version", a.handleSystemVersion)
	r.Post("/auth/login", a.handleLogin)
	r.Post("/auth/logout", a.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.Get("/me", a.handleMe)
		r.Get("/repositories", a.handleListRepositories)
		r.Get("/repositories/{name}/{format}", a.handleGetRepository)
		r.Get("/repositories/{name}/{format}/artifacts", a.handleListRepositoryArtifacts)
		r.Get("/formats", a.handleListFormats)
		r.Get("/system/stats", a.handleSystemStats)

		// Token self-service (admin can manage other users' tokens via
		// ?user_id=).
		r.Get("/tokens", a.handleListTokens)
		r.Post("/tokens", a.handleCreateToken)
		r.Delete("/tokens/{id}", a.handleRevokeToken)

		// Admin-only (the handler checks IsAdmin and returns 403 otherwise).
		r.Get("/users", a.handleListUsers)
		r.Post("/users", a.handleCreateUser)
		r.Delete("/users/{id}", a.handleDeactivateUser)
		r.Get("/webhooks", a.handleListWebhooks)
		r.Post("/webhooks", a.handleCreateWebhook)
		r.Delete("/webhooks/{id}", a.handleDeleteWebhook)
		r.Get("/audit", a.handleListAudit)
	})

	return r
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: msg})
}
