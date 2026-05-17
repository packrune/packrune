// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package pypi implements a Python Package Index compatible with `pip` and
// `twine`. Clients use it via:
//
//   pip install --index-url http://your-packrune-host/pypi/simple/  somepkg
//   twine upload --repository-url http://your-packrune-host/pypi/  dist/*
//
// The simple index follows PEP 503 (HTML) and PEP 691 (JSON). Uploads use the
// twine multipart form convention.
package pypi

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

const formatName = "pypi"

type formatImpl struct{}

func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string                                                 { return formatName }
func (formatImpl) DisplayName() string                                          { return "PyPI" }
func (formatImpl) Routes(chi.Router)                                            {}
func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
