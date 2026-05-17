// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package gomod_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/gomod"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage/cas"
	"github.com/packrune/packrune/internal/storage/fs"
	"github.com/packrune/packrune/migrations"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	tmp := t.TempDir()
	database, err := db.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite", DSN: filepath.Join(tmp, "t.db"),
		MaxOpenConns: 1, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.ApplyMigrations(ctx, migrations.FS()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	backend, err := fs.New(filepath.Join(tmp, "storage"))
	if err != nil {
		t.Fatalf("fs: %v", err)
	}
	store := repo.NewStore(database)
	r, _ := store.Ensure(ctx, "go", "gomod", repo.KindHosted, nil)
	h := gomod.NewHandler(gomod.HandlerConfig{
		Repo: r, Backend: backend, CAS: cas.New(backend), Store: store,
	})
	mux := http.NewServeMux()
	mux.Handle("/go/", http.StripPrefix("/go", h))
	return mux
}

func put(t *testing.T, url, contentType, body string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("put %s status %d, body=%s", url, resp.StatusCode, b)
	}
	_ = resp.Body.Close()
}

func TestGomod_FullRoundTrip(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	mod := "github.com/relteco/packrune-test"

	put(t, ts.URL+"/go/"+mod+"/@v/v1.0.0.info", "application/json",
		`{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`)
	put(t, ts.URL+"/go/"+mod+"/@v/v1.0.0.mod", "text/plain",
		"module "+mod+"\n\ngo 1.24\n")
	// .zip — keep small fake content; spec doesn't require validation here.
	put(t, ts.URL+"/go/"+mod+"/@v/v1.0.0.zip", "application/zip",
		"PK\x03\x04 not really a zip but ok for storage test")
	put(t, ts.URL+"/go/"+mod+"/@v/v1.1.0.info", "application/json",
		`{"Version":"v1.1.0","Time":"2026-02-01T00:00:00Z"}`)

	// list
	resp, err := http.Get(ts.URL + "/go/" + mod + "/@v/list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	got := strings.TrimSpace(string(body))
	if got != "v1.0.0\nv1.1.0" {
		t.Errorf("list = %q, want v1.0.0\\nv1.1.0", got)
	}

	// latest
	resp, err = http.Get(ts.URL + "/go/" + mod + "/@latest")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"Version":"v1.1.0"`) {
		t.Errorf("latest = %s", body)
	}

	// .info GET
	resp, err = http.Get(ts.URL + "/go/" + mod + "/@v/v1.0.0.info")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"v1.0.0"`) {
		t.Errorf("info = %s", body)
	}

	// .mod GET
	resp, err = http.Get(ts.URL + "/go/" + mod + "/@v/v1.0.0.mod")
	if err != nil {
		t.Fatalf("mod: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "module ") {
		t.Errorf("mod = %s", body)
	}

	// .zip GET
	resp, err = http.Get(ts.URL + "/go/" + mod + "/@v/v1.0.0.zip")
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("zip status %d", resp.StatusCode)
	}
}

func TestGomod_CapitalLetterEscape(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	put(t, ts.URL+"/go/github.com/!acme/!foo/@v/v0.1.0.info", "application/json",
		`{"Version":"v0.1.0","Time":"2026-01-01T00:00:00Z"}`)

	resp, err := http.Get(ts.URL + "/go/github.com/!acme/!foo/@v/list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "v0.1.0") {
		t.Errorf("list = %q", body)
	}
}
