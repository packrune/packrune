// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package npm implements the npm registry HTTP API on top of Packrune's
// storage and metadata layers. Clients point their npm config at
//
//	npm config set registry http://your-packrune-host/npm/
//
// and `npm install` / `npm publish` work as if it were registry.npmjs.org.
//
// Architecture:
//   - Packuments stored as artifact rows at path "packuments/<pkg>".
//   - Tarballs stored in CAS keyed by sha256; image-binding row at path
//     "tarballs/<pkg>/<filename>".
//   - Publish merges the new packument into the stored one, never
//     overwriting an existing version.
package npm

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

const formatName = "npm"

type formatImpl struct{}

func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string        { return formatName }
func (formatImpl) DisplayName() string { return "npm" }

// The npm format mounts a custom Handler at /npm in serve.go, so the format
// interface Routes hook is unused.
func (formatImpl) Routes(chi.Router) {}

func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
