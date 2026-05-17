// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// PyPI proxy mode. simple/<pkg>/ misses fall through to the upstream
// (typically https://pypi.org), the response HTML is parsed for file
// references, each file's metadata is recorded as a stub artifact (no
// content cached yet), and individual file downloads pull and cache the
// bytes on first access. Returns an HTML response rewritten to point file
// URLs at our /packages/<pkg>/<filename>.

package pypi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

// linkRE matches a PEP 503 simple-index file link:
//
//	<a href="https://files.pythonhosted.org/...../foo-1.0.0.tar.gz#sha256=abc..." ...>foo-1.0.0.tar.gz</a>
//
// Capture groups: 1=full URL (with fragment), 2=filename.
var linkRE = regexp.MustCompile(`<a\s+[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)

// proxyFetchSimple fetches upstream/simple/<pkg>/, parses links, records
// metadata (filename + sha256 if present), and returns the same HTML
// rewritten to point at our /packages/<pkg>/<filename>.
func (h *Handler) proxyFetchSimple(ctx context.Context, normPkg string) (string, error) {
	if h.proxy.Upstream == "" {
		return "", fmt.Errorf("pypi: proxy not configured")
	}
	url := strings.TrimRight(h.proxy.Upstream, "/") + "/simple/" + normPkg + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if h.proxy.Username != "" {
		req.SetBasicAuth(h.proxy.Username, h.proxy.Password)
	}
	resp, err := proxyClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("upstream %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Rewrite + record. We don't cache the bytes here — that happens lazily
	// on the first GET /packages/<pkg>/<filename> miss.
	matches := linkRE.FindAllSubmatch(body, -1)
	var out bytes.Buffer
	out.WriteString("<!DOCTYPE html><html><body>\n")
	for _, m := range matches {
		href := string(m[1])
		filename := string(m[2])
		fragment := ""
		if i := strings.Index(href, "#"); i >= 0 {
			fragment = href[i:]
		}
		out.WriteString(`<a href="../../packages/`)
		out.WriteString(normPkg)
		out.WriteString("/")
		out.WriteString(filename)
		out.WriteString(fragment)
		out.WriteString(`">`)
		out.WriteString(filename)
		out.WriteString("</a><br/>\n")

		// Stub artifact so /api/repositories/{}/{}/artifacts surfaces it.
		// The digest field carries the upstream sha256 when known; size is 0
		// until the file is actually fetched.
		digest := ""
		if strings.Contains(fragment, "sha256=") {
			if i := strings.Index(fragment, "sha256="); i >= 0 {
				digest = "sha256:" + fragment[i+len("sha256="):]
				if amp := strings.IndexAny(digest, "&"); amp >= 0 {
					digest = digest[:amp]
				}
			}
		}
		meta, _ := json.Marshal(map[string]string{"upstream_url": string(m[1])})
		_ = h.store.UpsertArtifact(ctx, h.repoID,
			"packages/"+normPkg+"/"+filename, digest, 0,
			"application/octet-stream", string(meta))
	}
	out.WriteString("</body></html>\n")
	return out.String(), nil
}

// proxyFetchFile pulls the upstream URL for a specific file (using the
// stored metadata's upstream_url), caches the bytes, and returns them.
func (h *Handler) proxyFetchFile(ctx context.Context, normPkg, filename string) ([]byte, error) {
	art, err := h.store.GetArtifact(ctx, h.repoID, "packages/"+normPkg+"/"+filename)
	if err != nil {
		return nil, fmt.Errorf("pypi: no upstream record for %s/%s: %w", normPkg, filename, err)
	}
	var meta struct {
		UpstreamURL string `json:"upstream_url"`
	}
	if art.Metadata != "" {
		_ = json.Unmarshal([]byte(art.Metadata), &meta)
	}
	if meta.UpstreamURL == "" {
		return nil, fmt.Errorf("pypi: artifact %s has no upstream_url", art.Path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.UpstreamURL, nil)
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
		return nil, fmt.Errorf("upstream %s: HTTP %d", meta.UpstreamURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	digest, _, err := h.cas.Put(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := h.store.UpsertArtifact(ctx, h.repoID,
		"packages/"+normPkg+"/"+filename, digest, int64(len(body)),
		"application/octet-stream", art.Metadata); err != nil {
		return nil, err
	}
	return body, nil
}
