# CWSO v1.0 Scope Statement

**Purpose.** This file is the single, quotable statement of what CWSO v1.0 means and what
it explicitly excludes. Every later phase, review, and release decision cites this file
instead of re-litigating scope. It exists to stop scope drift — the roadmap's §1.4
pattern — from recurring.

**Source.** Both sections below are quoted **verbatim** from
[`docs/plans/plan-cwso-v1.0-roadmap.md`](plans/plan-cwso-v1.0-roadmap.md)
(§1.5 "What 'v1.0' should mean" and §2.4 "Explicitly not in v1.0").

**Change control.** Changing this file requires a new version of the roadmap plan. The
quoted sections may only be updated by re-quoting a newer approved roadmap version —
never edited in place.

---

## What "v1.0" should mean

> A developer with Docker and one supported MCP client can, from a clean checkout, reach a
> working CWSO in **one command plus one config paste**, point it at **their own
> repository**, and have a sub-agent create a shadow workspace, edit real files at a real
> path, and merge the result back — with correct AST answers and an honest error whenever
> CWSO cannot do what was asked.

Everything in Part 2 serves that sentence. Anything that does not is v1.1.

---

## Explicitly not in v1.0

Each of these is real, working code. None is needed for the §1.5 definition. Freezing them
is the plan's main source of leverage.

| Deferred | Status | Re-entry |
|---|---|---|
| HAL / hardware-aware dispatch | built (`services/cwso-hal`) | v1.1 — keep working, don't extend |
| Sparse micro-agents | built (`services/cwso-sparse`) | v1.1 |
| Rollout / Polar trajectory capture | built (`services/cwso-rollout`) | v1.1 — opt-in profile only (C011) |
| SWE-bench evaluator (B11) | stub | After v1.0; the registry hook already exists |
| Terminal-Bench evaluator | not started | After v1.0. Benchmarking a pre-1.0 orchestrator measures its incompleteness. |
| Firecracker microVM tier | implemented with degraded fallback | v1.0 ships with the fallback path documented, not the tier promoted |
| Kubernetes operator, CRDs, autoscaling | not started | Not before v1.1, and only on observed demand |
| Merkle incremental AST indexer (P2-2) | not started | v1.1 — re-parsing is fine at v1.0 scale |
| Vault/SOPS secret management (T029) | not started | v1.0 is local-only; file-based secrets are acceptable **if** stated in `LIMITATIONS.md` |

---

## How to use this file

Cite this file in MR reviews whenever a proposed change exceeds v1.0 scope. If an MR adds
capability that is not required by the §1.5 definition, or touches anything on the §2.4
freeze list, link to this file and ask the author to either justify a roadmap version bump
or defer the work. Everything the roadmap defers is “Deferred to v1.1” or later — that label is
the roadmap's own (§2.2 phase graph). This file is the countermeasure to the §1.4 pattern:
building outward past open critical debt.
