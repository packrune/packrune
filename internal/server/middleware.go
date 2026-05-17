// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// HTTP middleware: request IDs, panic recovery, structured access logs.
// Everything here is small and self-contained — middleware that needs heavy
// machinery belongs in its own package.

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

// RequestID injects a short random ID into each request context and echoes it
// in the X-Request-ID response header. Idempotent: if the caller already sent
// X-Request-ID it is preserved.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom extracts the request ID injected by RequestID, or "" if none.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// RecoverPanic converts a panicking handler into a 500 response and logs the
// stack. Without this a single bad code path takes the whole process down.
func RecoverPanic(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic in handler",
						"err", rec,
						"path", r.URL.Path,
						"method", r.Method,
						"request_id", RequestIDFrom(r.Context()),
						"stack", string(debug.Stack()),
					)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// LogRequests emits a structured log line per request once the response is
// flushed. Quiet on health endpoints because they spam at 1Hz.
func LogRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				return
			}
			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"bytes", rw.written,
				"dur_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
				"request_id", RequestIDFrom(r.Context()),
			)
		})
	}
}

// statusRecorder wraps http.ResponseWriter so the access log can see the
// status code and byte count after the handler returns.
type statusRecorder struct {
	http.ResponseWriter
	status    int
	written   int
	wroteHead bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHead {
		return
	}
	s.status = code
	s.wroteHead = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHead {
		s.WriteHeader(http.StatusOK)
	}
	n, err := s.ResponseWriter.Write(b)
	s.written += n
	return n, err
}
