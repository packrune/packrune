// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package migrations holds the embedded SQL migrations for Packrune.
//
// Keeping the migrations at the repository root (under /migrations) is the
// conventional location and is compatible with golang-migrate's CLI for
// out-of-band operations. The Go file in this package exists solely so the
// embed directive can reach the .sql files without violating Go's
// "//go:embed only sees its own directory or below" rule.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

// FS returns an fs.FS containing every migration file.
func FS() fs.FS { return files }
