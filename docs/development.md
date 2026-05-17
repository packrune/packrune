# Development setup

Get a Packrune dev environment running in under ten minutes.

## Prerequisites

- **Go** 1.24+ (`brew install go` or [go.dev/dl](https://go.dev/dl/))
- **Node** 20+ (`brew install node` or [nodejs.org](https://nodejs.org/))
- **pnpm** 9+ (`npm install -g pnpm`)
- **golangci-lint** (`brew install golangci-lint`)
- **GNU make** (preinstalled on macOS/Linux)
- One of:
  - Nothing — Packrune runs against SQLite by default.
  - **Postgres** 15+ if you want to test the Postgres path.
  - **MinIO** if you want to test the S3 storage backend.

## First run

```bash
git clone https://github.com/packrune/packrune.git
cd packrune

# Backend — builds and runs against a local SQLite + filesystem storage.
make build
./bin/packrune --help

# Frontend — starts a Vite dev server that proxies API calls to the backend.
make web-install
make web-dev
```

The backend listens on `:8080` by default. The frontend dev server runs on
`:5173` and proxies `/api/*` and `/v2/*` (Docker registry) to `:8080`.

## Useful make targets

```text
make help                # list all targets
make build               # build the binary
make run                 # build + run with defaults
make test                # unit tests
make test-integration    # integration tests (slower)
make lint                # golangci-lint
make fmt                 # format Go + frontend
make check-headers       # verify SPDX headers
make cover               # coverage summary
make clean               # remove build artifacts
```

## Running against Postgres

```bash
docker run --rm -d --name packrune-pg \
  -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=packrune \
  -p 5432:5432 postgres:16

PACKRUNE_DB_DRIVER=postgres \
PACKRUNE_DB_DSN='postgres://postgres:dev@localhost:5432/packrune?sslmode=disable' \
  ./bin/packrune
```

## Running against S3 (MinIO)

```bash
docker run --rm -d --name packrune-minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=devkey -e MINIO_ROOT_PASSWORD=devsecret \
  minio/minio server /data --console-address ':9001'

PACKRUNE_STORAGE_BACKEND=s3 \
PACKRUNE_S3_ENDPOINT=http://localhost:9000 \
PACKRUNE_S3_BUCKET=packrune \
PACKRUNE_S3_ACCESS_KEY=devkey \
PACKRUNE_S3_SECRET_KEY=devsecret \
  ./bin/packrune
```

## Project layout

See [architecture.md](architecture.md) for a full map. Quick orientation:

- `cmd/packrune/` — entrypoint
- `internal/server/` — HTTP server + middleware
- `internal/format/<name>/` — package format implementations
- `internal/storage/` — blob backends
- `web/` — React frontend
- `docs/adr/` — design decisions, recorded

## Code style

- Run `make fmt && make lint && make test` before opening a PR.
- Every new source file gets the SPDX header — `make check-headers` enforces
  this, and CI will fail without it.
- See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the full house rules.

## Common pitfalls

- **"go: module not found"** — run `go mod tidy` once after pulling new deps.
- **Frontend can't reach API** — check the proxy config in
  `web/vite.config.ts`; you probably moved the backend port.
- **SQLite "database is locked"** — close any other Packrune instances. We
  default to SQLite WAL mode but only one writer is supported.
