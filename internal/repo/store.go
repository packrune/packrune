// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Database-backed repository CRUD. Format adapters use Store to resolve which
// Packrune repository owns an incoming request and to record artifact
// metadata.

package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/packrune/packrune/internal/db"
)

// ErrNotFound is returned when a repository lookup misses.
var ErrNotFound = errors.New("repo: not found")

// Store persists repositories and artifacts.
type Store struct {
	db *db.DB
}

// NewStore constructs a Store.
func NewStore(database *db.DB) *Store { return &Store{db: database} }

// Get returns the repository with the given (name, format) tuple.
func (s *Store) Get(ctx context.Context, name, format string) (Repository, error) {
	var r Repository
	var kind string
	q := s.db.Rebind(`SELECT id, name, format, kind, config, created_at, updated_at
		FROM repositories WHERE name = ? AND format = ? LIMIT 1`)
	err := s.db.QueryRowContext(ctx, q, name, format).Scan(
		&r.ID, &r.Name, &r.Format, &kind, &r.Config, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, fmt.Errorf("repo: get: %w", err)
	}
	r.Kind = Kind(kind)
	return r, nil
}

// Create inserts a new repository. ValidateName is enforced.
func (s *Store) Create(ctx context.Context, name, format string, kind Kind, config []byte) (Repository, error) {
	if err := ValidateName(name); err != nil {
		return Repository{}, err
	}
	if !kind.Valid() {
		return Repository{}, fmt.Errorf("repo: invalid kind %q", kind)
	}
	if len(config) == 0 {
		config = []byte("{}")
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	q := s.db.Rebind(`INSERT INTO repositories (id, name, format, kind, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, id, name, format, string(kind), string(config), now, now); err != nil {
		return Repository{}, fmt.Errorf("repo: create: %w", err)
	}
	return Repository{
		ID: id, Name: name, Format: format, Kind: kind,
		Config: config, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Ensure returns the repository with (name, format), creating it with the
// given kind+config if absent. Used by format-adapter bootstrap.
func (s *Store) Ensure(ctx context.Context, name, format string, kind Kind, config []byte) (Repository, error) {
	r, err := s.Get(ctx, name, format)
	if err == nil {
		return r, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	return s.Create(ctx, name, format, kind, config)
}

// List returns every repository, sorted by (format, name).
func (s *Store) List(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, format, kind, config, created_at, updated_at
		FROM repositories ORDER BY format, name`)
	if err != nil {
		return nil, fmt.Errorf("repo: list: %w", err)
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		var r Repository
		var kind string
		if err := rows.Scan(&r.ID, &r.Name, &r.Format, &kind, &r.Config, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan: %w", err)
		}
		r.Kind = Kind(kind)
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertArtifact records or updates an artifact row.
func (s *Store) UpsertArtifact(ctx context.Context, repoID, path, digest string, size int64, mediaType, metadata string) error {
	if metadata == "" {
		metadata = "{}"
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	q := s.db.Rebind(`INSERT INTO artifacts (id, repo_id, path, digest, size, media_type, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (repo_id, path) DO UPDATE SET digest = excluded.digest,
		    size = excluded.size, media_type = excluded.media_type, metadata = excluded.metadata`)
	if _, err := s.db.ExecContext(ctx, q, id, repoID, path, digest, size, mediaType, metadata, now); err != nil {
		return fmt.Errorf("repo: upsert artifact: %w", err)
	}
	return nil
}

// GetByID returns a repository by its primary key.
func (s *Store) GetByID(ctx context.Context, id string) (Repository, error) {
	var r Repository
	var kind string
	q := s.db.Rebind(`SELECT id, name, format, kind, config, created_at, updated_at
		FROM repositories WHERE id = ? LIMIT 1`)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&r.ID, &r.Name, &r.Format, &kind, &r.Config, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, fmt.Errorf("repo: get by id: %w", err)
	}
	r.Kind = Kind(kind)
	return r, nil
}

// groupConfig is the JSON shape of a group repository's Config field.
type groupConfig struct {
	Members []string `json:"members"`
}

// ResolveArtifact returns the artifact at (repo, path). If the repo is a
// group, walks its configured member repos in order and returns the first
// hit (same format only). Hosted/proxy repos behave like GetArtifact.
func (s *Store) ResolveArtifact(ctx context.Context, repoID, path string) (Artifact, error) {
	if art, err := s.GetArtifact(ctx, repoID, path); err == nil {
		return art, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Artifact{}, err
	}

	parent, err := s.GetByID(ctx, repoID)
	if err != nil {
		return Artifact{}, err
	}
	if parent.Kind != KindGroup {
		return Artifact{}, ErrNotFound
	}
	var gc groupConfig
	if len(parent.Config) > 0 {
		_ = json.Unmarshal(parent.Config, &gc)
	}
	for _, name := range gc.Members {
		member, err := s.Get(ctx, name, parent.Format)
		if err != nil {
			continue
		}
		art, err := s.GetArtifact(ctx, member.ID, path)
		if err == nil {
			return art, nil
		}
	}
	return Artifact{}, ErrNotFound
}

// ResolveArtifactsByPrefix combines the direct list with each member's
// list when the repo is a group. Duplicates (by path) are de-duplicated
// keeping the first occurrence — direct entries shadow members, earlier
// members shadow later ones.
func (s *Store) ResolveArtifactsByPrefix(ctx context.Context, repoID, prefix string) ([]Artifact, error) {
	direct, err := s.ListArtifactsByPrefix(ctx, repoID, prefix)
	if err != nil {
		return nil, err
	}

	parent, err := s.GetByID(ctx, repoID)
	if err != nil {
		return direct, nil
	}
	if parent.Kind != KindGroup {
		return direct, nil
	}
	var gc groupConfig
	if len(parent.Config) > 0 {
		_ = json.Unmarshal(parent.Config, &gc)
	}

	seen := map[string]bool{}
	combined := make([]Artifact, 0, len(direct))
	for _, a := range direct {
		seen[a.Path] = true
		combined = append(combined, a)
	}
	for _, name := range gc.Members {
		member, err := s.Get(ctx, name, parent.Format)
		if err != nil {
			continue
		}
		items, err := s.ListArtifactsByPrefix(ctx, member.ID, prefix)
		if err != nil {
			continue
		}
		for _, a := range items {
			if !seen[a.Path] {
				seen[a.Path] = true
				combined = append(combined, a)
			}
		}
	}
	return combined, nil
}

// GetArtifact returns one artifact by (repo_id, path).
func (s *Store) GetArtifact(ctx context.Context, repoID, path string) (Artifact, error) {
	var a Artifact
	q := s.db.Rebind(`SELECT id, repo_id, path, digest, size, media_type, metadata, created_at
		FROM artifacts WHERE repo_id = ? AND path = ? LIMIT 1`)
	err := s.db.QueryRowContext(ctx, q, repoID, path).Scan(
		&a.ID, &a.RepoID, &a.Path, &a.Digest, &a.Size, &a.MediaType, &a.Metadata, &a.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	if err != nil {
		return Artifact{}, fmt.Errorf("repo: get artifact: %w", err)
	}
	return a, nil
}

// ListArtifactsByPrefix returns artifacts whose path begins with prefix in
// the given repo.
func (s *Store) ListArtifactsByPrefix(ctx context.Context, repoID, prefix string) ([]Artifact, error) {
	q := s.db.Rebind(`SELECT id, repo_id, path, digest, size, media_type, metadata, created_at
		FROM artifacts WHERE repo_id = ? AND path LIKE ? ORDER BY path`)
	rows, err := s.db.QueryContext(ctx, q, repoID, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("repo: list artifacts: %w", err)
	}
	defer rows.Close()
	var out []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(&a.ID, &a.RepoID, &a.Path, &a.Digest, &a.Size, &a.MediaType, &a.Metadata, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan artifact: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteArtifact removes one (repo_id, path) row. Missing rows are not an
// error.
func (s *Store) DeleteArtifact(ctx context.Context, repoID, path string) error {
	q := s.db.Rebind(`DELETE FROM artifacts WHERE repo_id = ? AND path = ?`)
	if _, err := s.db.ExecContext(ctx, q, repoID, path); err != nil {
		return fmt.Errorf("repo: delete artifact: %w", err)
	}
	return nil
}

// Artifact is a stored artifact row.
type Artifact struct {
	ID        string
	RepoID    string
	Path      string
	Digest    string
	Size      int64
	MediaType string
	Metadata  string // JSON string
	CreatedAt time.Time
}
