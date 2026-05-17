// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package npm_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/npm"
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
	r, err := store.Ensure(ctx, "npm", "npm", repo.KindHosted, nil)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	h := npm.NewHandler(npm.HandlerConfig{
		Repo: r, Backend: backend, CAS: cas.New(backend), Store: store,
	})
	mux := http.NewServeMux()
	mux.Handle("/npm/", http.StripPrefix("/npm", h))
	return mux
}

func TestNpm_PublishAndFetchPackument(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	tarball := []byte("not a real tarball but fine for this test")
	encoded := base64.StdEncoding.EncodeToString(tarball)

	body := `{
		"_id": "pkr-test",
		"name": "pkr-test",
		"description": "Packrune npm test fixture",
		"dist-tags": {"latest": "1.0.0"},
		"versions": {
			"1.0.0": {
				"name": "pkr-test",
				"version": "1.0.0",
				"dist": {"tarball": "http://localhost/npm/pkr-test/-/pkr-test-1.0.0.tgz"}
			}
		},
		"_attachments": {
			"pkr-test-1.0.0.tgz": {
				"content_type": "application/octet-stream",
				"data": "` + encoded + `",
				"length": ` + intToStr(len(tarball)) + `
			}
		}
	}`

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/npm/pkr-test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("publish status = %d, body=%s", resp.StatusCode, b)
	}
	_ = resp.Body.Close()

	// fetch packument
	resp, err = http.Get(ts.URL + "/npm/pkr-test")
	if err != nil {
		t.Fatalf("get packument: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("packument status = %d", resp.StatusCode)
	}
	var pk struct {
		Name     string                     `json:"name"`
		DistTags map[string]string          `json:"dist-tags"`
		Versions map[string]json.RawMessage `json:"versions"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&pk)
	if pk.Name != "pkr-test" {
		t.Errorf("name = %q", pk.Name)
	}
	if pk.DistTags["latest"] != "1.0.0" {
		t.Errorf("latest = %q", pk.DistTags["latest"])
	}
	if _, ok := pk.Versions["1.0.0"]; !ok {
		t.Errorf("missing version 1.0.0")
	}

	// fetch tarball
	resp, err = http.Get(ts.URL + "/npm/pkr-test/-/pkr-test-1.0.0.tgz")
	if err != nil {
		t.Fatalf("get tarball: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, tarball) {
		t.Errorf("tarball mismatch")
	}
}

func TestNpm_DuplicateVersionRejected(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()
	tarball := []byte("x")
	enc := base64.StdEncoding.EncodeToString(tarball)
	body := func() string {
		return `{
			"_id":"dup","name":"dup",
			"dist-tags":{"latest":"1.0.0"},
			"versions":{"1.0.0":{"name":"dup","version":"1.0.0"}},
			"_attachments":{"dup-1.0.0.tgz":{"content_type":"application/octet-stream","data":"` + enc + `","length":1}}
		}`
	}
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/npm/dup", strings.NewReader(body()))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first publish status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/npm/dup", strings.NewReader(body()))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second publish status = %d, want 409", resp.StatusCode)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
