// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packrune/packrune/internal/api"
	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/migrations"
)

func newAPI(t *testing.T) (http.Handler, *auth.DBService) {
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

	svc := auth.NewDBService(database)
	store := repo.NewStore(database)

	a := &api.API{
		Auth:    svc,
		Store:   store,
		Version: "test", Commit: "test", Date: "test",
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", a.Router()))
	return mux, svc
}

func TestAPI_LoginAndMe(t *testing.T) {
	mux, svc := newAPI(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	if _, err := svc.CreateUser(ctx, "berk@relteco.com", "berk", "supersecret", true); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// login with bad password
	body, _ := json.Marshal(map[string]string{"username": "berk", "password": "wrong"})
	resp, _ := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// login OK
	body, _ = json.Marshal(map[string]string{"username": "berk", "password": "supersecret"})
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login status = %d, body=%s", resp.StatusCode, b)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "packrune_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatalf("no session cookie")
	}
	_ = resp.Body.Close()

	// /me with cookie
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/me", nil)
	req.AddCookie(cookie)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"username":"berk"`) {
		t.Errorf("me body = %s", b)
	}
	if !strings.Contains(string(b), `"is_admin":true`) {
		t.Errorf("admin flag = %s", b)
	}

	// /me without cookie → 401
	resp, _ = http.Get(ts.URL + "/api/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anon /me status = %d", resp.StatusCode)
	}
}

func TestAPI_RepositoriesEndpoint(t *testing.T) {
	mux, svc := newAPI(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	u, _ := svc.CreateUser(ctx, "x@y.z", "u", "pwpwpwpw", false)
	plain, _, _ := svc.IssueToken(ctx, u.ID, "test", []string{"session"}, 0)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/repositories", nil)
	req.Header.Set("Authorization", "Bearer "+plain)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body=%s", resp.StatusCode, b)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&page)
	// We have no repos yet — empty list is fine.
	if page.Items == nil {
		t.Errorf("items should be an empty array, not nil")
	}
}
