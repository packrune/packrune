// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Helm proxy mode. Chart download misses fall through to upstream and are
// cached locally. The index.yaml is regenerated from cached metadata, so a
// proxy repo only reflects charts that have actually been fetched. For a
// canonical upstream index, point clients at the upstream directly.

package helm

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

// ProxyConfig is parsed from a Repository's stored Config JSON.
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

// proxyFetchChart pulls <upstream>/<filename> and caches it as a chart artifact.
// Returns the raw chart bytes.
func (h *Handler) proxyFetchChart(ctx context.Context, filename string) ([]byte, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("helm: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/" + filename
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

	meta, err := parseChart(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upstream chart: %w", err)
	}
	digest, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	metaJSON, _ := json.Marshal(meta)
	chartPath := "charts/" + meta.Name + "/" + meta.Name + "-" + meta.Version + ".tgz"
	if err := h.store.UpsertArtifact(ctx, h.repoID, chartPath,
		digest, int64(len(body)), "application/octet-stream", string(metaJSON)); err != nil {
		return nil, err
	}
	return body, nil
}
