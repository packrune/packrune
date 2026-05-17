// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune serve` — the long-running HTTP server. Opens the DB, applies any
// pending migrations, wires the server, and runs until SIGINT/SIGTERM.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/docker"
	plog "github.com/packrune/packrune/internal/log"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/server"
	"github.com/packrune/packrune/internal/storage"
	"github.com/packrune/packrune/internal/storage/cas"
	"github.com/packrune/packrune/internal/storage/fs"
	"github.com/packrune/packrune/internal/web"
	"github.com/packrune/packrune/migrations"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger, err := plog.New(os.Stderr, cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	logger = logger.With("version", version, "commit", commit)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	if err := database.ApplyMigrations(ctx, migrations.FS()); err != nil {
		_ = database.Close()
		return fmt.Errorf("db: migrate: %w", err)
	}
	logger.Info("database ready", "driver", cfg.Database.Driver)

	server.SetVersionInfo(version, commit, date)

	// Wire storage.
	backend, uploadRoot, err := buildStorage(cfg.Storage)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("storage: %w", err)
	}
	contentStore := cas.New(backend)

	// Repository store + ensure a default docker hosted repo exists.
	repoStore := repo.NewStore(database)
	dockerRepo, err := repoStore.Ensure(ctx, "docker", "docker", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap docker repo: %w", err)
	}
	logger.Info("docker repository ready", "name", dockerRepo.Name, "id", dockerRepo.ID)

	dockerHandler := docker.NewHandler(docker.HandlerConfig{
		Logger:     logger,
		Repo:       dockerRepo,
		Backend:    backend,
		CAS:        contentStore,
		Store:      repoStore,
		UploadRoot: filepath.Join(uploadRoot, "docker"),
	})

	srv, err := server.New(cfg, logger, database)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("server: %w", err)
	}
	// Docker registry V2 surface.
	srv.Router().Mount("/v2", dockerHandler)

	// Embedded admin UI. Served from "/" so the SPA fallback works for any
	// client-side route. API and /v2/ are mounted ahead of this so they win.
	uiHandler, err := web.Handler()
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("web: %w", err)
	}
	srv.Router().Handle("/*", uiHandler)

	logger.Info("starting packrune", "addr", cfg.Server.Addr)
	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	logger.Info("packrune stopped cleanly")
	return nil
}

// buildStorage constructs the configured storage backend and returns the
// directory we can use to stage upload temp files. For S3 we currently bail —
// upload staging needs a local writable area regardless of backend; that
// will be addressed when the S3 backend lands.
func buildStorage(cfg config.StorageConfig) (storage.Backend, string, error) {
	switch cfg.Backend {
	case "fs":
		b, err := fs.New(cfg.FS.Root)
		if err != nil {
			return nil, "", err
		}
		return b, filepath.Join(cfg.FS.Root, "_uploads"), nil
	case "s3":
		return nil, "", errors.New("s3 backend not yet implemented")
	default:
		return nil, "", fmt.Errorf("unknown storage backend %q", cfg.Backend)
	}
}
