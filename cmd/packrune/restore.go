// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune restore` — inverse of backup. Refuses to overwrite an existing
// database file unless --force is passed, so an accidental restore can't
// silently destroy a running install.

package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/packrune/packrune/internal/config"
)

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	in := fs.String("input", "", "input .tar.gz file (required)")
	force := fs.Bool("force", false, "overwrite an existing database / storage tree")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return errors.New("--input FILE is required")
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.Database.Driver != "sqlite" {
		return fmt.Errorf("restore currently supports sqlite only (driver=%s)", cfg.Database.Driver)
	}
	if cfg.Storage.Backend != "fs" {
		return fmt.Errorf("restore currently supports fs storage only (backend=%s)", cfg.Storage.Backend)
	}

	if _, err := os.Stat(cfg.Database.DSN); err == nil && !*force {
		return fmt.Errorf("database %s already exists; pass --force to overwrite", cfg.Database.DSN)
	}

	f, err := os.Open(*in)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	if err := os.MkdirAll(filepath.Dir(cfg.Database.DSN), 0o755); err != nil {
		return fmt.Errorf("mkdir db parent: %w", err)
	}
	if err := os.MkdirAll(cfg.Storage.FS.Root, 0o755); err != nil {
		return fmt.Errorf("mkdir storage root: %w", err)
	}

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		var dst string
		switch {
		case hdr.Name == "db.sqlite":
			dst = cfg.Database.DSN
		case hdr.Name == "db.sqlite-wal":
			dst = cfg.Database.DSN + "-wal"
		case hdr.Name == "db.sqlite-shm":
			dst = cfg.Database.DSN + "-shm"
		case strings.HasPrefix(hdr.Name, "storage/"):
			rel := strings.TrimPrefix(hdr.Name, "storage/")
			dst = filepath.Join(cfg.Storage.FS.Root, filepath.FromSlash(rel))
		default:
			continue // unknown entry, skip safely
		}

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dst, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir parent of %s: %w", dst, err)
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("create %s: %w", dst, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return fmt.Errorf("write %s: %w", dst, err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dst, err)
		}
	}

	fmt.Fprintf(os.Stderr, "restored from %s\n", *in)
	return nil
}
