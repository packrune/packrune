// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package storage defines the blob-storage abstraction used by every format
// adapter. Backends live under sub-packages (fs, s3, gcs, ...). A
// content-addressable layer sits above any backend in package cas.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by Backend implementations when the requested key
// has no object. Callers should check with errors.Is.
var ErrNotFound = errors.New("storage: object not found")

// Backend is the contract every blob backend implements.
//
// Implementations MUST stream both reads and writes. They MUST NOT buffer
// arbitrarily large objects in memory.
type Backend interface {
	// Name returns the canonical name of the backend ("fs", "s3", ...).
	Name() string

	// Get opens an object for reading. The caller must Close the returned
	// reader. ErrNotFound is returned if the key is absent.
	Get(ctx context.Context, key string) (io.ReadCloser, Stat, error)

	// Put writes body to key, replacing any prior object. Returns the byte
	// count actually written.
	Put(ctx context.Context, key string, body io.Reader) (int64, error)

	// Append writes body at the end of the object at key, creating the object
	// if it does not exist. Used by chunked upload paths (Docker registry).
	// Implementations MUST stream body — do not buffer.
	Append(ctx context.Context, key string, body io.Reader) (int64, error)

	// Stat returns metadata for an object. ErrNotFound if absent.
	Stat(ctx context.Context, key string) (Stat, error)

	// Delete removes an object. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error

	// List enumerates keys beginning with prefix. Implementations should
	// paginate internally; pageToken is opaque and round-trips.
	List(ctx context.Context, prefix, pageToken string, limit int) (ListResult, error)
}

// Stat holds object metadata.
type Stat struct {
	Key       string
	Size      int64
	ModTime   time.Time
	MediaType string
	// ETag, when set, is an opaque value that changes whenever the object
	// content changes. Used for cache validation.
	ETag string
}

// ListResult is one page of List output.
type ListResult struct {
	Keys      []string
	NextToken string // empty if no more pages
}
