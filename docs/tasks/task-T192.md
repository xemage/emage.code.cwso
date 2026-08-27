# Task T192 — Fix JWT 401 mismatch between orchestrator and `phase2-integration.py`

**ID:** T192
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-16
**Completed:** —
**Based on:** Discovered incidentally during C019 (sandbox trustworthiness audit), MR !123 §6.3 — not part of C019's scope, logged separately per the orchestrator's defect-disposition call.

## Objective

`scripts/phase2-integration.py` (the `make smoke-local` entry point, and the closest
equivalent to the task-brief-referenced but nonexistent `scripts/cwso-smoke-test.sh` —
see note below) fails its first authenticated MCP call (`tools/list`) with
`401 invalid token`, even against a healthy stack with a correctly-bootstrapped
`.env.jwt.dev`. Root cause not yet chased down: both the orchestrator
(`orchestrator/internal/config/config.go:127-128`) and the test client
(`scripts/phase2-integration.py:64-77`) read `.env.jwt.dev`'s `KEY=VALUE` line and
pass it through as a single opaque secret string without parsing — the two *should*
end up with an identical value, but evidently do not. Confirmed **not** a regression
from any recent change: reproduces identically with and without C019's
`deploy/docker-compose.yml` diff applied (isolated via `git stash`).

## Evidence

Captured live during C019's MR !123 verification (both runs against the same
freshly-bootstrapped `.env.jwt.dev`):

```
$ git stash push -- deploy/docker-compose.yml   # isolate: is C019's diff the cause?
$ make smoke-local
...
--- waiting for orchestrator /healthz ---
  OK  /healthz reachable
--- waiting for git-shadow socket ---
  OK  /run/cwso/git-shadow.sock present
--- 1. tools/list shows shadow tools ---
  unexpected response: {'_http_status': 401, '_body': 'invalid token\n'}
--- tearing down ---
make: *** [Makefile:55: smoke-local] Error 1
$ git stash pop   # restore C019's diff — identical failure, confirms no regression
```

## Inputs

- `scripts/phase2-integration.py:64-77` (test client's secret-loading path)
- `orchestrator/internal/config/config.go:127-128` (server's secret-loading path)
- `.env.jwt.dev` (the shared secret source both are supposed to read identically)
- `scripts/cwso-token.sh` (C013 — a sibling secret-consuming script; per C013's own
  Tech Lead review, MR !116, this script's minted tokens WERE independently verified
  accepted by a running orchestrator container — so whatever the mismatch is, it is
  specific to `phase2-integration.py`'s own secret-loading path, not universal to
  every `.env.jwt.dev` consumer. Worth diffing `phase2-integration.py`'s parsing
  against `cwso-token.sh`'s working approach as a starting point.)

## Note on the referenced verification script

This project's `AGENTS.md`/task-brief convention references `scripts/cwso-smoke-test.sh`
as the canonical smoke-test entry point (see C019's and C062's briefs), but **this
script does not exist in the repository** (`find . -iname "*smoke*"` returns
nothing by that name). `make smoke-local` (→ `scripts/phase2-integration.py`) is the
closest actual equivalent. Resolving this naming/existence gap is out of this task's
scope, but is worth flagging to whichever task next touches brief conventions or
release verification tooling (candidates: C016, C018, C062) — recorded here so it
isn't silently rediscovered.

## Rails (read before starting)

### You MUST
- Root-cause the actual value mismatch between `config.go`'s and
  `phase2-integration.py`'s parsing of `.env.jwt.dev` (e.g. trailing newline
  handling, encoding, whitespace, quoting, multi-line file assumptions — compare both
  parsers line-by-line against the same real file)
- Fix whichever side is wrong so both derive an identical signing/verification value
  from the same `.env.jwt.dev` content
- Re-verify with `make smoke-local` reaching a passing `tools/list` call (not just
  reaching `/healthz`)
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Loosen JWT verification on the server side to work around the mismatch (the fix is
  making the two sides agree, not disabling the check)
- Change `scripts/cwso-token.sh` (C013) unless you find it's also affected — it was
  independently verified working in MR !116's review; if you touch it, re-verify
  against that same acceptance bar

## File ownership

- **May create/modify:** `scripts/phase2-integration.py`, `orchestrator/internal/config/config.go`
  (only the secret-parsing path, not broader config logic), `CHANGELOG.md`
  (Unreleased)
- **Must NOT touch:** `sandbox/**`, `deploy/docker-compose.yml`, MCP tool surface

## Acceptance criteria

1. `make smoke-local` passes the `tools/list` authenticated call (no `401`)
2. The fix is in the parsing/loading logic, not a loosening of server-side validation
3. `scripts/cwso-token.sh` (C013) still works if touched at all (re-verify per its own acceptance bar)
4. `git diff --stat` touches only the files listed under "File ownership"

## Verification commands

```bash
rm -f .env.jwt.dev
bash scripts/cwso-bootstrap-secrets.sh
make smoke-local   # must reach "PHASE 2 INTEGRATION TEST: PASS" or equivalent, not 401 on tools/list
```

## Git rails

- Branch: `agent/backend-developer/T192` from `develop`
- Commit: `fix(scripts): resolve JWT signing/verification mismatch in phase2-integration.py`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the root cause turns out to be in a shared dependency or a more structural secret-
handling issue than a simple parsing bug, report `technical` / `major` with findings
rather than shipping a narrow patch that doesn't address the real cause.

## Execution notes

### Root cause (confirmed via live A/B reproduction, not guessed)

Both `config.go` (via the docker-compose-staged `/run/secrets/jwt_secret`
copy of `.env.jwt.dev`) and `scripts/cwso-token.sh` (which reads
`.env.jwt.dev` directly, unconditionally) already agreed on the same
opaque secret value — the entire trimmed file content, including the
literal `JWT_SECRET=` key prefix, since neither actually parses `KEY=VALUE`.
That part of the system was **not** the bug, contrary to the brief's initial
hypothesis about `config.go:127-128` vs `phase2-integration.py:64-77` line
parsing.

The actual bug was in `resolve_jwt_secret()` (phase2-integration.py, a few
lines above the cited range): it checked `os.environ.get("CWSO_JWT_SECRET")`
*first*, in every mode (CI or not), before ever consulting
`.env.jwt.dev` via `load_local_jwt_secret()`. This is correct for CI (where
`deploy/docker-compose.ci.yml` threads that exact env var straight into the
orchestrator container's environment, per its own header comment and
`.gitlab-ci.yml`'s `CWSO_JWT_SECRET: "ci-ephemeral-secret-not-used-in-prod-ci-only"`
job variable) — but wrong for local/non-CI runs, where the orchestrator
(`deploy/docker-compose.yml`, no CI overlay) *never* reads a host
`CWSO_JWT_SECRET` env var at all; it only reads `/run/secrets/jwt_secret`,
staged verbatim from `.env.jwt.dev` by the `jwt-secret-fix` service (T191).

This repo's own dev environment happens to export
`CWSO_JWT_SECRET=ci-ephemeral-secret-not-used-in-prod-ci-only` in
`~/.bashrc` (mirroring the CI variable for local parity), which is present
in every shell session here — so the bug reproduced 100% deterministically,
exactly matching the brief's evidence log, independent of any C019 diff.

Confirmed by live A/B test, no code changes yet:
```
$ echo $CWSO_JWT_SECRET
ci-ephemeral-secret-not-used-in-prod-ci-only
$ make smoke-local
...
--- 1. tools/list shows shadow tools ---
  unexpected response: {'_http_status': 401, '_body': 'invalid token\n'}
make: *** [Makefile:128: smoke-local] Error 1

$ env -u CWSO_JWT_SECRET make smoke-local     # same code, just unset the stray env var
...
--- 1. tools/list shows shadow tools ---
  OK  shadow tools registered
...
  PHASE 2 INTEGRATION TEST: PASS
```
This isolated the root cause to `resolve_jwt_secret()`'s env-var-vs-file
precedence before any patch was written.

### What was changed and why

- **`scripts/phase2-integration.py`** (`resolve_jwt_secret()`): precedence
  is now mode-dependent instead of unconditional. In CI
  (`os.environ.get("CI")` truthy — matches `resolve_compose_files()`'s own
  existing CI-overlay gating), a configured `CWSO_JWT_SECRET` still wins
  first (matches `docker-compose.ci.yml`'s env passthrough). Outside CI,
  `.env.jwt.dev` (via the existing, unmodified `load_local_jwt_secret()`)
  is now tried *before* any pre-set `CWSO_JWT_SECRET`, matching what the
  local orchestrator container actually derives its secret from. A stray
  env var is only used as a last-resort fallback (non-CI, no local file),
  and random generation remains the final fallback in both modes. No
  change to `load_local_jwt_secret()` itself — its current "return the
  whole trimmed line, including the `KEY=` prefix" behavior for a
  single-line file is exactly what keeps it in agreement with
  `config.go`'s and `cwso-token.sh`'s equally-unparsed handling of the same
  file; "fixing" it to strip the `KEY=` prefix would have reintroduced a
  new mismatch against `config.go`, which was intentionally left untouched
  (see below).
- **`orchestrator/internal/config/config.go`**: **not modified.** Audited
  and confirmed its secret-parsing path (lines ~124-132) is not the wrong
  side — it already agrees with `scripts/cwso-token.sh`'s independently
  working approach (both treat the whole trimmed file/secret content as an
  opaque string, prefix included). Changing it to properly parse
  `KEY=VALUE` would have been a real improvement in isolation, but would
  have silently broken `scripts/cwso-token.sh` (C013, MR !116) as
  collateral damage, which the brief explicitly guards against. Left as-is
  per the rails: fix the side that's actually wrong.
- **`CHANGELOG.md`**: added an `### Fixed (T192)` entry under `## Unreleased`
  (inserted above the existing `### Fixed (T198)` entry, newest-first per
  this file's existing convention).

### Verification (real, not assumed)

All three runs below were executed in this worktree with Docker/Compose
available (`docker compose` v5.3.1); full transcripts captured during this
session:

1. **Before the fix**, with the dev environment's real (pre-existing)
   `CWSO_JWT_SECRET` still exported: `make smoke-local` reproduced the
   exact reported 401 on `tools/list` (see A/B test above) — confirms the
   bug is real and live, not hypothetical.
2. **After the fix**, same shell, same stray `CWSO_JWT_SECRET` still
   exported (not unset — this is the realistic condition the fix has to
   survive): `make smoke-local` ran the full Phase 2 suite (workspace
   creation/isolation, Go/Python AST queries, commit, permission-gate
   check, teardown) and printed `PHASE 2 INTEGRATION TEST: PASS`.
3. **CI-mode regression check**: `CI=1 make smoke-local` with a real local
   `.env.jwt.dev` present initially produced a 401 — traced to a *test
   artifact*, not a fix regression: `jwt-secret-fix` stages
   `/run/secrets/jwt_secret` from any locally-present `.env.jwt.dev`
   regardless of the CI overlay (a real CI runner has no such file), so the
   orchestrator's file-first precedence in `config.go` picked the file
   secret while my CI-mode Python precedence (deliberately) picked the CI
   env var — a divergence that can only happen when both a local secret
   file and CI-style env var coexist, which is not how the actual GitLab CI
   runner is provisioned (no host `.env.jwt.dev`, per `.gitlab-ci.yml`).
   Confirmed by removing (moving aside, not deleting) `.env.jwt.dev` to
   faithfully simulate an ephemeral CI checkout, then re-running
   `CI=1 make smoke-local`: full suite passed, `PHASE 2 INTEGRATION TEST:
   PASS`. `.env.jwt.dev` was restored immediately after.
4. **`scripts/cwso-token.sh` (C013) re-verified per its own acceptance
   bar**: started the stack directly via `docker compose up`, minted a
   token with `bash scripts/cwso-token.sh --role worker --ttl 300`, and
   POSTed a `tools/list` RPC to the running orchestrator's `/mcp` endpoint
   with that token — got `HTTP 200` (not 401), confirming the script is
   unaffected (it was not modified).

### Acceptance criteria

1. `make smoke-local` passes `tools/list` (no 401) — **met**, verified live
   (see #2 above), with the realistic stray-env-var condition present.
2. Fix is in the loading/parsing logic, not a loosening of server-side
   validation — **met**; `orchestrator/internal/transport/http.go`'s
   `verifyJWT()`/`authMiddleware()` were not touched.
3. `scripts/cwso-token.sh` still works — **met**, re-verified (see #4
   above); the file itself was not modified.
4. `git diff --stat` against `origin/develop` touches only files under
   "File ownership" plus this brief — **met**:
   `scripts/phase2-integration.py`, `CHANGELOG.md`,
   `docs/tasks/task-T192.md`. `orchestrator/internal/config/config.go` was
   intentionally left unchanged (see above).

### Blocker status

None.
