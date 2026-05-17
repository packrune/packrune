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
