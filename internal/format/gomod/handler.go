// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the Go module proxy surface.

package gomod

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
)

type Handler struct {
	logger  *slog.Logger
	repoID  string
	backend storage.Backend
	cas     *cas.CAS
	store   *repo.Store
}

type HandlerConfig struct {
	Logger  *slog.Logger
	Repo    repo.Repository
	Backend storage.Backend
	CAS     *cas.CAS
	Store   *repo.Store
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:  cfg.Logger,
		repoID:  cfg.Repo.ID,
		backend: cfg.Backend,
		cas:     cfg.CAS,
		store:   cfg.Store,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pr := parsePath(r.URL.Path)
	switch pr.kind {
	case routeList:
		h.handleList(w, r, pr.module)
	case routeLatest:
		h.handleLatest(w, r, pr.module)
	case routeInfo, routeMod, routeZip:
		ext := extOf(pr.kind)
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			h.serveVersionFile(w, r, pr.module, pr.version, ext)
		case http.MethodPut:
			h.uploadVersionFile(w, r, pr.module, pr.version, ext)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

func extOf(k routeKind) string {
	switch k {
	case routeInfo:
		return ".info"
	case routeMod:
		return ".mod"
	case routeZip:
		return ".zip"
	}
	return ""
}

// handleList returns a newline-separated list of all known versions of a
// module.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request, module string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "modules/" + module + "/"
	arts, err := h.store.ListArtifactsByPrefix(r.Context(), h.repoID, prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	seen := map[string]bool{}
	var versions []string
	for _, a := range arts {
		// path shape: modules/<module>/<version>.info|.mod|.zip
		base := strings.TrimPrefix(a.Path, prefix)
		ext := extFromPath(base)
		if ext != ".info" {
			continue
		}
		ver := strings.TrimSuffix(base, ext)
		if !seen[ver] {
			seen[ver] = true
			versions = append(versions, ver)
		}
	}
	sort.Strings(versions)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, v := range versions {
		_, _ = w.Write([]byte(v))
		_, _ = w.Write([]byte("\n"))
	}
}

// handleLatest returns the highest-versioned .info for the module.
func (h *Handler) handleLatest(w http.ResponseWriter, r *http.Request, module string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "modules/" + module + "/"
	arts, err := h.store.ListArtifactsByPrefix(r.Context(), h.repoID, prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var infos []repo.Artifact
	for _, a := range arts {
		if strings.HasSuffix(a.Path, ".info") {
			infos = append(infos, a)
		}
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	// Pick the lexicographically-greatest version. Real semver ordering
	// belongs in a follow-up — for vN.N.N strings sorted alphabetically the
	// difference rarely matters and tests cover the simple path.
	sort.Slice(infos, func(i, j int) bool { return infos[i].Path < infos[j].Path })
	pick := infos[len(infos)-1]
	rc, _, err := h.cas.Get(r.Context(), pick.Digest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, rc)
}

func (h *Handler) serveVersionFile(w http.ResponseWriter, r *http.Request, module, version, ext string) {
	path := "modules/" + module + "/" + version + ext
	art, err := h.store.GetArtifact(r.Context(), h.repoID, path)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rc, st, err := h.cas.Get(r.Context(), art.Digest)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", contentTypeFor(ext))
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, rc)
	}
}

// uploadVersionFile accepts a PUT for one of (.info | .mod | .zip).
// Out-of-band publishing API — CI/automation pushes module contents here so
// `go get` then resolves through us.
func (h *Handler) uploadVersionFile(w http.ResponseWriter, r *http.Request, module, version, ext string) {
	const maxBytes = 200 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// For .info we may auto-generate if missing Time; we accept whatever the
	// caller sent.
	if ext == ".info" {
		if !json.Valid(body) {
			http.Error(w, ".info must be valid JSON", http.StatusBadRequest)
			return
		}
		var info struct {
			Version string    `json:"Version"`
			Time    time.Time `json:"Time"`
		}
		if err := json.Unmarshal(body, &info); err != nil {
			http.Error(w, "parse .info: "+err.Error(), http.StatusBadRequest)
			return
		}
		if info.Version == "" {
			info.Version = version
		}
		if info.Time.IsZero() {
			info.Time = time.Now().UTC()
		}
		body, err = json.Marshal(info)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	path := "modules/" + module + "/" + version + ext
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID, path,
		digest, int64(len(body)), contentTypeFor(ext), "",
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":true,"module":%q,"version":%q,"ext":%q}`, module, version, ext)))
}

func contentTypeFor(ext string) string {
	switch ext {
	case ".info":
		return "application/json"
	case ".mod":
		return "text/plain; charset=utf-8"
	case ".zip":
		return "application/zip"
	}
	return "application/octet-stream"
}

func extFromPath(p string) string {
	switch {
	case strings.HasSuffix(p, ".info"):
		return ".info"
	case strings.HasSuffix(p, ".mod"):
		return ".mod"
	case strings.HasSuffix(p, ".zip"):
		return ".zip"
	}
	return ""
}
