// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune doctor` — diagnostic command that exercises every part of the
// install and reports green/yellow/red checkmarks. Useful as a first-line
// "is something wrong here?" answer for operators.

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/format"
	_ "github.com/packrune/packrune/internal/format/docker"
	_ "github.com/packrune/packrune/internal/format/gomod"
	_ "github.com/packrune/packrune/internal/format/helm"
	_ "github.com/packrune/packrune/internal/format/maven"
	_ "github.com/packrune/packrune/internal/format/npm"
	_ "github.com/packrune/packrune/internal/format/pypi"
	"github.com/packrune/packrune/internal/repo"
	"github.com/packrune/packrune/internal/storage/fs"
	"github.com/packrune/packrune/migrations"
)

type checkResult int

const (
	resOK checkResult = iota
	resWarn
	resFail
)

type check struct {
	name   string
	result checkResult
	detail string
}

func runDoctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := flags.String("config", "packrune.yaml", "path to config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	var checks []check

	checks = append(checks, check{
		name: "config", result: resOK,
		detail: fmt.Sprintf("loaded from %s; addr=%s db=%s storage=%s",
			*configPath, cfg.Server.Addr, cfg.Database.Driver, cfg.Storage.Backend),
	})
	checks = append(checks, check{
		name: "go runtime", result: resOK,
		detail: fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	})

	// Port availability
	if l, err := net.Listen("tcp", cfg.Server.Addr); err == nil {
		_ = l.Close()
		checks = append(checks, check{name: "port", result: resOK, detail: cfg.Server.Addr + " free"})
	} else {
		checks = append(checks, check{name: "port", result: resWarn,
			detail: cfg.Server.Addr + " busy (a packrune may already be running): " + err.Error()})
	}

	// DB connection + migrations + repo count
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if database, err := db.Open(ctx, cfg.Database); err == nil {
		defer database.Close()
		if err := database.ApplyMigrations(ctx, migrations.FS()); err != nil {
			checks = append(checks, check{name: "migrations", result: resFail, detail: err.Error()})
		} else {
			checks = append(checks, check{name: "migrations", result: resOK, detail: "schema current"})
			store := repo.NewStore(database)
			if repos, err := store.List(ctx); err == nil {
				detail := fmt.Sprintf("%d repositories", len(repos))
				if len(repos) == 0 {
					detail += " (none bootstrapped yet — they appear after the first `serve`)"
				}
				checks = append(checks, check{name: "repositories", result: resOK, detail: detail})
			} else {
				checks = append(checks, check{name: "repositories", result: resFail, detail: err.Error()})
			}
		}
	} else {
		checks = append(checks, check{name: "database", result: resFail, detail: err.Error()})
	}

	// Storage reachability + write probe
	if cfg.Storage.Backend == "fs" {
		if backend, err := fs.New(cfg.Storage.FS.Root); err == nil {
			testKey := ".doctor-probe-" + fmt.Sprintf("%d", time.Now().UnixNano())
			if _, err := backend.Put(ctx, testKey, strings.NewReader("ok")); err != nil {
				checks = append(checks, check{name: "storage", result: resFail,
					detail: "fs put failed: " + err.Error()})
			} else {
				_ = backend.Delete(ctx, testKey)
				checks = append(checks, check{name: "storage", result: resOK,
					detail: "fs read-write at " + cfg.Storage.FS.Root})
			}
		} else {
			checks = append(checks, check{name: "storage", result: resFail, detail: err.Error()})
		}
	} else {
		checks = append(checks, check{name: "storage", result: resWarn,
			detail: "backend=" + cfg.Storage.Backend + " — doctor only probes fs"})
	}

	// Format registry — every format we ship must be importable.
	formats := format.All()
	want := map[string]bool{"docker": true, "npm": true, "helm": true, "gomod": true, "pypi": true, "maven": true}
	names := []string{}
	for _, f := range formats {
		names = append(names, f.Name())
		delete(want, f.Name())
	}
	if len(want) == 0 {
		checks = append(checks, check{name: "formats", result: resOK,
			detail: strings.Join(names, ", ")})
	} else {
		missing := []string{}
		for k := range want {
			missing = append(missing, k)
		}
		checks = append(checks, check{name: "formats", result: resFail,
			detail: "missing: " + strings.Join(missing, ", ")})
	}

	// Render the report.
	worst := resOK
	for _, c := range checks {
		if c.result > worst {
			worst = c.result
		}
		fmt.Printf("  %s  %-14s  %s\n", icon(c.result), c.name, c.detail)
	}
	switch worst {
	case resOK:
		fmt.Fprintln(os.Stderr, "\ndoctor: all green.")
	case resWarn:
		fmt.Fprintln(os.Stderr, "\ndoctor: warnings present (above).")
	case resFail:
		fmt.Fprintln(os.Stderr, "\ndoctor: failures present (above).")
		return fmt.Errorf("doctor failed")
	}
	return nil
}

func icon(r checkResult) string {
	switch r {
	case resOK:
		return "✓"
	case resWarn:
		return "!"
	default:
		return "✗"
	}
}
