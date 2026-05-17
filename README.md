<div align="center">

# Packrune

**The fully free, fully self-hosted, universal artifact repository manager.**

One binary. Every format. Forever free.

[![License](https://img.shields.io/badge/license-AGPL--3.0%20%2B%20Commons%20Clause-7c5cff.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8.svg)](go.mod)
[![Image](https://img.shields.io/badge/image-packrune%2Fpackrune-2496ED.svg)](https://hub.docker.com/r/packrune/packrune)

</div>

Packrune speaks **Docker / OCI**, **npm**, **Helm**, **Go modules**, **PyPI**,
and **Maven** from one binary. Every feature ships in every install: no
enterprise edition, no license server, no telemetry. The combined
**AGPL-3.0 + Commons Clause** license makes it legally impossible for anyone
to sell Packrune or sell a hosted Packrune service — including the people
who write it.

---

## Table of contents

1. [Quick start with Docker](#quick-start-with-docker)
2. [Local development](#local-development)
3. [The `ptcli` operator](#the-ptcli-operator)
4. [Supported formats](#supported-formats)
5. [Repository types](#repository-types)
6. [Web UI tour](#web-ui-tour)
7. [Configuration reference](#configuration-reference)
8. [Operations](#operations)
9. [Architecture](#architecture)
10. [Developer handbook](#developer-handbook)
11. [Project layout](#project-layout)
12. [Roadmap](#roadmap)
13. [License](#license)

---

## Quick start with Docker

The fastest way to a working install. Single image, persistent volume, ready
in 30 seconds.

```bash
docker run -d --name packrune \
  -p 8080:8080 \
  -v packrune-data:/data \
  packrune/packrune:latest

# Bootstrap an admin user.
docker exec -it packrune /app/packrune users add \
  --email you@example.com --username admin --password 'change-me-please' --admin
```

Open <http://localhost:8080>, sign in, and you have:

| URL                                          | Use it for                                   |
| -------------------------------------------- | -------------------------------------------- |
| `http://localhost:8080`                      | Admin UI (login + everything else)           |
| `http://localhost:8080/v2/`                  | `docker push` / `docker pull` (OCI registry) |
| `http://localhost:8080/npm/`                 | `npm publish` / `npm install --registry=…`   |
| `http://localhost:8080/helm/`                | `helm repo add …`                            |
| `http://localhost:8080/go/`                  | `GOPROXY=…/go,direct go get …`               |
| `http://localhost:8080/pypi/simple/`         | `pip install --index-url=…`                  |
| `http://localhost:8080/maven/`               | `mvn deploy / mvn dependency:get`            |

### Docker compose

For a one-file deployment with sensible defaults:

```yaml
# docker-compose.yml
services:
  packrune:
    image: packrune/packrune:latest
    restart: unless-stopped
    ports: ["8080:8080"]
    volumes: ["packrune-data:/data"]
    environment:
      PACKRUNE_LOG_FORMAT: json

volumes:
  packrune-data:
```

Bring it up with `docker compose up -d` and tail logs with
`docker compose logs -f packrune`.

### Other registries

Mirror images are published to GitHub Container Registry too:

```bash
docker pull ghcr.io/packrune/packrune:latest
```

Both registries point at the exact same multi-arch image (`linux/amd64`,
`linux/arm64`).

### Scaling out

Drop the embedded SQLite + filesystem storage and switch to Postgres + an
S3-compatible bucket by setting environment variables:

```yaml
environment:
  PACKRUNE_DATABASE_DRIVER: postgres
  PACKRUNE_DATABASE_DSN: postgres://packrune:secret@postgres:5432/packrune?sslmode=disable
  PACKRUNE_STORAGE_BACKEND: s3
  PACKRUNE_STORAGE_S3_ENDPOINT: https://s3.us-east-1.amazonaws.com
  PACKRUNE_STORAGE_S3_BUCKET: packrune-artifacts
  PACKRUNE_STORAGE_S3_ACCESS_KEY: AKIA...
  PACKRUNE_STORAGE_S3_SECRET_KEY: ...
```

Run as many replicas as you need behind a load balancer; they share state
through Postgres and the bucket.

### Kubernetes / Helm

A Helm chart lives at [`deploy/helm/packrune`](deploy/helm/packrune). The
short version:

```bash
helm install packrune ./deploy/helm/packrune \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=packrune.example.com
```

Full values in [`deploy/helm/packrune/values.yaml`](deploy/helm/packrune/values.yaml).

### systemd

For bare-metal / VM deployments, drop the binary in `/usr/local/bin` and use
the unit file at [`deploy/systemd/packrune.service`](deploy/systemd/packrune.service).
Full instructions in [`deploy/README.md`](deploy/README.md).

---

## Local development

```bash
git clone https://github.com/packrune/packrune.git
cd packrune

# Backend dependencies — Go 1.25+, GNU make.
make build

# Frontend dependencies — Node 20+, pnpm 9+.
cd web && pnpm install && pnpm build && cd ..
make build     # rebuild the binary now that the UI is embedded

./bin/packrune users add --email you@example.com --username you --password 'pw' --admin
./bin/packrune
```

That's a working server at `http://localhost:8080`. Source-modifying loop:

```bash
# Terminal 1 — backend
./bin/packrune

# Terminal 2 — frontend with hot reload, proxied to backend
cd web && pnpm dev   # http://localhost:5173

# Terminal 3 — tests on file change (optional, use any watcher)
make test
```

All `make` targets:

| Target                  | What it does                                  |
| ----------------------- | --------------------------------------------- |
| `make build`            | Build `bin/packrune` with embedded UI         |
| `make run`              | `go run ./cmd/packrune`                       |
| `make test`             | Unit tests with `-race`                       |
| `make test-integration` | Integration tests under `test/`               |
| `make lint`             | `golangci-lint run`                           |
| `make fmt`              | `gofmt` + frontend formatter                  |
| `make cover`            | Coverage profile                              |
| `make check-headers`    | SPDX header enforcement                       |
| `make web-install`      | `pnpm install` in `web/`                      |
| `make web-dev`          | Frontend dev server                           |
| `make web-build`        | Compile UI into `internal/web/dist`           |
| `make clean`            | Remove build artefacts                        |

---

## The `ptcli` operator

Repo ships with `./ptcli`, an interactive control panel. Banner, status line,
menu groups for setup, build, run, docker, admin, helpers:

```bash
./ptcli            # interactive menu
./ptcli doctor     # quick diagnostic
./ptcli full       # build + serve + URL panel
./ptcli backup     # tar.gz snapshot of SQLite + storage
./ptcli passwd     # reset a user's password
```

Every menu item has a name alias (`./ptcli backup`, `./ptcli gc`,
`./ptcli docker-up`, …) so it composes into shell pipelines and crontabs.

---

## Supported formats

Every format is a real wire-protocol implementation, not a thin proxy.

### Docker / OCI

```bash
# Configure docker daemon: { "insecure-registries": ["localhost:8080"] }
# (or run Packrune behind TLS in production)

docker tag alpine:3.20 localhost:8080/library/alpine:3.20
docker push localhost:8080/library/alpine:3.20
docker pull localhost:8080/library/alpine:3.20

curl http://localhost:8080/v2/_catalog
curl http://localhost:8080/v2/library/alpine/tags/list
```

### npm

```bash
# In a package directory
npm publish --registry http://localhost:8080/npm/

# As a consumer
npm install your-package --registry http://localhost:8080/npm/
```

`.npmrc` for an entire org:

```ini
registry=http://localhost:8080/npm/
@yourscope:registry=http://localhost:8080/npm/
```

### Helm

```bash
helm plugin install https://github.com/chartmuseum/helm-push  # one-off
helm cm-push ./mychart-0.1.0.tgz http://localhost:8080/helm/

helm repo add packrune http://localhost:8080/helm/
helm install my-release packrune/mychart
```

### Go modules

Modules are pushed via PUT (CI publishes), pulled through GOPROXY:

```bash
curl -X PUT --data-binary '{"Version":"v1.0.0","Time":"2026-05-17T00:00:00Z"}' \
  http://localhost:8080/go/example.com/m/@v/v1.0.0.info
curl -X PUT --data-binary @go.mod \
  http://localhost:8080/go/example.com/m/@v/v1.0.0.mod
curl -X PUT --data-binary @module.zip \
  http://localhost:8080/go/example.com/m/@v/v1.0.0.zip

# Consumers
export GOPROXY=http://localhost:8080/go,direct
go get example.com/m@v1.0.0
```

### PyPI

```bash
twine upload --repository-url http://localhost:8080/pypi/ dist/*

pip install --index-url http://localhost:8080/pypi/simple/ mypackage
```

### Maven

In your `pom.xml`:

```xml
<distributionManagement>
  <repository>
    <id>packrune</id>
    <url>http://localhost:8080/maven/</url>
  </repository>
</distributionManagement>
```

```bash
mvn deploy
mvn dependency:get -Dartifact=com.example:mylib:1.0.0
```

---

## Repository types

Every format supports three repository kinds:

### Hosted

Your team pushes; Packrune stores. On a fresh install one hosted repo per
format is created automatically (`docker`, `npm`, `helm`, `go`, `pypi`,
`maven`).

### Proxy

Cache an upstream registry. First request fetches from upstream and stores
in CAS; every subsequent request serves from local. Supports HTTP basic
auth for private upstreams.

Configuration is a JSON blob on the repository row:

```json
{
  "upstream": "https://registry-1.docker.io",
  "username": "optional",
  "password": "optional"
}
```

Typical upstreams:

| Format | Common upstream                   |
| ------ | --------------------------------- |
| Docker | `https://registry-1.docker.io`    |
| npm    | `https://registry.npmjs.org`      |
| Helm   | `https://charts.bitnami.com/bitnami` |
| Go     | `https://proxy.golang.org`        |
| PyPI   | `https://pypi.org`                |
| Maven  | `https://repo1.maven.org/maven2`  |

### Group

One URL that fans out across multiple member repos. First member that has
the artifact wins. Useful when a build needs to resolve from your hosted
repo first and fall back to a public proxy.

```json
{ "members": ["docker", "dockerhub-proxy"] }
```

Member resolution lives in the store layer
([`internal/repo/store.go`](internal/repo/store.go)) so every format gets it
without per-adapter code.

---

## Web UI tour

The admin UI lives at `/`. Every page is responsive at desktop widths and
internationalised (English + Turkish out of the box; adding a language is one
JSON file in [`web/src/i18n/`](web/src/i18n/)). Five themes ship by default:
**Aurora**, **Midnight**, **Daybreak**, **Terminal**, **Mono** — switch live
from the Profile page or the picker in the top right.

Pages:

- **Dashboard** — bento layout of stats, recent repositories, per-format
  artifact counts.
- **Repositories** — every repo as a glass card with the format colour
  badge; click into the detail view.
- **Repository detail** — paginated artifact list with substring filter and
  download counts.
- **Tokens** — issue scoped tokens for CI / clients. The plaintext is shown
  once at creation and never again.
- **Users** (admin) — provision accounts, mark admin, deactivate.
- **Webhooks** (admin) — subscribe to events (`artifact.created`,
  `artifact.deleted`, `repo.created`, …). Payloads are signed with
  HMAC-SHA256 in the `X-Packrune-Signature` header. Failed deliveries
  retry with exponential backoff up to 32 minutes.
- **Audit log** (admin) — every authentication decision + write action,
  newest first, 15-second auto-refresh.
- **Profile** — theme picker, language picker, account info.
- **Settings** (admin) — read-only view of the active configuration with
  secret fields redacted.

Press **⌘K** (or Ctrl+K) anywhere to open the command palette and fuzzy-jump
to any page or repository.

---

## Configuration reference

Configuration loads in three layers; later layers override earlier:

1. Built-in defaults (single-node SQLite + local filesystem, port 8080).
2. YAML file at the path passed to `--config` (default `packrune.yaml`).
3. Environment variables of the form `PACKRUNE_<SECTION>_<KEY>`.

The full schema:

```yaml
server:
  addr: ":8080"
  external_url: "https://packrune.example.com"
  read_timeout: 60        # seconds; 0 disables
  write_timeout: 0        # 0 lets large uploads run as long as they need

database:
  driver: sqlite          # sqlite | postgres
  dsn: "/data/packrune.db" # or postgres://user:pass@host:5432/db?sslmode=...
  max_open_conns: 25
  max_idle_conns: 5

storage:
  backend: fs             # fs | s3
  fs:
    root: "/data/storage"
  s3:
    endpoint: ""          # leave empty for AWS S3; set for MinIO/R2/etc.
    bucket: ""
    region: "us-east-1"
    access_key: ""
    secret_key: ""
    use_path: false       # MinIO and some R2 setups need true

auth:
  token_secret: ""        # auto-generated on first start if blank; 32+ bytes
  session_ttl: 24         # hours
  allow_signup: false     # admins provision accounts

log:
  level: info             # debug | info | warn | error
  format: text            # text | json
```

Environment overrides map directly: `PACKRUNE_DATABASE_DRIVER=postgres`,
`PACKRUNE_STORAGE_S3_BUCKET=mybucket`, etc.

---

## Operations

### Backup

```bash
packrune backup --output snapshot.tar.gz
```

Captures SQLite (`packrune.db` + WAL + SHM) and the entire filesystem
storage tree. For Postgres deployments use `pg_dump`; for S3-backed
storage snapshot the bucket out-of-band.

### Restore

```bash
packrune restore --input snapshot.tar.gz [--force]
```

`--force` is required if a database already exists at the configured DSN.

### Garbage collection

`packrune gc` walks the CAS for blobs that no `artifacts` row references
and deletes them. Safe to run while serving traffic.

```bash
packrune gc --dry-run     # report orphans + freed bytes, no deletes
packrune gc               # actually delete
```

### Diagnostics

```bash
packrune doctor
```

Reports green/yellow/red on config, runtime, port availability, DB,
migrations, repositories, storage, and format registry. Exits non-zero on
any failure — composes cleanly into liveness checks.

### Health endpoints

| Path        | Use                                              |
| ----------- | ------------------------------------------------ |
| `/healthz`  | Liveness — returns 200 if the process is running |
| `/readyz`   | Readiness — returns 200 if DB and storage are up |
| `/version`  | Build info JSON                                  |
| `/api/system/stats`   | Repo + artifact counts (auth required) |
| `/api/system/config`  | Sanitised live config (admin only)     |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       packrune binary                           │
│                                                                 │
│  ┌──────────────┐   ┌──────────────┐   ┌────────────────────┐   │
│  │  HTTP server │   │  Admin UI    │   │  Background jobs   │   │
│  │   (chi)      │   │  (embedded   │   │  - webhook retry   │   │
│  │              │   │   React SPA) │   │  - audit writer    │   │
│  └──────┬───────┘   └──────┬───────┘   └──────────┬─────────┘   │
│         │                  │                      │             │
│         ▼                  ▼                      │             │
│  ┌─────────────────────────────────────────┐      │             │
│  │           Format adapters                │      │            │
│  │  docker · npm · helm · go · pypi · maven │      │            │
│  │  (each implements internal/format.Format)│      │            │
│  └──────────────┬──────────────────────────┘      │             │
│                 │                                 │             │
│                 ▼                                 ▼             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Core services                         │   │
│  │  - Repository store (hosted/proxy/group resolution)      │   │
│  │  - Auth (users, tokens, OIDC*, RBAC*)                    │   │
│  │  - Audit log                                             │   │
│  └──────────┬─────────────────────────────┬────────────────┘    │
│             │                             │                     │
│             ▼                             ▼                     │
│  ┌──────────────────────┐    ┌────────────────────────────┐     │
│  │   Storage backend    │    │      Metadata store        │     │
│  │  (interface)         │    │  (Postgres or SQLite)      │     │
│  │  - fs · s3           │    │                            │     │
│  │  + CAS dedup layer   │    │                            │     │
│  └──────────────────────┘    └────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
                                          * tracked in PHASES.md
```

Components:

- **HTTP layer** is a `chi` router. Each format mounts its handlers under a
  dedicated path prefix (`/v2`, `/npm`, …) via `http.StripPrefix`.
- **Storage abstraction** is content-addressable. A SHA-256-keyed blob is
  shared by every repository that references it; identical layers across
  Docker images and npm tarballs occupy disk exactly once.
- **Format adapters** all implement the same `internal/format.Format`
  interface. Adding a new format means creating a new sub-package and
  implementing the contract.
- **Metadata** lives in Postgres or SQLite; the same SQL works on both via
  a dialect-aware placeholder rebinder.
- **Background jobs** include the webhook retry worker (exponential backoff,
  up to 6 attempts, ~32 minute max delay) and the audit log writer.

Full design notes live in [`docs/architecture.md`](docs/architecture.md).
Architectural decisions are recorded as ADRs under [`docs/adr/`](docs/adr).

---

## Developer handbook

### Contribution flow

External contributors:

1. **Fork** `packrune/packrune` to your own GitHub account.
2. `git checkout -b feature/<short-name>` from `main`.
3. Make your change; ensure `make build && make test && make lint` is clean
   locally before pushing.
4. Open a pull request against `packrune/packrune:main`. Describe **why**
   in the PR body; the diff already shows the what.
5. By submitting a PR you accept the [CLA](CLA.md). It exists to keep the
   "always free, never sold" guarantee legally enforceable — read it once,
   then forget about it.

Maintainers:

1. Branch directly off `main` for small fixes; use `feature/<name>` for
   anything that touches multiple files or surfaces.
2. Force-pushes on `main` are banned (`refs/heads/main` is protected).
3. Tag releases as `vMAJOR.MINOR.PATCH[-prerelease]`; the GitHub Actions
   release workflow takes care of multi-arch images and binary archives.

### House rules

- **One pattern, repeated.** When you add a new format, copy the closest
  existing one and adjust. Resist creating new abstractions for a single
  caller.
- **No `utils`, `helpers`, `misc`, `common`.** They become unsearchable
  junk drawers. Name files for what they do.
- **Comments explain WHY, not WHAT.** Code says what; comments say why a
  non-obvious decision exists. Every exported symbol gets a godoc; every
  source file gets a 2-3 line "what's in here" comment at the top.
- **Errors have one shape.** Wrap with `fmt.Errorf("doing X: %w", err)`,
  log with structured fields, return. No mixed `panic` / `return err` in
  the same package.
- **No `TODO`, `FIXME`, or "temporary" code in merged PRs.** Open an
  issue and link it, or finish the work.
- **Tests live next to code.** `foo.go` → `foo_test.go`. Integration tests
  go under `test/`.
- **SPDX header on every source file** (see
  [`scripts/check-headers.sh`](scripts/check-headers.sh)). CI rejects the
  PR otherwise.

### Adding a new package format

Every adapter implements [`internal/format.Format`](internal/format/format.go).
To add, for example, a Cargo / crates.io format:

1. `cp -r internal/format/_template internal/format/cargo`
   (no template exists yet; use `internal/format/gomod` as a reference —
    it's the simplest current adapter).
2. Implement `Name`, `DisplayName`, `Routes`, `OnUpload`, `OnDelete`,
   `Index`.
3. Register in an `init()` so the format becomes available on import.
4. Mount the handler in [`cmd/packrune/serve.go`](cmd/packrune/serve.go)
   under its URL prefix, wrapped in `http.StripPrefix`.
5. Add an integration test in `internal/format/cargo/handler_test.go`
   exercising the protocol's wire shape (publish, fetch, list, …).
6. Document the consumer-side commands in this README's
   [Supported formats](#supported-formats) table.

### Adding a language

Translations are `web/src/i18n/<lang>/common.json`. To add Spanish:

1. `cp -r web/src/i18n/en web/src/i18n/es`
2. Translate the JSON values.
3. Register the locale in [`web/src/i18n/index.ts`](web/src/i18n/index.ts).
4. Open a PR — that's it.

Keys stay in English so they're greppable across the codebase. RTL is
supported by Tailwind's `dir-` modifiers but not yet exercised; an Arabic or
Hebrew translation would be the first real RTL test.

### Adding a theme

Themes are CSS custom-property overrides under `:root[data-theme="..."]`.
Add a block in [`web/src/styles/global.css`](web/src/styles/global.css),
register the theme in [`web/src/themes/themes.ts`](web/src/themes/themes.ts)
with its swatch colours, and translate the label in each `i18n/*/common.json`.

### Where to start reading the code

In order of importance:

1. [`internal/format/format.go`](internal/format/format.go) — the
   interface every adapter implements. ~80 lines.
2. [`internal/repo/store.go`](internal/repo/store.go) — repository CRUD
   and the `ResolveArtifact` / `ResolveArtifactsByPrefix` group-resolution
   path.
3. [`internal/storage/cas/cas.go`](internal/storage/cas/cas.go) — the
   content-addressable layer that gives us cross-format dedup.
4. [`internal/format/docker/`](internal/format/docker/) — the most
   complete adapter; pick this when you're learning the format pattern.
5. [`cmd/packrune/serve.go`](cmd/packrune/serve.go) — the wiring file.
   Everything else is referenced from here.

---

## Project layout

```
packrune/
├── cmd/packrune/        main + subcommand entrypoints
├── internal/
│   ├── api/             JSON admin API
│   ├── audit/           audit log writer + reader
│   ├── auth/            users, tokens, password hashing
│   ├── config/          YAML + env loader
│   ├── db/              sql.DB wrapper + migration runner
│   ├── format/          format adapters
│   │   ├── docker/      Docker / OCI registry
│   │   ├── gomod/       Go module proxy
│   │   ├── helm/        Helm chart repo
│   │   ├── maven/       Maven 2 layout
│   │   ├── npm/         npm registry
│   │   └── pypi/        PyPI simple + JSON
│   ├── log/             slog setup
│   ├── repo/            Repository entity + Store
│   ├── server/          HTTP server + middleware
│   ├── storage/         blob backend interface
│   │   ├── fs/          filesystem backend
│   │   └── cas/         content-addressable layer
│   ├── web/             embed.FS for the React SPA
│   └── webhook/         dispatcher + retry worker
├── migrations/          SQL schema migrations (embedded)
├── web/                 React + Vite + Tailwind frontend
│   ├── src/
│   │   ├── components/  shared components (Glass, Aurora, Shell, …)
│   │   ├── i18n/        translations
│   │   ├── lib/         API client
│   │   ├── pages/       route components
│   │   ├── themes/      theme system
│   │   └── styles/      global CSS + design tokens
├── docs/                architecture, ADRs, format notes
├── deploy/              Docker compose, Helm chart, systemd unit
├── scripts/             repo tooling (SPDX header check, …)
├── test/                cross-package integration tests
├── ptcli                operator control panel (bash)
└── Makefile             build/test/lint targets
```

---

## Roadmap

[`PHASES.md`](PHASES.md) is the canonical roadmap. Every item is a
checkbox; every checked box is a feature that works today. The file
follows a "phase 0 through phase 12" structure that mirrors the order
the project was built in. The current bias of unchecked items is enterprise
auth (OIDC / SAML / LDAP / 2FA), active-active replication, and the
mobile-responsive UI pass.

If you're looking for a first contribution, the items tagged "deferred" in
PHASES.md are the most scoped — each is a finite piece of work with a
clear acceptance criterion.

---

## License

[GNU Affero General Public License v3.0](LICENSE) with the
[Commons Clause License Condition v1.0](COMMONS-CLAUSE).

In plain English:

- Run it, self-host it, modify it. Including inside a company. For free.
- Fork it, contribute back, build community tooling around it.
- You may **not** sell Packrune.
- You may **not** sell a hosted/managed service whose value comes from Packrune.

[`NOTICE`](NOTICE) is a one-page summary in non-lawyer English.
[`LICENSE`](LICENSE) and [`COMMONS-CLAUSE`](COMMONS-CLAUSE) are the
source-of-truth legal text.

By submitting a contribution you agree to the terms in [`CLA.md`](CLA.md).
The CLA exists for exactly one reason: to make the "always free, never
sold" promise legally unbreakable, even by the maintainers.
