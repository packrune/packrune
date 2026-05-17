// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the npm registry surface. Mounted at /npm.

package npm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
)

// Handler serves the npm registry surface for one Packrune npm repository.
type Handler struct {
	logger   *slog.Logger
	repoID   string
	repoKind string
	proxy    ProxyConfig
	backend  storage.Backend
	cas      *cas.CAS
	store    *repo.Store
}

// HandlerConfig is the dependency bundle passed to NewHandler.
type HandlerConfig struct {
	Logger  *slog.Logger
	Repo    repo.Repository
	Backend storage.Backend
	CAS     *cas.CAS
	Store   *repo.Store
}

// NewHandler constructs a Handler bound to one Packrune npm repository.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:   cfg.Logger,
		repoID:   cfg.Repo.ID,
		repoKind: string(cfg.Repo.Kind),
		proxy:    ParseProxyConfig(cfg.Repo.Config),
		backend:  cfg.Backend,
		cas:      cfg.CAS,
		store:    cfg.Store,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pr := parsePath(r.URL.Path)
	switch pr.kind {
	case routeRoot:
		h.handleRoot(w, r)
	case routePing:
		h.handlePing(w, r)
	case routeWhoami:
		h.handleWhoami(w, r)
	case routeSearch:
		h.handleSearch(w, r)
	case routePackument:
		h.handlePackument(w, r, pr.pkg)
	case routeVersion:
		h.handleVersion(w, r, pr.pkg, pr.ref)
	case routeTarball:
		h.handleTarball(w, r, pr.pkg, pr.ref)
	default:
		writeError(w, http.StatusNotFound, "unknown route")
	}
}

func (h *Handler) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"db_name":"registry","engine":"packrune"}`))
}

func (h *Handler) handlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func (h *Handler) handleWhoami(w http.ResponseWriter, _ *http.Request) {
	// Anonymous registry today. Once we wire token auth (Faz 7) this returns
	// the authenticated user's name.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"username":"anonymous"}`))
}

func (h *Handler) handleSearch(w http.ResponseWriter, _ *http.Request) {
	// Stub: empty results. Real search lands in Faz 7 when the indexer comes
	// online.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"objects":[],"total":0,"time":"now"}`))
}

func (h *Handler) handlePackument(w http.ResponseWriter, r *http.Request, pkg string) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.servePackument(w, r, pkg, r.Method == http.MethodHead)
	case http.MethodPut:
		h.publishPackument(w, r, pkg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) servePackument(w http.ResponseWriter, r *http.Request, pkg string, headOnly bool) {
	art, err := h.store.ResolveArtifact(r.Context(), h.repoID, "packuments/"+pkg)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) && h.repoKind == "proxy" {
			body, perr := h.proxyFetchPackument(r.Context(), pkg)
			if perr == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusOK)
				if !headOnly {
					_, _ = w.Write(body)
				}
				return
			}
		}
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "package not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rc, st, err := h.cas.Get(r.Context(), art.Digest)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "package blob missing")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	w.WriteHeader(http.StatusOK)
	if !headOnly {
		_, _ = io.Copy(w, rc)
	}
}

func (h *Handler) handleVersion(w http.ResponseWriter, r *http.Request, pkg, version string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Read the packument and extract the requested version object.
	art, err := h.store.ResolveArtifact(r.Context(), h.repoID, "packuments/"+pkg)
	if err != nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	rc, _, err := h.cas.Get(r.Context(), art.Digest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var pkmt packument
	if err := json.Unmarshal(body, &pkmt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// dist-tags shortcut: "latest" → resolve to actual version.
	if v, ok := pkmt.DistTags[version]; ok {
		version = v
	}
	raw, ok := pkmt.Versions[version]
	if !ok {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (h *Handler) handleTarball(w http.ResponseWriter, r *http.Request, pkg, filename string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	art, err := h.store.ResolveArtifact(r.Context(), h.repoID, "tarballs/"+pkg+"/"+filename)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) && h.repoKind == "proxy" {
			body, perr := h.proxyFetchTarball(r.Context(), pkg, filename)
			if perr == nil {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusOK)
				if r.Method != http.MethodHead {
					_, _ = w.Write(body)
				}
				return
			}
		}
		writeError(w, http.StatusNotFound, "tarball not found")
		return
	}
	rc, st, err := h.cas.Get(r.Context(), art.Digest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	w.Header().Set("ETag", `"`+art.Digest+`"`)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, rc)
	}
}

// publishPackument accepts a publish PUT and merges it into the stored
// packument, extracting any _attachments as CAS-stored tarballs.
func (h *Handler) publishPackument(w http.ResponseWriter, r *http.Request, pkg string) {
	const maxPublish = 100 << 20 // 100 MiB hard cap
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPublish))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	incoming, err := parsePackument(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if incoming.Name != pkg {
		writeError(w, http.StatusBadRequest, "packument name does not match URL")
		return
	}

	// Load existing packument, if any.
	stored := &packument{Name: pkg}
	if art, err := h.store.GetArtifact(r.Context(), h.repoID, "packuments/"+pkg); err == nil {
		if rc, _, err := h.cas.Get(r.Context(), art.Digest); err == nil {
			if existing, err := io.ReadAll(rc); err == nil {
				_ = json.Unmarshal(existing, stored)
			}
			_ = rc.Close()
		}
	}

	// Persist tarballs from _attachments.
	for filename, att := range incoming.Attachments {
		raw, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			writeError(w, http.StatusBadRequest, "attachment "+filename+": base64: "+err.Error())
			return
		}
		digestStr, _, err := h.cas.Put(r.Context(), bytes.NewReader(raw))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store tarball: "+err.Error())
			return
		}
		if err := h.store.UpsertArtifact(
			r.Context(), h.repoID, "tarballs/"+pkg+"/"+filename,
			digestStr, int64(len(raw)), "application/octet-stream", "",
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Merge the publish into the stored packument.
	if err := incoming.mergeInto(stored); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	out, err := stored.marshal()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(out))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID, "packuments/"+pkg, digest, int64(len(out)),
		"application/json", "",
	); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"ok":true,"id":"` + pkg + `"}`))
}
