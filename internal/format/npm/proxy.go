// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// npm proxy mode. Cache-misses on packuments and tarballs fall through to
// the configured upstream (typically https://registry.npmjs.org), and the
// response is stored before being served to the client.

package npm

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

// ProxyConfig is parsed from repository.Config JSON. Empty Upstream means
// no proxy.
type ProxyConfig struct {
	Upstream string `json:"upstream"`
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

func (h *Handler) proxyFetchPackument(ctx context.Context, pkg string) ([]byte, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("npm: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/" + pkg
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
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
	// Store under packuments/<pkg>.
	digest, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cache packument: %w", err)
	}
	if err := h.store.UpsertArtifact(ctx, h.repoID,
		"packuments/"+pkg, digest, int64(len(body)),
		"application/json", ""); err != nil {
		return nil, err
	}
	return body, nil
}

func (h *Handler) proxyFetchTarball(ctx context.Context, pkg, filename string) ([]byte, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("npm: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/" + pkg + "/-/" + filename
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
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
	digest, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cache tarball: %w", err)
	}
	if err := h.store.UpsertArtifact(ctx, h.repoID,
		"tarballs/"+pkg+"/"+filename, digest, int64(len(body)),
		"application/octet-stream", ""); err != nil {
		return nil, err
	}
	return body, nil
}
