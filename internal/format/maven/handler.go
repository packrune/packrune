// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP entrypoint for the Maven 2 repository.

package maven

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

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
	if p == "" {
		http.Error(w, "Maven repository root", http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.serveFile(w, r, p)
	case http.MethodPut:
		h.uploadFile(w, r, p)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, p string) {
	art, err := h.store.ResolveArtifact(r.Context(), h.repoID, "files/"+p)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) && h.repoKind == "proxy" {
			body, perr := h.proxyFetchFile(r.Context(), p)
			if perr == nil {
				w.Header().Set("Content-Type", contentTypeFor(p))
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				if r.Method != http.MethodHead {
					_, _ = w.Write(body)
				}
				return
			}
		}
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
	w.Header().Set("Content-Type", contentTypeFor(p))
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, rc)
	}
}

func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request, p string) {
	const maxBytes = 256 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID, "files/"+p, digest, int64(len(body)),
		contentTypeFor(p), "",
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// On primary artifact uploads (.jar, .pom), auto-generate checksum
	// siblings if the client did not push them, and regenerate
	// maven-metadata.xml at the artifactId level.
	if isPrimaryArtifact(p) {
		if err := h.ensureChecksums(r, p, body); err != nil {
			h.logger.Warn("checksum write failed", "path", p, "err", err)
		}
		if err := h.regenerateMetadata(r, p); err != nil {
			h.logger.Warn("metadata regen failed", "path", p, "err", err)
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// ensureChecksums writes sha1, sha256, md5 sidecar files unless they already
// exist (clients sometimes push them explicitly).
func (h *Handler) ensureChecksums(r *http.Request, p string, body []byte) error {
	checks := map[string]func([]byte) string{
		".sha1":   func(b []byte) string { s := sha1.Sum(b); return hex.EncodeToString(s[:]) },
		".sha256": func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) },
		".md5":    func(b []byte) string { s := md5.Sum(b); return hex.EncodeToString(s[:]) },
	}
	for ext, fn := range checks {
		sidecar := p + ext
		if _, err := h.store.GetArtifact(r.Context(), h.repoID, "files/"+sidecar); err == nil {
			continue
		}
		content := []byte(fn(body))
		digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(content))
		if err != nil {
			return err
		}
		if err := h.store.UpsertArtifact(
			r.Context(), h.repoID, "files/"+sidecar, digest,
			int64(len(content)), "text/plain", "",
		); err != nil {
			return err
		}
	}
	return nil
}

// regenerateMetadata walks every uploaded artifact for this (groupId,
// artifactId) and rewrites maven-metadata.xml.
func (h *Handler) regenerateMetadata(r *http.Request, p string) error {
	groupPath, artifactID, _ := splitArtifactPath(p)
	if groupPath == "" || artifactID == "" {
		return nil
	}
	prefix := "files/" + groupPath + "/" + artifactID + "/"
	arts, err := h.store.ListArtifactsByPrefix(r.Context(), h.repoID, prefix)
	if err != nil {
		return err
	}
	verSet := map[string]bool{}
	for _, a := range arts {
		// path: files/<group>/<artifactId>/<version>/<artifactId>-<version>.<ext>
		rest := strings.TrimPrefix(a.Path, prefix)
		if i := strings.Index(rest, "/"); i > 0 {
			verSet[rest[:i]] = true
		}
	}
	if len(verSet) == 0 {
		return nil
	}
	versions := make([]string, 0, len(verSet))
	for v := range verSet {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	meta := metadata{
		GroupID:    strings.ReplaceAll(groupPath, "/", "."),
		ArtifactID: artifactID,
		Versioning: metadataVersioning{
			Latest:      versions[len(versions)-1],
			Release:     versions[len(versions)-1],
			Versions:    metadataVersions{Version: versions},
			LastUpdated: time.Now().UTC().Format("20060102150405"),
		},
	}
	body, err := xml.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	body = append([]byte(xml.Header), body...)

	digest, _, err := h.cas.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	metaPath := "files/" + groupPath + "/" + artifactID + "/maven-metadata.xml"
	return h.store.UpsertArtifact(
		r.Context(), h.repoID, metaPath, digest, int64(len(body)),
		"application/xml", "",
	)
}

// splitArtifactPath returns (groupPath, artifactId, version) parsed from
// "g/r/o/u/p/artifactId/version/filename".
func splitArtifactPath(p string) (groupPath, artifactID, version string) {
	parts := strings.Split(p, "/")
	if len(parts) < 4 {
		return "", "", ""
	}
	// filename is parts[len-1]; version is parts[len-2]; artifactId is parts[len-3];
	// everything before is the groupId path.
	version = parts[len(parts)-2]
	artifactID = parts[len(parts)-3]
	groupPath = strings.Join(parts[:len(parts)-3], "/")
	return groupPath, artifactID, version
}

func isPrimaryArtifact(p string) bool {
	base := path.Base(p)
	for _, suf := range []string{".jar", ".pom", ".war", ".aar", ".module"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	return false
}

func contentTypeFor(p string) string {
	switch {
	case strings.HasSuffix(p, ".xml"):
		return "application/xml"
	case strings.HasSuffix(p, ".pom"):
		return "application/xml"
	case strings.HasSuffix(p, ".sha1"), strings.HasSuffix(p, ".sha256"), strings.HasSuffix(p, ".md5"):
		return "text/plain"
	case strings.HasSuffix(p, ".jar"), strings.HasSuffix(p, ".war"), strings.HasSuffix(p, ".aar"):
		return "application/java-archive"
	}
	return "application/octet-stream"
}

// metadata is the XML shape of maven-metadata.xml at the artifactId level.
type metadata struct {
	XMLName    xml.Name           `xml:"metadata"`
	GroupID    string             `xml:"groupId"`
	ArtifactID string             `xml:"artifactId"`
	Versioning metadataVersioning `xml:"versioning"`
}

type metadataVersioning struct {
	Latest      string           `xml:"latest"`
	Release     string           `xml:"release"`
	Versions    metadataVersions `xml:"versions"`
	LastUpdated string           `xml:"lastUpdated"`
}

type metadataVersions struct {
	Version []string `xml:"version"`
}

// Sanity helper for tests.
var _ = fmt.Sprintf
