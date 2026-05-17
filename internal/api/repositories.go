// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Repository + format introspection endpoints.

package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
	"github.com/packrune/packrune/internal/repo"
)

type repoView struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Format        string    `json:"format"`
	Kind          string    `json:"kind"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ArtifactCount int       `json:"artifact_count"`
}

func (a *API) handleListRepositories(w http.ResponseWriter, r *http.Request) {
	rs, err := a.Store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]repoView, 0, len(rs))
	for _, x := range rs {
		count := 0
		if arts, err := a.Store.ListArtifactsByPrefix(r.Context(), x.ID, ""); err == nil {
			count = len(arts)
		}
		out = append(out, repoView{
			ID: x.ID, Name: x.Name, Format: x.Format, Kind: string(x.Kind),
			CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, ArtifactCount: count,
		})
	}
	writeJSON(w, http.StatusOK, struct {
		Items []repoView `json:"items"`
	}{Items: out})
}

func (a *API) handleGetRepository(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	fmtName := chi.URLParam(r, "format")
	x, err := a.Store.Get(r.Context(), name, fmtName)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	count := 0
	if arts, err := a.Store.ListArtifactsByPrefix(r.Context(), x.ID, ""); err == nil {
		count = len(arts)
	}
	writeJSON(w, http.StatusOK, repoView{
		ID: x.ID, Name: x.Name, Format: x.Format, Kind: string(x.Kind),
		CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, ArtifactCount: count,
	})
}

type formatView struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func (a *API) handleListFormats(w http.ResponseWriter, _ *http.Request) {
	all := format.All()
	out := make([]formatView, 0, len(all))
	for _, f := range all {
		out = append(out, formatView{Name: f.Name(), DisplayName: f.DisplayName()})
	}
	writeJSON(w, http.StatusOK, struct {
		Items []formatView `json:"items"`
	}{Items: out})
}
