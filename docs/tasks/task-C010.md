# Task C010 — Remove phase2/phase4 profile gates

**ID:** C010
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C001, C002, C003, C004, C005 (gate CG0)
**Created:** 2026-08-12
**Completed:** 2026-08-16
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B3); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

`deploy/docker-compose.yml` gates `git-shadow` behind `profiles: ["phase2"]` (line 52)
and `merge-engine` behind `profiles: ["phase4"]` (line 71), under a stale comment
("kept disabled until implemented", line 46) — but both services **are** implemented
and CI-built. A bare `docker compose up` therefore starts the orchestrator alone.
Remove the gates so the default path starts the full stack.

## Inputs

- `deploy/docker-compose.yml` (lines 46–83)
- `README.md` quick-start (post-C002: uses `--profile phase2 --profile phase4`)
- `docs/user/installation-v3.md` quick-start (post-C002: same)

## Rails (read before starting)

### You MUST
- Delete the `profiles:` lines from the `git-shadow` and `merge-engine` services
- Delete the stale comment `# Phase 2+ sidecars (kept disabled until implemented)` (line 46)
- Update the README and installation-v3 quick-start blocks to drop the `--profile` flags (keep the two blocks byte-identical, and update the C002 HTML comment to read `<!-- profiles removed in v0.8.0 (C010) -->`)
- Verify with `docker compose config` that all three services render without profiles
- Note the change in `CHANGELOG.md` under a new `## Unreleased` heading

### You MUST NOT
- Change any other compose key (environment, volumes, healthcheck, security_opt, tmpfs, read_only, cap_drop) — the hardening stays exactly as-is
- Add the rollout service (that is C011)
- Touch Dockerfiles, application code, or scripts
- Remove the `secrets:` block or `.env.jwt.dev` reference (C012 owns secret bootstrap)

## File ownership

- **May create/modify:** `deploy/docker-compose.yml`, `README.md` (quick-start block), `docs/user/installation-v3.md` (quick-start block), `CHANGELOG.md` (Unreleased section)
- **Must NOT touch:** `deploy/Dockerfile.*`, `orchestrator/*`, `services/*`, `scripts/*`

## Steps (execute in order)

1. Remove the two `profiles:` lines and the stale comment from the compose file.
2. `docker compose -f deploy/docker-compose.yml config --services` → must list `orchestrator`, `git-shadow`, `merge-engine`.
3. Update both quick-start blocks (drop `--profile phase2 --profile phase4`; keep byte-identical).
4. Add the CHANGELOG `## Unreleased` entry.
5. Run the verification commands.

## Expected outputs

- Compose file with no profile gates on git-shadow/merge-engine
- Byte-identical, profile-free quick-starts in README + installation-v3
- CHANGELOG Unreleased entry

## Acceptance criteria

1. `docker compose config --services` lists all three services with no profile flags
2. `docker compose up -d` (no flags) starts orchestrator + git-shadow + merge-engine; all reach healthy
3. Quick-start blocks remain byte-identical across the two docs
4. `git diff --stat` touches exactly the 4 owned files

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml config --services
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps --format '{{.Name}} {{.Status}}'
curl -sS http://127.0.0.1:8080/healthz
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/devops-engineer/C010` from `develop`
- Commit: `feat(deploy): start git-shadow and merge-engine in default compose profile`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a sidecar fails to become healthy in the default profile, do not paper over it by
re-adding the gate — capture logs and report `technical` / `critical`.

## Execution notes

Implemented and verified with real Docker (Docker Desktop, WSL integration): removed
the two `profiles:` lines + stale comment only; `docker compose config --services`
confirmed all three services with no profile flags; `docker compose up -d` brought up
all three, `orchestrator` reached `(healthy)`, `/healthz` returned `ok`; torn down
cleanly. Discovered during verification (not caused by this change, confirmed
pre-existing): a fresh checkout has no `.env.jwt.dev`, so `orchestrator` fails to
start with a JWT-secret config error until one is created — out of scope per this
brief's rails (`secrets:` block off-limits), reserved for C012. A throwaway,
gitignored, never-committed dev secret was used locally only to unblock verification
and was deleted before committing.

Independent Tech Lead review (MR !113) returned **CONDITIONAL_PASS**: this task's own
diff (compose profile removal, quick-start reconciliation, CHANGELOG entry) confirmed
correct, complete, and scoped to exactly the 4 owned files — no security-relevant
compose keys touched. The tracked condition from that review attaches to **C012**, not
to this task — see `docs/tasks/task-C012.md` § "Release-gating condition" and the
`active-tasks.md` footnote ¹. Merged to `develop` 2026-08-16 (squash).
