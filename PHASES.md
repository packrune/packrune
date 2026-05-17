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
- [x] Garbage collection (`packrune gc [--dry-run]` sweeps CAS for blobs
      with no artifact rows pointing at them; reports scanned/freed bytes)
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

- [x] `web/` scaffold: Vite 6 + React 19 + TypeScript 5.6
- [x] Tailwind CSS v4 setup (via `@tailwindcss/vite`)
- [x] Biome (lint + format) config
- [ ] Vitest setup (deferred — added when first frontend test lands)
- [x] Design tokens (colors, spacing, typography, glass opacity, gradients) in `global.css`
- [x] Theme system: CSS variables + runtime `data-theme` switch + localStorage persist
- [x] Built-in themes: Aurora, Midnight, Daybreak, Terminal, Mono
- [x] i18n setup (i18next + react-i18next), `en` + `tr` baseline
- [ ] RTL-ready layout primitives (Tailwind v4 supports `dir-` natively; revisit when first RTL string lands)
- [x] Glass surface primitive component (`<Glass>`, 3 elevations)
- [x] Aurora gradient background component (CSS-keyframe animated, theme-driven colors)
- [x] Motion primitives (Landing uses `motion/react` for entrance + stagger)
- [ ] TanStack Router setup (deferred — added with real routes in Faz 7)
- [x] TanStack Query setup (`QueryClient` in App, ready for hooks)
- [x] API client (`lib/api.ts` — fetchJSON + ApiError + typed getVersion)
- [ ] Auth flow: login screen, token storage, session refresh (Faz 7)
- [x] Layout shell: sidebar, topbar, theme + language switchers (`Shell.tsx`)
- [x] Landing page that exercises every primitive (glass, aurora, motion, theme, i18n)
- [x] Go `embed.FS` integration: `internal/web` mounted at `/*`, SPA fallback to index.html, API/registry paths protected from fallback
- [x] Placeholder `dist/index.html` so the binary is self-contained even before `pnpm build`

**Faz 3 done.** The binary now ships a beautiful glass + aurora landing page
(once `make web-build` runs once) at `/`, while `/v2/` and `/api/` continue
to serve their respective backends. Theme and language switching work live
without page reload. Real authenticated pages land in Faz 7.

## Faz 4 — npm format

- [x] npm registry API (`/<pkg>`, `/<pkg>/<version>`, `/-/ping`, `/-/whoami`)
- [x] Packument storage + merge-on-publish (existing versions rejected)
- [x] Tarball storage + retrieval (CAS-backed)
- [x] `npm publish` flow — multipart PUT with _attachments base64 tarball
- [ ] `npm unpublish` flow (deferred — rarely used)
- [x] Dist-tags (latest resolves correctly)
- [x] Scoped packages (`@scope/pkg`)
- [x] Go-level integration tests (publish + fetch + duplicate-version conflict)
- [ ] CLI-level tests with the real `npm` CLI (deferred to Faz 12)

**Faz 4 done.** `npm publish` works against `/npm/`, `npm install` resolves.

## Faz 5 — Helm format

- [x] Helm Chart Repository API (`index.yaml` generation, chart download)
- [x] Chart upload via chartmuseum-style multipart `POST /api/charts`
- [x] Chart.yaml parsed inline; full metadata recorded for index generation
- [x] `index.yaml` rebuilt from artifact rows (no per-upload tgz reparse)
- [x] `GET /api/charts` chartmuseum-compatible listing
- [ ] Provenance (`.prov`) file support (deferred)
- [x] Go-level integration test (real gzipped tar with Chart.yaml round-trip)
- [ ] CLI-level test with `helm cm-push` (deferred to Faz 12)

**Faz 5 done.** `helm repo add packrune http://host/helm/` works for install.

## Faz 6 — Go modules format

- [x] GOPROXY protocol implementation
- [x] `/@v/list` endpoint
- [x] `/@v/<version>.info` endpoint
- [x] `/@v/<version>.mod` endpoint
- [x] `/@v/<version>.zip` endpoint
- [x] `/@latest` endpoint
- [x] Capital-letter `!<lower>` URL escape decoding (GOPROXY spec)
- [x] Out-of-band upload API (PUT each artifact) for CI publishing
- [ ] Optional sumdb support (deferred)
- [x] Go-level integration tests (full round-trip + escape decoding)
- [ ] CLI-level test with `go get` against `GOPROXY=http://host/go,direct` (deferred)

**Faz 6 done.** `GOPROXY=http://host/go,direct go get pkg@v1.0.0` resolves.

## Faz 7 — Frontend pages (the actual UI)

The pages users actually see. Each item ships a polished, animated, themed,
internationalized page.

- [x] Login page (glass card, motion entrance, real /api/auth/login wire-up)
- [x] Dashboard (bento layout, stat cards, repository preview, "you" card,
      per-format chip strip, live /api/system/stats refresh)
- [x] Repositories list (bento grid, format-colored badges)
- [x] Tokens management page (issue / copy-once / revoke)
- [x] Users & teams page (admin: list/create/deactivate, crown badge,
      self-deactivation guard)
- [x] Audit log timeline (admin, color-icon results, 15s auto-refresh)
- [x] Shell sign-out button + sidebar Link routes + active state
- [x] Repository detail page (artifact list with substring filter)
- [x] Global Cmd+K command palette (pages + repos + sign-out)
- [x] Webhooks management page (admin: list/create/delete)
- [x] Profile page (visual theme picker, language picker, version footer)
- [x] System settings page (read-only config view with secret redaction)
- [ ] Onboarding wizard for the first user (deferred — `packrune users add` covers it)
- [ ] Package detail (versions, deps, README render, stats)
- [ ] 2FA on Profile
- [ ] Accessibility audit (WCAG 2.1 AA target)
- [ ] Responsive breakpoint pass (≥768px first-class; mobile read-only)

**Faz 7 substantially done.** Login → dashboard → repositories → tokens →
audit → users all wired against the live JSON API with full i18n (en/tr)
and the 5-theme system; sign-out works from the sidebar. Detail pages,
command palette, and settings/profile screens land next.

## Faz 8 — Proxy + group repos

- [x] Per-format proxy support: **Docker** (manifest + blob fall-through
      with caching into CAS + per-image artifact rows)
- [x] Repo.Config-driven upstream configuration (`{"upstream":"https://..."}`)
- [x] Cache write-through: upstream digest verified against requested digest
      before storing; mismatches are dropped
- [x] Go-level integration test (fake upstream → proxy fetch → second-fetch
      from cache → blob proxy)
- [x] Per-format proxy support: **npm** (packument + tarball fall-through)
- [x] Per-format proxy support: **Helm** (chart tarball fall-through; index
      stays generated from cached charts)
- [x] Per-format proxy support: **Go modules** (every @v/* file falls through)
- [x] Per-format proxy support: **Maven** (every layout file falls through;
      maven-metadata.xml fetched but not cached, so version lists stay fresh)
- [ ] Per-format proxy support: PyPI simple HTML rewriting (deferred —
      file proxy works once cached, but the simple/<pkg>/ HTML still needs
      URL rewriting to point at us)
- [ ] Cache eviction policy (LRU + age) — deferred
- [ ] Cache invalidation hooks — deferred
- [x] Group repository resolver — Store.ResolveArtifact +
      ResolveArtifactsByPrefix walk members, first-member-wins on path
      conflicts; every format handler's read paths use these so a single
      group repo can fan out to N hosted/proxy members. Tested.
- [ ] Proxy auth (upstream bearer/basic credentials) — deferred

## Faz 9 — PyPI format

- [x] PyPI simple index (PEP 503, HTML)
- [x] PyPI JSON API (PEP 691)
- [x] `twine upload` multipart flow (POST /)
- [x] PEP 503 name normalization (Sample_Package → sample-package)
- [x] SHA-256 hash anchors on file links
- [x] Go-level integration test (upload + simple + JSON + download)
- [ ] CLI-level test with `pip install` and `twine upload` (deferred)

**Faz 9 done.** `pip install --index-url http://host/pypi/simple/ pkg` works.

## Faz 10 — Maven format

- [x] Maven repository layout (groupId/artifactId/version/<files>)
- [x] `maven-metadata.xml` auto-regenerated on each primary-artifact upload
- [x] Checksum files (sha1, sha256, md5) auto-generated if client omits them
- [x] Signature (`.asc`) passthrough — uploads stored unchanged
- [ ] Snapshot vs release handling refinements (deferred — release path works)
- [x] Go-level integration test (jar+pom upload, metadata regen, sha1 sidecar)
- [ ] CLI-level test with `mvn deploy` / `mvn dependency:get` (deferred)

**Faz 10 done.** `mvn deploy` against `/maven/` populates the layout + metadata.

## Faz 11 — Replication, webhooks, SSO

- [x] Webhook delivery with HMAC-SHA256 signing (X-Packrune-Signature)
- [x] Webhook admin API (GET/POST/DELETE /api/webhooks, admin-only)
- [x] webhook_deliveries table with status/attempts/last_error columns
      ready for the retry worker
- [ ] Webhook outbox worker with exponential backoff retry (deferred)
- [ ] Wire dispatcher into format handlers' OnUpload/OnDelete hooks (deferred)
- [ ] Active-passive replication (deferred)
- [ ] Active-active replication (deferred)
- [ ] SAML SSO (deferred)
- [ ] LDAP integration (deferred)
- [ ] Audit log retention + export (deferred)

## Faz 12 — Deployment & dogfood

- [x] Dockerfile (multi-stage: node → golang-alpine → distroless static)
- [x] Helm chart at `deploy/helm/packrune` with values.yaml + templates
- [x] docker-compose.yml example with packrune-data volume + commented
      postgres/minio HA blocks
- [x] systemd unit file at `deploy/systemd/packrune.service` with hardening
- [x] deploy/README.md walking through Docker, Helm, and systemd paths
- [x] `packrune backup` — tar.gz of SQLite + fs storage tree
- [x] `packrune restore` — inverse, with --force overwrite guard
- [x] Multi-arch image build in CI (.github/workflows/release.yml with
      docker buildx for linux/amd64 + linux/arm64; static binaries for
      darwin/linux/windows × amd64/arm64; GitHub Release on tag push)
- [ ] Project website / docs site — deferred
- [ ] Dogfood: publish our own Helm chart through our own Helm format
      (post-release)
