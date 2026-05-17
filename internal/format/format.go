// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package format defines the contract every package format (Docker, npm,
// Helm, Go modules, PyPI, Maven, ...) implements. The core server speaks to
// formats only through this interface; adding a new format means creating a
// new sub-package and registering it.
//
// This interface is the single most important type in the codebase. Resist
// the urge to grow it. Every new method here is paid for by every format
// that ever exists.
package format

import (
	"context"
	"io"

	"github.com/go-chi/chi/v5"
)

// Format is the contract a package format implements.
type Format interface {
	// Name returns the canonical short name ("docker", "npm", "helm", ...).
	Name() string

	// DisplayName returns the human-friendly name shown in the UI.
	DisplayName() string

	// Routes mounts the format-specific HTTP handlers onto r. The caller owns
	// the path prefix; the format wires its protocol-specified paths beneath
	// that.
	Routes(r chi.Router)

	// OnUpload is invoked after a successful artifact write so the format can
	// schedule any format-specific index work (e.g. regenerate
	// maven-metadata.xml, update index.yaml, refresh npm packument).
	//
	// Implementations must be quick. Heavy work belongs in a background job
	// kicked off here.
	OnUpload(ctx context.Context, repo Repo, artifact Artifact) error

	// OnDelete is invoked after a successful artifact deletion.
	OnDelete(ctx context.Context, repo Repo, ref string) error

	// Index rebuilds the format-specific index from authoritative state.
	// Called by the indexer worker on demand (e.g. after a storage repair).
	Index(ctx context.Context, repo Repo) error
}

// Repo is the format-facing view of a repository. Formats only need to
// read/write blobs and enumerate paths; they don't need to know whether the
// repository is hosted, proxied, or grouped.
type Repo interface {
	ID() string
	Name() string
	Format() string
	Kind() string // "hosted" | "proxy" | "group"

	// Read opens the blob at the given logical path for reading.
	Read(ctx context.Context, path string) (io.ReadCloser, int64, error)

	// Write stores the body at path, returning the SHA-256 digest and byte
	// count.
	Write(ctx context.Context, path string, body io.Reader) (digest string, n int64, err error)

	// Delete removes the blob at path.
	Delete(ctx context.Context, path string) error

	// List enumerates blob paths beginning with prefix.
	List(ctx context.Context, prefix string) ([]string, error)
}

// Artifact is the metadata produced by an upload.
type Artifact struct {
	Path      string
	Digest    string
	Size      int64
	MediaType string
}
