// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Webhook admin endpoints.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/webhook"
)

func (a *API) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if !u.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	if a.Webhooks == nil {
		writeJSON(w, http.StatusOK, struct {
			Items []webhook.Webhook `json:"items"`
		}{Items: []webhook.Webhook{}})
		return
	}
	items, err := a.Webhooks.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items []webhook.Webhook `json:"items"`
	}{Items: items})
}

func (a *API) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if !u.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	var req struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	hk, err := a.Webhooks.Create(r.Context(), req.Name, req.URL, req.Secret, req.Events)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, hk)
}

func (a *API) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if !u.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := a.Webhooks.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
