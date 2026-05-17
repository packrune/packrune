// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package fs is the filesystem storage backend. It maps keys to files under a
// configurable root directory. Atomic writes use a temp-file-then-rename
// pattern so partially-written objects never appear under their final key.
package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/packrune/packrune/internal/storage"
)

// Backend stores objects on the local filesystem under Root.
type Backend struct {
	root string
}

// New constructs a filesystem Backend rooted at the given directory. The
// directory is created on demand.
func New(root string) (*Backend, error) {
	if root == "" {
		return nil, errors.New("fs: root must not be empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fs: mkdir root: %w", err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("fs: abs root: %w", err)
	}
	return &Backend{root: abs}, nil
}

// Name returns "fs".
func (b *Backend) Name() string { return "fs" }

// Get opens the file at key for reading.
func (b *Backend) Get(_ context.Context, key string) (io.ReadCloser, storage.Stat, error) {
	p, err := b.resolve(key)
	if err != nil {
		return nil, storage.Stat{}, err
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, storage.Stat{}, storage.ErrNotFound
		}
		return nil, storage.Stat{}, fmt.Errorf("fs: open %s: %w", key, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, storage.Stat{}, fmt.Errorf("fs: stat %s: %w", key, err)
	}
	return f, storage.Stat{
		Key:     key,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// Put writes body to a temp file inside the destination directory and renames
// it into place once the copy finishes. Crashes mid-write leave a stray
// .tmp* file that cleanup jobs can sweep, but never a half-formed object.
func (b *Backend) Put(_ context.Context, key string, body io.Reader) (int64, error) {
	dst, err := b.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, fmt.Errorf("fs: mkdir for %s: %w", key, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".pkr-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("fs: tempfile: %w", err)
	}
	n, copyErr := io.Copy(tmp, body)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp.Name())
		if copyErr != nil {
			return 0, fmt.Errorf("fs: write %s: %w", key, copyErr)
		}
		return 0, fmt.Errorf("fs: close %s: %w", key, closeErr)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())
		return 0, fmt.Errorf("fs: rename to %s: %w", key, err)
	}
	return n, nil
}

// Append opens the file at key for append, creating it if necessary, and
// copies body into it. Atomicity guarantees are weaker than Put: a crash
// mid-append leaves the partial bytes in place. Callers manage that risk
// (e.g. Docker uploads recover by hashing what was written on next chunk).
func (b *Backend) Append(_ context.Context, key string, body io.Reader) (int64, error) {
	dst, err := b.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, fmt.Errorf("fs: mkdir for append %s: %w", key, err)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("fs: open append %s: %w", key, err)
	}
	n, copyErr := io.Copy(f, body)
	closeErr := f.Close()
	if copyErr != nil {
		return n, fmt.Errorf("fs: append %s: %w", key, copyErr)
	}
	if closeErr != nil {
		return n, fmt.Errorf("fs: close append %s: %w", key, closeErr)
	}
	return n, nil
}

// Stat returns size and modtime for key.
func (b *Backend) Stat(_ context.Context, key string) (storage.Stat, error) {
	p, err := b.resolve(key)
	if err != nil {
		return storage.Stat{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storage.Stat{}, storage.ErrNotFound
		}
		return storage.Stat{}, fmt.Errorf("fs: stat %s: %w", key, err)
	}
	return storage.Stat{Key: key, Size: info.Size(), ModTime: info.ModTime()}, nil
}

// Delete removes the file at key. Missing files are not an error.
func (b *Backend) Delete(_ context.Context, key string) error {
	p, err := b.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fs: delete %s: %w", key, err)
	}
	return nil
}

// List walks the tree under prefix. The pageToken is the key of the last
// returned entry from the previous page; this keeps the implementation
// simple at the cost of paginated walks doing a (cheap) prefix scan each
// time.
func (b *Backend) List(_ context.Context, prefix, pageToken string, limit int) (storage.ListResult, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	root, err := b.resolve(prefix)
	if err != nil {
		return storage.ListResult{}, err
	}

	var keys []string
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		err := filepath.Walk(root, func(p string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(b.root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, prefix) {
				keys = append(keys, rel)
			}
			return nil
		})
		if err != nil {
			return storage.ListResult{}, fmt.Errorf("fs: walk %s: %w", prefix, err)
		}
	}
	sort.Strings(keys)

	// Apply pageToken cursor.
	if pageToken != "" {
		i := sort.SearchStrings(keys, pageToken)
		// We want strictly after pageToken.
		if i < len(keys) && keys[i] == pageToken {
			i++
		}
		keys = keys[i:]
	}
	var next string
	if len(keys) > limit {
		keys = keys[:limit]
		next = keys[len(keys)-1]
	}
	return storage.ListResult{Keys: keys, NextToken: next}, nil
}

// resolve joins key onto root with directory-traversal protection.
func (b *Backend) resolve(key string) (string, error) {
	key = strings.TrimPrefix(filepath.ToSlash(key), "/")
	if key == "" {
		return b.root, nil
	}
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("fs: invalid key %q (contains '..')", key)
	}
	p := filepath.Join(b.root, filepath.FromSlash(key))
	if !strings.HasPrefix(p, b.root) {
		return "", fmt.Errorf("fs: invalid key %q (escapes root)", key)
	}
	return p, nil
}
