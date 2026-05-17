// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Go module proxy URL parsing. Module paths may contain '/' (they look like
// repo URLs) and capital letters are encoded as '!<lower>' on the wire per
// the GOPROXY spec.

package gomod

import (
	"errors"
	"strings"
)

type routeKind int

const (
	routeUnknown routeKind = iota
	routeList              // GET /<module>/@v/list
	routeLatest            // GET /<module>/@latest
	routeInfo              // GET|PUT /<module>/@v/<version>.info
	routeMod               // GET|PUT /<module>/@v/<version>.mod
	routeZip               // GET|PUT /<module>/@v/<version>.zip
)

type parsedRoute struct {
	kind    routeKind
	module  string // decoded, e.g. "github.com/Acme/Foo"
	version string // semver string, e.g. "v1.2.3"
}

// parsePath returns the route classification for a Go proxy URL.
func parsePath(p string) parsedRoute {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return parsedRoute{kind: routeUnknown}
	}

	// /<module>/@latest
	if strings.HasSuffix(p, "/@latest") {
		mod, err := decodeModule(strings.TrimSuffix(p, "/@latest"))
		if err != nil {
			return parsedRoute{kind: routeUnknown}
		}
		return parsedRoute{kind: routeLatest, module: mod}
	}

	// /<module>/@v/list
	if strings.HasSuffix(p, "/@v/list") {
		mod, err := decodeModule(strings.TrimSuffix(p, "/@v/list"))
		if err != nil {
			return parsedRoute{kind: routeUnknown}
		}
		return parsedRoute{kind: routeList, module: mod}
	}

	// /<module>/@v/<version>.<ext>
	if i := strings.LastIndex(p, "/@v/"); i > 0 {
		rest := p[i+len("/@v/"):]
		// rest is "<version>.<ext>"
		var ext string
		var kind routeKind
		switch {
		case strings.HasSuffix(rest, ".info"):
			ext, kind = ".info", routeInfo
		case strings.HasSuffix(rest, ".mod"):
			ext, kind = ".mod", routeMod
		case strings.HasSuffix(rest, ".zip"):
			ext, kind = ".zip", routeZip
		default:
			return parsedRoute{kind: routeUnknown}
		}
		version := strings.TrimSuffix(rest, ext)
		if version == "" {
			return parsedRoute{kind: routeUnknown}
		}
		mod, err := decodeModule(p[:i])
		if err != nil {
			return parsedRoute{kind: routeUnknown}
		}
		return parsedRoute{kind: kind, module: mod, version: version}
	}

	return parsedRoute{kind: routeUnknown}
}

// decodeModule converts a wire-encoded module path back to its canonical
// form. Capital letters are escaped as '!<lower>'; everything else is
// passed through unchanged.
func decodeModule(s string) (string, error) {
	if s == "" {
		return "", errors.New("empty module path")
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '!' {
			if i+1 >= len(s) {
				return "", errors.New("trailing '!' in module path")
			}
			next := s[i+1]
			if next < 'a' || next > 'z' {
				return "", errors.New("invalid escape in module path")
			}
			b.WriteByte(next - ('a' - 'A'))
			i++
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}
