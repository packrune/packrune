# Contributing to Packrune

Welcome. We're glad you're here. This document is short on purpose — you should
spend more time writing code than reading guidelines.

## Quick start

```bash
# Clone
git clone https://github.com/<org>/packrune.git
cd packrune

# Backend (Go 1.23+)
go build ./cmd/packrune
./packrune --help

# Frontend (Node 20+, pnpm 9+)
cd web
pnpm install
pnpm dev
```

Full local development setup, including running a Postgres or SQLite backing
store, is documented in [`docs/development.md`](docs/development.md).

## What we'd love help with

- Implementing a new package format (see [Adding a format](#adding-a-new-format))
- Translating the UI (see [`web/src/i18n/`](web/src/i18n/))
- Designing a new theme (see [`web/src/themes/`](web/src/themes/))
- Reporting bugs with a minimal reproducer
- Improving documentation, especially the ADRs

## License & CLA

Packrune is licensed under **AGPL-3.0 + Commons Clause** (see [LICENSE](LICENSE) and
[COMMONS-CLAUSE](COMMONS-CLAUSE)). In plain English: free to use, modify, and
self-host — including in a company — but **nobody can sell Packrune or sell a
hosted version of it.** That's a deliberate, permanent choice.

By submitting a contribution you agree to the terms in [CLA.md](CLA.md). The CLA
exists for one reason: to make sure nobody — including us — can ever relicense
your work to remove the "always free" guarantee.

Every new source file must include this SPDX header (or the language-appropriate
comment syntax of the same):

```go
// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors
```

## House rules for code

We optimize for **the next person to read this file**. That's how open source
projects stay alive.

1. **One pattern, repeated.** New code matches the shape of existing code in the
   same area. If you think the existing shape is wrong, open an issue first.
2. **Names tell you what.** `internal/format/docker/manifest.go` should not need
   a tour guide. No `utils.go`, `helpers.go`, `common.go`, `misc.go` — those are
   junk drawers.
3. **Comments explain WHY, not WHAT.** Every exported symbol gets a godoc
   comment. Every file starts with a 2–3 line summary of what it does. If the
   code does something non-obvious, write the *reason* in a comment.
4. **Errors have one shape.** Wrap with `fmt.Errorf("doing X: %w", err)`, log
   with structured fields, return — that's it. No mixed `panic` / `return err`.
5. **No "temporary" code.** No `// TODO: fix later` in merged PRs. Open an issue
   and link it, or finish the work.
6. **Tests live next to code.** `foo.go` → `foo_test.go`. Integration tests go
   under `test/`.

## Adding a new format

Every package format (npm, Docker, Maven, ...) implements the same Go interface
in [`internal/format/format.go`](internal/format/format.go). Adding a new format
should be:

1. `cp -r internal/format/_template internal/format/<yourformat>`
2. Implement the interface methods.
3. Register in `internal/format/registry.go`.
4. Add integration tests in `test/format/<yourformat>/`.
5. Update [`docs/formats/<yourformat>.md`](docs/formats/) with the protocol
   spec link and any quirks.

If your format needs core changes (new storage primitive, new auth shape), open
a design issue first — we'd rather talk before you spend a weekend.

## Translating the UI

Translations live in `web/src/i18n/<lang>/*.json`. To add a new language:

1. Copy `web/src/i18n/en/` to `web/src/i18n/<lang>/`.
2. Translate the JSON values (keys stay in English).
3. Register the language in `web/src/i18n/index.ts`.
4. Open a PR. We'll merge it. That's the whole process.

## Designing a new theme

Themes live in `web/src/themes/<name>.ts`. Each theme exports color tokens,
glass opacity values, gradient definitions, and motion characteristics. Copy the
closest existing theme and adjust.

## Reporting bugs

A useful bug report:
- The version (`packrune version`)
- What you did
- What you expected
- What happened
- Minimal config + logs

A great bug report includes a reproducer (`docker-compose.yml`, a shell script,
or a failing test). We will love you for it.

## Pull request etiquette

- Branch off `main`. Keep PRs focused on one thing.
- Run `make lint test` locally before pushing.
- Write a PR description that says **what** and **why**. The diff already shows
  *how*.
- Be patient with reviewers; be kind to reviewees.

That's it. Welcome aboard.
