// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// npm packument: the JSON document describing a package and every published
// version. On publish we receive a packument that contains _attachments
// (base64 tarballs); we strip those, store the tarball as a CAS blob, and
// merge the version metadata into the stored packument.

package npm

import (
	"encoding/json"
	"errors"
	"fmt"
)

// packument is a minimal view of the npm packument schema. We preserve
// unknown fields by using json.RawMessage for versions and dist-tags so we
// don't lose information the publisher cared about.
type packument struct {
	ID          string                     `json:"_id,omitempty"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	DistTags    map[string]string          `json:"dist-tags,omitempty"`
	Versions    map[string]json.RawMessage `json:"versions,omitempty"`
	Readme      string                     `json:"readme,omitempty"`
	Maintainers []json.RawMessage          `json:"maintainers,omitempty"`
	Time        map[string]string          `json:"time,omitempty"`
	Attachments map[string]attachment      `json:"_attachments,omitempty"`
}

// attachment is one entry of the publish _attachments map.
type attachment struct {
	ContentType string `json:"content_type"`
	Data        string `json:"data"`
	Length      int64  `json:"length"`
}

// parsePackument unmarshals body as a publish packument.
func parsePackument(body []byte) (*packument, error) {
	var p packument
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("npm: parse packument: %w", err)
	}
	if p.Name == "" {
		return nil, errors.New("npm: packument missing 'name'")
	}
	return &p, nil
}

// mergeInto folds incoming versions and dist-tags into base. New version
// numbers are added; existing versions are NOT overwritten (npm publish
// semantics — republishing an existing version is an error).
//
// _attachments are not merged: callers extract them out-of-band.
func (p *packument) mergeInto(base *packument) error {
	if base.Name == "" {
		base.Name = p.Name
	}
	if base.ID == "" {
		base.ID = p.ID
	}
	if p.Description != "" {
		base.Description = p.Description
	}
	if p.Readme != "" {
		base.Readme = p.Readme
	}

	if base.Versions == nil {
		base.Versions = map[string]json.RawMessage{}
	}
	for v, meta := range p.Versions {
		if _, exists := base.Versions[v]; exists {
			return fmt.Errorf("npm: version %s already published", v)
		}
		base.Versions[v] = meta
	}

	if base.DistTags == nil {
		base.DistTags = map[string]string{}
	}
	for tag, ver := range p.DistTags {
		base.DistTags[tag] = ver
	}

	if base.Time == nil {
		base.Time = map[string]string{}
	}
	for k, v := range p.Time {
		base.Time[k] = v
	}

	if len(p.Maintainers) > 0 {
		base.Maintainers = p.Maintainers
	}

	return nil
}

// marshal returns the stored shape of the packument (no _attachments).
func (p *packument) marshal() ([]byte, error) {
	p.Attachments = nil
	return json.MarshalIndent(p, "", "  ")
}
