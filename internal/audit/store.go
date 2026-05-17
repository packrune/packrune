// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Read-side audit log access. The Writer in audit.go pumps rows in; this
// file lets the admin UI page through them.

package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/packrune/packrune/internal/db"
)

// Record is one row of the audit log.
type Record struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id,omitempty"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type,omitempty"`
	TargetID   string    `json:"target_id,omitempty"`
	Result     string    `json:"result"`
	Metadata   string    `json:"metadata"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Reader exposes paged reads of audit_log.
type Reader struct {
	db *db.DB
}

// NewReader constructs a Reader.
func NewReader(database *db.DB) *Reader { return &Reader{db: database} }

// List returns the most-recent `limit` audit records before `before` (set
// `before` to a zero time for the first page).
func (r *Reader) List(ctx context.Context, before time.Time, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if before.IsZero() {
		q := `SELECT id, user_id, action, target_type, target_id, result, metadata, remote_addr, created_at
			FROM audit_log ORDER BY created_at DESC LIMIT ?`
		rows, err = r.db.QueryContext(ctx, r.db.Rebind(q), limit)
	} else {
		q := `SELECT id, user_id, action, target_type, target_id, result, metadata, remote_addr, created_at
			FROM audit_log WHERE created_at < ? ORDER BY created_at DESC LIMIT ?`
		rows, err = r.db.QueryContext(ctx, r.db.Rebind(q), before, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var userID sql.NullString
		if err := rows.Scan(&rec.ID, &userID, &rec.Action, &rec.TargetType, &rec.TargetID,
			&rec.Result, &rec.Metadata, &rec.RemoteAddr, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit: scan: %w", err)
		}
		if userID.Valid {
			rec.UserID = userID.String
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
