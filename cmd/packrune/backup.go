// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune backup` — produce a portable tar.gz of the SQLite database file
// and the local storage tree. Run while the server is *stopped* (or with a
// frozen volume) for a coherent snapshot.

package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/packrune/packrune/internal/config"
)

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	out := fs.String("output", "packrune-backup.tar.gz", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.Database.Driver != "sqlite" {
		return fmt.Errorf("backup currently supports sqlite only (driver=%s). "+
			"For Postgres, use pg_dump", cfg.Database.Driver)
	}
	if cfg.Storage.Backend != "fs" {
		return fmt.Errorf("backup currently supports fs storage only (backend=%s). "+
			"For S3-style backends, snapshot the bucket separately", cfg.Storage.Backend)
	}

	f, err := os.Create(*out)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// DB file
	if err := tarFile(tw, cfg.Database.DSN, "db.sqlite"); err != nil {
		return fmt.Errorf("tar db: %w", err)
	}
	// SQLite WAL + SHM (if present) — needed for consistency
	for _, suffix := range []string{"-wal", "-shm"} {
		p := cfg.Database.DSN + suffix
		if _, err := os.Stat(p); err == nil {
			if err := tarFile(tw, p, "db.sqlite"+suffix); err != nil {
				return fmt.Errorf("tar db%s: %w", suffix, err)
			}
		}
	}
	// Storage tree — tolerate a brand-new install where storage/ doesn't
	// exist yet (no artifacts pushed). Backup still captures the DB.
	if _, err := os.Stat(cfg.Storage.FS.Root); err == nil {
		if err := tarDir(tw, cfg.Storage.FS.Root, "storage"); err != nil {
			return fmt.Errorf("tar storage: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "wrote backup to %s\n", *out)
	return nil
}

func tarFile(tw *tar.Writer, srcPath, name string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    name,
		Mode:    int64(info.Mode().Perm()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func tarDir(tw *tar.Writer, root, prefix string) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		hdr := &tar.Header{
			Name:    name,
			Mode:    int64(info.Mode().Perm()),
			ModTime: info.ModTime(),
		}
		if info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		hdr.Typeflag = tar.TypeReg
		hdr.Size = info.Size()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
