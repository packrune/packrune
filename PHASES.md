# Packrune — Phase Roadmap

This is the canonical progress tracker. Each phase ships a usable milestone.
Tick `[ ]` → `[x]` as items complete. If new work is discovered, add it under
the right phase.

**Resume rule:** after compaction or a new session, read this file first, then
the last `[x]` and continue from the first `[ ]` below it.

Legend: `[ ]` not started · `[~]` in progress · `[x]` done · `[!]` blocked

---

## Faz 0 — Foundation (project bootstrap)

Get the repo set up so contributors and CI can start working. No runnable
product yet, just the scaffold.

- [x] Pick license (AGPL-3.0 + Commons Clause)
- [x] LICENSE file (AGPL-3.0 full text)
- [x] COMMONS-CLAUSE file
- [x] NOTICE file (plain-English license summary)
- [x] CLA.md (Contributor License Agreement)
- [x] CONTRIBUTING.md (contributor quick-start + house rules)
- [x] CODE_OF_CONDUCT.md
- [x] README.md (pitch + status + license)
- [x] PHASES.md (this file)
- [x] .gitignore
- [x] .editorconfig
- [x] go.mod / go.sum init (module path: `github.com/packrune/packrune`, swap when org chosen)
- [x] Top-level directory skeleton (`cmd/`, `internal/`, `pkg/`, `web/`, `docs/`, `test/`, `migrations/`, `scripts/`)
- [x] `Makefile` (build, test, lint, run, fmt, check-headers targets)
- [x] SPDX header check script (`scripts/check-headers.sh`, bash 3.2 compatible)
- [x] golangci-lint config (`.golangci.yml`)
- [x] GitHub Actions workflow: lint + test + build (`.github/workflows/ci.yml`)
- [x] GitHub Actions workflow: SPDX header enforcement (`.github/workflows/spdx-check.yml`)
- [x] `docs/architecture.md` — high-level overview
- [x] `docs/adr/0001-technology-choices.md`
- [x] `docs/adr/0002-license-choice.md`
- [x] `docs/development.md` — local dev setup

**Faz 0 done.** Repo scaffolding complete, CI ready, license + SPDX enforcement
wired up. Next: Faz 1 — core backend infrastructure.

## Faz 1 — Core backend infrastructure

The plumbing every format will share. Nothing user-visible yet, but everything
above it depends on this being right.

- [x] `cmd/packrune/main.go` — entrypoint, version, config load, subcommand dispatch
- [x] `cmd/packrune/serve.go` — server subcommand wiring (DB + storage + formats)
- [x] `cmd/packrune/users.go` — `packrune users add` bootstrap CLI
- [x] Config system (YAML + reflection-based env override, no Viper)
- [x] Structured logging (stdlib `log/slog`, text + JSON modes)
- [x] HTTP server skeleton (chi router, graceful shutdown, request ID, panic recovery, access log)
- [x] Health + readiness endpoints (`/healthz`, `/readyz`, `/version`)
- [ ] Optional Prometheus metrics endpoint (deferred — Faz 11)
- [x] Database layer — `internal/db` with Postgres + SQLite drivers, Rebind helper
- [x] Migrations: custom runner + `0001_initial` schema (golang-migrate compatible table shape)
- [x] Storage abstraction interface (`internal/storage`)
- [x] Filesystem storage backend (with traversal protection + atomic Put + Append)
- [ ] S3-compatible storage backend (deferred — Faz 8)
- [x] Content-addressable storage layer (`internal/storage/cas`, sha256 dedup)
- [x] User model (`users` table, bcrypt password hashing)
- [ ] Organization + team model (deferred — Faz 11)
- [x] Token model (`tokens` table, hashed at rest, display prefix)
- [ ] Session model (deferred — bearer tokens cover API; UI sessions in Faz 3)
- [x] RBAC permission table + `Role` enum (service layer in Faz 7 with UI)
- [ ] OIDC integration (deferred — Faz 11)
- [x] Repository entity (`internal/repo`, hosted/proxy/group kinds)
- [x] Repository store (CRUD + artifact upsert/lookup)
- [x] `Format` interface — the contract every format implements
- [x] Format registry (compile-time registration via init())
- [x] Audit log writer (`internal/audit`, fire-and-forget pattern)
- [ ] Webhook dispatcher (deferred — Faz 11)

**Faz 1 done.** Server boots, opens DB, runs migrations, exposes health
endpoints, supports `users add` CLI, ready for format adapters to plug in.

## Faz 2 — Docker / OCI format (first real format)

Picked first because the spec is the most mature, test tooling is universal,
and it gets us "useful" the fastest.

- [x] Docker Registry HTTP API V2 endpoints (`/v2/`)
- [x] V2 path parser (handles slashed image names like `library/hello-world`)
- [x] Docker error response envelope (codes per spec)
- [x] Blob upload — POST start, PATCH append, PUT finalize, plus monolithic POST?digest=
- [x] Upload session manager (on-disk staging, cancel via DELETE)
- [x] Blob digest verification (hashed on finalize, mismatch → 400)
- [x] Blob pull (GET + HEAD by digest)
- [x] Blob "soft delete" (unbinds image-blob link; CAS GC later)
- [x] Manifest upload (PUT, OCI + Docker schemas; both tag and digest bindings)
- [x] Manifest pull (GET + HEAD, by tag or digest)
- [x] Manifest delete
- [x] Tag listing (`/v2/<name>/tags/list`)
- [x] Catalog endpoint (`/v2/_catalog`)
- [x] Go-level integration tests (handshake, blob round-trip, manifest+tag list)
- [x] curl-level smoke test (push blob → pull blob → push manifest → list tags → catalog)
- [ ] Token auth flow (`/v2/token`, scope grant) — Faz 2 polish
- [ ] Garbage collection job (orphan blobs) — Faz 2 polish
- [ ] OCI distribution spec conformance suite run in CI — Faz 2 polish
- [ ] HTTPS / TLS support for real `docker push` from CLI without
      `--insecure-registry` — Faz 12

**Faz 2 core done.** The registry serves spec-compliant push/pull for blobs
and manifests, with proper tag and catalog listing, against the shared CAS
storage. Real-world `docker push` works once the daemon is told the registry
is insecure (or TLS lands in Faz 12).

## Faz 3 — Frontend foundation

The skeleton + design system. Pages come in Faz 7. This phase is "you can log
in and see an empty shell that already looks better than Nexus."

- [ ] `web/` scaffold: Vite + React 19 + TypeScript
- [ ] Tailwind CSS v4 setup
- [ ] Biome (lint + format) config
- [ ] Vitest setup
- [ ] Design tokens (colors, spacing, typography, glass opacity, gradients)
- [ ] Theme system: CSS variables + runtime switch
- [ ] Built-in themes: Aurora, Midnight, Daybreak, Terminal, Mono
- [ ] i18n setup (i18next + react-i18next), `en` + `tr` baseline
- [ ] RTL-ready layout primitives
- [ ] Glass surface primitive component (`<Glass>`)
- [ ] Aurora gradient background component (animated, low-CPU)
- [ ] Motion primitives (page transition, hover, tap)
- [ ] TanStack Router setup, layout shell
- [ ] TanStack Query setup, API client
- [ ] Auth flow: login screen, token storage, session refresh
- [ ] Layout shell: sidebar, topbar, command palette (Cmd+K) trigger
- [ ] Empty-state component (used everywhere)
- [ ] Go `embed.FS` integration: build embeds `web/dist` into the binary

## Faz 4 — npm format

- [ ] npm registry API (`/<pkg>`, `/<pkg>/<version>`, `/-/v1/search`)
- [ ] Packument generation
- [ ] Tarball storage + retrieval
- [ ] `npm publish` flow (PUT package)
- [ ] `npm unpublish` flow
- [ ] Dist-tags
- [ ] Scoped packages
- [ ] Integration tests with the `npm` CLI

## Faz 5 — Helm format

- [ ] Helm Chart Repository API (index.yaml + chart tarballs)
- [ ] Chart upload (push)
- [ ] Chart pull
- [ ] Provenance (`.prov`) file support
- [ ] `index.yaml` incremental regeneration on upload
- [ ] Integration tests with the `helm` CLI

## Faz 6 — Go modules format

- [ ] GOPROXY protocol implementation
- [ ] `/@v/list` endpoint
- [ ] `/@v/<version>.info` endpoint
- [ ] `/@v/<version>.mod` endpoint
- [ ] `/@v/<version>.zip` endpoint
- [ ] `/@latest` endpoint
- [ ] Optional sumdb support
- [ ] Integration tests with the `go` CLI

## Faz 7 — Frontend pages (the actual UI)

The pages users actually see. Each item ships a polished, animated, themed,
internationalized page.

- [ ] Login + onboarding wizard
- [ ] Dashboard (bento layout, recent activity, storage doluluk)
- [ ] Repositories list (grid + list toggle, filter chips)
- [ ] Repository detail (package browser + tabs)
- [ ] Package detail (versions, deps, README render, stats)
- [ ] Global search + Cmd+K command palette
- [ ] Users & teams
- [ ] Tokens management
- [ ] System settings (storage, auth, SMTP, replication)
- [ ] Audit log timeline
- [ ] Profile (theme, language, 2FA)
- [ ] Accessibility audit (WCAG 2.1 AA target)
- [ ] Responsive breakpoint pass (≥768px first-class; mobile read-only)

## Faz 8 — Proxy + group repos

- [ ] Upstream registry client (per-format)
- [ ] Cache eviction policy (LRU + age)
- [ ] Cache invalidation hooks
- [ ] Group repository resolver (first-match across members)
- [ ] Per-format proxy support: Docker, npm, Helm, Go, PyPI, Maven
- [ ] Proxy auth (upstream credentials, optional)

## Faz 9 — PyPI format

- [ ] PyPI simple index (PEP 503, HTML)
- [ ] PyPI JSON API (PEP 691)
- [ ] `twine upload` flow
- [ ] `pip install` flow against local
- [ ] Integration tests with `pip` and `twine`

## Faz 10 — Maven format

- [ ] Maven repository layout (groupId/artifactId/version)
- [ ] `maven-metadata.xml` generation
- [ ] Snapshot vs release handling
- [ ] Checksum files (sha1, md5, sha256, sha512)
- [ ] Signature (`.asc`) passthrough
- [ ] Integration tests with `mvn deploy` / `mvn dependency:get`

## Faz 11 — Replication, webhooks, SSO

- [ ] Active-passive replication
- [ ] Active-active replication (conflict resolution rules)
- [ ] Webhook delivery (with retry + signing)
- [ ] SAML SSO
- [ ] LDAP integration
- [ ] Audit log retention + export

## Faz 12 — Deployment & dogfood

- [ ] Multi-arch Docker image (amd64, arm64)
- [ ] Helm chart (and we serve it from our own Helm format — dogfood)
- [ ] Docker compose example
- [ ] Systemd unit file example
- [ ] Backup + restore CLI commands
- [ ] Release automation (goreleaser)
- [ ] Project website (separate repo, statically hosted)
