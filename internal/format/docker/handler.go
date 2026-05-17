// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the Docker Registry V2 surface. Dispatches an incoming
// request to the right handler based on parsePath, then invokes the
// method-specific logic in blob.go / manifest.go / upload.go.

package docker

import (
	"log/slog"
	"net/http"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
)

// Handler serves the /v2/ surface for one Packrune docker repository. Multiple
// docker repos are supported via separate Handler instances mounted on
// distinct path prefixes (faz 2 polish — today there is a single default
// repo).
type Handler struct {
	logger   *slog.Logger
	repoID   string
	repoName string
	backend  storage.Backend
	cas      *cas.CAS
	store    *repo.Store
	uploads  *uploadManager
}

// HandlerConfig is the dependency bundle passed to NewHandler.
type HandlerConfig struct {
	Logger      *slog.Logger
	Repo        repo.Repository
	Backend     storage.Backend
	CAS         *cas.CAS
	Store       *repo.Store
	UploadRoot  string // directory used to stage in-progress uploads
}

// NewHandler constructs a Handler bound to one Packrune docker repository.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:   cfg.Logger,
		repoID:   cfg.Repo.ID,
		repoName: cfg.Repo.Name,
		backend:  cfg.Backend,
		cas:      cfg.CAS,
		store:    cfg.Store,
		uploads:  newUploadManager(cfg.UploadRoot),
	}
}

// ServeHTTP is the single entrypoint mounted at /v2 by the server.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// All Docker registry responses include this header to identify the API
	// version. Set it eagerly; handlers can still write their own headers.
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

	pr := parsePath(r.URL.Path)
	switch pr.kind {
	case routeVersion:
		h.handleVersion(w, r)
	case routeCatalog:
		h.handleCatalog(w, r)
	case routeTagsList:
		h.handleTagsList(w, r, pr.name)
	case routeManifest:
		h.handleManifest(w, r, pr.name, pr.reference)
	case routeBlob:
		h.handleBlob(w, r, pr.name, pr.reference)
	case routeUploadStart:
		h.handleUploadStart(w, r, pr.name)
	case routeUploadSession:
		h.handleUploadSession(w, r, pr.name, pr.reference)
	default:
		writeError(w, http.StatusNotFound, errCodeUnsupported, "unknown route")
	}
}

// handleVersion: GET /v2/ — registry version check (Docker auth handshake
// also looks for this 200 OK).
func (h *Handler) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}
