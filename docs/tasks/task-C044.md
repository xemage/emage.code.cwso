# Task C044 — UDS perms 0o660 + shared GID (or documented limitation)

**ID:** C044
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B12, P2-5); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

The sidecar Unix sockets are created `0o666` (world read-write) — "not acceptable for
prod" per scorecard P2-5. Tighten to `0o660` with a shared GID between orchestrator
and sidecars, **or** — if the container UID/GID alignment makes that impractical this
phase — document the limitation in the security section. Either outcome closes B12
honestly.

## Inputs

- `services/cwso-git-shadow/src/main.rs` (socket bind; also merge-engine's)
- `deploy/docker-compose.yml` (`CWSO_IPC_ALLOWED_UIDS` / `CWSO_IPC_ALLOWED_GIDS` env)
- Scorecard P2-5 (`docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`)
- `SECURITY.md` (where the limitation goes if documented)

## Rails (read before starting)

### You MUST
- Attempt the real fix first: bind sockets `0o660`, align a shared GID across the orchestrator + sidecar containers (the `CWSO_IPC_ALLOWED_*` env vars already exist — use them)
- Verify orchestrator ↔ sidecar IPC still works after tightening (the smoke test must pass)
- If the fix lands: remove the P2-5 marker, update `docs/DEBT-REGISTER.md` (B12 → `fixed`, closing task C044)
- If the fix cannot land this phase: document the 0o666 limitation, its blast radius (private compose network, local-only deployment), and its v1.1 remediation in `SECURITY.md` and `docs/DEBT-REGISTER.md` (B12 → `documented-limitation`)

### You MUST NOT
- Weaken any other socket, file, or container permission to make this easier
- Break IPC to make perms tighter — the smoke test is the arbiter
- Leave the outcome ambiguous: the MR must state plainly which of the two outcomes shipped
- Touch the orchestrator's auth/JWT code

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `services/cwso-merge-engine/**` (socket bind only), `deploy/docker-compose.yml` (GID alignment only), `SECURITY.md`, `docs/DEBT-REGISTER.md` (B12 row)
- **Must NOT touch:** `orchestrator/*` (except reading), other services, `schemas/*`

## Steps (execute in order)

1. Locate both services' socket bind code and current perms.
2. Implement 0o660 + shared GID.
3. Run the smoke test (C018) to prove IPC still works.
4. Land the fix or document the limitation; update DEBT-REGISTER accordingly.

## Expected outputs

- Tightened socket perms (or documented limitation)
- DEBT-REGISTER B12 resolved one way or the other

## Acceptance criteria

1. Sockets are 0o660 with working IPC, **or** the limitation is documented in SECURITY.md
2. Smoke test passes
3. DEBT-REGISTER B12 = `fixed` or `documented-limitation` (not blank)

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml up -d --build
docker exec cwso-git-shadow stat -c '%a' /run/cwso/git-shadow.sock   # 660 if fixed
bash scripts/cwso-smoke-test.sh
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/backend-developer/C044` from `develop`
- Commit: `fix(security): tighten sidecar socket permissions` (or `docs(security): document UDS permission limitation`)
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
