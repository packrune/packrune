// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Chart.yaml extraction from an uploaded Helm chart tarball. A chart is a
// gzip-compressed tar of a directory containing at minimum a Chart.yaml file.

package helm

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// chartMetadata mirrors the subset of Chart.yaml we care about for indexing
// and display.
type chartMetadata struct {
	APIVersion  string            `yaml:"apiVersion" json:"apiVersion,omitempty"`
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version" json:"version"`
	AppVersion  string            `yaml:"appVersion,omitempty" json:"appVersion,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Type        string            `yaml:"type,omitempty" json:"type,omitempty"`
	Keywords    []string          `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Home        string            `yaml:"home,omitempty" json:"home,omitempty"`
	Sources     []string          `yaml:"sources,omitempty" json:"sources,omitempty"`
	Maintainers []chartMaintainer `yaml:"maintainers,omitempty" json:"maintainers,omitempty"`
	Icon        string            `yaml:"icon,omitempty" json:"icon,omitempty"`
}

type chartMaintainer struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`
	URL   string `yaml:"url,omitempty" json:"url,omitempty"`
}

// parseChart reads a gzipped tar from r and returns the parsed Chart.yaml.
// The reader is consumed; callers buffer it themselves if they also need to
// store the raw bytes.
func parseChart(r io.Reader) (chartMetadata, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return chartMetadata{}, fmt.Errorf("helm: gunzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return chartMetadata{}, fmt.Errorf("helm: tar: %w", err)
		}
		if !strings.HasSuffix(path.Clean(hdr.Name), "/Chart.yaml") &&
			path.Base(hdr.Name) != "Chart.yaml" {
			continue
		}
		// Chart.yaml lives at <chart>/Chart.yaml at the top level. We don't
		// care which prefix, just that the basename matches and depth is 2.
		parts := strings.Split(strings.Trim(hdr.Name, "/"), "/")
		if len(parts) != 2 || parts[1] != "Chart.yaml" {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return chartMetadata{}, fmt.Errorf("helm: read Chart.yaml: %w", err)
		}
		var meta chartMetadata
		if err := yaml.Unmarshal(body, &meta); err != nil {
			return chartMetadata{}, fmt.Errorf("helm: parse Chart.yaml: %w", err)
		}
		if meta.Name == "" || meta.Version == "" {
			return chartMetadata{}, errors.New("helm: Chart.yaml is missing name or version")
		}
		return meta, nil
	}
	return chartMetadata{}, errors.New("helm: Chart.yaml not found in tarball")
}
