// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package config loads, validates, and exposes runtime configuration.
//
// Sources are layered (later overrides earlier):
//
//  1. Built-in defaults (see Defaults).
//  2. A YAML file at the path passed to Load, if it exists.
//  3. Environment variables named PACKRUNE_<SECTION>_<KEY>, where SECTION and
//     KEY are the uppercased yaml tags. For example PACKRUNE_SERVER_ADDR or
//     PACKRUNE_STORAGE_FS_ROOT.
//
// We deliberately avoid Viper and its transitive dependencies; the YAML +
// reflection-based env override is short enough to read in one sitting.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the full runtime configuration for a Packrune process.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	Log      LogConfig      `yaml:"log"`
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string `yaml:"addr"`
	// ExternalURL is the externally-visible URL of this instance, used for
	// redirects and Docker auth realm hints.
	ExternalURL string `yaml:"external_url"`
	// ReadTimeoutSeconds limits request read time. 0 disables.
	ReadTimeoutSeconds int `yaml:"read_timeout"`
	// WriteTimeoutSeconds limits response write time. 0 disables — blob
	// uploads can legitimately take a long time and we never want to cut them.
	WriteTimeoutSeconds int `yaml:"write_timeout"`
}

// DatabaseConfig configures the metadata store.
type DatabaseConfig struct {
	// Driver is "sqlite" or "postgres".
	Driver string `yaml:"driver"`
	// DSN is the connection string. For sqlite this is a file path; for
	// postgres a "postgres://user:pass@host:port/db?..." URL.
	DSN string `yaml:"dsn"`
	// MaxOpenConns / MaxIdleConns are passed through to database/sql.
	// Ignored for sqlite which is single-writer.
	MaxOpenConns int `yaml:"max_open_conns"`
	MaxIdleConns int `yaml:"max_idle_conns"`
}

// StorageConfig configures the blob storage backend.
type StorageConfig struct {
	// Backend selects a backend: "fs" or "s3".
	Backend string   `yaml:"backend"`
	FS      FSConfig `yaml:"fs"`
	S3      S3Config `yaml:"s3"`
}

// FSConfig configures the filesystem backend.
type FSConfig struct {
	// Root is the directory under which all blobs live.
	Root string `yaml:"root"`
}

// S3Config configures the S3-compatible backend.
type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	// UsePath toggles path-style addressing. MinIO requires this.
	UsePath bool `yaml:"use_path"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	// TokenSecret is the HMAC key used to sign session tokens. Must be at
	// least 32 bytes; generated automatically at first startup if blank.
	TokenSecret string `yaml:"token_secret"`
	// SessionTTLHours is how long a login session lasts.
	SessionTTLHours int `yaml:"session_ttl"`
	// AllowSignup controls whether unauthenticated users can self-register.
	// Defaults to false; admins create accounts.
	AllowSignup bool `yaml:"allow_signup"`
}

// LogConfig configures structured logging.
type LogConfig struct {
	// Level: "debug", "info", "warn", "error".
	Level string `yaml:"level"`
	// Format: "text" (human) or "json" (machine).
	Format string `yaml:"format"`
}

// Defaults returns a Config populated with sensible defaults for a single-node
// "just run the binary" deploy.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:                ":8080",
			ExternalURL:         "http://localhost:8080",
			ReadTimeoutSeconds:  60,
			WriteTimeoutSeconds: 0,
		},
		Database: DatabaseConfig{
			Driver:       "sqlite",
			DSN:          "data/packrune.db",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
		},
		Storage: StorageConfig{
			Backend: "fs",
			FS:      FSConfig{Root: "data/storage"},
		},
		Auth: AuthConfig{
			SessionTTLHours: 24,
			AllowSignup:     false,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load builds a Config by layering defaults, an optional YAML file, and
// environment-variable overrides. A missing file at path is not an error
// unless required is true.
func Load(path string, required bool) (Config, error) {
	cfg := Defaults()

	if path != "" {
		data, err := os.ReadFile(filepath.Clean(path))
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config %q: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist) && !required:
			// fine — fall through to env overrides
		default:
			return cfg, fmt.Errorf("read config %q: %w", path, err)
		}
	}

	applyEnv(reflect.ValueOf(&cfg).Elem(), "PACKRUNE")

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate returns the first invariant violation it finds.
func (c Config) Validate() error {
	if c.Server.Addr == "" {
		return errors.New("server.addr is required")
	}
	switch c.Database.Driver {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("database.driver: unknown driver %q (want sqlite|postgres)", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return errors.New("database.dsn is required")
	}
	switch c.Storage.Backend {
	case "fs":
		if c.Storage.FS.Root == "" {
			return errors.New("storage.fs.root is required when backend=fs")
		}
	case "s3":
		if c.Storage.S3.Bucket == "" {
			return errors.New("storage.s3.bucket is required when backend=s3")
		}
	default:
		return fmt.Errorf("storage.backend: unknown backend %q", c.Storage.Backend)
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level: unknown level %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format: unknown format %q", c.Log.Format)
	}
	return nil
}

// applyEnv walks the Config struct via reflection and overrides any field
// whose corresponding PACKRUNE_... environment variable is set. Adding a new
// config field requires no change here, as long as the field carries a yaml
// tag.
func applyEnv(v reflect.Value, prefix string) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		ft := t.Field(i)
		tag := strings.SplitN(ft.Tag.Get("yaml"), ",", 2)[0]
		if tag == "" || tag == "-" {
			continue
		}
		envKey := prefix + "_" + strings.ToUpper(tag)

		if f.Kind() == reflect.Struct {
			applyEnv(f, envKey)
			continue
		}

		val, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}
		setFieldFromString(f, val)
	}
}

func setFieldFromString(f reflect.Value, s string) {
	switch f.Kind() {
	case reflect.String:
		f.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.SetInt(n)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			f.SetBool(b)
		}
	}
}
