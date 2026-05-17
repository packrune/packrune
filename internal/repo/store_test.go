// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package repo_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/migrations"
)

func newStore(t *testing.T) *repo.Store {
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
	return repo.NewStore(database)
}

func TestStore_ResolveArtifact_HostedRepo(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	r, _ := s.Ensure(ctx, "hosted-1", "docker", repo.KindHosted, nil)
	_ = s.UpsertArtifact(ctx, r.ID, "blobs/sha256:abc", "sha256:abc", 10, "", "")

	art, err := s.ResolveArtifact(ctx, r.ID, "blobs/sha256:abc")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if art.Path != "blobs/sha256:abc" {
		t.Errorf("path = %q", art.Path)
	}

	if _, err := s.ResolveArtifact(ctx, r.ID, "blobs/missing"); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("missing: got %v, want ErrNotFound", err)
	}
}

func TestStore_ResolveArtifact_GroupWalksMembers(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	m1, _ := s.Ensure(ctx, "docker-hosted", "docker", repo.KindHosted, nil)
	m2, _ := s.Ensure(ctx, "dockerhub-proxy", "docker", repo.KindHosted, nil)
	_ = s.UpsertArtifact(ctx, m1.ID, "blobs/sha256:in-m1", "sha256:aaa", 1, "", "")
	_ = s.UpsertArtifact(ctx, m2.ID, "blobs/sha256:in-m2", "sha256:bbb", 1, "", "")

	groupCfg, _ := json.Marshal(map[string]any{"members": []string{"docker-hosted", "dockerhub-proxy"}})
	group, err := s.Ensure(ctx, "docker-all", "docker", repo.KindGroup, groupCfg)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Hit on member 1
	art, err := s.ResolveArtifact(ctx, group.ID, "blobs/sha256:in-m1")
	if err != nil {
		t.Fatalf("resolve m1: %v", err)
	}
	if art.Digest != "sha256:aaa" {
		t.Errorf("digest = %q, want sha256:aaa", art.Digest)
	}

	// Hit on member 2 (so the walk advances)
	art, err = s.ResolveArtifact(ctx, group.ID, "blobs/sha256:in-m2")
	if err != nil {
		t.Fatalf("resolve m2: %v", err)
	}
	if art.Digest != "sha256:bbb" {
		t.Errorf("digest = %q, want sha256:bbb", art.Digest)
	}

	// Miss everywhere
	if _, err := s.ResolveArtifact(ctx, group.ID, "blobs/sha256:nope"); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("nope: got %v, want ErrNotFound", err)
	}
}

func TestStore_ResolveArtifactsByPrefix_GroupDedup(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	m1, _ := s.Ensure(ctx, "h1", "docker", repo.KindHosted, nil)
	m2, _ := s.Ensure(ctx, "h2", "docker", repo.KindHosted, nil)
	// Both members have the same logical path but with different digests; the
	// first member wins by group ordering.
	_ = s.UpsertArtifact(ctx, m1.ID, "refs/x/manifests/latest", "sha256:from-m1", 1, "", "")
	_ = s.UpsertArtifact(ctx, m2.ID, "refs/x/manifests/latest", "sha256:from-m2", 1, "", "")
	_ = s.UpsertArtifact(ctx, m2.ID, "refs/x/manifests/old", "sha256:m2-only", 1, "", "")

	groupCfg, _ := json.Marshal(map[string]any{"members": []string{"h1", "h2"}})
	group, _ := s.Ensure(ctx, "g", "docker", repo.KindGroup, groupCfg)

	arts, err := s.ResolveArtifactsByPrefix(ctx, group.ID, "refs/x/manifests/")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("got %d arts, want 2", len(arts))
	}
	byPath := map[string]string{}
	for _, a := range arts {
		byPath[a.Path] = a.Digest
	}
	// "latest" must come from h1 (first member), shadowing h2's copy.
	if byPath["refs/x/manifests/latest"] != "sha256:from-m1" {
		t.Errorf("latest = %q, want sha256:from-m1", byPath["refs/x/manifests/latest"])
	}
	// "old" only existed in h2; should still be picked up.
	if byPath["refs/x/manifests/old"] != "sha256:m2-only" {
		t.Errorf("old = %q, want sha256:m2-only", byPath["refs/x/manifests/old"])
	}
}
