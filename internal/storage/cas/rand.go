// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package cas

import "crypto/rand"

// randomRead is a tiny alias that exists only so cas.go can refer to a
// single-name function without importing crypto/rand directly into the body
// of Put (which keeps Put's import list minimal and readable).
func randomRead(b []byte) (int, error) { return rand.Read(b) }
