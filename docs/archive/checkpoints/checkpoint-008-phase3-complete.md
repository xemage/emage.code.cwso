# Checkpoint 008 — Phase 3 Complete

**Date:** 2026-05-15  
**Sequence:** 008  
**Phase:** Phase 3 → Phase 4 transition  
**Written by:** Orchestrator

---

## Phase 3 Summary

Phase 3 (MCP Transport + Job Infrastructure) is **complete and promoted to develop** with all quality and security gates passed.

### Completed Tasks (this phase)

| ID | Title | MR | Commit | Status |
|----|-------|----|--------|--------|
| T030 | Streamable HTTP SSE transport | !3 | 47f4fbe | done |
| T031 | Async job runner pool | !4 | 8f3a83b | done |
| T032 | dispatch_concurrent_jobs tool | !5 | 3f1e1bc | done |
| T033 | Event-sourced memory broker | !6 | 93d9bb7 | done |
| T034 | Telemetry throttling + JSON-RPC notifications | !7 | 3d40be9 | done |
| T035 | Phase 3 integration tests | !8 | ec87db7 | done |
| T036 | Phase 3 Tech Lead gate (+ T036-fix) | !9 | 2c334bc | PASS |
| T037 | Phase 3 Security gate (+ T037-fix) | !10 | 31e12aa | PASS |

### Key Decisions

| ADR | Decision |
|-----|----------|
| ADR-T030 | RunHTTP accepts broker as optional parameter; nil falls back to eventbus-only SSE path |
| ADR-T034 | Telemetry throttle uses deterministic per-topic window with terminal-state bypass |
| ADR-T036-fix | pump() goroutine leak fixed via done-channel select; rateLimiterStore uses TTL eviction |
| ADR-T037-fix | Unknown JWT roles rejected 403 (deny-by-default); exp/iss/aud validation mandatory; per-IP SSE cap=10; security headers added |

### Artifacts produced

- `orchestrator/internal/transport/http.go` — HTTP/SSE transport with JWT auth, rate limiting, security headers
- `orchestrator/internal/transport/telemetry.go` — per-topic telemetry throttle
- `orchestrator/internal/jobs/manager.go` — async job manager
- `orchestrator/internal/memorybroker/broker.go` — event-sourced memory broker
- `orchestrator/internal/memorybroker/publisher.go` — tee publisher
- `orchestrator/internal/server/server.go` — server wiring
- `orchestrator/internal/integration/integration_test.go` — 4 Phase 3 integration tests
- `TECHNICAL-DEBT.md` — TD-01 through TD-09 registered

---

## Current State (develop HEAD: 31e12aa)

### All packages green
```
ok  github.com/emage/cwso/orchestrator/internal/config
ok  github.com/emage/cwso/orchestrator/internal/eventbus
ok  github.com/emage/cwso/orchestrator/internal/integration
ok  github.com/emage/cwso/orchestrator/internal/jobs
ok  github.com/emage/cwso/orchestrator/internal/memorybroker
ok  github.com/emage/cwso/orchestrator/internal/server
ok  github.com/emage/cwso/orchestrator/internal/shadow
ok  github.com/emage/cwso/orchestrator/internal/tools
ok  github.com/emage/cwso/orchestrator/internal/transport
```

---

## Phase 4 — Next tasks (all unblocked)

| ID | Title | Owner | Priority |
|----|-------|-------|----------|
| T040 | KVM/Firecracker host validation | devops-engineer | P0 |
| T041 | Docker baseline runner | devops-engineer | P0 |
| T042 | gVisor runner | devops-engineer | P0 |
| T043 | Firecracker runner + snapshot CoW | devops-engineer | P0 |
| T044 | Sandbox tier router | backend-developer | P0 |
| T045 | cwso-merge-engine Rust crate | backend-developer | P0 |
| T046 | AST diff + semantic merge algorithm | backend-developer | P0 |
| T047 | merge_concurrent_results tool | backend-developer | P0 |
| T048 | Conflict matrix escalation | backend-developer | P1 |

**T040 is the next P0 critical-path task.**

---

## Token budget

| Phase | Budget | Used (est.) |
|-------|--------|-------------|
| Phase 1 | 80k | ~30k |
| Phase 2 | 80k | ~45k |
| Phase 3 | 120k | ~95k |
| QA/Security (Phase 3 gates) | 60k | ~40k |
| **Phase 4** | **120k** | 0 |
