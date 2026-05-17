-- SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
-- Copyright (C) 2026 Packrune Contributors
--
-- Webhook subscriptions. Per-event delivery state lives in a separate table
-- so retries and queue depth don't bloat the subscription row.

CREATE TABLE webhooks (
    id         TEXT NOT NULL PRIMARY KEY,
    name       TEXT NOT NULL,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL DEFAULT '',
    events     TEXT NOT NULL DEFAULT '',  -- comma-separated; "*" matches all
    is_active  INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhooks_active ON webhooks (is_active);

CREATE TABLE webhook_deliveries (
    id              TEXT NOT NULL PRIMARY KEY,
    webhook_id      TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event           TEXT NOT NULL,
    payload         TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending|delivered|failed
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMP,
    delivered_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries (status, next_attempt_at);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at);
