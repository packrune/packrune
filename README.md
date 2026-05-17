<div align="center">

# Packrune

**The fully free, fully self-hosted, universal artifact repository manager.**

One binary. Every format. Forever free.

[![License: AGPL-3.0 + Commons Clause](https://img.shields.io/badge/license-AGPL--3.0%20%2B%20Commons%20Clause-blue.svg)](LICENSE)
[![Status: Pre-alpha](https://img.shields.io/badge/status-pre--alpha-orange.svg)](PHASES.md)

</div>

---

## Why Packrune

Artifactory and Nexus call themselves "self-hosted" but ask enterprise money for
features every team needs: replication, SSO, audit, multi-format support. Their
free tiers are deliberately crippled to push you onto a $30k/year contract for
software you're running on your own metal.

Packrune flips that:

- **Every feature is free.** No "community" / "enterprise" split. There is one
  Packrune. It does everything.
- **Self-hosted means self-hosted.** One binary, one config file. No license
  server, no telemetry call-home, no nag screens.
- **Always free, forever.** The license (AGPL-3.0 + Commons Clause) makes it
  legally impossible for anyone — including the maintainers — to sell Packrune
  or sell hosted Packrune as a service. See [LICENSE](LICENSE) and
  [COMMONS-CLAUSE](COMMONS-CLAUSE).

## What Packrune does

A single artifact repository manager that natively speaks every major package
format:

| Format     | Status     | Phase |
| ---------- | ---------- | ----- |
| Docker / OCI | Planned  | 2     |
| npm        | Planned    | 4     |
| Helm       | Planned    | 5     |
| Go modules | Planned    | 6     |
| PyPI       | Planned    | 9     |
| Maven      | Planned    | 10    |

For every format you get the three repository types you actually use:

- **Hosted** — your own artifacts.
- **Proxy** — caches an upstream registry (Docker Hub, npmjs.org, ...).
- **Group** — one URL that fans out to multiple hosted + proxy repos.

Plus storage that doesn't waste disk (content-addressable dedup across
formats), an admin UI that doesn't look like SourceForge, multi-language
support, multiple themes, and a JSON API for everything.

## Status

Packrune is in **pre-alpha**. The roadmap and progress live in
[PHASES.md](PHASES.md) — every box ticked is a feature that works.

If you're here early, the most useful contributions right now are:

- Trying to break the foundation we're building (architecture review, security
  review).
- Designing themes (see [`web/src/themes/`](web/src/themes/) once the frontend
  scaffold lands).
- Translating once we have user-facing strings.

## Try it

Not yet — pre-alpha. Star the repo and watch [PHASES.md](PHASES.md) flip from
`[ ]` to `[x]`.

## Contributing

We mean it when we say we want contributors. Read [CONTRIBUTING.md](CONTRIBUTING.md);
it's short. Submit a PR; we'll respond. The CLA in [CLA.md](CLA.md) exists for
exactly one reason: to make the "always free, never sold" promise legally
unbreakable.

## License

[GNU Affero General Public License v3.0](LICENSE) with the
[Commons Clause License Condition v1.0](COMMONS-CLAUSE).

In plain English:

- ✅ Run it. Self-host it. Modify it. Use it inside your company for free.
- ✅ Fork it. Contribute back. Build community tooling around it.
- ❌ Sell Packrune.
- ❌ Sell a hosted/managed service whose value comes from Packrune.

If you want the legal text, read [NOTICE](NOTICE) for a one-page summary, or
read [LICENSE](LICENSE) and [COMMONS-CLAUSE](COMMONS-CLAUSE) for the source of
truth.
