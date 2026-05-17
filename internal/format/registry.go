// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Compile-time format registry. Each format sub-package calls Register from an
// init() function so the main binary only needs to import the format package
// for its side effects.

package format

import (
	"fmt"
	"sort"
	"sync"
)

var (
	regMu      sync.RWMutex
	registered = map[string]Format{}
)

// Register makes a Format available by name. Panics on duplicate registration,
// which is appropriate because duplicates are programmer errors discoverable at
// startup, not runtime.
func Register(f Format) {
	regMu.Lock()
	defer regMu.Unlock()
	name := f.Name()
	if name == "" {
		panic("format: Register called with empty name")
	}
	if _, exists := registered[name]; exists {
		panic(fmt.Sprintf("format: duplicate registration for %q", name))
	}
	registered[name] = f
}

// Lookup returns the Format registered under name, or false.
func Lookup(name string) (Format, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	f, ok := registered[name]
	return f, ok
}

// All returns every registered Format, sorted by name. The slice is a copy and
// safe to mutate.
func All() []Format {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Format, 0, len(registered))
	names := make([]string, 0, len(registered))
	for n := range registered {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, registered[n])
	}
	return out
}
