// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Package repo models the Repository entity. A repository is a named bucket
// inside Packrune, scoped to exactly one package format and one of three
// kinds: hosted (we own the data), proxy (we cache an upstream), or group
// (we federate over members).
package repo

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Kind enumerates the three repository topologies.
type Kind string

const (
	KindHosted Kind = "hosted"
	KindProxy  Kind = "proxy"
	KindGroup  Kind = "group"
)

// Valid reports whether k is a recognized Kind.
func (k Kind) Valid() bool {
	switch k {
	case KindHosted, KindProxy, KindGroup:
		return true
	}
	return false
}

// Repository is the persistent metadata of a repository. Configuration that
// is kind-specific (proxy upstream URL, group members, retention rules) is
// stored as a JSON-encoded blob in Config, parsed by each format adapter.
type Repository struct {
	ID        string
	Name      string
	Format    string
	Kind      Kind
	Config    []byte // opaque JSON; format adapter interprets
	CreatedAt time.Time
	UpdatedAt time.Time
}

// nameRe restricts repository names to a friendly subset: alphanum plus dash,
// underscore, and dot. This dodges nearly every URL-encoding and path-shell
// hazard without limiting expressiveness in any practical case.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// ValidateName returns an error if name is unacceptable as a repo identifier.
func ValidateName(name string) error {
	if name == "" {
		return errors.New("repo: name must not be empty")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("repo: name %q: must start with [A-Za-z0-9] and contain only [A-Za-z0-9._-]", name)
	}
	return nil
}
