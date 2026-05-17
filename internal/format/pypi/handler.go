// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the PyPI surface. Routes (mounted at /pypi):
//
//   GET  /simple/                       - all packages HTML (PEP 503)
//   GET  /simple/<pkg>/                 - one package's files HTML (PEP 503)
//   GET  /packages/<pkg>/<filename>     - file download
//   POST /                              - twine upload (multipart form)

package pypi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
)

type Handler struct {
	logger   *slog.Logger
	repoID   string
	repoKind string
	proxy    ProxyConfig
	backend  storage.Backend
	cas      *cas.CAS
	store    *repo.Store
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
	p := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case p == "" && r.Method == http.MethodPost:
		h.handleUpload(w, r)
	case p == "" || p == "simple/" || p == "simple":
		h.handleIndex(w, r)
	case strings.HasPrefix(p, "simple/"):
		rest := strings.TrimPrefix(p, "simple/")
		rest = strings.TrimSuffix(rest, "/")
		if rest == "" {
			h.handleIndex(w, r)
			return
		}
		h.handlePackage(w, r, rest)
	case strings.HasPrefix(p, "packages/"):
		rest := strings.TrimPrefix(p, "packages/")
		i := strings.Index(rest, "/")
		if i < 0 {
			http.NotFound(w, r)
			return
		}
		h.handleDownload(w, r, rest[:i], rest[i+1:])
	default:
		http.NotFound(w, r)
	}
}

// handleIndex: PEP 503 listing of every known package.
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	names, err := h.allPackageNames(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Strings(names)

	if wantJSON(r) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta":     map[string]string{"api-version": "1.0"},
			"projects": mapEach(names, func(n string) map[string]string { return map[string]string{"name": n} }),
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, "<!DOCTYPE html><html><body>\n")
	for _, n := range names {
		_, _ = fmt.Fprintf(w, `<a href="%s/">%s</a><br/>`+"\n", html.EscapeString(n), html.EscapeString(n))
	}
	_, _ = fmt.Fprint(w, "</body></html>\n")
}

// handlePackage: PEP 503 listing of one package's files.
func (h *Handler) handlePackage(w http.ResponseWriter, r *http.Request, name string) {
	norm := normalize(name)
	arts, err := h.store.ResolveArtifactsByPrefix(r.Context(), h.repoID, "packages/"+norm+"/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(arts) == 0 {
		if h.repoKind == "proxy" {
			html, perr := h.proxyFetchSimple(r.Context(), norm)
			if perr == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(html))
				return
			}
		}
		http.NotFound(w, r)
		return
	}

	if wantJSON(r) {
		files := make([]map[string]any, 0, len(arts))
		for _, a := range arts {
			filename := pathBase(a.Path)
			files = append(files, map[string]any{
				"filename": filename,
				"url":      "../../packages/" + norm + "/" + filename,
				"hashes":   map[string]string{"sha256": strings.TrimPrefix(a.Digest, "sha256:")},
			})
		}
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta":  map[string]string{"api-version": "1.0"},
			"name":  norm,
			"files": files,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!DOCTYPE html><html><body>\n")
	for _, a := range arts {
		filename := pathBase(a.Path)
		_, _ = fmt.Fprintf(w,
			`<a href="../../packages/%s/%s#sha256=%s">%s</a><br/>`+"\n",
			html.EscapeString(norm), html.EscapeString(filename),
			html.EscapeString(strings.TrimPrefix(a.Digest, "sha256:")),
			html.EscapeString(filename),
		)
	}
	_, _ = fmt.Fprint(w, "</body></html>\n")
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request, pkg, filename string) {
	art, err := h.store.GetArtifact(r.Context(), h.repoID, "packages/"+pkg+"/"+filename)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Proxy mode: bytes may not be cached yet (size=0 stub). Pull them now.
	if h.repoKind == "proxy" && art.Size == 0 {
		body, perr := h.proxyFetchFile(r.Context(), pkg, filename)
		if perr == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			if r.Method != http.MethodHead {
				_, _ = w.Write(body)
			}
			return
		}
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
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, rc)
	}
}

// handleUpload: twine POST. Multipart form with fields including ":action",
// "name", "version", "content" (file). We store under packages/<norm>/<filename>.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 200 << 20
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "missing 'name' field", http.StatusBadRequest)
		return
	}
	version := r.FormValue("version")
	file, fh, err := r.FormFile("content")
	if err != nil {
		http.Error(w, "missing 'content' field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		http.Error(w, "read content: "+err.Error(), http.StatusBadRequest)
		return
	}

	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	norm := normalize(name)
	meta, _ := json.Marshal(map[string]string{
		"name":    name,
		"version": version,
	})
	path := "packages/" + norm + "/" + fh.Filename
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID, path, digest, int64(len(body)),
		"application/octet-stream", string(meta),
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) allPackageNames(ctx context.Context) ([]string, error) {
	arts, err := h.store.ListArtifactsByPrefix(ctx, h.repoID, "packages/")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range arts {
		rest := strings.TrimPrefix(a.Path, "packages/")
		if i := strings.Index(rest, "/"); i > 0 {
			n := rest[:i]
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// normalize implements PEP 503 name normalization.
var normalizeRE = regexp.MustCompile(`[-_.]+`)

func normalize(name string) string {
	return normalizeRE.ReplaceAllString(strings.ToLower(name), "-")
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func wantJSON(r *http.Request) bool {
	a := r.Header.Get("Accept")
	return strings.Contains(a, "application/vnd.pypi.simple.v1+json") ||
		strings.Contains(a, "application/json")
}

func mapEach[T any, R any](in []T, f func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
