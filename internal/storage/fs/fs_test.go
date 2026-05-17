// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package fs_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/fs"
)

func TestFSBackend_RoundTrip(t *testing.T) {
	b, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	ctx := context.Background()
	const key = "blobs/sha256/abc/foo.bin"
	want := []byte("hello, packrune")

	if _, err := b.Put(ctx, key, bytes.NewReader(want)); err != nil {
		t.Fatalf("put: %v", err)
	}

	r, st, err := b.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Close()

	if st.Size != int64(len(want)) {
		t.Errorf("size = %d, want %d", st.Size, len(want))
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFSBackend_NotFound(t *testing.T) {
	b, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, _, err = b.Get(context.Background(), "nope/missing")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestFSBackend_RejectsTraversal(t *testing.T) {
	b, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = b.Put(context.Background(), "../escape", strings.NewReader("x"))
	if err == nil {
		t.Fatalf("expected error on traversal key")
	}
}

func TestFSBackend_DeleteMissingIsOK(t *testing.T) {
	b, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := b.Delete(context.Background(), "never/existed"); err != nil {
		t.Errorf("delete missing returned %v, want nil", err)
	}
}

func TestFSBackend_List(t *testing.T) {
	b, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()
	keys := []string{"a/1", "a/2", "a/3", "b/1"}
	for _, k := range keys {
		if _, err := b.Put(ctx, k, strings.NewReader(k)); err != nil {
			t.Fatalf("put %s: %v", k, err)
		}
	}

	res, err := b.List(ctx, "a/", "", 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Keys) != 3 {
		t.Errorf("got %d keys under a/, want 3 (%v)", len(res.Keys), res.Keys)
	}
}
