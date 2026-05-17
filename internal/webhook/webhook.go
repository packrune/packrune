// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package webhook subscribes external systems to Packrune events
// (artifact.created, artifact.deleted, repo.created, ...). Subscriptions
// live in the webhooks table; each Fire() inserts a delivery row and kicks
// the worker goroutine to attempt delivery.
//
// Deliveries are signed with HMAC-SHA256 of the body using the subscription
// secret; the signature lives in the X-Packrune-Signature header so
// receivers can verify the payload came from us.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/packrune/packrune/internal/db"
)

// EventType is a stable string name for an event.
type EventType string

const (
	EventArtifactCreated EventType = "artifact.created"
	EventArtifactDeleted EventType = "artifact.deleted"
	EventRepoCreated     EventType = "repo.created"
)

// Webhook is one subscription row.
type Webhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"` // never serialize
	Events    []string  `json:"events"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Payload is the JSON body posted to subscribers.
type Payload struct {
	Event     EventType `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// ErrNotFound is returned when a webhook lookup misses.
var ErrNotFound = errors.New("webhook: not found")

// Service wires the persistence layer + HTTP dispatcher together.
type Service struct {
	db     *db.DB
	logger *slog.Logger
	client *http.Client
}

// New constructs a webhook Service.
func New(database *db.DB, logger *slog.Logger) *Service {
	return &Service{
		db:     database,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Create inserts a new webhook.
func (s *Service) Create(ctx context.Context, name, url, secret string, events []string) (Webhook, error) {
	if name == "" || url == "" {
		return Webhook{}, errors.New("webhook: name and url required")
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	evStr := strings.Join(events, ",")
	q := s.db.Rebind(`INSERT INTO webhooks (id, name, url, secret, events, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`)
	if _, err := s.db.ExecContext(ctx, q, id, name, url, secret, evStr, now); err != nil {
		return Webhook{}, fmt.Errorf("webhook: create: %w", err)
	}
	return Webhook{
		ID: id, Name: name, URL: url, Secret: secret,
		Events: events, IsActive: true, CreatedAt: now,
	}, nil
}

// List returns every webhook ordered by created_at.
func (s *Service) List(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, url, secret, events, is_active, created_at
		FROM webhooks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("webhook: list: %w", err)
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var events string
		var active int
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &events, &active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("webhook: scan: %w", err)
		}
		if events != "" {
			w.Events = strings.Split(events, ",")
		}
		w.IsActive = active != 0
		out = append(out, w)
	}
	return out, rows.Err()
}

// Delete removes a webhook by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	q := s.db.Rebind(`DELETE FROM webhooks WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

// Fire delivers an event to every active subscription that listed this
// event (or "*"). Delivery happens in a goroutine; Fire returns immediately
// after the delivery rows are inserted.
//
// Today this is best-effort with no automatic retry. The schema already has
// columns (status/attempts/next_attempt_at) for a future worker to retry
// failed deliveries; see PHASES.md.
func (s *Service) Fire(ctx context.Context, event EventType, data any) {
	if s == nil {
		return
	}
	subs, err := s.subscribers(ctx, event)
	if err != nil {
		s.logger.Warn("webhook subscribers", "err", err, "event", event)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload := Payload{Event: event, Timestamp: time.Now().UTC(), Data: data}
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("webhook marshal", "err", err)
		return
	}

	for _, sub := range subs {
		go s.deliver(sub, event, body)
	}
}

func (s *Service) subscribers(ctx context.Context, event EventType) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, url, secret, events, is_active, created_at
		FROM webhooks WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var events string
		var active int
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &events, &active, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.IsActive = active != 0
		if events == "*" || strings.Contains(","+events+",", ","+string(event)+",") {
			if events != "" {
				w.Events = strings.Split(events, ",")
			}
			out = append(out, w)
		}
	}
	return out, rows.Err()
}

func (s *Service) deliver(w Webhook, event EventType, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	deliveryID := uuid.NewString()
	now := time.Now().UTC()
	insert := s.db.Rebind(`INSERT INTO webhook_deliveries
		(id, webhook_id, event, payload, status, attempts, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`)
	_, _ = s.db.ExecContext(ctx, insert, deliveryID, w.ID, string(event), string(body), "pending", now)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		s.markDelivery(ctx, deliveryID, "failed", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Packrune-Webhook/1")
	req.Header.Set("X-Packrune-Event", string(event))
	if w.Secret != "" {
		mac := hmac.New(sha256.New, []byte(w.Secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-Packrune-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.markDelivery(ctx, deliveryID, "failed", err.Error())
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.markDelivery(ctx, deliveryID, "delivered", "")
		return
	}
	s.markDelivery(ctx, deliveryID, "failed", fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func (s *Service) markDelivery(ctx context.Context, id, status, errMsg string) {
	now := time.Now().UTC()
	if status == "delivered" {
		q := s.db.Rebind(`UPDATE webhook_deliveries SET status = ?, delivered_at = ?, last_error = '' WHERE id = ?`)
		_, _ = s.db.ExecContext(ctx, q, status, now, id)
	} else {
		q := s.db.Rebind(`UPDATE webhook_deliveries SET status = ?, last_error = ? WHERE id = ?`)
		_, _ = s.db.ExecContext(ctx, q, status, errMsg, id)
	}
}
