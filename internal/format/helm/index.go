// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// index.yaml generation. The Helm Chart Repository spec requires a single
// index.yaml that lists every chart and version available. We rebuild it on
// demand from the artifact rows; it is cheap because we already store the
// parsed Chart.yaml as JSON metadata on each row.

package helm

import (
	"encoding/json"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/packrune/packrune/internal/repo"
)

type helmIndex struct {
	APIVersion string                      `yaml:"apiVersion"`
	Entries    map[string][]helmIndexEntry `yaml:"entries"`
	Generated  string                      `yaml:"generated"`
	ServerInfo helmServerInfo              `yaml:"serverInfo,omitempty"`
}

type helmIndexEntry struct {
	APIVersion  string            `yaml:"apiVersion,omitempty"`
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	AppVersion  string            `yaml:"appVersion,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Type        string            `yaml:"type,omitempty"`
	Keywords    []string          `yaml:"keywords,omitempty"`
	Home        string            `yaml:"home,omitempty"`
	Sources     []string          `yaml:"sources,omitempty"`
	Maintainers []chartMaintainer `yaml:"maintainers,omitempty"`
	Icon        string            `yaml:"icon,omitempty"`
	Created     string            `yaml:"created,omitempty"`
	Digest      string            `yaml:"digest"`
	URLs        []string          `yaml:"urls"`
}

type helmServerInfo struct {
	ContextPath string `yaml:"contextPath,omitempty"`
}

// buildIndex constructs the index.yaml payload from artifacts.
func buildIndex(arts []repo.Artifact, contextPath string) ([]byte, error) {
	idx := helmIndex{
		APIVersion: "v1",
		Entries:    map[string][]helmIndexEntry{},
		Generated:  time.Now().UTC().Format(time.RFC3339),
		ServerInfo: helmServerInfo{ContextPath: contextPath},
	}

	for _, a := range arts {
		var meta chartMetadata
		if a.Metadata != "" {
			if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
				// Skip charts whose metadata is corrupt rather than failing the
				// whole index.
				continue
			}
		}
		if meta.Name == "" || meta.Version == "" {
			continue
		}
		entry := helmIndexEntry{
			APIVersion:  meta.APIVersion,
			Name:        meta.Name,
			Version:     meta.Version,
			AppVersion:  meta.AppVersion,
			Description: meta.Description,
			Type:        meta.Type,
			Keywords:    meta.Keywords,
			Home:        meta.Home,
			Sources:     meta.Sources,
			Maintainers: meta.Maintainers,
			Icon:        meta.Icon,
			Created:     a.CreatedAt.UTC().Format(time.RFC3339),
			Digest:      a.Digest,
			URLs:        []string{meta.Name + "-" + meta.Version + ".tgz"},
		}
		idx.Entries[meta.Name] = append(idx.Entries[meta.Name], entry)
	}

	return yaml.Marshal(idx)
}
