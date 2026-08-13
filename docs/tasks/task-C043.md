# Task C043 — Connection pooling in the shadow client

**ID:** C043
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B13, P2-6); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

`orchestrator/internal/shadow/client.go` opens one connection per request and
serializes every RPC through `Client.mu` — it "will throttle under Phase-3 concurrent
dispatch", i.e. under the concurrency the product is named for. Add connection pooling
so concurrent dispatch is honest.

## Inputs

- `orchestrator/internal/shadow/client.go` (the client; P2-6 noted at line 5)
- `orchestrator/internal/tools/dispatch_tools.go` (the concurrent-dispatch caller)

## Rails (read before starting)

### You MUST
- Implement a bounded connection pool (configurable size, sensible default) over the UDS socket
- Remove the global serialization of all RPCs (per-connection synchronization is fine; one global mutex for everything is not)
- Add a soak test: N concurrent dispatches (N ≥ 16) complete without connection exhaustion, deadlock, or cross-talk between responses
- Remove the P2-6 marker and update `docs/DEBT-REGISTER.md` (B13 → `fixed`, closing task C043)
- Keep the client's external API unchanged for callers

### You MUST NOT
- Change the wire protocol or the tool surface
- Introduce unbounded connection growth (a leak under load is worse than a throttle)
- Touch the Rust git-shadow service (the fix is client-side)
- Add retry/circuit-breaker logic beyond pooling (v1.1)

## File ownership

- **May create/modify:** `orchestrator/internal/shadow/**`, `docs/DEBT-REGISTER.md` (B13 row)
- **Must NOT touch:** `services/*`, other `orchestrator/internal/*` packages, `schemas/*`

## Steps (execute in order)

1. Read the client and its callers.
2. Implement the bounded pool.
3. Soak test: N≥16 concurrent dispatches.
4. Remove marker; update DEBT-REGISTER.

## Expected outputs

- Pooled shadow client
- Concurrency soak test
- P2-6 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. N≥16 concurrent dispatches complete without exhaustion/deadlock/cross-talk
2. `go test ./internal/shadow/...` passes (including the soak test)
3. Client API unchanged for callers
4. DEBT-REGISTER B13 = `fixed` / C043

## Verification commands

```bash
cd orchestrator && go test ./internal/shadow/... -count=1
go vet ./internal/shadow/...
grep -n "P2-6" internal/shadow/client.go   # = no hits
```

## Git rails

- Branch: `agent/backend-developer/C043` from `develop`
- Commit: `perf(shadow): add connection pooling to sidecar client`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If pooling exposes a pre-existing race, do not paper over it with the old mutex —
report `technical` / `major` with a race-detector log (`go test -race`).

## Execution notes

<filled during execution>
