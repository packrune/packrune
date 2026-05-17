-- SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
-- Copyright (C) 2026 Packrune Contributors
--
-- Initial schema. Compatible with SQLite and Postgres without dialect-specific
-- syntax. Times are stored as ISO-8601 strings via DEFAULT CURRENT_TIMESTAMP,
-- which both engines understand; the Go code parses them through time.Time's
-- standard layouts.

CREATE TABLE users (
    id            TEXT NOT NULL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    username      TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tokens (
    id           TEXT NOT NULL PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    hash         TEXT NOT NULL UNIQUE,
    prefix       TEXT NOT NULL,
    scopes       TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMP,
    expires_at   TIMESTAMP,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tokens_user_id ON tokens (user_id);

CREATE TABLE repositories (
    id         TEXT NOT NULL PRIMARY KEY,
    name       TEXT NOT NULL,
    format     TEXT NOT NULL,
    kind       TEXT NOT NULL,
    config     TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (name, format)
);

CREATE TABLE artifacts (
    id         TEXT NOT NULL PRIMARY KEY,
    repo_id    TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    digest     TEXT NOT NULL,
    size       INTEGER NOT NULL,
    media_type TEXT NOT NULL DEFAULT '',
    metadata   TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (repo_id, path)
);

CREATE INDEX idx_artifacts_digest ON artifacts (digest);
CREATE INDEX idx_artifacts_repo_id ON artifacts (repo_id);

CREATE TABLE permissions (
    id         TEXT NOT NULL PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_id    TEXT REFERENCES repositories(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, repo_id, role)
);

CREATE INDEX idx_permissions_user_id ON permissions (user_id);
CREATE INDEX idx_permissions_repo_id ON permissions (repo_id);

CREATE TABLE audit_log (
    id          TEXT NOT NULL PRIMARY KEY,
    user_id     TEXT,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id   TEXT NOT NULL DEFAULT '',
    result      TEXT NOT NULL,
    metadata    TEXT NOT NULL DEFAULT '{}',
    remote_addr TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_log_user_id_created_at ON audit_log (user_id, created_at);
CREATE INDEX idx_audit_log_action_created_at  ON audit_log (action, created_at);

-- schema_migrations tracks which migrations have run. golang-migrate-compatible
-- name and shape so we can adopt that tool later without a schema migration.
CREATE TABLE schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty   BOOLEAN NOT NULL
);
