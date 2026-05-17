// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Liveness, readiness, and version endpoints. Kept dead-simple by design —
// load balancers and orchestrators do not need clever logic here.

package server

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// HandleHealth is the liveness probe. It returns 200 OK if the process is
// running. Do not add dependencies here — if Postgres is down we still want
// the LB to see us alive (use /readyz for "ready to serve traffic").
func HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// HandleReady is the readiness probe. Once dependency checks (DB ping,
// storage reachable) are wired in, this will reflect them.
func HandleReady(w http.ResponseWriter, _ *http.Request) {
	// TODO(faz1): replace with real DB + storage probes once those are wired.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// HandleVersion returns build info as JSON.
func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	resp := struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
		Go      string `json:"go"`
	}{
		Version: serverVersion,
		Commit:  serverCommit,
		Date:    serverDate,
		Go:      runtime.Version(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// These vars are set from main via the build-time ldflags. The /version
// endpoint mirrors what `packrune --version` prints.
var (
	serverVersion = "dev"
	serverCommit  = "none"
	serverDate    = "unknown"
)

// SetVersionInfo lets main.go push build info into this package without
// creating an import cycle.
func SetVersionInfo(version, commit, date string) {
	serverVersion = version
	serverCommit = commit
	serverDate = date
}
