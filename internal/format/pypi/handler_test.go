// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package pypi_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/pypi"
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
	backend, _ := fs.New(filepath.Join(tmp, "storage"))
	store := repo.NewStore(database)
	r, _ := store.Ensure(ctx, "pypi", "pypi", repo.KindHosted, nil)
	h := pypi.NewHandler(pypi.HandlerConfig{
		Repo: r, Backend: backend, CAS: cas.New(backend), Store: store,
	})
	mux := http.NewServeMux()
	mux.Handle("/pypi/", http.StripPrefix("/pypi", h))
	mux.Handle("/pypi", http.StripPrefix("/pypi", h))
	return mux
}

func TestPyPI_UploadAndIndex(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	wheelBody := []byte("PK fake wheel bytes")

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("name", "Sample_Package")
	_ = mw.WriteField("version", "1.0.0")
	_ = mw.WriteField(":action", "file_upload")
	fw, _ := mw.CreateFormFile("content", "sample_package-1.0.0-py3-none-any.whl")
	_, _ = fw.Write(wheelBody)
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/pypi/", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status %d, body=%s", resp.StatusCode, b)
	}
	_ = resp.Body.Close()

	// simple index lists the package (normalized: sample-package)
	resp, err = http.Get(ts.URL + "/pypi/simple/")
	if err != nil {
		t.Fatalf("simple: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "sample-package") {
		t.Errorf("simple = %s", b)
	}

	// per-package page
	resp, err = http.Get(ts.URL + "/pypi/simple/sample-package/")
	if err != nil {
		t.Fatalf("pkg: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "sample_package-1.0.0-py3-none-any.whl") {
		t.Errorf("pkg index missing file: %s", b)
	}
	if !strings.Contains(string(b), "#sha256=") {
		t.Errorf("missing sha256 hash anchor: %s", b)
	}

	// download
	resp, err = http.Get(ts.URL + "/pypi/packages/sample-package/sample_package-1.0.0-py3-none-any.whl")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, wheelBody) {
		t.Errorf("wheel bytes mismatch")
	}

	// JSON API
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/pypi/simple/sample-package/", nil)
	req.Header.Set("Accept", "application/vnd.pypi.simple.v1+json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"name"`) || !strings.Contains(string(b), `"files"`) {
		t.Errorf("json shape = %s", b)
	}
}
