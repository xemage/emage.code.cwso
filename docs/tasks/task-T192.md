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

<filled during execution>
