// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package cas implements a content-addressable storage layer that sits above
// any storage.Backend. Objects are keyed by SHA-256 digest, so identical bytes
// are stored exactly once across every format and every repository. Repository
// "paths" map to digests through a separate metadata table (kept by the
// caller in the database).
//
// Why this matters: when the same alpine:3.20 layer is referenced by every
// project's CI Dockerfile, traditional registries store it once per repo.
// Packrune stores it once, period.
package cas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/packrune/packrune/internal/storage"
)

// CAS wraps an underlying storage.Backend and exposes a digest-keyed API.
type CAS struct {
	b storage.Backend
}

// New wraps b in a content-addressable layer.
func New(b storage.Backend) *CAS {
	return &CAS{b: b}
}

// Put streams body into a temp buffer (well, the backend's own write path),
// computing the SHA-256 as it goes, then stores the result at the digest
// key. Returns the digest in "sha256:<hex>" form and the byte count written.
//
// Note: we don't double-buffer. We tee the read into a hasher and let the
// backend handle the actual write. The backend's atomic rename ensures we
// never see a partial digest-keyed object.
func (c *CAS) Put(ctx context.Context, body io.Reader) (string, int64, error) {
	h := sha256.New()
	tr := io.TeeReader(body, h)

	// We need the digest BEFORE we know the final key — but the backend wants
	// the key up front. Two passes are unavoidable without a sized buffer, so:
	// (1) write to a temp key, (2) rename via copy+delete to the digest key.
	// For local FS this is cheap; for S3 we'll override CAS.Put with a real
	// multipart-upload-then-rename later.
	tempKey := tempKeyFor()
	n, err := c.b.Put(ctx, tempKey, tr)
	if err != nil {
		return "", 0, fmt.Errorf("cas: temp put: %w", err)
	}
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	finalKey := KeyFor(digest)

	// If we already have this content under the digest key, the temp upload
	// was redundant. Drop it and return the existing digest.
	if _, err := c.b.Stat(ctx, finalKey); err == nil {
		_ = c.b.Delete(ctx, tempKey)
		return digest, n, nil
	}

	// Copy temp -> final, then delete temp. Cheap for FS, fine for now on S3.
	r, _, err := c.b.Get(ctx, tempKey)
	if err != nil {
		return "", 0, fmt.Errorf("cas: reopen temp: %w", err)
	}
	defer r.Close()
	if _, err := c.b.Put(ctx, finalKey, r); err != nil {
		return "", 0, fmt.Errorf("cas: rename to digest: %w", err)
	}
	_ = c.b.Delete(ctx, tempKey)
	return digest, n, nil
}

// Get opens the object at the given digest for reading.
func (c *CAS) Get(ctx context.Context, digest string) (io.ReadCloser, storage.Stat, error) {
	if err := validateDigest(digest); err != nil {
		return nil, storage.Stat{}, err
	}
	return c.b.Get(ctx, KeyFor(digest))
}

// Stat returns metadata for the object at digest.
func (c *CAS) Stat(ctx context.Context, digest string) (storage.Stat, error) {
	if err := validateDigest(digest); err != nil {
		return storage.Stat{}, err
	}
	return c.b.Stat(ctx, KeyFor(digest))
}

// Delete removes the object at digest. Use with care — multiple references
// to the same digest are normal in a CAS world, so this should normally only
// be called by the garbage collector after reference counting.
func (c *CAS) Delete(ctx context.Context, digest string) error {
	if err := validateDigest(digest); err != nil {
		return err
	}
	return c.b.Delete(ctx, KeyFor(digest))
}

// KeyFor returns the storage key for a digest. It fans out the first byte of
// the hex so we don't end up with millions of files in a single directory.
//
//	"sha256:abcdef..." -> "blobs/sha256/ab/abcdef..."
func KeyFor(digest string) string {
	if i := strings.Index(digest, ":"); i >= 0 {
		alg, hex := digest[:i], digest[i+1:]
		if len(hex) >= 2 {
			return "blobs/" + alg + "/" + hex[:2] + "/" + hex
		}
	}
	return "blobs/unknown/" + digest
}

func validateDigest(d string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(d, prefix) {
		return fmt.Errorf("cas: digest %q: unsupported algorithm", d)
	}
	hexPart := d[len(prefix):]
	if len(hexPart) != 64 {
		return fmt.Errorf("cas: digest %q: wrong length", d)
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return fmt.Errorf("cas: digest %q: not hex: %w", d, err)
	}
	return nil
}

// tempKeyFor produces a random temp key under a known prefix so the GC can
// reap leftovers from crashed uploads.
func tempKeyFor() string {
	var b [16]byte
	// rand is fine here — collisions don't matter, we just need uniqueness.
	_, _ = randomRead(b[:])
	return "blobs/_uploads/" + hex.EncodeToString(b[:])
}
