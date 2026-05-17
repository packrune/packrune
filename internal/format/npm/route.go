// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// URL parsing for the npm registry surface. We mount the npm handler under
// "/npm" in production, so paths arrive with that prefix already stripped.

package npm

import "strings"

type routeKind int

const (
	routeUnknown   routeKind = iota
	routeRoot                // GET /
	routePing                // GET /-/ping
	routeWhoami              // GET /-/whoami
	routeSearch              // GET /-/v1/search
	routePackument           // GET|PUT /<pkg>
	routeVersion             // GET /<pkg>/<version>
	routeTarball             // GET /<pkg>/-/<filename>
)

type parsedRoute struct {
	kind routeKind
	pkg  string // package name; may include "@scope/"
	ref  string // version for routeVersion, filename for routeTarball
}

func parsePath(p string) parsedRoute {
	p = strings.TrimPrefix(p, "/")
	switch {
	case p == "":
		return parsedRoute{kind: routeRoot}
	case p == "-/ping":
		return parsedRoute{kind: routePing}
	case p == "-/whoami":
		return parsedRoute{kind: routeWhoami}
	case strings.HasPrefix(p, "-/v1/search"):
		return parsedRoute{kind: routeSearch}
	}

	// Tarball: /<pkg>/-/<filename>. Use last "/-/" so scoped names work.
	if i := strings.LastIndex(p, "/-/"); i > 0 {
		return parsedRoute{kind: routeTarball, pkg: p[:i], ref: p[i+3:]}
	}

	// Otherwise: packument or version. Scoped packages embed a slash.
	parts := strings.Split(p, "/")
	if len(parts) >= 1 && strings.HasPrefix(parts[0], "@") {
		switch len(parts) {
		case 2:
			return parsedRoute{kind: routePackument, pkg: parts[0] + "/" + parts[1]}
		case 3:
			return parsedRoute{kind: routeVersion, pkg: parts[0] + "/" + parts[1], ref: parts[2]}
		}
		return parsedRoute{kind: routeUnknown}
	}
	switch len(parts) {
	case 1:
		return parsedRoute{kind: routePackument, pkg: parts[0]}
	case 2:
		return parsedRoute{kind: routeVersion, pkg: parts[0], ref: parts[1]}
	}
	return parsedRoute{kind: routeUnknown}
}
