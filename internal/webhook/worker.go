// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Retry worker for failed webhook deliveries. Runs in the background, picks
// up rows where status=failed and next_attempt_at <= now (or NULL), retries,
// and applies exponential backoff up to a cap.

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxAttempts     = 6
	pollInterval    = 30 * time.Second
	deliveryTimeout = 15 * time.Second
)

// backoff returns the delay to wait before attempt n (1-indexed).
//
//	1 → 1m, 2 → 2m, 3 → 4m, 4 → 8m, 5 → 16m, 6 → 32m
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}

// StartWorker launches the retry loop. Returns immediately; the goroutine
// runs until ctx is canceled. Call this once at server startup.
func (s *Service) StartWorker(ctx context.Context) {
	if s == nil {
		return
	}
	go s.workerLoop(ctx)
}

func (s *Service) workerLoop(ctx context.Context) {
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.runOnce(ctx)
		}
	}
}

type pendingRow struct {
	deliveryID string
	webhookID  string
	event      string
	payload    string
	attempts   int
	webhookURL string
	secret     string
}

func (s *Service) runOnce(ctx context.Context) {
	now := time.Now().UTC()
	q := s.db.Rebind(`SELECT d.id, d.webhook_id, d.event, d.payload, d.attempts, w.url, w.secret
		FROM webhook_deliveries d
		INNER JOIN webhooks w ON w.id = d.webhook_id
		WHERE d.status = 'failed'
		  AND d.attempts < ?
		  AND (d.next_attempt_at IS NULL OR d.next_attempt_at <= ?)
		ORDER BY d.created_at ASC
		LIMIT 50`)
	rows, err := s.db.QueryContext(ctx, q, maxAttempts, now)
	if err != nil {
		s.logger.Warn("webhook: scan pending", "err", err)
		return
	}
	defer rows.Close()

	var pending []pendingRow
	for rows.Next() {
		var p pendingRow
		if err := rows.Scan(&p.deliveryID, &p.webhookID, &p.event, &p.payload,
			&p.attempts, &p.webhookURL, &p.secret); err != nil {
			s.logger.Warn("webhook: scan", "err", err)
			continue
		}
		pending = append(pending, p)
	}
	_ = rows.Err()

	for _, p := range pending {
		s.retryOne(ctx, p)
	}
}

func (s *Service) retryOne(ctx context.Context, p pendingRow) {
	dctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	body := []byte(p.payload)
	req, err := http.NewRequestWithContext(dctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		s.markRetry(ctx, p, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Packrune-Webhook/1")
	req.Header.Set("X-Packrune-Event", p.event)
	req.Header.Set("X-Packrune-Delivery-Retry", fmt.Sprintf("%d", p.attempts+1))
	if p.secret != "" {
		mac := hmac.New(sha256.New, []byte(p.secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-Packrune-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.markRetry(ctx, p, err.Error())
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now().UTC()
		q := s.db.Rebind(`UPDATE webhook_deliveries
			SET status = 'delivered', attempts = attempts + 1, delivered_at = ?, last_error = ''
			WHERE id = ?`)
		if _, err := s.db.ExecContext(ctx, q, now, p.deliveryID); err != nil {
			s.logger.Warn("webhook: mark delivered", "err", err)
		}
		return
	}
	s.markRetry(ctx, p, fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func (s *Service) markRetry(ctx context.Context, p pendingRow, errMsg string) {
	attempts := p.attempts + 1
	nextAttempt := sql.NullTime{Time: time.Now().UTC().Add(backoff(attempts)), Valid: true}

	var q string
	if attempts >= maxAttempts {
		// Give up — leave status=failed, clear next_attempt_at so the worker
		// stops picking it up.
		q = s.db.Rebind(`UPDATE webhook_deliveries
			SET attempts = ?, last_error = ?, next_attempt_at = NULL
			WHERE id = ?`)
		if _, err := s.db.ExecContext(ctx, q, attempts, errMsg, p.deliveryID); err != nil {
			s.logger.Warn("webhook: mark giveup", "err", err)
		}
		return
	}
	q = s.db.Rebind(`UPDATE webhook_deliveries
		SET attempts = ?, last_error = ?, next_attempt_at = ?
		WHERE id = ?`)
	if _, err := s.db.ExecContext(ctx, q, attempts, errMsg, nextAttempt, p.deliveryID); err != nil {
		s.logger.Warn("webhook: mark retry", "err", err)
	}
}
