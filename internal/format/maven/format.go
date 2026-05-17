// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package maven implements a Maven 2 repository compatible with the `mvn`
// CLI. Clients use it via:
//
//   <repository>
//     <id>packrune</id>
//     <url>http://your-packrune-host/maven/</url>
//   </repository>
//
// The Maven layout is documented at
// https://maven.apache.org/repository/layout.html. We accept arbitrary PUTs
// under the layout root, auto-regenerate maven-metadata.xml on jar uploads,
// and serve every artifact + checksum file back.
package maven

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/format"
)

const formatName = "maven"

type formatImpl struct{}

func init() {
	format.Register(&formatImpl{})
}

func (formatImpl) Name() string                                                 { return formatName }
func (formatImpl) DisplayName() string                                          { return "Maven" }
func (formatImpl) Routes(chi.Router)                                            {}
func (formatImpl) OnUpload(context.Context, format.Repo, format.Artifact) error { return nil }
func (formatImpl) OnDelete(context.Context, format.Repo, string) error          { return nil }
func (formatImpl) Index(context.Context, format.Repo) error                     { return nil }
