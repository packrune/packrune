# ADR-0002: License choice — AGPL-3.0 + Commons Clause

**Status:** Accepted
**Date:** 2026-05-17

## Context

Packrune's founding premise is that it must be **free forever, for everyone, for
every use case** — and that **no one** (including the maintainers) should be
able to sell it or sell a hosted version of it. The competitive context is
Artifactory and Nexus, both of which ship "open source" editions that are
deliberately weaker than their commercial editions to drive sales.

We need a license that:

1. Permits anyone — individuals, companies, governments — to self-host Packrune
   for free, including for internal commercial purposes.
2. Forbids selling Packrune itself.
3. Forbids running Packrune as a paid managed service (the "AWS does Elasticsearch"
   scenario that ended permissive licensing for many infra projects).
4. Forbids creating closed-source paid derivatives.
5. Is enforceable.

## Decision

**GNU Affero General Public License v3.0** (`AGPL-3.0`)
**+ Commons Clause License Condition v1.0**

The AGPL provides:
- Strong copyleft — derivative works must be AGPL.
- Network-use clause (§13) — running a modified Packrune as a service requires
  publishing your modifications to users of that service.

The Commons Clause adds, on top of the AGPL grant:
- "The grant of rights under the License will not include the right to **Sell**
  the Software."
- "Sell" is defined broadly to include providing, for a fee, "a product or
  service whose value derives, entirely or substantially, from the functionality
  of the Software."

In combination this gives us:

| Use case                                            | Permitted? |
| --------------------------------------------------- | ---------- |
| Personal self-host                                  | ✅          |
| Corporate internal self-host                        | ✅          |
| Modify and use modified version internally          | ✅          |
| Modify and run modified version as network service  | ✅ but you must publish your changes |
| Sell copies of Packrune                             | ❌          |
| Sell a hosted Packrune service                      | ❌          |
| Sell closed-source paid fork                        | ❌          |
| Provide paid support / consulting                   | ⚠️ Grey — see "open questions" |

The CLA (see `CLA.md`) reinforces this contractually: contributors agree their
work will never be relicensed to remove the "no sale" condition. Even the
maintainers cannot rug-pull.

## Rejected alternatives

### Apache 2.0 / MIT

Permits commercial resale. Defeats premise #2 and #3 entirely. Would have led
exactly to the AWS-vs-Elasticsearch outcome.

### Plain AGPL-3.0

Permits *selling* the software. The copyleft network clause prevents
closed-source modifications, but a competitor could legally fork Packrune, keep
their fork open per AGPL, and sell hosted Packrune-the-product. Defeats
premise #3.

### PolyForm Noncommercial 1.0.0

Forbids *all* commercial use. Defeats premise #1 — companies could not use
Packrune internally for their own engineering org, which is exactly the
audience we want to liberate from Artifactory/Nexus fees.

### Elastic License v2 (ELv2)

Permits commercial internal use; forbids running as a managed service. Close to
our needs, but:
- Forbids "circumventing license key functionality" — implies a licensing
  model we don't want to bake in.
- Less battle-tested in courts than AGPL.
- Single-license rather than a well-known copyleft + well-known rider.

### Server Side Public License (SSPL)

MongoDB's license. The "all software needed to provide the service" obligation
is broad to the point of being chilling — would scare off legitimate corporate
self-hosters who fear they have to open-source their entire infrastructure.
We want corporate self-host to be friction-free.

### Business Source License (BSL)

Time-delayed open source (becomes Apache 2.0 after N years). The "becomes
Apache" part defeats the "forever free" promise — in N years anyone could fork
and sell. We want the protection to be permanent.

### Functional Source License (FSL) — Sentry

Also converts to Apache 2.0 after 2 years. Same flaw as BSL.

## Trade-offs we accept

- **OSI calls this "source-available", not "open source".** True. We do not
  qualify for OSI's specific definition because the Commons Clause restricts a
  field of use (commercial sale). We are at peace with this. The user-visible
  freedoms (run, modify, share, self-host) are stronger than what most
  "open source" projects offer in practice.
- **Some companies have policies against AGPL software.** Real cost. Mitigated
  by the fact that Packrune is a standalone service consumed via standard
  protocols (Docker, npm, etc.) — internal users do not link against it.
- **The Commons Clause has critics in the OSS community.** Real. The criticism
  is largely about projects that started permissive and added Commons Clause
  later (a perceived bait-and-switch). Starting from day one with this license
  avoids that critique entirely.

## Open questions

- **Paid support / consulting:** Commons Clause forbids services "whose value
  derives substantially from the Software." A consultant who helps a customer
  *operate* Packrune is arguably in the grey zone. Our current read: training,
  configuration help, custom integration work — fine. Selling "Packrune
  Hosting" or "Packrune Plus" — not fine. We will publish guidance under
  `docs/licensing-guidance.md` once the question gets asked in earnest.
- **Trademark:** "Packrune" should be registered as a trademark to prevent
  third parties from selling unrelated products under our name. Track in
  Faz 12.

## Mechanics

- `LICENSE` — full AGPL-3.0 text (verbatim from gnu.org).
- `COMMONS-CLAUSE` — Commons Clause License Condition v1.0 (verbatim).
- `NOTICE` — plain-English summary referencing both.
- Every source file carries the SPDX header:
  `SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause`
- `CLA.md` — contributor agreement that locks the license in.

## Revisit when

- A jurisdiction rules the Commons Clause invalid (extremely unlikely but
  possible).
- The OSI or FSF publishes a license that fits our needs more cleanly without
  losing any of premises 1–5.
