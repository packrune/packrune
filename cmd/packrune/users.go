// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// `packrune users` — out-of-band user management. Today the only subcommand
// is `add`, which bootstraps the first admin. Future subcommands (list,
// passwd, deactivate) will live here so the UI is not a hard dependency for
// operations.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/migrations"
)

func runUsers(args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  packrune users add --email E --username U [--admin] [--password P]`)
		return fmt.Errorf("missing subcommand")
	}
	switch args[0] {
	case "add":
		return runUsersAdd(args[1:])
	default:
		return fmt.Errorf("unknown users subcommand %q", args[0])
	}
}

func runUsersAdd(args []string) error {
	fs := flag.NewFlagSet("users add", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	email := fs.String("email", "", "email address (required)")
	username := fs.String("username", "", "username (required)")
	admin := fs.Bool("admin", false, "grant admin role")
	password := fs.String("password", "", "password (optional; prompts if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *username == "" {
		fs.Usage()
		return fmt.Errorf("--email and --username are required")
	}

	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	pw := *password
	if pw == "" {
		pw, err = promptPassword()
		if err != nil {
			return err
		}
	}

	ctx := context.Background()
	database, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer database.Close()
	if err := database.ApplyMigrations(ctx, migrations.FS()); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	svc := auth.NewDBService(database)
	u, err := svc.CreateUser(ctx, *email, *username, pw, *admin)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	fmt.Printf("created user %s (id=%s, admin=%v)\n", u.Username, u.ID, u.IsAdmin)
	return nil
}

func promptPassword() (string, error) {
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Print("Password: ")
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	// Not a TTY: read one line from stdin (useful for piping in tests/CI).
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
