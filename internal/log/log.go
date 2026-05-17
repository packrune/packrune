// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package log wires up the standard library's log/slog logger to Packrune's
// configuration. There is intentionally no separate logging framework — slog
// covers what we need and one fewer dependency is one fewer thing to learn.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New constructs a *slog.Logger from a level and format. Level is one of
// "debug" | "info" | "warn" | "error". Format is "text" or "json".
func New(out io.Writer, level, format string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: lvl, AddSource: lvl <= slog.LevelDebug}

	var h slog.Handler
	switch strings.ToLower(format) {
	case "json":
		h = slog.NewJSONHandler(out, opts)
	case "text", "":
		h = slog.NewTextHandler(out, opts)
	default:
		return nil, fmt.Errorf("log: unknown format %q", format)
	}
	return slog.New(h), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log: unknown level %q", s)
	}
}
