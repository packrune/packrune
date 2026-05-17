// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// System-level introspection: version, aggregate stats. Used by the
// dashboard.

package api

import (
	"net/http"
	"runtime"
)

func (a *API) handleSystemVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
		Go      string `json:"go"`
	}{
		Version: a.Version,
		Commit:  a.Commit,
		Date:    a.Date,
		Go:      runtime.Version(),
	})
}

// handleSystemConfig returns a sanitized view of the active configuration
// (admin-only). Secret fields are redacted.
func (a *API) handleSystemConfig(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	if !me.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	if a.Config == nil {
		writeError(w, http.StatusServiceUnavailable, "config not wired")
		return
	}
	c := *a.Config
	// Redact anything that could be sensitive.
	c.Auth.TokenSecret = redact(c.Auth.TokenSecret)
	c.Storage.S3.SecretKey = redact(c.Storage.S3.SecretKey)
	c.Database.DSN = redact(c.Database.DSN)
	writeJSON(w, http.StatusOK, c)
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "[redacted]"
	}
	return s[:4] + "…" + "[redacted]"
}

func (a *API) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	rs, err := a.Store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats := struct {
		RepositoryCount int            `json:"repository_count"`
		ArtifactCount   int            `json:"artifact_count"`
		PerFormat       map[string]int `json:"per_format"`
	}{
		PerFormat: map[string]int{},
	}
	stats.RepositoryCount = len(rs)
	for _, x := range rs {
		arts, _ := a.Store.ListArtifactsByPrefix(r.Context(), x.ID, "")
		stats.ArtifactCount += len(arts)
		stats.PerFormat[x.Format] += len(arts)
	}
	writeJSON(w, http.StatusOK, stats)
}
