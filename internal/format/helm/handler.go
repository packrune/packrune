// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the Helm Chart Repository surface.
//
// Routes:
//   GET  /index.yaml                 - chart index
//   GET  /<name>-<version>.tgz       - chart download
//   POST /api/charts                 - upload (multipart "chart" field)
//   GET  /api/charts                 - list (chartmuseum-compatible JSON)
//   GET  /healthz                    - liveness

package helm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
)

// Handler serves the Helm registry surface for one Packrune helm repository.
type Handler struct {
	logger   *slog.Logger
	repoID   string
	repoKind string
	proxy    ProxyConfig
	backend  storage.Backend
	cas      *cas.CAS
	store    *repo.Store
	// ContextPath is reported in /index.yaml's serverInfo so clients fetching
	// the index get the right base URL for charts.
	contextPath string
}

// HandlerConfig is the dependency bundle passed to NewHandler.
type HandlerConfig struct {
	Logger      *slog.Logger
	Repo        repo.Repository
	Backend     storage.Backend
	CAS         *cas.CAS
	Store       *repo.Store
	ContextPath string // e.g. "/helm"
}

// NewHandler constructs a Handler bound to one Packrune helm repository.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:      cfg.Logger,
		repoID:      cfg.Repo.ID,
		repoKind:    string(cfg.Repo.Kind),
		proxy:       ParseProxyConfig(cfg.Repo.Config),
		backend:     cfg.Backend,
		cas:         cfg.CAS,
		store:       cfg.Store,
		contextPath: cfg.ContextPath,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case p == "" || p == "index.yaml":
		h.handleIndex(w, r)
	case p == "healthz":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	case p == "api/charts" && r.Method == http.MethodPost:
		h.handleUpload(w, r)
	case p == "api/charts" && r.Method == http.MethodGet:
		h.handleList(w, r)
	case strings.HasSuffix(p, ".tgz"):
		h.handleDownload(w, r, p)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	arts, err := h.store.ResolveArtifactsByPrefix(r.Context(), h.repoID, "charts/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err := buildIndex(arts, h.contextPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request, filename string) {
	// filename is "<name>-<version>.tgz"; find the artifact whose path ends with it.
	arts, err := h.store.ResolveArtifactsByPrefix(r.Context(), h.repoID, "charts/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var match *repo.Artifact
	for i := range arts {
		if strings.HasSuffix(arts[i].Path, "/"+filename) {
			match = &arts[i]
			break
		}
	}
	if match == nil {
		if h.repoKind == "proxy" {
			body, perr := h.proxyFetchChart(r.Context(), filename)
			if perr == nil {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				_, _ = w.Write(body)
				return
			}
		}
		http.NotFound(w, r)
		return
	}
	rc, st, err := h.cas.Get(r.Context(), match.Digest)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	_, _ = io.Copy(w, rc)
}

// handleUpload accepts a multipart "chart" field containing the .tgz bytes.
// Compatible with the helm cm-push plugin's POST /api/charts.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	const maxChart = 64 << 20 // 64 MiB
	if err := r.ParseMultipartForm(maxChart); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("chart")
	if err != nil {
		http.Error(w, "missing 'chart' field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, maxChart))
	if err != nil {
		http.Error(w, "read chart: "+err.Error(), http.StatusBadRequest)
		return
	}

	meta, err := parseChart(bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "store chart: "+err.Error(), http.StatusInternalServerError)
		return
	}

	metaJSON, _ := json.Marshal(meta)
	chartPath := "charts/" + meta.Name + "/" + meta.Name + "-" + meta.Version + ".tgz"
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID, chartPath,
		digest, int64(len(body)), "application/octet-stream", string(metaJSON),
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(struct {
		Saved bool `json:"saved"`
	}{Saved: true})
}

// handleList: GET /api/charts — chartmuseum-compatible {name: [entry,...]}.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	arts, err := h.store.ResolveArtifactsByPrefix(r.Context(), h.repoID, "charts/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := map[string][]chartMetadata{}
	for _, a := range arts {
		var meta chartMetadata
		if a.Metadata == "" {
			continue
		}
		if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
			continue
		}
		out[meta.Name] = append(out[meta.Name], meta)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
