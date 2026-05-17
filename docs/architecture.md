# Packrune Architecture

This document is a map. Read it before opening source files; it will save you
hours.

## One-paragraph summary

Packrune is a single Go binary that speaks every major package-manager protocol
(Docker/OCI, npm, Helm, Go, PyPI, Maven) on top of a shared core: pluggable
storage with content-addressable dedup, a Postgres or SQLite metadata store,
RBAC auth, and a React+Tailwind admin UI embedded into the binary. Adding a new
format means implementing one Go interface; the core handles storage, auth,
replication, audit, and UI generically.

## Component map

```
┌─────────────────────────────────────────────────────────────────┐
│                       Packrune binary                            │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐   ┌────────────────────┐    │
│  │  HTTP server │   │   Admin UI   │   │  Background jobs   │    │
│  │   (chi)      │   │  (embedded   │   │  - indexer         │    │
│  │              │   │   React SPA) │   │  - GC              │    │
│  └──────┬───────┘   └──────┬───────┘   │  - replication     │    │
│         │                  │           │  - webhook deliver │    │
│         │                  │           └──────────┬─────────┘    │
│         ▼                  ▼                      │              │
│  ┌─────────────────────────────────────────┐      │              │
│  │           Format adapters                │      │              │
│  │  docker · npm · helm · go · pypi · maven │      │              │
│  │  (each implements internal/format.Format)│      │              │
│  └──────────────┬──────────────────────────┘      │              │
│                 │                                 │              │
│                 ▼                                 ▼              │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Core services                         │    │
│  │  - Repository registry (hosted/proxy/group)              │    │
│  │  - Auth (users, tokens, OIDC, RBAC)                      │    │
│  │  - Audit log                                             │    │
│  │  - Metrics + tracing                                     │    │
│  └──────────┬─────────────────────────────┬────────────────┘    │
│             │                             │                      │
│             ▼                             ▼                      │
│  ┌──────────────────────┐    ┌────────────────────────────┐     │
│  │   Storage backend    │    │      Metadata store        │     │
│  │  (interface)         │    │  (Postgres or SQLite)      │     │
│  │  - fs · s3 · gcs ·   │    │  via sqlc                  │     │
│  │    azure             │    │                            │     │
│  │  + CAS dedup layer   │    │                            │     │
│  └──────────────────────┘    └────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

## Directory layout

```
packrune/
├── cmd/
│   └── packrune/              # main()
├── internal/
│   ├── config/                # config load + validate
│   ├── server/                # HTTP server, middleware, routing root
│   ├── auth/                  # users, tokens, OIDC, RBAC
│   ├── storage/               # blob storage interface + backends
│   │   ├── fs/
│   │   ├── s3/
│   │   ├── gcs/
│   │   └── cas/               # content-addressable layer
│   ├── db/                    # sqlc-generated queries + migrations runner
│   ├── repo/                  # repository entity (hosted/proxy/group)
│   ├── format/                # the Format interface + registry
│   │   ├── format.go          # interface definition
│   │   ├── registry.go        # compile-time registry
│   │   ├── docker/
│   │   ├── npm/
│   │   ├── helm/
│   │   ├── go/
│   │   ├── pypi/
│   │   └── maven/
│   ├── proxy/                 # upstream caching for proxy repos
│   ├── group/                 # group repo resolver
│   ├── indexer/               # background metadata indexer
│   ├── audit/                 # audit log writer
│   ├── webhook/               # webhook dispatcher
│   ├── replication/           # replication engine
│   └── web/                   # embed.FS for the React SPA
├── pkg/                       # (intentionally rare — only stable public types)
├── web/                       # React + Vite + Tailwind frontend source
│   ├── src/
│   │   ├── routes/            # TanStack Router routes
│   │   ├── components/        # shared components (glass, aurora, ...)
│   │   ├── features/          # feature-specific UI (repositories, users, ...)
│   │   ├── themes/            # theme definitions
│   │   ├── i18n/              # translation files
│   │   └── lib/               # API client, hooks, utils (but named)
│   └── public/
├── migrations/                # SQL migrations (golang-migrate format)
├── test/                      # integration tests
│   └── format/                # CLI-based tests per format
├── docs/
│   ├── architecture.md        # this file
│   ├── development.md
│   ├── formats/               # per-format protocol notes
│   └── adr/                   # architecture decision records
├── scripts/                   # build helpers, header check, etc.
└── .github/workflows/         # CI
```

## Key abstractions

### `Format` interface

Defined in `internal/format/format.go`. Every package format implements:

```go
type Format interface {
    Name() string
    Routes(r chi.Router)              // mount HTTP routes
    OnUpload(ctx, repo, blob) error   // post-upload hook for indexing
    OnDelete(ctx, repo, ref) error    // post-delete hook
    Index(ctx, repo) error            // rebuild format-specific index
}
```

If you find yourself wanting to add methods to this interface, stop and open an
issue — interface bloat is how Nexus got Nexus.

### Storage interface

Defined in `internal/storage/storage.go`. Backends implement read/write/delete
of opaque blobs keyed by a virtual path. The CAS layer sits *above* the backend
and rewrites paths to `sha256/<hex>`, giving us free cross-format dedup.

### Repository entity

Every artifact lives in a `Repository`. Three flavors:

- **Hosted** — Packrune owns the data.
- **Proxy** — Packrune caches a remote registry.
- **Group** — Packrune fans a single URL out to a list of hosted+proxy members,
  first-match wins.

All three implement the same HTTP-facing contract per format, so to the client
they're indistinguishable.

## What we deliberately don't do

- **No plugin system.** Formats are compiled in. Wanting "plugin loading at
  runtime" almost always means "I want a sloppier interface" — say no.
- **No internal message queue dependency.** Background jobs use a Postgres
  outbox or an in-process worker pool. Redis is optional, never required.
- **No microservices.** One binary. If you want to scale, run multiple copies
  behind a load balancer; they share Postgres + S3.
- **No "enterprise" feature flags.** Every feature is on for every user.

## Performance posture

- Read hot path (`docker pull`, `npm install`) goes:
  HTTP → format adapter → CAS lookup → storage backend stream. Zero DB hits on
  blob fetch once the manifest is resolved.
- Write hot path triggers async indexing. We never block an upload on index
  rebuild. Index lag is bounded and observable.
- Memory: streaming everywhere. We never buffer a full artifact in RAM.

## Security posture

- Every endpoint authenticated unless explicitly public (the registry "version
  check" `GET /v2/` is one of the few).
- Tokens are scoped (repo + action). A leaked CI token compromises only its
  scope.
- Audit log records every write and every access decision (allow + deny).
- No telemetry. Ever. Not even anonymous version pings.
