// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune gc` — sweep CAS for blobs nothing in the artifacts table
// references, then delete them. Safe to run while the server is up: a fresh
// upload that races us still gets recorded in the DB after we've snapshotted
// the reference set; worst case its blob survives until the next GC.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/internal/storage/cas"
	"github.com/packrune/packrune/internal/storage/fs"
)

func runGC(args []string) error {
	flags := flag.NewFlagSet("gc", flag.ContinueOnError)
	configPath := flags.String("config", "packrune.yaml", "path to config file")
	dryRun := flags.Bool("dry-run", false, "list what would be deleted without doing it")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.Storage.Backend != "fs" {
		return fmt.Errorf("gc currently supports fs storage only (backend=%s)", cfg.Storage.Backend)
	}

	ctx := context.Background()
	database, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer database.Close()

	backend, err := fs.New(cfg.Storage.FS.Root)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	_ = cas.New(backend) // import for side; we walk via backend directly

	// Snapshot the digest reference set from the DB.
	rows, err := database.QueryContext(ctx, `SELECT DISTINCT digest FROM artifacts`)
	if err != nil {
		return fmt.Errorf("query digests: %w", err)
	}
	defer rows.Close()

	referenced := map[string]struct{}{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return fmt.Errorf("scan digest: %w", err)
		}
		referenced[d] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Walk every blob key under "blobs/" and delete the orphans.
	var (
		scanned    int
		orphans    int
		freedBytes int64
		token      string
	)
	for {
		page, err := backend.List(ctx, "blobs/", token, 1000)
		if err != nil {
			return fmt.Errorf("list blobs: %w", err)
		}
		for _, key := range page.Keys {
			scanned++
			// key shape: blobs/sha256/<2hex>/<64hex>
			parts := strings.Split(key, "/")
			if len(parts) < 4 {
				continue
			}
			alg := parts[1]
			hex := parts[3]
			digest := alg + ":" + hex
			if _, ok := referenced[digest]; ok {
				continue
			}
			// Orphan; size first, then delete.
			st, err := backend.Stat(ctx, key)
			if err == nil {
				freedBytes += st.Size
			}
			orphans++
			if !*dryRun {
				if err := backend.Delete(ctx, key); err != nil {
					fmt.Fprintf(os.Stderr, "warn: delete %s: %v\n", key, err)
				}
			}
		}
		if page.NextToken == "" {
			break
		}
		token = page.NextToken
	}

	verb := "deleted"
	if *dryRun {
		verb = "would delete"
	}
	fmt.Printf("gc: scanned %d blobs, %s %d orphans (%s freed)\n",
		scanned, verb, orphans, humanBytes(freedBytes))
	return nil
}

func humanBytes(n int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)
	switch {
	case n < KB:
		return fmt.Sprintf("%d B", n)
	case n < MB:
		return fmt.Sprintf("%.1f KB", float64(n)/KB)
	case n < GB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	}
}
