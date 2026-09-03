# Artifact: release-v1.0.0

## Metadata
- Producer: orchestrator
- Created: 2026-09-03
- Based on: docs/plans/plan-cwso-v1.0-roadmap.md, docs/plans/plan-cwso-v1.0-phase6-release-v1.md,
  docs/SCOPE-v1.0.md, docs/tasks/completed-tasks.md (C001–C063, T190–T204)
- develop tip: 420625c
- Prior GA tag: v0.6.1

## Latest release: v1.0.0

## Release intent

v1.0.0 is the culmination of the entire CWSO v1.0 roadmap (`docs/plans/plan-cwso-v1.0-roadmap.md`,
human-approved 2026-08-13). It delivers the roadmap's own testable definition of "v1.0"
(`docs/SCOPE-v1.0.md`, quoted verbatim from the roadmap §1.5):

> A developer with Docker and one supported MCP client can, from a clean checkout, reach a
> working CWSO in **one command plus one config paste**, point it at **their own
> repository**, and have a sub-agent create a shadow workspace, edit real files at a real
> path, and merge the result back — with correct AST answers and an honest error whenever
> CWSO cannot do what was asked.

This is not a claim — it is evidence. `scripts/cwso-smoke-test.sh` (C018) **is** the v1.0
definition-of-done executable, and it was re-run as this release's own entry criterion (see
"Evidence" below) on a genuinely fresh `git clone`, not a pre-warmed dev machine.

Six roadmap phases and one release-gate phase landed since the prior GA tag (v0.6.1,
2026-08-08):

| Phase | Theme | Tasks |
|---|---|---|
| 0 | Honest Baseline | C001–C005 |
| 1 | One-Command Stack | C010–C019 |
| 2 | Real Filesystem | C020–C024, C035 |
| 3 | Protocol Conformance | C030–C034, C036, C037 |
| 4 | Correctness | C040–C044, C042 |
| 5 | One Document | C050–C054 |
| 6 | Release Gate | C060, C061, C063, C062 (this release) |

Plus two fast-follow security fixes dispatched after C061's audit and shipped in this same
release: **T202** (dashboard rate-limit/logging gap, F-C061-01) and **T204** (RUSTSEC-2026-0269
wasmtime CVE patch, discovered as a side effect of T202's own CI pipeline).

## Install

```bash
# One command (recommended — this is the v1.0 headline feature, C016)
git clone https://gitlab.com/em-age/emage.code.cwso.git && cd emage.code.cwso
make up
# Bootstraps a dev JWT secret, builds all four images, starts the stack, waits for
# health, mints an MCP token, and prints a ready-to-paste MCP client config block.
# See docs/user/README.md for the full walkthrough and per-client config notes.

# Container registry (individual images)
docker pull registry.gitlab.com/em-age/emage.code.cwso/orchestrator:v1.0.0
docker pull registry.gitlab.com/em-age/emage.code.cwso/git-shadow:v1.0.0
docker pull registry.gitlab.com/em-age/emage.code.cwso/merge-engine:v1.0.0
docker pull registry.gitlab.com/em-age/emage.code.cwso/rollout:v1.0.0
```

## Evidence

Re-run at the actual release commit (`420625c`), from a genuinely fresh `git clone` into an
empty scratch directory on a host with **zero** pre-existing `.env.jwt.dev`, CWSO containers,
volumes, or networks (verified before starting, not assumed):

- **`make up`** (C016's release-gating condition): bootstrapped secrets, built all four
  images, started the stack, reached `/healthz` 200, minted a token, and printed a valid
  MCP client config block — zero manual steps, from a clean checkout.
- **`scripts/cwso-smoke-test.sh`** (C018, the v1.0 definition-of-done executable): all 7
  stages `[PASS]` — `health` → `create_shadow_workspace` → `write_shadow_file` →
  `query_ast` → `commit_shadow` → `merge_concurrent_results` → `teardown` — exit 0. Host
  confirmed fully clean afterward (`docker ps -a` / `volume ls` / `network ls` all zero
  `cwso` entries).
- **`docs/DEBT-REGISTER.md`**: 28/28 rows classified, zero unclassified (C060).
- **`docs/artifacts/security-v1.0-audit-v1.md`**: fresh full-v1.0-surface OWASP Top 10 audit,
  **VERDICT: PASS, zero CRITICAL/HIGH** (C061, closes T010). Both MEDIUM findings from that
  audit are closed in this same release: F-C061-01 by T202 (dashboard rate-limiting/logging),
  F-C061-02 (missing baseline CI security tools) is tracked as **T203**, deliberately
  sequenced to ship *after* v1.0.0 per an explicit coordinator decision — a pure
  CI-detection-gap with no active exploit path, unlike F-C061-01's live, exploitable-today gap.
- **`docs/LIMITATIONS.md`**: published (C063), covering all three required
  `documented-limitation` debt-register rows (B1, R-1, R-6) plus the two `v1.1`-classified
  LOW findings from C061 (R-11, R-12) and the explicitly-deferred feature set (§4).
- **`scripts/check-version-drift.sh`** (C001): passes — README "Current state" and the newest
  CHANGELOG heading both read `v1.0.0`.

## Highlights

### Phase 0 — Honest baseline (C001–C005)

The roadmap's own starting discipline: a CI-enforced guard (`scripts/check-version-drift.sh`)
so README and CHANGELOG can never silently disagree on the current version again (C001);
reconciled quick-start commands (C002); `docs/DEBT-REGISTER.md` published as the single
tracked-debt ledger (C003); the task ledger reconciled with its own briefs (C004); and
`docs/SCOPE-v1.0.md` (C005) — the single, quotable statement of what v1.0 means and
explicitly excludes, verbatim-sourced from the roadmap and change-controlled ever since.

### Phase 1 — One-command stack (C010–C019)

The single biggest day-to-day usability change in this release. `make up` (C016) collapses
a previously 7-step manual startup into one command: bootstrap a dev JWT secret (C012,
hardened for permission/ownership correctness by T191), build, start, wait for health, mint
a token (`scripts/cwso-token.sh`, C013), and print a ready-to-paste MCP client config block.
Supporting work: removed dead phase2/phase4 compose profile gates (C010); `cwso-rollout`
moved behind an explicit opt-in profile, not started by default (C011); enable-all-features
folded into compose defaults (C014); the user's own repository mountable read-write via
`CWSO_WORKSPACE_HOST` (C015); `scripts/cwso-doctor.sh` pre/post-flight diagnostics (C017);
and an explicit sandbox-trustworthiness decision establishing gVisor, not KVM, as the
default non-privileged path (C019) — the hardening precondition every later trust-boundary
decision in this release (C024's `noexec` tmpfs, C044's UDS permissions) builds on.

### Phase 2 — Real filesystem (C020–C024, C035)

ADR-012 decided shadow workspaces get a real, materialized filesystem path, not an
IPC-only abstraction (C020) — the NO-GO fallback (C025, conditional documentation) never
triggered. Implemented: write-side materialization (C021), write-back into the real git
object database (C022), and full lifecycle/crash-safety handling (C023). Hardened during
review: `fd`-anchored recursive read-back walks close a real TOCTOU-class path-confinement
finding, tracked as a v1.0-blocker and fixed before merge (C035, R-3). Proven end-to-end in
CI, not just unit-tested: a real compiled Go test binary, run via `docker exec` against the
live, materialized path inside the actual `noexec`/no-compiler-toolchain hardened container
(C024) — the same constraint C019 established.

### Phase 3 — Protocol conformance (C030–C034, C036, C037)

An honest, evidenced accounting of the hand-rolled MCP kernel's real conformance: a gap
table classifying every method Implemented/Partial/Missing against the spec (C030), ADR-013
deciding to keep the hand-rolled kernel and prove it via a conformance suite rather than
adopt an SDK (C031), and that suite itself — 16 test functions asserting spec-shaped
requests/responses/errors, catching and fixing two real bugs along the way (a
capability-advertised-but-never-delivered `listChanged` flag, and a collapsed
Parse-error/Invalid-Request error-code distinction) (C032). Verified against **3 real,
independently-implemented MCP clients** over both stdio and Streamable HTTP, not assumed
(C033) — which caught a genuine, reproducible bug: `resources/list` returned `null` instead
of `[]` when empty, a Go nil-slice-marshaling bug that crashed one real client outright,
fixed the same day (C036). A permanent CI contract-snapshot test now catches this whole bug
class going forward (C034). The OAuth-fallback/bearer-auth documentation gap C033 surfaced
was corrected to read as client-general, not VS-Code-specific (C037).

### Phase 4 — Correctness (C040–C044, C042)

Five tasks, two genuine HIGH-severity concurrency findings caught and fixed in review before
merge — neither rubber-stamped. Parent-commit tracking lets a workspace chain onto its real
git ancestor (C041); the first review round caught a real race (unsynchronized concurrent
commits against the same workspace could silently orphan one commit), fixed with a
per-workspace serialization primitive and an adversarially-reproduced regression test.
Connection pooling replaces a single global mutex-serialized socket with a bounded,
per-connection pool for real throughput (C043) — sequenced *after* C041's fix landed, since
C041's race was latent only because of the very mutex C043 removes. Real lexical
scope/binding resolution for `find_references`, across all four wired grammars, replacing
identifier-text matching (C040). UDS socket permissions re-verified live at `0o660` with the
correct peer-credential allowlist (C044); a second HIGH finding surfaced in re-review — the
opt-in `rollout` sidecar coincidentally shared IPC-authorized identity with a mount it never
uses — fixed by removing the unused mount entirely. Real three-way merge with a structured
conflict-escalation matrix, replacing silent/corrupted partial merges with per-unit
collision rows on genuine conflicts (C042) — the last task in this phase, hard-dependent on
C041's fix.

### Phase 5 — One document (C050–C054)

Five old, overlapping installation/IDE guides collapsed into one current, source-verified
user guide (C050), then deleted rather than left to rot in search results (C051, git history
preserves them). Deployment documentation received from emage.code's own knowledge-projection
layer via a paired handover (C052). Contributor-process documentation split cleanly out of
user documentation into a new root `CONTRIBUTING.md` (C053). Closed with a real
clean-machine verification: a fresh `git clone`, all 21 documented command blocks run for
real, zero genuine failures, including a deliberate-failure scenario validated against its
documented recovery path (C054) — the same discipline this release's own entry-criteria
check (see "Evidence" above) repeats at the actual release commit.

### Phase 6 — Release gate (C060, C061, C063, C062)

All 28 debt-register rows re-verified directly against current code and classified — zero
left unclassified (C060). A fresh, full-v1.0-surface OWASP Top 10 security audit — not a
resumption of the long-stale, never-started T010 — verified both statically and live against
a running stack: **PASS, zero CRITICAL/HIGH** (C061, closes T010, open since 2026-08-06).
`docs/LIMITATIONS.md` published, closing the loop on every `documented-limitation` debt row
(C063). This release (C062) is the last link in that chain.

### Security fast-follows (T202, T204)

Both closed in this same release, both independently reviewed by Tech Lead **and** Security
Engineer with real, reproduced evidence (not rubber-stamped):

- **T202** closes F-C061-01 (SECURITY:MEDIUM): the operator dashboard's `GET` routes
  silently inherited a rate-limiter exemption meant only for `/mcp`'s SSE stream, leaving
  150 wrong-token dashboard requests completely unthrottled and unlogged. Fixed with a
  narrow, path-specific exemption and structured auth-failure logging (IP + path only, never
  the attempted token).
- **T204** patches RUSTSEC-2026-0269 (CVSS 8.8, HIGH — a `wasmtime` filesystem sandbox
  escape), discovered as a side effect of T202's own CI pipeline. A `Cargo.lock`-only bump
  (`wasmtime` 36.0.13 → 36.0.14); confirmed not live v1.0 attack surface (`cwso-sparse` has
  no `docker-compose.yml` service block at all) but patched anyway since it's a hard
  `ci_must_pass` CI gate and a real, HIGH-severity finding in a sandbox-trust-boundary crate
  by category.

## What's changed

| Area | Scope | Details |
|---|---|---|
| Deployment/DX | `make up`/`down`/`logs`/`smoke`/`doctor` | One-command stack, diagnostics, definition-of-done smoke test (C016–C018) |
| Filesystem | Real, materialized shadow workspace paths | Write-side projection, write-back, lifecycle/crash-safety, TOCTOU hardening (C020–C024, C035) |
| Protocol | MCP conformance | Gap table, conformance suite, 3-client verification, 2 real bugs fixed (C030–C037) |
| Merge engine | Three-way merge + conflict matrix | Structured per-unit conflict rows, no silent/corrupted partial merges (C042) |
| Concurrency | Correctness hardening | 2 real HIGH-severity races found and fixed pre-merge (C041, C043) |
| Security | Dashboard rate-limiting, IPC identity, sandbox CVE | T202, C044, T204 |
| Documentation | Single user guide + LIMITATIONS.md + SCOPE-v1.0.md | C050–C054, C063, C005 |
| Process | Zero-unclassified debt register, fresh security audit | C060, C061 |

## Breaking changes

None. All prior MCP tool contracts, request/response shapes, and the `/mcp` HTTP transport
are unchanged (C032/C034 conformance work verifies this, not just asserts it). Existing
compose-based deployments continue to work; `make up` is a new, additive convenience path.

## Migration guide

No action required for existing deployments — drop-in replacement for v0.6.1. New
deployments should use `make up` per `docs/user/README.md` rather than the retired manual
multi-step flow.

## Known limitations

See [docs/LIMITATIONS.md](../LIMITATIONS.md) for the full, evidenced disclosure — every
`documented-limitation` debt-register row, the two `v1.1`-classified LOW security findings,
and the explicitly-deferred feature set (HAL, sparse micro-agents, Rollout/Polar capture,
SWE-bench/Terminal-Bench evaluators, Firecracker promotion, Kubernetes operator, Merkle
incremental indexing, Vault/SOPS secrets). Highlights:

- The MCP kernel implements a documented subset of the spec by design (ADR-013) — 6/16
  methods and 8/9 notifications are genuinely unimplemented in v1.0.
- The dev/Compose JWT signing secret is file-based, acceptable for v1.0's local-only
  deployment model, not for production.
- `git-shadow`'s shadow-workspace projection mount is `noexec` with no compiler toolchain
  in the runtime image — a deliberate hardening consequence (C019), CI works around it
  (C024).
- **T203** (missing baseline CI security tools, F-C061-02) is a known, tracked, P1 gap
  deliberately shipping *after* this release — a CI-detection gap, not an active exploit
  path.

See also [docs/DEBT-REGISTER.md](../DEBT-REGISTER.md) (full classification) and
[docs/artifacts/security-v1.0-audit-v1.md](security-v1.0-audit-v1.md) (the audit this
release's security posture is evidenced by).
