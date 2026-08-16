# Task C014 — Fold enable-all-features into compose defaults

**ID:** C014
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-12
**Completed:** 2026-08-16
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

`scripts/cwso-enable-all-features.sh` must currently be sourced before startup — a
feature that must be enabled for the product to work is not a feature flag. Move every
variable the script sets into the compose file's `environment:` defaults so the manual
source step disappears.

## Inputs

- `scripts/cwso-enable-all-features.sh`
- `scripts/cwso-enable-all-features.env.example`
- `deploy/docker-compose.yml` (post-C010 state)
- `docs/user/installation-v3.md` §2 (the source-the-script step)

## Rails (read before starting)

### You MUST
- Inventory every variable the script exports and map each to the service that consumes it (verify consumption in code/config before moving — do not move a variable nothing reads)
- Add the variables to the appropriate services' `environment:` blocks in `deploy/docker-compose.yml`, using `${VAR:-default}` form only where a user override is genuinely meaningful
- Mark the script **deprecated**: add a header comment "DEPRECATED since v0.8.0 — defaults folded into deploy/docker-compose.yml (C014); kept for reference, will be deleted in a later release" — do not delete it yet
- Update `docs/user/installation-v3.md` §2 to remove the source step
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Move secrets into the compose file — only non-secret feature/config flags; anything secret stays in `.env.jwt.dev`
- Delete the script or the env example (deprecation, not deletion, this phase)
- Change variable *values* while moving them — same behavior, new home
- Touch application code

## File ownership

- **May create/modify:** `deploy/docker-compose.yml` (environment blocks), `scripts/cwso-enable-all-features.sh` (deprecation header only), `docs/user/installation-v3.md` (§2), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/*`, `services/*`, Dockerfiles, other scripts

## Steps (execute in order)

1. List every export in the script; for each, grep the codebase for its consumer.
2. Move consumed variables into compose `environment:` blocks.
3. List any variable with no consumer in the MR description (do not move it; flag for orchestrator).
4. Deprecation header on the script.
5. `docker compose config` renders cleanly; `docker compose up -d` reaches healthy with no manual env.
6. Update installation-v3.md §2; CHANGELOG.

## Expected outputs

- Compose file carrying the feature defaults
- Deprecated script header
- installation-v3.md §2 without the source step
- CHANGELOG entry

## Acceptance criteria

1. Stack starts healthy with **no** manual env sourcing
2. Every moved variable is consumed by a service (evidence: grep reference in MR)
3. Unmoved/unconsumed variables are listed in the MR, not silently dropped
4. `git diff --stat` touches exactly the 4 owned files

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml config | grep -c "CWSO_"   # increased vs before
env -i bash -c 'docker compose -f deploy/docker-compose.yml up -d'     # clean env, no sourcing
docker compose -f deploy/docker-compose.yml ps --format '{{.Name}} {{.Status}}'
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/devops-engineer/C014` from `develop` (rebased on merged C010)
- Commit: `feat(deploy): fold feature flags into compose environment defaults`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a variable's consumer cannot be found, do not guess — leave it, list it, flag
`unclear_requirements` / `minor`.

## Execution notes

Inventoried every export in `scripts/cwso-enable-all-features.sh`/`.env.example`,
verified each consumer by grep before moving: 20 variables onto `orchestrator`
(`CWSO_HAL_SOCKET`, `CWSO_HHD_*` x10, `CWSO_SPARSE_*` x3, `CWSO_AST_SPIKE_*` x2, 4
`CWSO_ROLLOUT_*` flags), 1 (`CWSO_ROLLOUT_HTTP_BIND`) onto `rollout`. Values unchanged
from the original example — relocated only. `CWSO_ROLLOUT_UPSTREAM_URL` was already
present on `rollout` from C011 and left as-is. Deprecation header added to the
original script verbatim per the brief; script and its `.env.example` kept, not
deleted. Caught and correctly fixed a brief discrepancy: the brief said the doc update
was in `docs/user/installation-v3.md` §2, but the actual manual "source the script"
step was in §10 ("Recommended daily workflow") — fixed the real location instead of
guessing or silently doing nothing. Verified live: `docker compose config` var count
increased as expected; `env -i bash -c 'docker compose up -d'` (genuinely clean
environment, nothing sourced) reached healthy.

Independent Tech Lead review (MR !121) returned **CONDITIONAL_PASS**: every moved
variable independently re-traced to a real consumer in `config.go`/`config.rs`
(reviewer's count matched the worker's exactly), no secrets moved into the compose
file, deprecation header verbatim-correct, the §2→§10 self-correction verified
legitimate, file ownership clean, values unchanged. One issue: the `CHANGELOG.md`
prose undercounted the moved variables (said "19 vars... x9", should be "20 vars...
x10" — the MR description's own evidence table already had the correct count).
Reviewer explicitly recommended a trivial fix-up rather than a full re-review; the
orchestrator applied it directly (`80676fc`), confirmed the pipeline stayed green,
and merged. MR !121 merged to `develop` 2026-08-16 (squash) — unblocks **C019**, the
third and final task in the sequential `deploy/docker-compose.yml` chain
(C011 → C014 → C019), dispatched immediately after merge.
