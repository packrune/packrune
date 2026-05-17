// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package maven_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/maven"
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
	r, _ := store.Ensure(ctx, "maven", "maven", repo.KindHosted, nil)
	h := maven.NewHandler(maven.HandlerConfig{
		Repo: r, Backend: backend, CAS: cas.New(backend), Store: store,
	})
	mux := http.NewServeMux()
	mux.Handle("/maven/", http.StripPrefix("/maven", h))
	return mux
}

func putRaw(t *testing.T, url, contentType string, body []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("put %s status %d body=%s", url, resp.StatusCode, b)
	}
	_ = resp.Body.Close()
}

func TestMaven_UploadAndMetadataRegen(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	jar := []byte("PK\x03\x04 fake jar bytes")
	pom := []byte(`<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion></project>`)

	base := ts.URL + "/maven/com/relteco/packrune-test"
	putRaw(t, base+"/1.0.0/packrune-test-1.0.0.jar", "application/java-archive", jar)
	putRaw(t, base+"/1.0.0/packrune-test-1.0.0.pom", "application/xml", pom)
	putRaw(t, base+"/1.1.0/packrune-test-1.1.0.jar", "application/java-archive", jar)

	// jar download
	resp, err := http.Get(base + "/1.0.0/packrune-test-1.0.0.jar")
	if err != nil {
		t.Fatalf("get jar: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, jar) {
		t.Errorf("jar bytes mismatch")
	}

	// auto-generated sha1
	resp, err = http.Get(base + "/1.0.0/packrune-test-1.0.0.jar.sha1")
	if err != nil {
		t.Fatalf("get sha1: %v", err)
	}
	got, _ = io.ReadAll(resp.Body)
	if len(got) != 40 {
		t.Errorf("sha1 length = %d, want 40", len(got))
	}

	// auto-generated maven-metadata.xml at the artifactId level
	resp, err = http.Get(base + "/maven-metadata.xml")
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "<groupId>com.relteco</groupId>") {
		t.Errorf("metadata missing groupId: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "<artifactId>packrune-test</artifactId>") {
		t.Errorf("metadata missing artifactId: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "<version>1.0.0</version>") || !strings.Contains(bodyStr, "<version>1.1.0</version>") {
		t.Errorf("metadata missing versions: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "<latest>1.1.0</latest>") {
		t.Errorf("latest != 1.1.0: %s", bodyStr)
	}
}
