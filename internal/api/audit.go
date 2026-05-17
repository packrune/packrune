// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Audit log read endpoint (admin-only).

package api

import (
	"net/http"
	"time"

	"github.com/packrune/packrune/internal/audit"
)

func (a *API) handleListAudit(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	if !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	if a.Audit == nil {
		writeJSON(w, http.StatusOK, struct {
			Items []audit.Record `json:"items"`
		}{Items: []audit.Record{}})
		return
	}

	var before time.Time
	if s := r.URL.Query().Get("before"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			before = t
		}
	}
	items, err := a.Audit.List(r.Context(), before, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items []audit.Record `json:"items"`
	}{Items: items})
}
