// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/webhook"
	"github.com/packrune/packrune/migrations"
)

func TestWebhook_FireAndDeliver(t *testing.T) {
	// Receiver that records the body + signature.
	var (
		mu   sync.Mutex
		body []byte
		sig  string
	)
	rec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = b
		sig = r.Header.Get("X-Packrune-Signature")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer rec.Close()

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

	svc := webhook.New(database, slog.New(slog.DiscardHandler))
	wh, err := svc.Create(ctx, "test", rec.URL, "topsecret", []string{string(webhook.EventArtifactCreated)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if wh.ID == "" {
		t.Fatalf("empty id")
	}

	svc.Fire(ctx, webhook.EventArtifactCreated, map[string]string{"foo": "bar"})

	// Wait briefly for the goroutine.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := body != nil
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if body == nil {
		t.Fatalf("webhook never delivered")
	}
	if sig == "" {
		t.Fatalf("missing signature header")
	}
	mac := hmac.New(sha256.New, []byte("topsecret"))
	_, _ = mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != want {
		t.Errorf("sig = %q, want %q", sig, want)
	}
}
