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

	"golang.org/x/term"

	"github.com/packrune/packrune/internal/auth"
	"github.com/packrune/packrune/internal/config"
	"github.com/packrune/packrune/internal/db"
	"github.com/packrune/packrune/migrations"
)

func runUsers(args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  packrune users add --email E --username U [--admin] [--password P]
  packrune users list
  packrune users passwd --username U [--password P]`)
		return fmt.Errorf("missing subcommand")
	}
	switch args[0] {
	case "add":
		return runUsersAdd(args[1:])
	case "list", "ls":
		return runUsersList(args[1:])
	case "passwd", "password":
		return runUsersPasswd(args[1:])
	default:
		return fmt.Errorf("unknown users subcommand %q", args[0])
	}
}

func runUsersList(args []string) error {
	fs := flag.NewFlagSet("users list", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath, false)
	if err != nil {
		return fmt.Errorf("config: %w", err)
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
	users, err := auth.NewDBService(database).ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		fmt.Println("(no users — create one with `packrune users add`)")
		return nil
	}
	fmt.Printf("%-26s  %-22s  %-7s  %-7s  %s\n", "USERNAME", "EMAIL", "ADMIN", "ACTIVE", "CREATED")
	for _, u := range users {
		fmt.Printf("%-26s  %-22s  %-7v  %-7v  %s\n",
			u.Username, u.Email, u.IsAdmin, u.IsActive, u.CreatedAt.Format("2006-01-02 15:04"))
	}
	return nil
}

func runUsersPasswd(args []string) error {
	fs := flag.NewFlagSet("users passwd", flag.ContinueOnError)
	configPath := fs.String("config", "packrune.yaml", "path to config file")
	username := fs.String("username", "", "username or email (required)")
	password := fs.String("password", "", "new password (prompts if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		fs.Usage()
		return fmt.Errorf("--username (or email) is required")
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
	if err := svc.SetPasswordByUsername(ctx, *username, pw); err != nil {
		return err
	}
	fmt.Printf("password updated for %s (existing sessions invalidated)\n", *username)
	return nil
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
	// os.Stdin.Fd() is uintptr on every platform; cast to int for x/term.
	// On Windows syscall.Stdin is a Handle (uintptr), so going through
	// os.Stdin.Fd() keeps this portable.
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Print("Password: ")
		b, err := term.ReadPassword(fd)
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
