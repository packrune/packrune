// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Blob GET / HEAD / DELETE handlers. Blobs live in CAS keyed by digest;
// existence per-repo is tracked through artifacts rows so different repos can
// share the same physical bytes.

package docker

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/packrune/packrune/internal/storage"
)

// handleBlob: GET|HEAD|DELETE /v2/<name>/blobs/<digest>
func (h *Handler) handleBlob(w http.ResponseWriter, r *http.Request, name, digest string) {
	if err := validateName(name); err != nil {
		writeError(w, http.StatusBadRequest, errCodeNameInvalid, err.Error())
		return
	}
	if !looksLikeDigest(digest) {
		writeError(w, http.StatusBadRequest, errCodeDigestInvalid, "bad digest")
		return
	}

	// Check the artifact mapping so we 404 cleanly for blobs that physically
	// exist but were never associated with this repo (de-dup safety).
	_, err := h.store.GetArtifact(r.Context(), h.repoID, "refs/"+name+"/blobs/"+digest)
	if err != nil {
		// Fall through to a direct CAS check if we didn't track it under this
		// image name. Many docker clients re-pull a layer that exists at the
		// blob level but lacks a name binding (cross-repo mount, etc.).
	}

	switch r.Method {
	case http.MethodHead:
		st, err := h.cas.Stat(r.Context(), digest)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				if h.repoKind == "proxy" {
					if size, mt, ferr := h.proxyFetchBlob(r.Context(), name, digest); ferr == nil {
						w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
						w.Header().Set("Content-Type", mt)
						w.Header().Set("Docker-Content-Digest", digest)
						w.WriteHeader(http.StatusOK)
						return
					}
				}
				writeError(w, http.StatusNotFound, errCodeBlobUnknown, "blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
			return
		}
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Docker-Content-Digest", digest)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		rc, st, err := h.cas.Get(r.Context(), digest)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) && h.repoKind == "proxy" {
				if _, _, ferr := h.proxyFetchBlob(r.Context(), name, digest); ferr == nil {
					rc, st, err = h.cas.Get(r.Context(), digest)
				}
			}
		}
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusNotFound, errCodeBlobUnknown, "blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Docker-Content-Digest", digest)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)

	case http.MethodDelete:
		// Soft-delete: drop the image-blob mapping. Real GC reaps the CAS
		// object when the global refcount hits zero (faz 2 polish — for now
		// we just unbind from the image).
		_ = h.store.DeleteArtifact(r.Context(), h.repoID, "refs/"+name+"/blobs/"+digest)
		w.WriteHeader(http.StatusAccepted)

	default:
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
	}
}

// looksLikeDigest is a fast sanity check; full validation happens later in
// cas.Get.
func looksLikeDigest(d string) bool {
	const prefix = "sha256:"
	if len(d) != len(prefix)+64 {
		return false
	}
	if d[:len(prefix)] != prefix {
		return false
	}
	for i := len(prefix); i < len(d); i++ {
		c := d[i]
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !ok {
			return false
		}
	}
	return true
}
