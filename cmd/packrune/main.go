// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Command packrune is the Packrune artifact repository manager.
//
// Without arguments it starts the server. Subcommands are available for
// out-of-band operations:
//
//	packrune serve              # start the HTTP server (default)
//	packrune users add ...      # create a user
//	packrune --version          # print version
//	packrune --help             # show help
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Build-time variables, injected by ldflags. See Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "packrune: %v\n", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		return runServe(args)
	}
	switch args[0] {
	case "-v", "--version", "version":
		printVersion()
		return nil
	case "-h", "--help", "help":
		printHelp()
		return nil
	case "serve":
		return runServe(args[1:])
	case "users":
		return runUsers(args[1:])
	case "backup":
		return runBackup(args[1:])
	default:
		// Treat unknown leading args as flags to the default serve command.
		// `packrune --config foo.yaml` should still work.
		if len(args[0]) > 0 && args[0][0] == '-' {
			return runServe(args)
		}
		printHelp()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printVersion() {
	fmt.Printf("packrune %s (commit %s, built %s)\n", version, commit, date)
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("  go: %s\n", info.GoVersion)
	}
}

func printHelp() {
	fmt.Println(`Packrune — universal artifact repository manager

Usage:
  packrune [--config FILE]              start the HTTP server
  packrune serve [--config FILE]        same as above, explicit
  packrune users add --email E --username U [--admin]
                                        create a user (prompts for password)
  packrune backup [--output FILE]       snapshot SQLite + fs storage to .tar.gz
  packrune --version                    print version
  packrune --help                       this message

Configuration is loaded from the file given to --config (default
packrune.yaml in the working directory). Any field can be overridden via
PACKRUNE_<SECTION>_<KEY> environment variables.

License: AGPL-3.0 + Commons Clause — free to self-host, never for sale.`)
}
