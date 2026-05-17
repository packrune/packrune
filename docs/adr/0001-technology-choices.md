# ADR-0001: Technology choices

**Status:** Accepted
**Date:** 2026-05-17

## Context

We are building a self-hosted artifact repository manager (Packrune) that will
support many package formats (Docker/OCI, npm, Helm, Go, PyPI, Maven) and run
as a single binary on commodity infrastructure. We need to pick a language,
data store, storage abstraction, and frontend stack.

## Constraints

- Single-binary deploy (no JVM, no separate frontend server).
- Low operational complexity (one team should be able to run it without
  becoming a full-time Packrune operator).
- High concurrency for blob streaming (this is mostly a network IO workload).
- Approachable codebase for outside contributors.
- License: AGPL-3.0 + Commons Clause — no library that conflicts with this is
  acceptable.

## Decision

### Backend: Go

- Single static binary, trivial cross-compile.
- Strong HTTP / concurrency primitives in the stdlib.
- Mature ecosystem precedent: Docker Registry, Harbor, ChartMuseum, Gitea, Drone
  — every major OSS repository tool of the last decade is Go.
- Memory footprint and startup time make it fine to run in tiny environments
  (homelab) and at scale.

**Rejected alternatives:**
- *Java/Kotlin* (Nexus's stack): heavy JVM, slow startup, harder for outside
  contributors, opaque ops story. Explicitly what we are trying to be lighter
  than.
- *Rust*: faster runtime, slower development velocity for HTTP/CRUD workloads.
  The maintenance surface of this project is breadth (many formats) more than
  depth (per-format perf), and Go fits that shape better.
- *TypeScript/Node*: weak story for single-binary distribution; harder to ship
  static binaries that include native deps.

### HTTP routing: chi

- Stdlib-shaped, minimal API surface.
- Middleware composition that doesn't get in the way.
- No framework lock-in.

### Database: Postgres (prod) + SQLite (single-node)

- Identical SQL via sqlc-generated, type-safe queries.
- Postgres for HA / replication / large deployments.
- SQLite for homelab / single-instance / dev. Same code path, zero divergence.

**Rejected:** ORMs. Reflection-based ORMs make queries opaque and migrations
scary. sqlc gives us static SQL files + generated Go.

### Migrations: golang-migrate

- Plain SQL files.
- Bidirectional.
- Well-understood tooling.

### Logging: stdlib `log/slog`

- Structured.
- Zero new dependency.

### Storage: pluggable interface

- `internal/storage.Backend` interface with file, S3, GCS, Azure
  implementations.
- A content-addressable layer (`internal/storage/cas`) sits above the backend
  and rewrites paths to `sha256/<hex>`, giving us automatic cross-format dedup
  and immutable artifact guarantees.

**Rejected:** baking S3-only assumptions into core. Homelab users with a single
disk should not have to run MinIO.

### Background work: in-process workers + Postgres outbox

- No Redis requirement.
- Redis is optional for very large deployments; not part of the baseline.

### Frontend: React 19 + Vite + Tailwind v4 + TypeScript

- The most contributor-friendly stack in 2026. Lower onboarding cost than
  Svelte/Solid despite some preference for them.
- TanStack Router + TanStack Query: type-safe routing, real server-state
  management.
- shadcn/ui approach: copy components into our tree rather than depend on a
  component library. License hygiene + total control.
- Motion (formerly Framer Motion): the de facto animation lib.

**Rejected:**
- *HTMX + Go templates*: my initial instinct. Excellent for "boring admin
  panels" but the user has set a high bar for UI quality (glassmorphism +
  aurora + bento + motion + multi-theme + i18n). React earns its keep here.
- *Svelte/SolidJS*: smaller contributor pool today.
- *Pre-built component library (MUI, Mantine, Chakra)*: harder to make feel
  bespoke; license/dep entanglement.

### Frontend embed: `embed.FS`

- The compiled `web/dist/` is embedded into the Go binary at build time.
- Single-binary promise preserved.

## Consequences

- Outside contributors need Go (~1.23+) and Node (~20+) installed. Acceptable.
- We commit to maintaining a Tailwind v4 + React 19 frontend; major framework
  jumps will cost us.
- The `Format` interface is the most important type in the codebase. Every
  format implements it. We will protect it from bloat aggressively.

## Revisit when

- A format we want to add genuinely cannot be expressed via the `Format`
  interface.
- The Go ecosystem changes meaningfully (e.g. stdlib gains something that
  replaces chi or sqlc cleanly).
- Replication scale requirements outgrow the Postgres outbox model.
