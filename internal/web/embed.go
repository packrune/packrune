// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package web embeds the compiled React frontend (web/dist) into the Go
// binary. Until `pnpm build` (or `make web-build`) runs at least once, the
// embedded payload is a small placeholder that points the operator at the
// build command. Either way the binary remains self-contained.
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend. The
// handler supports single-page-application routing: any path that isn't a
// concrete file falls back to /index.html so client-side routes work on
// direct loads.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API and registry paths are mounted ahead of this handler — we
		// should never see them. But if the operator ever shuffles things,
		// avoid accidentally serving index.html for an unhandled API call.
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"),
			strings.HasPrefix(r.URL.Path, "/v2/"),
			r.URL.Path == "/v2":
			http.NotFound(w, r)
			return
		}

		// Strip query, check if a concrete file exists; otherwise serve
		// index.html (SPA fallback).
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				r.URL.Path = "/index.html"
			}
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
