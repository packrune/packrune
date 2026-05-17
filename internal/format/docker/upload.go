// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Blob upload session management.
//
// Docker pushes a blob in three legs:
//   1. POST /v2/<name>/blobs/uploads/        -> returns Location with <uuid>
//   2. PATCH /v2/<name>/blobs/uploads/<uuid> -> appends bytes (0..N times)
//   3. PUT /v2/<name>/blobs/uploads/<uuid>?digest=sha256:... -> finalize
//
// Sessions are flat temp files on local disk. When the session completes we
// hash, move into the CAS (storage), and delete the temp file.

package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/google/uuid"
)

// uploadManager owns the on-disk staging area for in-progress uploads.
type uploadManager struct {
	root string

	mu       sync.Mutex
	sessions map[string]*uploadSession
}

type uploadSession struct {
	id   string
	path string
	size int64
}

func newUploadManager(root string) *uploadManager {
	if err := os.MkdirAll(root, 0o755); err != nil {
		// Failing to create the upload dir is fatal; log and move on. We let
		// the first real upload surface the error.
		_ = err
	}
	return &uploadManager{
		root:     root,
		sessions: map[string]*uploadSession{},
	}
}

// start creates a new upload session and returns its ID.
func (m *uploadManager) start() (*uploadSession, error) {
	id := uuid.NewString()
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return nil, fmt.Errorf("upload: mkdir root: %w", err)
	}
	p := filepath.Join(m.root, id)
	f, err := os.Create(p)
	if err != nil {
		return nil, fmt.Errorf("upload: create %s: %w", id, err)
	}
	_ = f.Close()
	s := &uploadSession{id: id, path: p, size: 0}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// get returns an existing session.
func (m *uploadManager) get(id string) (*uploadSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// drop removes a session from the manager and deletes its temp file.
func (m *uploadManager) drop(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if ok {
		_ = os.Remove(s.path)
	}
}

// append writes body to the session, returning the new total size.
func (s *uploadSession) append(body io.Reader) (int64, error) {
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("upload: open append %s: %w", s.id, err)
	}
	defer f.Close()
	n, err := io.Copy(f, body)
	s.size += n
	if err != nil {
		return s.size, fmt.Errorf("upload: append %s: %w", s.id, err)
	}
	return s.size, nil
}

// readAll opens the staged content for hashing/streaming on finalize.
func (s *uploadSession) readAll() (*os.File, error) {
	return os.Open(s.path)
}

// hashFile computes the sha256 of the file at path.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), n, nil
}

// handleUploadStart: POST /v2/<name>/blobs/uploads/  -> 202 Accepted with
// Location header pointing at the new session.
func (h *Handler) handleUploadStart(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
		return
	}
	if err := validateName(name); err != nil {
		writeError(w, http.StatusBadRequest, errCodeNameInvalid, err.Error())
		return
	}

	// Monolithic upload: POST /v2/<name>/blobs/uploads/?digest=sha256:... with
	// the body containing the entire blob. We still allocate a session under
	// the hood, fold the body in, then finalize.
	digest := r.URL.Query().Get("digest")
	sess, err := h.uploads.start()
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
		return
	}

	if digest != "" && r.ContentLength != 0 {
		if _, err := sess.append(r.Body); err != nil {
			h.uploads.drop(sess.id)
			writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
			return
		}
		if err := h.finalize(r, name, sess, digest); err != nil {
			h.uploads.drop(sess.id)
			writeError(w, http.StatusBadRequest, errCodeDigestInvalid, err.Error())
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		w.WriteHeader(http.StatusCreated)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, sess.id))
	w.Header().Set("Docker-Upload-UUID", sess.id)
	w.Header().Set("Range", "0-0")
	w.WriteHeader(http.StatusAccepted)
}

// handleUploadSession: PATCH (append) | PUT (finalize) | GET (status) | DELETE (cancel)
func (h *Handler) handleUploadSession(w http.ResponseWriter, r *http.Request, name, sessionID string) {
	sess, ok := h.uploads.get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, errCodeBlobUploadUnknown, "upload session not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Docker-Upload-UUID", sess.id)
		w.Header().Set("Range", fmt.Sprintf("0-%d", sess.size-1))
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPatch:
		n, err := sess.append(r.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, sess.id))
		w.Header().Set("Docker-Upload-UUID", sess.id)
		w.Header().Set("Range", fmt.Sprintf("0-%d", n-1))
		w.WriteHeader(http.StatusAccepted)

	case http.MethodPut:
		// Optional final chunk attached to PUT.
		if r.ContentLength > 0 {
			if _, err := sess.append(r.Body); err != nil {
				writeError(w, http.StatusInternalServerError, errCodeUnsupported, err.Error())
				return
			}
		}
		digest := r.URL.Query().Get("digest")
		if digest == "" {
			writeError(w, http.StatusBadRequest, errCodeDigestInvalid, "missing digest query parameter")
			return
		}
		if err := h.finalize(r, name, sess, digest); err != nil {
			writeError(w, http.StatusBadRequest, errCodeDigestInvalid, err.Error())
			return
		}
		h.uploads.drop(sess.id)
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		h.uploads.drop(sess.id)
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, errCodeUnsupported, "method not allowed")
	}
}

// finalize verifies the computed digest matches the client-claimed one, then
// stores the blob in the CAS-backed storage and records it as an artifact.
func (h *Handler) finalize(r *http.Request, name string, sess *uploadSession, claimedDigest string) error {
	actual, size, err := hashFile(sess.path)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	if actual != claimedDigest {
		return fmt.Errorf("digest mismatch: client=%s actual=%s", claimedDigest, actual)
	}
	f, err := sess.readAll()
	if err != nil {
		return fmt.Errorf("reopen temp: %w", err)
	}
	defer f.Close()

	if _, _, err := h.cas.Put(r.Context(), f); err != nil {
		return fmt.Errorf("cas put: %w", err)
	}

	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID,
		"blobs/"+actual, actual, size, "application/octet-stream", "",
	); err != nil {
		return fmt.Errorf("record artifact: %w", err)
	}
	// Also record the per-image association so /tags/list and GC can find it.
	if err := h.store.UpsertArtifact(
		r.Context(), h.repoID,
		"refs/"+name+"/blobs/"+actual, actual, size, "application/octet-stream", "",
	); err != nil {
		return fmt.Errorf("record image artifact: %w", err)
	}
	return nil
}

// validateName is a permissive sanity check on the Docker image name segment.
// Spec: lowercase alnum, [._-], and '/' as a separator.
func validateName(name string) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '/':
		default:
			return fmt.Errorf("invalid character %q in name", r)
		}
	}
	return nil
}

// parseContentLength is here so tests can inject sizes; production code uses
// the standard library directly.
func parseContentLength(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}
