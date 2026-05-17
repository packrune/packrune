// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package helm implements a Helm Chart Repository at /helm. Clients add the
// repo with
//
//	helm repo add packrune http://your-packrune-host/helm/
//
// and `helm install` works against it. Uploads use the chartmuseum-style
// POST /api/charts multipart endpoint that the popular `helm cm-push` plugin
// already speaks.
//
// Architecture:
//   - .tgz chart bytes go into CAS.
//   - Each upload records an artifact row at "charts/<name>/<version>.tgz"
//     with the parsed Chart.yaml stored as JSON metadata, so /index.yaml can
//     be rebuilt without re-parsing every tarball on every request.
package helm

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

const formatName = "helm"

type formatImpl struct{}

func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string                                                 { return formatName }
func (formatImpl) DisplayName() string                                          { return "Helm" }
func (formatImpl) Routes(chi.Router)                                            {}
func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
