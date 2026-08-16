# Task C017 — scripts/cwso-doctor.sh diagnostics

**ID:** C017
**Owner:** devops-engineer
**Status:** in_progress
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C017 row); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

Create `scripts/cwso-doctor.sh`: a pre-flight and post-flight diagnostic that checks
everything the one-command stack depends on and prints `[OK]` / `[WARN]` / `[FAIL]`
per line. It turns "doesn't work" into a specific, actionable answer — including
surfacing the sandbox degraded-mode detection that already exists in `sandbox/router.go`.

## Inputs

- `sandbox/router.go` (existing degraded-mode detection — surface it, don't reimplement)
- `deploy/docker-compose.yml` (ports, sockets, services)
- `scripts/cwso-token.sh` (C013 — for token-validity check)

## Rails (read before starting)

### You MUST
- Check, in this order, printing `[OK]`/`[WARN]`/`[FAIL]` per line:
  1. `docker` and `docker compose` available
  2. Port 8080 free (or owned by cwso-orchestrator)
  3. `/dev/kvm` presence → `[WARN]` (not FAIL) when absent: sandbox runs degraded
  4. vhost-net presence → `[WARN]` when absent
  5. `.env.jwt.dev` exists and is gitignored
  6. Sidecar sockets (`/run/cwso/git-shadow.sock`, `/run/cwso/merge-engine.sock`) when stack is running
  7. `http://127.0.0.1:8080/healthz` returns 200 when stack is running
  8. A freshly minted token is accepted (when stack running)
- Exit 0 if no `[FAIL]`, exit 1 otherwise (`[WARN]` does not fail the run)
- Print a one-line suggested fix after every `[WARN]`/`[FAIL]`
- Add a `doctor` target to the Makefile that calls the script
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Attempt to fix anything — diagnose only
- Require the stack to be running (doctor must work pre-flight on a clean host)
- Print secrets or tokens
- Modify `sandbox/router.go` — read its detection logic and mirror the conclusion

## File ownership

- **May create/modify:** `scripts/cwso-doctor.sh` (new), `Makefile` (add `doctor` target only), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `sandbox/*`, `deploy/*`, `orchestrator/*`, `services/*`, other Makefile targets

## Steps (execute in order)

1. Read `sandbox/router.go` to mirror the degraded-mode conclusion.
2. Implement the script per the rails; `chmod +x`.
3. Test pre-flight (stack down) and post-flight (stack up).
4. Test on a path simulating no-KVM (e.g., temporarily point the check at a nonexistent device) → `[WARN]`, exit 0.
5. Makefile target + CHANGELOG.

## Expected outputs

- `scripts/cwso-doctor.sh` (executable)
- `Makefile` `doctor` target
- CHANGELOG entry

## Acceptance criteria

1. Pre-flight on a clean host produces sensible `[OK]`/`[WARN]` lines and exit 0
2. Missing KVM → `[WARN]` degraded-sandbox line, exit 0
3. A `[FAIL]` condition (e.g., port 8080 squatted by another process) → exit 1 with a suggested fix
4. No secret material in output

## Verification commands

```bash
bash scripts/cwso-doctor.sh; echo "exit=$?"
make up >/dev/null 2>&1 && bash scripts/cwso-doctor.sh; echo "exit=$?"
make doctor
make down
```

## Git rails

- Branch: `agent/devops-engineer/C017` from `develop` (rebased on merged C010)
- Commit: `feat(scripts): add cwso-doctor diagnostics`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
