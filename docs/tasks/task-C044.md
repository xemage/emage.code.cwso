# Task C044 — UDS perms 0o660 + shared GID (or documented limitation)

**ID:** C044
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B12, P2-5); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md
**Re-scoped:** 2026-08-27 (orchestrator, pre-dispatch) — see "Orchestrator finding" below.
This is now a **verification + doc-correction + documentation** task, not a
socket-permission implementation task. Read the finding before doing anything else.

## Orchestrator finding (verify independently — do not just trust this)

This brief and `docs/DEBT-REGISTER.md` row B12 both currently claim the sidecar UDS
sockets are bound `0o666` (world read-write) and need tightening to `0o660` with a
shared GID. While scoping Phase 4 for dispatch, direct inspection of `origin/develop`
found this claim does **not** match the current code:

- `services/cwso-git-shadow/src/main.rs` (near the `UnixListener::bind` call) already
  calls `std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))`
  — confirmed present since the very first sidecar commit (`35c556e`), i.e. this was
  never actually `0o666` in shipped code, contrary to the original P2-5 scorecard note.
- `services/cwso-merge-engine/src/ipc.rs` binds its socket the same way, also already
  `0o660`.
- The shared-GID half of the requirement also appears already satisfied: **T197**
  (merged, PASS) corrected `CWSO_IPC_ALLOWED_GIDS` in `deploy/docker-compose.yml` from
  a stale `"0,100"` to the orchestrator image's real live gid `"0,101"` for both
  `git-shadow` and `merge-engine`, and added a regression check
  (`scripts/check-ipc-gid-drift.sh`) so this can't silently drift again. Both services'
  `IpcAuthzPolicy::from_env()`/`allows()` logic is `uid OR gid`, already confirmed
  working by T197's own independent Tech Lead review.

**This finding was not independently re-verified as part of a live, working stack** —
it is a static-source read only. Do not skip your own verification pass (Step 1 below)
on the strength of this note alone; confirm it yourself against the real running
containers before touching any tracked file.

## Objective

Given the finding above, this task is now:

1. **Independently re-verify** (live, against a real running stack, not just source
   reading) that both sidecar sockets are genuinely `0o660` and that IPC across the
   currently-configured GIDs genuinely works — do not take the orchestrator's
   source-level finding above as sufficient on its own.
2. **Correct the stale text**: `docs/tasks/task-C044.md` (this file — update the
   "Objective"/historical framing once your own verification confirms or refines the
   finding above) and `docs/DEBT-REGISTER.md` row B12 (currently says `0o666`,
   `open`/`v1.0-blocker`; if your verification confirms the fix is already in place,
   correct the description and set B12 to `closed`/`fixed`, citing the actual
   evidence — file:line, live verification output, T197's MR — the same way every
   other closed row in that register cites its evidence).
3. **Write the still-missing `SECURITY.md` section** documenting the IPC authorization
   model: socket permission bits (`0o660`), the `CWSO_IPC_ALLOWED_UIDS`/
   `CWSO_IPC_ALLOWED_GIDS` allowlist mechanism, the `uid OR gid` semantics, where the
   values come from (live image uid/gid lookup, not hardcoded), and the drift
   regression check (`scripts/check-ipc-gid-drift.sh`). `SECURITY.md` currently has
   **zero** mentions of sockets/UDS/IPC at all — this documentation gap is real and
   independent of whether the permission fix itself needed (re)doing.

If, contrary to the orchestrator's finding, your own live verification finds the
sockets or GID alignment are **not** actually correct today (e.g. a regression, or a
container/image variant this finding didn't check), fall back to this brief's
original intent: implement the real fix (tighten to `0o660` with a shared GID) or
document the limitation in `SECURITY.md`, per the original "You MUST" list below —
whichever the evidence supports. Do not force a "verification only" outcome if the
system doesn't actually behave as the finding claims.

## Inputs

- `services/cwso-git-shadow/src/main.rs` (socket bind; already `0o660`, re-verify)
- `services/cwso-merge-engine/src/ipc.rs` (socket bind; already `0o660`, re-verify)
- `deploy/docker-compose.yml` (`CWSO_IPC_ALLOWED_UIDS` / `CWSO_IPC_ALLOWED_GIDS` env,
  corrected by T197 — re-verify the values are still live-correct)
- `scripts/check-ipc-gid-drift.sh` (T197's regression check — run it)
- Scorecard P2-5 (`docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`) — historical source
  of the (now apparently stale) `0o666` claim
- `docs/DEBT-REGISTER.md` (B12 row)
- `SECURITY.md` (where the new IPC-authorization section goes)
- `docs/tasks/completed-tasks.md` T197 entry (context on the GID fix and its own
  independent Tech Lead review)

## Rails (read before starting)

### You MUST
- Independently, live-verify current socket permissions and GID alignment against a
  real running stack (`docker compose up`, `stat -c '%a'` on both sockets, confirm
  IPC actually round-trips) — do not rely on source-reading alone, and do not rely on
  the orchestrator's finding alone
- Run `bash scripts/check-ipc-gid-drift.sh` and report its real output
- Run the smoke test (C018, `scripts/cwso-smoke-test.sh` or `make smoke-local`) to
  confirm IPC end-to-end
- Correct `docs/DEBT-REGISTER.md` B12 to match verified reality (status/disposition/
  evidence), whichever way the evidence points
- Write the new `SECURITY.md` IPC-authorization section per Objective #3
- If (and only if) your own verification finds a real gap the orchestrator's finding
  missed: implement the real fix or document the limitation, per the original rails
  below, and still close out B12 honestly (`fixed` or `documented-limitation`, never
  ambiguous)

### You MUST NOT
- Weaken any other socket, file, or container permission
- Break IPC — the smoke test is the arbiter
- Leave the DEBT-REGISTER outcome ambiguous
- Touch the orchestrator's auth/JWT code
- Claim the fix is "already done" in `docs/DEBT-REGISTER.md` without your own live
  verification evidence backing that claim (a corrected static-source claim is not
  good enough for closing a `v1.0-blocker` row — this project's established bar, see
  every other `fixed` row in the register, requires reproduced evidence)

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `services/cwso-merge-engine/**` (socket bind only, if a real fix turns out to still be needed), `deploy/docker-compose.yml` (GID alignment only, if needed), `SECURITY.md`, `docs/DEBT-REGISTER.md` (B12 row), `docs/tasks/task-C044.md` (this file's own historical framing/execution notes)
- **Must NOT touch:** `orchestrator/*` (except reading), other services, `schemas/*`

## Steps (execute in order)

1. Stand up a real stack (`docker compose -f deploy/docker-compose.yml up -d --build`); independently check both sockets' live permission bits and both containers' effective uid/gid.
2. Run `scripts/check-ipc-gid-drift.sh` and the smoke test; capture real output.
3. Based on real evidence (not the orchestrator's static finding alone): either confirm the fix is already in place, or implement it / document the limitation if it genuinely isn't.
4. Correct `docs/DEBT-REGISTER.md` B12 and this brief's historical framing to match verified reality, citing evidence.
5. Write the new `SECURITY.md` IPC-authorization section.

## Expected outputs

- Live verification evidence (real command output, not assumed) for socket perms + GID alignment + IPC round-trip
- `docs/DEBT-REGISTER.md` B12 resolved with accurate, evidence-backed text
- New `SECURITY.md` section documenting the IPC authorization model
- (Only if your verification finds a real gap) an actual permission/GID fix

## Acceptance criteria

1. Live-verified evidence that sockets are `0o660` with working IPC, **or** a genuine limitation is documented in `SECURITY.md` — whichever the evidence supports
2. Smoke test passes
3. `docs/DEBT-REGISTER.md` B12 = `fixed` or `documented-limitation` (not blank, not stale text), with evidence cited
4. `SECURITY.md` has a real IPC-authorization section (did not exist before this task)

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml up -d --build
docker exec cwso-git-shadow stat -c '%a' /run/cwso/git-shadow.sock
docker exec cwso-merge-engine stat -c '%a' /run/cwso/merge-engine.sock
bash scripts/check-ipc-gid-drift.sh
make smoke-local   # or scripts/cwso-smoke-test.sh, per C018/C062 precedent
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/backend-developer/C044` from `develop`
- Commit: `docs(security): verify and document sidecar UDS authorization model` (or `fix(security): tighten sidecar socket permissions` if a real fix is still needed)
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
