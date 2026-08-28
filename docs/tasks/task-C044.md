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

> **Backend-developer update (2026-08-27, execution):** independently re-verified live
> against a real running `docker compose` stack — this finding is confirmed accurate on
> every point (both sockets `0o660`, GID allowlists live-correct, IPC round-trips
> end-to-end via the smoke test's `merge_concurrent_results` stage). See "Execution
> notes" at the bottom of this file for full transcripts. No code fix was needed.

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

**Outcome: the orchestrator's pre-dispatch finding was independently confirmed live.**
This closed out as a verification + documentation task, with no code changes to
`services/cwso-git-shadow/**` or `services/cwso-merge-engine/**` and no `deploy/docker-compose.yml`
GID edits — none were needed.

### 1. Live verification performed (2026-08-27)

Brought up a real stack from this worktree:
```
bash scripts/cwso-bootstrap-secrets.sh          # generates .env.jwt.dev (gitignored, not committed)
docker compose -f deploy/docker-compose.yml up -d --build
```

Socket permission bits:
```
$ docker exec cwso-git-shadow stat -c '%a %U:%G %n' /run/cwso/git-shadow.sock
660 cwso:cwso /run/cwso/git-shadow.sock
$ docker exec cwso-merge-engine stat -c '%a %U:%G %n' /run/cwso/merge-engine.sock
660 cwso:cwso /run/cwso/merge-engine.sock
```

Effective container identities:
```
$ docker exec cwso-git-shadow id
uid=100(cwso) gid=101(cwso) groups=101(cwso)
$ docker exec cwso-merge-engine id
uid=100(cwso) gid=101(cwso) groups=101(cwso)
$ docker exec cwso-orchestrator id
uid=100(cwso) gid=101(cwso) groups=101(cwso)
```
All three containers in this build resolve to the same `uid=100/gid=101`, so the
orchestrator's connections satisfy `CWSO_IPC_ALLOWED_UIDS` directly (`0,100`), independent
of the GID allowlist — consistent with the "uid OR gid" `allows()` semantics and with
T197's own note that the GID entry was latent drift, not a live gap.

GID drift regression check:
```
$ bash scripts/check-ipc-gid-drift.sh
Live orchestrator 'cwso' identity: uid=100 gid=101
OK: git-shadow CWSO_IPC_ALLOWED_UIDS="0,100" contains live uid=100
OK: git-shadow CWSO_IPC_ALLOWED_GIDS="0,101" contains live gid=101
OK: merge-engine CWSO_IPC_ALLOWED_UIDS="0,100" contains live uid=100
OK: merge-engine CWSO_IPC_ALLOWED_GIDS="0,101" contains live gid=101
$ echo $?
0
```

Smoke test (`scripts/cwso-smoke-test.sh`), all 7 stages, including
`merge_concurrent_results` which round-trips both sockets end-to-end:
```
[PASS] health
[PASS] create_shadow_workspace (workspace_uuid=960bcdec-9d20-43f1-8056-065b3ebc2afc)
[PASS] write_shadow_file
[PASS] query_ast (find_definition SmokeGreet -> 1 hit(s))
[PASS] commit_shadow (commit_oid=ddcb9234e899a7185e4c4f8587089339937f5af8)
[PASS] merge_concurrent_results (outcome=success, status=merged)
[PASS] teardown (dropped 960bcdec-9d20-43f1-8056-065b3ebc2afc)
CWSO SMOKE TEST: ALL STAGES PASS
```
The script's own EXIT trap tore the stack down (`docker compose down -v --remove-orphans`)
cleanly afterward — verified with a final `docker ps`/`docker volume ls` (no CWSO
containers or volumes left running).

### 2. Conclusion vs. the orchestrator's pre-dispatch finding

Confirmed, with independent live evidence, not just corrected source-reading: both
sidecar sockets are genuinely `0o660` in a real running stack, and the currently-deployed
`CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS` values in `deploy/docker-compose.yml`
genuinely match the live orchestrator image's `cwso` uid/gid, with IPC actually
round-tripping (not just "listener bound" — the full MCP flow through both sidecars via
`merge_concurrent_results` passed). No regression or container/image variant was found
that the static finding missed. Per the brief's fallback rails, this means the original
"implement the fix" mandate does not apply — this stayed a verification + doc-correction
task throughout, and no fallback implementation branch of the brief was triggered.

### 3. What was changed

- `docs/DEBT-REGISTER.md`: B12 row corrected from `0o666`/`open`/`v1.0-blocker` to
  `closed`/`fixed`, with `file:line` citations and a new evidence note in the "Notes on
  the `fixed` rows" section quoting the live command output above (not the static-source
  claim alone). Phase-2 historical scorecard row for P2-5 updated to match.
- `SECURITY.md`: new "Sidecar IPC authorization (Unix domain sockets)" section — socket
  permission bits, the peer-credential `uid OR gid` allowlist mechanism
  (`CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS`), where those values come from (live
  image uid/gid lookup, never hardcoded), the drift regression check
  (`scripts/check-ipc-gid-drift.sh`), and a summary of this task's live verification
  evidence. `SECURITY.md` had zero prior mentions of sockets/UDS/IPC.
- `docs/tasks/task-C044.md` (this file): this Execution notes section.
- No changes to `services/cwso-git-shadow/**`, `services/cwso-merge-engine/**`, or
  `deploy/docker-compose.yml` — verification found no gap requiring them.
- `.env.jwt.dev` was generated locally by `scripts/cwso-bootstrap-secrets.sh` to bring the
  stack up; it is `.gitignore`d (`.env*` pattern) and was never staged or committed.

### 4. Acceptance criteria

1. Live-verified evidence that sockets are `0o660` with working IPC — **met**, see
   transcripts above; no genuine limitation was found, so no fallback documentation of a
   limitation was needed.
2. Smoke test passes — **met**, all 7 stages PASS.
3. `docs/DEBT-REGISTER.md` B12 = `fixed`, with evidence cited — **met**.
4. `SECURITY.md` has a real IPC-authorization section that did not exist before this
   task — **met**.

### Blocker status
None.

### 5. Addendum (2026-08-28) — follow-up fix for SEC-C044-001/002/003

Independent Security Engineer re-review of this task's C044 verification pass (sections
1–4 above) found a real gap the original verification-only pass missed. This addendum
documents the fix, applied as a **new, separate commit** on this same branch (not an
amendment to the original commit).

**Finding recap.**
- **SEC-C044-001 (HIGH):** the opt-in `rollout` service in `deploy/docker-compose.yml`
  (`profiles: ["rollout"]`, not part of the default stack) shared the same uid=100/gid=101
  `cwso` identity as orchestrator/git-shadow/merge-engine (coincidental: Debian
  `addgroup --system`/`adduser --system` in `deploy/Dockerfile.rollout` landing on the same
  numbers as Alpine's independently-assigned identity in `deploy/Dockerfile.orchestrator`)
  and mounted the same `cwso-runtime` volume both sidecar sockets live on. Because
  `IpcAuthzPolicy::allows()` (`services/cwso-git-shadow/src/main.rs`,
  `services/cwso-merge-engine/src/ipc.rs`) is `uid OR gid`, a compromised `rollout`
  container would have passed both sidecars' authorization check purely on that numeric
  coincidence — despite having zero code today that dials either socket (speculative,
  provisioned wiring for a not-yet-built trajectory-drain feature).
- **SEC-C044-002 (MEDIUM):** `scripts/check-ipc-gid-drift.sh` existed and worked (confirmed
  in the original pass, section 1 above) but was not wired into CI.
- **SEC-C044-003 (MEDIUM):** the "must match the orchestrator container's actual, live
  identity" phrasing added to `SECURITY.md` by the original commit overstated
  exclusivity/uniqueness — the allowlist is a numeric check, not an
  orchestrator-specific one, exactly as SEC-C044-001 demonstrated.
- Two LOW findings, folded into this same pass per the delegation brief's explicit
  direction (not opened as separate tasks): the undocumented `uid=0` allowlist entry, and
  the undocumented fail-open/fail-closed behavior of `IpcAuthzPolicy::from_env()`.

**Scope note.** C044's original file ownership was docs-only. This fix required touching
`deploy/docker-compose.yml` (the `rollout` service's `volumes:` entry only) and
`.gitlab-ci.yml` (one new CI job only) — both explicitly authorized by the orchestrator for
this follow-up, scoped narrowly as described in the dispatch brief. No other service block
or CI job was touched.

**Fix — SEC-C044-001.** Removed the `cwso-runtime:/run/cwso` volume mount from the
`rollout` service block in `deploy/docker-compose.yml`. `rollout` now has no filesystem
path to either `.sock` file. Decided to **remove the mount, not pin a distinct
uid/gid or document as accepted risk** (per explicit direction: nothing currently uses
this access, so removing the unused attack surface is safest; a real future feature can
add scoped access back deliberately). `CWSO_ROLLOUT_SOCKET` was deliberately **left set**
(not removed) — removing it would not change runtime behavior at all, since
`services/cwso-rollout/src/config.rs`'s own built-in default resolves to the identical
path; leaving it explicit, with a comment explaining it is now unreachable, is more honest
to a future reader than silently relying on the same value resolving via a different code
path. Live-confirmed consequence (see verification below): the sidecar's IPC listener
thread (`services/cwso-rollout/src/ipc.rs`, spawned unconditionally in
`services/cwso-rollout/src/main.rs`) fails to `bind()` this now-unreachable path at
startup and logs one non-fatal error; the HTTP proxy (gated separately by
`CWSO_ROLLOUT_PROXY_ENABLED`) is unaffected and the container reports healthy via its own
`/healthz`.

**Fix — SEC-C044-002.** Added `security:ipc-gid-drift` to `.gitlab-ci.yml`'s `audit` stage,
running `scripts/check-ipc-gid-drift.sh`. Because the script needs a live Docker daemon
(`docker build`/`docker run --entrypoint id`), the job extends `.docker-socket` (the
`dind`-tagged runner) with a `docker:27` image — the same infrastructure pattern
`build:orchestrator`/`.docker-base` use — rather than the plain `docker`-tagged
golang/rust images `go:audit`/`rust:audit` use, since this script's needs differ from
theirs. Gated the same way as the other audit-stage jobs: MR pipelines plus `develop`/
`main` branch pipelines.

**Fix — SEC-C044-003 + LOW items.** `SECURITY.md`'s "Sidecar IPC authorization" section
was updated: point 3 corrected to state the allowlist is a numeric check (not
orchestrator-exclusive), with a dedicated correction paragraph naming SEC-C044-003
explicitly; a new point 4 addendum documents the CI wiring; a new point 5 documents the
`uid=0` allowlist entry's purpose (operator root-context debugging access — no process in
the default stack currently connects as uid=0, since every Dockerfile ends in `USER cwso`
before the sidecar's own listener starts); a new point 6 documents
`IpcAuthzPolicy::from_env()`'s actual behavior, read directly from
`services/cwso-git-shadow/src/main.rs` and `services/cwso-merge-engine/src/ipc.rs` (both
identical): **unset → fails closed** (falls back to the process's own
`geteuid()`/`getegid()`, effectively authorizing nobody but itself); **malformed/empty →
fails closed** (`parse_id_csv()` returns `Err`, propagated via `?` in `main()`, so the
sidecar refuses to start rather than run permissively); **non-Linux build target only
(`#[cfg(not(target_os = "linux"))]`) → fails open** (`Ok(true)`, `SO_PEERCRED` is
Linux-only) — flagged explicitly as not a live concern since every shipped Dockerfile
builds on a Linux base image, not silently treated as fine. A renumbered point 8 documents
the SEC-C044-001 finding and fix as a named, resolved risk (not a silently-dropped mount).

**DEBT-REGISTER update.** Added a new row **R-7** (`closed`/`fixed`, `deploy/docker-compose.yml`
rollout service block) for SEC-C044-001, following the same closed/fixed-with-evidence
pattern as prior rows (e.g. R-3/R-5's C022-follow-up shape) — a new row rather than folding
into B12, since this is a distinct issue (reachability via identity coincidence) from B12's
subject (the sockets' own permission bits and GID alignment, both still correctly `0o660`/
aligned and not reopened by this finding). B12's own evidence note gained a one-paragraph
cross-reference to R-7 so a reader of B12 alone is not left thinking this class of finding
was missed entirely.

**Verification (real output, reproduced 2026-08-28).**

1. `docker compose -f deploy/docker-compose.yml --profile rollout config` — resolved
   `rollout` service definition has no `volumes:` key at all (confirmed via a
   `python3 -c "import yaml; ..."` parse of the resolved config, not just a text grep).

2. `docker compose -f deploy/docker-compose.yml --profile rollout up -d --build` (default
   stack + rollout profile together) — all four containers reached a running/healthy
   state (`cwso-orchestrator`, `cwso-rollout` both report `(healthy)`; `cwso-git-shadow`/
   `cwso-merge-engine` have no healthcheck defined, matching their pre-existing config).
   `docker exec cwso-rollout ls -la /run/cwso` shows the directory exists (baked into
   `deploy/Dockerfile.rollout`) but is **empty** — no `.sock` file, confirming no mount and
   no reachable socket at the container level, not just in the compose file text.
   `docker inspect cwso-rollout --format '{{json .Mounts}}'` returns `[]` (no mounts at
   all), while the same inspect on `cwso-git-shadow` still shows the `cwso-runtime` volume
   mounted at `/run/cwso` as before (git-shadow/merge-engine unaffected). `docker logs
   cwso-rollout` shows exactly the predicted one-time non-fatal error
   (`"cwso-rollout IPC server exited","error":"bind cwso-rollout socket
   \"/run/cwso/rollout.sock\""`) immediately followed by the proxy starting normally
   (`"starting rollout proxy"`, `"cwso-rollout proxy listening"`). `docker exec cwso-rollout
   id` / `docker exec cwso-orchestrator id` both report `uid=100(cwso) gid=101(cwso)
   groups=101(cwso)` — live-confirms the identity-coincidence premise of SEC-C044-001 was
   real, not hypothetical. Torn down cleanly after with
   `docker compose -f deploy/docker-compose.yml --profile rollout down -v
   --remove-orphans` (confirmed via `docker ps -a`/`docker volume ls` showing nothing
   `cwso`-related left).

3. `bash scripts/cwso-smoke-test.sh` (default, non-`rollout`-profile stack; run against the
   already-up stack from step 2, before rollout-specific teardown) — all 7 stages
   `[PASS]` (health, create_shadow_workspace, write_shadow_file, query_ast, commit_shadow,
   merge_concurrent_results outcome=success/status=merged, teardown). Confirms removing
   `rollout`'s mount did not affect the default stack. Note: the script's own `docker
   compose ... down -v --remove-orphans` EXIT trap does not target profile-gated services
   it did not start (`rollout` was left running after this step, by design of `docker
   compose down` scoping to active-by-default services) — cleaned up separately in step 2's
   explicit `--profile rollout down`.

4. `glab ci lint` — `✓ CI/CD YAML is valid!`. Additionally cross-checked by loading both
   `.gitlab-ci.yml` and `deploy/docker-compose.yml` with `python3 -c "import yaml; ..."`
   (both parse cleanly).

5. `bash scripts/check-ipc-gid-drift.sh` (run against the live stack from step 2) — exits
   `0`: `Live orchestrator 'cwso' identity: uid=100 gid=101`, all four `OK:` lines for
   `git-shadow`/`merge-engine` × `CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS`. Confirms
   this fix (reachability-only) did not disturb the identity-allowlist alignment T197
   fixed.

**Files changed (this addendum's commit):** `deploy/docker-compose.yml` (rollout service's
`volumes:` entry removed + explanatory comments), `.gitlab-ci.yml` (new
`security:ipc-gid-drift` job), `SECURITY.md` ("Sidecar IPC authorization" section: point 3
correction, new points 5/6/8, point 4 CI-wiring addendum, point 7 renumbered from the
original point 5), `docs/DEBT-REGISTER.md` (new row R-7, B12 evidence cross-reference),
`docs/tasks/task-C044.md` (this addendum).

**Blocker status:** None. The fail-open path found in
`IpcAuthzPolicy::from_env()` (non-Linux `SO_PEERCRED` fallback) was evaluated per the
dispatch brief's explicit instruction not to silently wave off a fail-open finding — it is
called out here and in `SECURITY.md` point 6 as a real fail-open behavior, but is not
routed as a new finding for the orchestrator because it is unreachable in every shipped
deployment target (all `deploy/Dockerfile.*` build on Linux base images); flagged for
revisiting only if a non-Linux build target is ever added to the deployment matrix.
