// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package gomod implements the Go module proxy protocol on top of Packrune's
// storage and metadata. Clients use it with:
//
//	GOPROXY=http://your-packrune-host/go,direct
//
// We accept module version uploads via a small PUT API so operators can push
// module contents from CI without depending on git; standard `go get` then
// resolves and downloads through us like any GOPROXY.
package gomod

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

const formatName = "gomod"

type formatImpl struct{}

func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string                                                 { return formatName }
func (formatImpl) DisplayName() string                                          { return "Go modules" }
func (formatImpl) Routes(chi.Router)                                            {}
func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
