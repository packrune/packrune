// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Manifest GET / HEAD / PUT / DELETE handlers and the tag-list endpoint.
//
// A manifest is a JSON document referenced either by tag or by digest. We
// store the bytes in CAS like any other blob; we also record two artifact
// rows per push:
//
//   refs/<image>/manifests/<tag>     -> digest    (mutable tag -> manifest)
//   refs/<image>/manifests/<digest>  -> digest    (immutable digest -> self)
//
// Tag listing reads the first row group.

package docker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage"
)

// handleManifest dispatches the manifest verb.
func (h *Handler) handleManifest(w http.ResponseWriter, r *http.Request, name, reference string) {
	if err := validateName(name); err != nil {
		writeError(w, http.StatusBadRequest, errCodeNameInvalid, err.Error())
		return
	}

	switch r.Method {
	case http.MethodHead, http.MethodGet:
		h.serveManifest(w, r, name, reference, r.Method == http.MethodHead)
	case http.MethodPut:
		h.uploadManifest(w, r, name, reference)
	case http.MethodDelete:
		h.deleteManifest(w, r, name, reference)
	default:
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
	}
}

func (h *Handler) serveManifest(w http.ResponseWriter, r *http.Request, name, reference string, headOnly bool) {
	path := manifestPath(name, reference)
	art, err := h.store.GetArtifact(r.Context(), h.repoID, path)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, errCodeManifestUnknown, "manifest not found")
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}

	rc, st, err := h.cas.Get(r.Context(), art.Digest)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, errCodeManifestUnknown, "manifest blob missing")
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}
	defer rc.Close()

	mediaType := art.MediaType
	if mediaType == "" {
		mediaType = "application/vnd.oci.image.manifest.v1+json"
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", art.Digest)
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	w.WriteHeader(http.StatusOK)
	if !headOnly {
		_, _ = io.Copy(w, rc)
	}
}

func (h *Handler) uploadManifest(w http.ResponseWriter, r *http.Request, name, reference string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20)) // 4 MiB cap on manifests
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeManifestInvalid, "read body: "+err.Error())
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, errCodeManifestInvalid, "not valid JSON")
		return
	}

	sum := sha256.Sum256(body)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	if _, _, err := h.cas.Put(r.Context(), bytes.NewReader(body)); err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, "store manifest: "+err.Error())
		return
	}

	mediaType := r.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/vnd.oci.image.manifest.v1+json"
	}

	// Tag binding (mutable).
	if !strings.HasPrefix(reference, "sha256:") {
		tagPath := manifestPath(name, reference)
		if err := h.store.UpsertArtifact(r.Context(), h.repoID, tagPath, digest, int64(len(body)), mediaType, ""); err != nil {
			writeError(w, http.StatusInternalServerError, errCodeUnsupported, "record tag: "+err.Error())
			return
		}
	}
	// Digest binding (immutable, always written so a digest-referenced pull
	// works even when the manifest was pushed by tag only).
	digPath := manifestPath(name, digest)
	if err := h.store.UpsertArtifact(r.Context(), h.repoID, digPath, digest, int64(len(body)), mediaType, ""); err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, "record digest: "+err.Error())
		return
	}

	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Location", "/v2/"+name+"/manifests/"+digest)
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) deleteManifest(w http.ResponseWriter, r *http.Request, name, reference string) {
	path := manifestPath(name, reference)
	if err := h.store.DeleteArtifact(r.Context(), h.repoID, path); err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleTagsList: GET /v2/<name>/tags/list
func (h *Handler) handleTagsList(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
		return
	}
	if err := validateName(name); err != nil {
		writeError(w, http.StatusBadRequest, errCodeNameInvalid, err.Error())
		return
	}

	prefix := "refs/" + name + "/manifests/"
	arts, err := h.store.ListArtifactsByPrefix(r.Context(), h.repoID, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}
	var tags []string
	for _, a := range arts {
		ref := strings.TrimPrefix(a.Path, prefix)
		if strings.HasPrefix(ref, "sha256:") {
			continue // skip digest bindings; only tags are listed
		}
		tags = append(tags, ref)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{Name: name, Tags: tags})
}

// handleCatalog: GET /v2/_catalog — list of image names known to this repo.
func (h *Handler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
		return
	}
	arts, err := h.store.ListArtifactsByPrefix(r.Context(), h.repoID, "refs/")
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}
	seen := map[string]bool{}
	var names []string
	for _, a := range arts {
		// path shape: refs/<name>/(manifests|blobs)/<ref>
		s := strings.TrimPrefix(a.Path, "refs/")
		if i := strings.LastIndex(s, "/manifests/"); i >= 0 {
			s = s[:i]
		} else if i := strings.LastIndex(s, "/blobs/"); i >= 0 {
			s = s[:i]
		}
		if s != "" && !seen[s] {
			seen[s] = true
			names = append(names, s)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Repositories []string `json:"repositories"`
	}{Repositories: names})
}

// manifestPath returns the artifacts.path key under which a (image, ref)
// pair is recorded.
func manifestPath(name, reference string) string {
	return "refs/" + name + "/manifests/" + reference
}
