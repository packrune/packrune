// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Docker proxy mode. When a Packrune docker repository is configured with
// kind=proxy + an upstream URL, cache-miss requests fall through to the
// upstream, the response is stored in CAS, and subsequent requests hit
// local storage.
//
// Today this supports anonymous upstreams (Docker Hub public images,
// ghcr.io public, quay.io public). Authenticated upstreams require a
// bearer-token negotiation that we'll add in Faz 8 polish.

package docker

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

// ProxyConfig is parsed from a repository's stored Config JSON. An empty
// Upstream disables proxy behavior. Username/Password, when set, are sent
// as HTTP Basic auth (sufficient for ghcr.io with a PAT and most
// enterprise registries; Docker Hub's full bearer-token negotiation is
// deferred).
type ProxyConfig struct {
	Upstream string `json:"upstream"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ParseProxyConfig extracts ProxyConfig from a repository's Config JSON.
// Returns the zero value (no proxy) on empty or malformed input.
func ParseProxyConfig(raw []byte) ProxyConfig {
	if len(raw) == 0 {
		return ProxyConfig{}
	}
	var cfg ProxyConfig
	_ = json.Unmarshal(raw, &cfg)
	return cfg
}

// proxyClient is the HTTP client used for upstream fetches. Short timeouts
// keep us responsive when an upstream is misbehaving.
var proxyClient = &http.Client{Timeout: 60 * time.Second}

// proxyFetch performs an authenticated-or-anonymous GET against the upstream
// and returns the response. Caller must close the body. Returns an error
// (not a 404 body) on non-2xx.
func (h *Handler) proxyFetch(ctx context.Context, urlPath string, accept []string) (*http.Response, error) {
	if h.proxy.Upstream == "" {
		return nil, fmt.Errorf("docker: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + urlPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for _, a := range accept {
		req.Header.Add("Accept", a)
	}
	if h.proxy.Username != "" {
		req.SetBasicAuth(h.proxy.Username, h.proxy.Password)
	}
	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("upstream %s: HTTP %d", url, resp.StatusCode)
	}
	return resp, nil
}

// proxyFetchBlob fetches a blob from upstream, stores it in CAS, and records
// the per-image binding so subsequent local lookups hit.
func (h *Handler) proxyFetchBlob(ctx context.Context, name, digest string) (size int64, mediaType string, err error) {
	resp, err := h.proxyFetch(ctx, "/v2/"+name+"/blobs/"+digest, nil)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("docker: read upstream blob: %w", err)
	}
	stored, n, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("docker: cache upstream blob: %w", err)
	}
	if stored != digest {
		// Upstream returned content that doesn't match the digest we asked
		// for. That's a corrupt or malicious upstream; we drop it.
		_ = h.cas.Delete(ctx, stored)
		return 0, "", fmt.Errorf("docker: upstream digest mismatch: got %s want %s", stored, digest)
	}
	mediaType = resp.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	_ = h.store.UpsertArtifact(
		ctx, h.repoID, "refs/"+name+"/blobs/"+digest,
		digest, n, mediaType, "",
	)
	return n, mediaType, nil
}

// proxyFetchManifest fetches a manifest from upstream by reference (tag or
// digest), stores the JSON body in CAS, and records both tag and digest
// bindings.
func (h *Handler) proxyFetchManifest(ctx context.Context, name, reference string) (digest, mediaType string, body []byte, err error) {
	accept := []string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}
	resp, err := h.proxyFetch(ctx, "/v2/"+name+"/manifests/"+reference, accept)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("docker: read upstream manifest: %w", err)
	}
	digest = resp.Header.Get("Docker-Content-Digest")
	mediaType = resp.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/vnd.oci.image.manifest.v1+json"
	}

	stored, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return "", "", nil, fmt.Errorf("docker: cache upstream manifest: %w", err)
	}
	if digest == "" {
		digest = stored
	}

	// Record tag binding when the client asked by tag (not digest).
	if !strings.HasPrefix(reference, "sha256:") {
		_ = h.store.UpsertArtifact(
			ctx, h.repoID, manifestPath(name, reference),
			digest, int64(len(body)), mediaType, "",
		)
	}
	_ = h.store.UpsertArtifact(
		ctx, h.repoID, manifestPath(name, digest),
		digest, int64(len(body)), mediaType, "",
	)
	return digest, mediaType, body, nil
}
