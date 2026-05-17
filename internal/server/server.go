// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package server hosts the HTTP server that fronts every Packrune subsystem.
// It is intentionally thin: it composes middleware, mounts feature-specific
// routers, and runs the listener loop. Business logic lives in feature
// packages.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
)

// Server is a Packrune HTTP server. Construct with New and run with Run.
type Server struct {
	cfg    config.Config
	logger *slog.Logger
	db     *db.DB
	router chi.Router
	http   *http.Server
}

// New constructs a Server wired up with middleware, health endpoints, and the
// root API router. It does not start listening; call Run for that. The
// provided DB is owned by the server lifecycle and will be closed on Run
// return.
func New(cfg config.Config, logger *slog.Logger, database *db.DB) (*Server, error) {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(RecoverPanic(logger))
	r.Use(LogRequests(logger))

	r.Get("/healthz", HandleHealth)
	r.Get("/readyz", HandleReady)
	r.Get("/version", HandleVersion)

	// Feature routers will mount here in later phases:
	//   r.Mount("/v2", dockerRouter)         // Docker registry
	//   r.Mount("/api", apiRouter)           // Admin/JSON API
	//   r.Handle("/*", staticUI)             // Embedded React SPA

	s := &Server{
		cfg:    cfg,
		logger: logger,
		db:     database,
		router: r,
		http: &http.Server{
			Addr:         cfg.Server.Addr,
			Handler:      r,
			ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
			WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		},
	}
	return s, nil
}

// Router exposes the underlying chi router so other packages can mount their
// routes. Used during startup wiring only.
func (s *Server) Router() chi.Router { return s.router }

// Run listens on cfg.Server.Addr and serves until ctx is canceled, at which
// point it shuts the listener down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errc := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		if s.db != nil {
			_ = s.db.Close()
		}
		if err, ok := <-errc; ok && err != nil {
			return err
		}
		return nil
	case err, ok := <-errc:
		if ok && err != nil {
			return err
		}
		return nil
	}
}
