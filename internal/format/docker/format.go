// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package docker implements the Docker Registry HTTP API V2 / OCI distribution
// spec. It is registered as a Packrune format named "docker" and serves
// every container image push/pull.
//
// Architecture:
//   - Blobs flow through the storage.Backend, content-addressed in CAS.
//   - Upload sessions are temp files under <storage>/_uploads/docker/<uuid>.
//   - Manifest digests and tag mappings live in the artifacts table, keyed
//     by (repo_id, path) where path is e.g. "manifests/library/nginx:latest".
package docker

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

// formatImpl wires docker into the format.Format interface so the registry
// (compile-time) can discover it.
type formatImpl struct{}

const formatName = "docker"

// init registers the Docker format. Importing this package for its side
// effects is enough to make the format available.
func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string        { return formatName }
func (formatImpl) DisplayName() string { return "Docker / OCI" }

// Routes is intentionally empty here because the Docker registry mounts a
// custom Handler at a top-level path (`/v2`). The chi-style Routes hook in
// format.Format is for formats whose routing fits a single sub-router; Docker
// owns its own top-level URL space and is wired in cmd/packrune/serve.go.
func (formatImpl) Routes(chi.Router) {}

func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
