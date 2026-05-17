// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package helm_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/helm"
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
		Driver: "sqlite", DSN: filepath.Join(tmp, "test.db"),
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
	r, _ := store.Ensure(ctx, "helm", "helm", repo.KindHosted, nil)
	h := helm.NewHandler(helm.HandlerConfig{
		Repo: r, Backend: backend, CAS: cas.New(backend), Store: store, ContextPath: "/helm",
	})
	mux := http.NewServeMux()
	mux.Handle("/helm/", http.StripPrefix("/helm", h))
	return mux
}

// fakeChart returns a minimal valid Helm chart tarball.
func fakeChart(t *testing.T, name, version, description string) []byte {
	t.Helper()
	var gzbuf bytes.Buffer
	gz := gzip.NewWriter(&gzbuf)
	tw := tar.NewWriter(gz)

	chartYaml := []byte("apiVersion: v2\nname: " + name + "\nversion: " + version + "\ndescription: " + description + "\n")
	if err := tw.WriteHeader(&tar.Header{
		Name: name + "/Chart.yaml", Mode: 0o644, Size: int64(len(chartYaml)),
		ModTime: time.Now(), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("hdr: %v", err)
	}
	if _, err := tw.Write(chartYaml); err != nil {
		t.Fatalf("write: %v", err)
	}
	values := []byte("foo: bar\n")
	if err := tw.WriteHeader(&tar.Header{
		Name: name + "/values.yaml", Mode: 0o644, Size: int64(len(values)),
		ModTime: time.Now(), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("hdr2: %v", err)
	}
	if _, err := tw.Write(values); err != nil {
		t.Fatalf("write2: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tw close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz close: %v", err)
	}
	return gzbuf.Bytes()
}

func TestHelm_UploadAndIndex(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	chart := fakeChart(t, "mychart", "0.1.0", "test chart")

	// upload
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("chart", "mychart-0.1.0.tgz")
	_, _ = fw.Write(chart)
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/helm/api/charts", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status %d, body=%s", resp.StatusCode, b)
	}
	_ = resp.Body.Close()

	// index.yaml
	resp, err = http.Get(ts.URL + "/helm/index.yaml")
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	defer resp.Body.Close()
	indexBytes, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(indexBytes), "mychart") {
		t.Errorf("index missing chart name: %s", indexBytes)
	}

	var idx struct {
		APIVersion string `yaml:"apiVersion"`
		Entries    map[string][]struct {
			Name    string   `yaml:"name"`
			Version string   `yaml:"version"`
			URLs    []string `yaml:"urls"`
		} `yaml:"entries"`
	}
	if err := yaml.Unmarshal(indexBytes, &idx); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	if idx.APIVersion != "v1" {
		t.Errorf("apiVersion = %q", idx.APIVersion)
	}
	entries := idx.Entries["mychart"]
	if len(entries) != 1 || entries[0].Version != "0.1.0" {
		t.Fatalf("entries = %+v", entries)
	}
	if len(entries[0].URLs) == 0 || entries[0].URLs[0] != "mychart-0.1.0.tgz" {
		t.Fatalf("urls = %+v", entries[0].URLs)
	}

	// download
	resp, err = http.Get(ts.URL + "/helm/mychart-0.1.0.tgz")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, chart) {
		t.Errorf("chart bytes mismatch (%d vs %d)", len(got), len(chart))
	}

	// list /api/charts
	resp, err = http.Get(ts.URL + "/helm/api/charts")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	var list map[string][]struct{ Name, Version string }
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list["mychart"]) != 1 {
		t.Errorf("list[mychart] = %+v", list["mychart"])
	}
}
