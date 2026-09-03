# Task C011 — Add cwso-rollout behind an opt-in profile

**ID:** C011
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-12
**Completed:** 2026-08-16
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B4); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

`deploy/Dockerfile.rollout` exists and CI builds and publishes the image, but
`deploy/docker-compose.yml` never references it — a service that ships in CI cannot be
started by the documented path. Add `cwso-rollout` to the compose file behind an
**opt-in** `rollout` profile (opt-in is correct: it is genuinely optional, per roadmap §2.4).

## Inputs

- `deploy/docker-compose.yml` (post-C010 state)
- `deploy/Dockerfile.rollout`
- `services/cwso-rollout/` (for port/healthcheck/env conventions; v0.5.1 CHANGELOG notes a `GET /healthz` route and a Dockerfile HEALTHCHECK)

## Rails (read before starting)

### You MUST
- Add a `rollout` service with `profiles: ["rollout"]`, built from `deploy/Dockerfile.rollout`
- Mirror the existing services' hardening posture (`read_only`, `cap_drop: ["ALL"]`, `security_opt: no-new-privileges`, tmpfs where applicable) unless the rollout service demonstrably cannot run with one of them — if so, document the exception in a compose comment
- Wire the orchestrator's rollout-related env vars only if the orchestrator already reads them (check `orchestrator/internal/config/`) — do not invent new config
- Add a CHANGELOG `## Unreleased` entry
- Document the opt-in in the README quick-start area with one line: `docker compose -f deploy/docker-compose.yml --profile rollout up -d`

### You MUST NOT
- Put rollout in the default profile
- Change the rollout service's application code
- Add new secrets or credentials; reuse the existing `jwt_secret` secret if the service needs it
- Touch the orchestrator, git-shadow, or merge-engine service definitions

## File ownership

- **May create/modify:** `deploy/docker-compose.yml` (add rollout service only), `README.md` (one line), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `deploy/Dockerfile.rollout`, `services/cwso-rollout/*`, other compose services

## Steps (execute in order)

1. Read `deploy/Dockerfile.rollout` and the rollout service's config expectations.
2. Add the service definition with the `rollout` profile.
3. `docker compose -f deploy/docker-compose.yml --profile rollout config --services` → includes `rollout`.
4. `docker compose -f deploy/docker-compose.yml config --services` (no profile) → does **not** include `rollout`.
5. Build and start with the profile; confirm the container reaches healthy.
6. CHANGELOG + README line.

## Expected outputs

- Compose file with opt-in `rollout` profile
- README one-liner, CHANGELOG entry

## Acceptance criteria

1. Default `docker compose up` does NOT start rollout
2. `--profile rollout` starts it and it becomes healthy
3. `git diff --stat` touches exactly the 3 owned files

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml config --services | grep -c rollout   # = 0
docker compose -f deploy/docker-compose.yml --profile rollout config --services | grep -c rollout   # = 1
docker compose -f deploy/docker-compose.yml --profile rollout up -d --build
docker compose -f deploy/docker-compose.yml ps --format '{{.Name}} {{.Status}}'
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/devops-engineer/C011` from `develop` (rebased on merged C010)
- Commit: `feat(deploy): add opt-in rollout profile to compose`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the rollout image fails to build or health-check, do not weaken the healthcheck —
capture logs and report `technical` / `major`.

## Execution notes

Implemented per brief: added `rollout` service to `deploy/docker-compose.yml`, built
from `deploy/Dockerfile.rollout`, gated behind opt-in `profiles: ["rollout"]` only.
Verified with real Docker: `docker compose config --services` (no flags) excludes
`rollout`; `--profile rollout config --services` includes it; `--profile rollout up -d
--build` starts it healthy alongside the default services. Mirrored sibling services'
hardening posture with one documented compose-comment exception (no writable Parquet
trajectory-store mount, since that path stays disabled under this profile). Wired only
orchestrator env vars already read by `orchestrator/internal/config/`; reused the
existing `jwt_secret` compose secret, no new credentials. README one-liner + CHANGELOG
entry added.

Independent Tech Lead review (MR !119) returned **PASS, no conditions**: profile
gating, hardening parity with siblings, env-var provenance (checked against actual
source, nothing invented), and file ownership all independently verified.

This branch required three separate `develop`-merge conflict-resolution rounds before
it could land cleanly (`docs/tasks/active-tasks.md`/`CHANGELOG.md` contention from
C012's and C012's-ledger-archival's concurrent edits to the same files) — resolved
each time by concatenating both sides' entries rather than dropping either. Merged to
`develop` 2026-08-16 (squash), unblocking **C014** (dispatched immediately), the
second task in the sequential `deploy/docker-compose.yml` chain
(C011 → C014 → C019).
