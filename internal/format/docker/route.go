// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Docker Registry V2 URL parsing.
//
// The registry spec lets <name> contain slashes ("library/hello-world"), which
// makes router-pattern matching painful. We do it by hand: strip the /v2/
// prefix, recognize the trailing route shape, take whatever's left as <name>.

package docker

import "strings"

// routeKind tags which endpoint a URL maps to.
type routeKind int

const (
	routeUnknown       routeKind = iota
	routeVersion                 // GET /v2/
	routeCatalog                 // GET /v2/_catalog
	routeTagsList                // GET /v2/<name>/tags/list
	routeManifest                // GET|HEAD|PUT|DELETE /v2/<name>/manifests/<reference>
	routeBlob                    // GET|HEAD|DELETE /v2/<name>/blobs/<digest>
	routeUploadStart             // POST /v2/<name>/blobs/uploads/
	routeUploadSession           // GET|PATCH|PUT|DELETE /v2/<name>/blobs/uploads/<uuid>
)

// parsedRoute is the structured result of parsing a registry URL path.
type parsedRoute struct {
	kind      routeKind
	name      string // image name, may contain '/'
	reference string // tag/digest for manifests and blobs; session id for uploads
}

// parsePath maps a request path (everything after the /v2 prefix; leading
// slash optional) to a parsedRoute.
func parsePath(p string) parsedRoute {
	p = strings.TrimPrefix(p, "/v2")
	p = strings.TrimPrefix(p, "/")

	if p == "" {
		return parsedRoute{kind: routeVersion}
	}
	if p == "_catalog" {
		return parsedRoute{kind: routeCatalog}
	}

	// /<name>/tags/list
	if strings.HasSuffix(p, "/tags/list") {
		return parsedRoute{kind: routeTagsList, name: strings.TrimSuffix(p, "/tags/list")}
	}

	// /<name>/manifests/<reference>
	if i := strings.Index(p, "/manifests/"); i > 0 {
		return parsedRoute{
			kind:      routeManifest,
			name:      p[:i],
			reference: p[i+len("/manifests/"):],
		}
	}

	// /<name>/blobs/uploads/ (start) — note trailing slash matters
	if strings.HasSuffix(p, "/blobs/uploads/") {
		return parsedRoute{kind: routeUploadStart, name: strings.TrimSuffix(p, "/blobs/uploads/")}
	}
	if strings.HasSuffix(p, "/blobs/uploads") {
		return parsedRoute{kind: routeUploadStart, name: strings.TrimSuffix(p, "/blobs/uploads")}
	}

	// /<name>/blobs/uploads/<uuid>
	if i := strings.Index(p, "/blobs/uploads/"); i > 0 {
		return parsedRoute{
			kind:      routeUploadSession,
			name:      p[:i],
			reference: p[i+len("/blobs/uploads/"):],
		}
	}

	// /<name>/blobs/<digest>
	if i := strings.Index(p, "/blobs/"); i > 0 {
		return parsedRoute{
			kind:      routeBlob,
			name:      p[:i],
			reference: p[i+len("/blobs/"):],
		}
	}

	return parsedRoute{kind: routeUnknown}
}
