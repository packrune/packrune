// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package audit writes structured audit records to the audit_log table. Every
// authentication decision and every write-side action should produce one
// record so operators can answer "who did what, when".
//
// The writer is intentionally fire-and-forget at the HTTP layer: a failed
// audit write logs an error but does not break the user-visible operation.
// Audit availability should not compromise system availability.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/packrune/packrune/internal/db"
)

// Result is the outcome of an audited action.
type Result string

const (
	ResultAllow Result = "allow"
	ResultDeny  Result = "deny"
	ResultOK    Result = "ok"
	ResultError Result = "error"
)

// Event captures one auditable thing that happened.
type Event struct {
	// UserID is the actor; "" for anonymous.
	UserID string
	// Action is a short verb in lowercase ("login", "token.issue",
	// "repo.create", "blob.upload", ...).
	Action string
	// TargetType / TargetID identify what the action affected.
	TargetType string
	TargetID   string
	// Result reports whether the action was allowed/denied/ok/error.
	Result Result
	// Metadata is free-form structured detail.
	Metadata map[string]any
	// RemoteAddr is the client IP if known.
	RemoteAddr string
}

// Writer persists Events.
type Writer struct {
	db *db.DB
}

// NewWriter constructs a Writer.
func NewWriter(database *db.DB) *Writer {
	return &Writer{db: database}
}

// Write persists e. A nil Writer or nil db silently no-ops, which keeps tests
// and bootstrap paths simple.
func (w *Writer) Write(ctx context.Context, e Event) error {
	if w == nil || w.db == nil {
		return nil
	}
	md := "{}"
	if len(e.Metadata) > 0 {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("audit: marshal metadata: %w", err)
		}
		md = string(b)
	}

	var userID any
	if e.UserID != "" {
		userID = e.UserID
	}

	id := uuid.NewString()
	q := w.db.Rebind(`INSERT INTO audit_log
		(id, user_id, action, target_type, target_id, result, metadata, remote_addr, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if _, err := w.db.ExecContext(ctx, q,
		id, userID, e.Action, e.TargetType, e.TargetID, string(e.Result),
		md, e.RemoteAddr, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}
