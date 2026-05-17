// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Go modules proxy mode. Cache-misses on @v files fall through to an
// upstream GOPROXY (typically https://proxy.golang.org).

package gomod

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

// proxyFetchVersion fetches the upstream <module>/@v/<version><ext>, caches
// it, and returns the bytes.
func (h *Handler) proxyFetchVersion(ctx context.Context, module, version, ext string) ([]byte, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("gomod: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/" + module + "/@v/" + version + ext
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
		return nil, err
	}
	path := "modules/" + module + "/" + version + ext
	if err := h.store.UpsertArtifact(ctx, h.repoID, path,
		digest, int64(len(body)), contentTypeFor(ext), ""); err != nil {
		return nil, err
	}
	return body, nil
}
