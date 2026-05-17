// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package docker_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/docker"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage/cas"
	"github.com/packrune/packrune/internal/storage/fs"
	"github.com/packrune/packrune/migrations"
)

// newHandler stands up a fully wired Docker handler against ephemeral temp
// dirs so tests can hit it with httptest.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	return newHandlerWithRepo(t, "docker", repo.KindHosted, nil)
}

func newHandlerWithRepo(t *testing.T, name string, kind repo.Kind, config []byte) http.Handler {
	t.Helper()
	ctx := context.Background()
	tmp := t.TempDir()

	database, err := db.Open(ctx, dockerTestDBConfig(tmp))
	if err != nil {
		t.Fatalf("db open: %v", err)
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
	r, err := store.Ensure(ctx, name, "docker", kind, config)
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	h := docker.NewHandler(docker.HandlerConfig{
		Repo:       r,
		Backend:    backend,
		CAS:        cas.New(backend),
		Store:      store,
		UploadRoot: filepath.Join(tmp, "uploads"),
	})

	mux := http.NewServeMux()
	mux.Handle("/v2/", h)
	mux.Handle("/v2", h)
	return mux
}

func dockerTestDBConfig(tmp string) config.DatabaseConfig {
	return config.DatabaseConfig{
		Driver: "sqlite", DSN: filepath.Join(tmp, "test.db"),
		MaxOpenConns: 1, MaxIdleConns: 1,
	}
}

func TestDocker_RegistryHandshake(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v2/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
		t.Errorf("API-Version header = %q, want registry/2.0", got)
	}
}

func TestDocker_BlobPushPullRoundTrip(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	payload := []byte("the unbearable lightness of packrune")
	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	// POST upload start
	resp, err := http.Post(ts.URL+"/v2/library/hello/blobs/uploads/", "", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("upload start status = %d, want 202", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("no Location header")
	}
	_ = resp.Body.Close()

	// PUT finalize with the whole payload in body + digest in query
	req, err := http.NewRequest(http.MethodPut, ts.URL+loc+"?digest="+digest, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("put status = %d, want 201, body=%s", resp.StatusCode, body)
	}
	_ = resp.Body.Close()

	// GET blob
	resp, err = http.Get(ts.URL + "/v2/library/hello/blobs/" + digest)
	if err != nil {
		t.Fatalf("get blob: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("get blob status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestDocker_ProxyFallthroughOnMiss(t *testing.T) {
	manifest := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`)
	blob := []byte("upstream blob")
	blobSum := sha256.Sum256(blob)
	blobDigest := "sha256:" + hex.EncodeToString(blobSum[:])
	manifestSum := sha256.Sum256(manifest)
	manifestDigest := "sha256:" + hex.EncodeToString(manifestSum[:])

	// Fake "Docker Hub" upstream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/library/test/manifests/latest":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", manifestDigest)
			_, _ = w.Write(manifest)
		case r.URL.Path == "/v2/library/test/blobs/"+blobDigest:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(blob)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	cfg, _ := json.Marshal(map[string]string{"upstream": upstream.URL})
	ts := httptest.NewServer(newHandlerWithRepo(t, "dockerhub-proxy", repo.KindProxy, cfg))
	defer ts.Close()

	// Manifest GET should fall through to upstream, cache, and serve.
	resp, err := http.Get(ts.URL + "/v2/library/test/manifests/latest")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, manifest) {
		t.Errorf("manifest bytes mismatch")
	}
	_ = resp.Body.Close()

	// Second manifest GET should be served from cache (upstream still works
	// either way, but verifying body identity is enough for now).
	resp, err = http.Get(ts.URL + "/v2/library/test/manifests/latest")
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	body2, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, body2) {
		t.Errorf("cached manifest mismatch")
	}
	_ = resp.Body.Close()

	// Blob proxy GET.
	resp, err = http.Get(ts.URL + "/v2/library/test/blobs/" + blobDigest)
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, blob) {
		t.Errorf("blob bytes mismatch")
	}
}

func TestDocker_ManifestPushPullAndTagList(t *testing.T) {
	ts := httptest.NewServer(newHandler(t))
	defer ts.Close()

	manifest := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`)
	mt := "application/vnd.oci.image.manifest.v1+json"

	// PUT manifest
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/v2/myorg/myapp/manifests/v1", bytes.NewReader(manifest))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mt)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("put status = %d, body=%s", resp.StatusCode, body)
	}
	dig := resp.Header.Get("Docker-Content-Digest")
	if !strings.HasPrefix(dig, "sha256:") {
		t.Fatalf("bad digest header %q", dig)
	}
	_ = resp.Body.Close()

	// GET by tag
	resp, err = http.Get(ts.URL + "/v2/myorg/myapp/manifests/v1")
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("get tag status = %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, manifest) {
		t.Errorf("tag-fetched manifest mismatch")
	}
	_ = resp.Body.Close()

	// GET by digest
	resp, err = http.Get(ts.URL + "/v2/myorg/myapp/manifests/" + dig)
	if err != nil {
		t.Fatalf("get digest: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("get digest status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// tags/list shows our tag
	resp, err = http.Get(ts.URL + "/v2/myorg/myapp/tags/list")
	if err != nil {
		t.Fatalf("tag list: %v", err)
	}
	defer resp.Body.Close()
	var tl struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tl)
	if tl.Name != "myorg/myapp" || len(tl.Tags) != 1 || tl.Tags[0] != "v1" {
		t.Errorf("tag list = %+v, want {myorg/myapp [v1]}", tl)
	}
}
