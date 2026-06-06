# Phase 9 Validation Gate — Features E + F + G (Rollout-as-a-Service)

**Target:** Phase 9 rollout stack (T132–T137) on `develop`  
**Based on:** ADR-010, `rollout-architecture-v1.md`, `qa-phase9-report-v1.md`, tasks T132–T137  
**Date:** 2026-06-05  
**Gate MR:** T138 — merged !47 → `5d2cfca` (squash `011d8c8`)

Scope reviewed on `develop` after T137 merge (`c1c56d6`):

- **T132:** `cwso-rollout` hyper proxy + capture (MR !41, `267922c`).
- **T133:** Trajectory builder + prefix merging (MR !42, `18b5a40`).
- **T134:** Parquet/LZ4 trajectory store (MR !43, `26761ab`).
- **T136:** Programmatic merge rewards (MR !45, `892142f`).
- **T137:** Polar REST API (MR !46, `c1c56d6`, pipeline #2579885204).

Evidence: `go test ./... -race` green; `cargo test -p cwso-rollout` green; T137 CI all 11 jobs success.

---

## Gate Verdict: Implementation Review

**Gate:** implementation  
**Executor:** tech-lead  
**Date:** 2026-06-05  
**Target:** Phase 9 Features E+F+G (T132–T137)

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | scope | KV prefix router (T135) not landed; submit returns synthetic `prefix_key`. | Implement T135 before production trainer fleet; non-blocking for PoC gate. |
| 2 | low | scope | Orchestrator `/v1/chat/completions` is 501 stub; proxy on sidecar. | Document deployment: trainers point `base_url` at `cwso-rollout` when proxy enabled. |
| 3 | low | storage | Trajectory store v1 stores raw completion records; chain columns deferred. | Extend store schema when T137 session lifecycle hardens. |

### Summary

Rollout capture, trajectory assembly, Parquet persistence, merge rewards, and Polar REST API
deliver the architecture v1 contract. Trainer e2e integration tests pass in Go. No critical/high
findings.

---

## Gate Verdict: Security Audit

**Gate:** security  
**Executor:** security-engineer  
**Date:** 2026-06-05  
**Target:** Phase 9 Features E+F+G (T132–T137)

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | supply chain | CI audits remain `allow_failure: true` (inherited T094). | Harden before v0.3.0 production release (T139). |
| 2 | low | auth | Rollout REST routes require JWT (same as `/mcp`). | Ensure trainer clients use scoped tokens in production. |

### Security controls verified

- **Proxy redaction:** Authorization stripped at capture (T132).
- **UDS peer auth:** Sidecar IPC uses SO_PEERCRED pattern (ADR-010).
- **Deterministic rewards:** Merge SM table; no LLM-as-judge (architecture §11).
- **No secrets in trajectories:** Parquet schema excludes raw API keys.

Detailed checklist: `docs/artifacts/security-phase9-checklist-v1.md`.

---

## Combined Gate Outcome

| Gate | Verdict |
|------|---------|
| Implementation (Tech-Lead) | **PASS** |
| Security | **PASS** |

**Phase 9 Feature E+F+G gate: PASS** — proceed to T139 release readiness after T138 merge.
