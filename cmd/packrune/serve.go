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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/packrune/packrune/internal/api"
	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format/docker"
	"github.com/packrune/packrune/internal/format/gomod"
	"github.com/packrune/packrune/internal/format/helm"
	"github.com/packrune/packrune/internal/format/maven"
	"github.com/packrune/packrune/internal/format/npm"
	"github.com/packrune/packrune/internal/format/pypi"
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
	// npm-hosted bootstrap repo.
	npmRepo, err := repoStore.Ensure(ctx, "npm", "npm", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap npm repo: %w", err)
	}
	logger.Info("npm repository ready", "name", npmRepo.Name, "id", npmRepo.ID)
	npmHandler := npm.NewHandler(npm.HandlerConfig{
		Logger:  logger,
		Repo:    npmRepo,
		Backend: backend,
		CAS:     contentStore,
		Store:   repoStore,
	})

	// helm-hosted bootstrap repo + handler.
	helmRepo, err := repoStore.Ensure(ctx, "helm", "helm", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap helm repo: %w", err)
	}
	logger.Info("helm repository ready", "name", helmRepo.Name, "id", helmRepo.ID)
	helmHandler := helm.NewHandler(helm.HandlerConfig{
		Logger:      logger,
		Repo:        helmRepo,
		Backend:     backend,
		CAS:         contentStore,
		Store:       repoStore,
		ContextPath: "/helm",
	})

	// gomod-hosted bootstrap repo + handler.
	gomodRepo, err := repoStore.Ensure(ctx, "go", "gomod", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap gomod repo: %w", err)
	}
	logger.Info("gomod repository ready", "name", gomodRepo.Name, "id", gomodRepo.ID)
	gomodHandler := gomod.NewHandler(gomod.HandlerConfig{
		Logger:  logger,
		Repo:    gomodRepo,
		Backend: backend,
		CAS:     contentStore,
		Store:   repoStore,
	})

	// pypi-hosted bootstrap repo + handler.
	pypiRepo, err := repoStore.Ensure(ctx, "pypi", "pypi", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap pypi repo: %w", err)
	}
	logger.Info("pypi repository ready", "name", pypiRepo.Name, "id", pypiRepo.ID)
	pypiHandler := pypi.NewHandler(pypi.HandlerConfig{
		Logger: logger, Repo: pypiRepo, Backend: backend, CAS: contentStore, Store: repoStore,
	})

	// maven-hosted bootstrap repo + handler.
	mavenRepo, err := repoStore.Ensure(ctx, "maven", "maven", repo.KindHosted, nil)
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("bootstrap maven repo: %w", err)
	}
	logger.Info("maven repository ready", "name", mavenRepo.Name, "id", mavenRepo.ID)
	mavenHandler := maven.NewHandler(maven.HandlerConfig{
		Logger: logger, Repo: mavenRepo, Backend: backend, CAS: contentStore, Store: repoStore,
	})

	// Each format mounts under a /<prefix> path. chi.Mount routes the
	// request but doesn't rewrite r.URL.Path; we use http.StripPrefix so the
	// inner handler sees its own URL space (clean code; less prefix-aware
	// boilerplate inside each format adapter).
	srv.Router().Mount("/v2", http.StripPrefix("/v2", dockerHandler))
	srv.Router().Mount("/npm", http.StripPrefix("/npm", npmHandler))
	srv.Router().Mount("/helm", http.StripPrefix("/helm", helmHandler))
	srv.Router().Mount("/go", http.StripPrefix("/go", gomodHandler))
	srv.Router().Mount("/pypi", http.StripPrefix("/pypi", pypiHandler))
	srv.Router().Mount("/maven", http.StripPrefix("/maven", mavenHandler))

	// JSON admin API.
	authSvc := auth.NewDBService(database)
	jsonAPI := &api.API{
		Logger:  logger,
		Auth:    authSvc,
		Store:   repoStore,
		Version: version, Commit: commit, Date: date,
	}
	srv.Router().Mount("/api", jsonAPI.Router())

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
