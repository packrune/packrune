// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Maven proxy mode. Cache-miss GETs fall through to an upstream Maven
// repository (typically https://repo1.maven.org/maven2). Maven's layout
// makes proxying trivial: every file lives at a deterministic path, so we
// forward verbatim and cache the response. metadata XML is fetched fresh
// each time so version lists stay current (no caching).

package maven

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProxyConfig struct {
	Upstream string `json:"upstream"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func ParseProxyConfig(raw []byte) ProxyConfig {
	if len(raw) == 0 {
		return ProxyConfig{}
	}
	var cfg ProxyConfig
	_ = json.Unmarshal(raw, &cfg)
	return cfg
}

var proxyClient = &http.Client{Timeout: 60 * time.Second}

// proxyFetchFile fetches <upstream>/<path> and caches the response (unless
// path looks like maven-metadata.xml, which is fetched but not cached so
// version lists stay current).
func (h *Handler) proxyFetchFile(ctx context.Context, path string) ([]byte, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("maven: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if h.proxy.Username != "" {
		req.SetBasicAuth(h.proxy.Username, h.proxy.Password)
	}
	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Skip caching maven-metadata.xml so version lists pick up new uploads
	// upstream without us having to invalidate.
	if strings.HasSuffix(path, "/maven-metadata.xml") {
		return body, nil
	}

	digest, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := h.store.UpsertArtifact(ctx, h.repoID, "files/"+path,
		digest, int64(len(body)), contentTypeFor(path), ""); err != nil {
		return nil, err
	}
	return body, nil
}
