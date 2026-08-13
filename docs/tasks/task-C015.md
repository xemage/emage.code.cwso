# Task C015 — Mount the user's repository (CWSO_WORKSPACE_HOST)

**ID:** C015
**Owner:** devops-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C010, C019
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C015 row, open question 3); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

The orchestrator mounts `../sample-workspace:/workspace:ro` — a demo, not a product.
Introduce `CWSO_WORKSPACE_HOST` so a developer points CWSO at **their own repository**,
defaulting to `sample-workspace` for the smoke test, and re-evaluate the `:ro` mount:
a shadow-workspace orchestrator that cannot write is a demo.

## Inputs

- `deploy/docker-compose.yml` (orchestrator `volumes:`, line 32)
- `orchestrator/internal/config/` (how `CWSO_WORKSPACE` is consumed)
- `docs/artifacts/sandbox-trustworthiness-v1.md` (C019 — the evidence this read-write default cites)
- Roadmap Approval, decision 3 (2026-08-13): **read-write is approved**, conditional on C019's non-KVM sandbox trustworthiness evidence. If C019 escalated a NO-GO, stop and confirm the default with the orchestrator before shipping read-write.

## Rails (read before starting)

### You MUST
- Change the orchestrator volume to `${CWSO_WORKSPACE_HOST:-../sample-workspace}:/workspace:rw`
- Validate at startup (compose or entrypoint level) that the host path exists and is a directory; fail with a clear message if not
- Document in `docs/user/installation-v3.md`: how to set `CWSO_WORKSPACE_HOST`, that the mount is read-write, and that shadow workspaces (not the mounted repo) are where agent edits land — the mount is the source of truth agents branch *from*
- Cite `docs/artifacts/sandbox-trustworthiness-v1.md` (C019) in the docs section covering the read-write default — the mount's safety story rests on that evidence
- Keep `sample-workspace` as the default so C018's smoke test needs no configuration
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Default the mount to the user's home directory, `/`, or any broad path — only an explicit user-set path or the sample default
- Remove the read-only fallback option: document that a user may append `:ro` deliberately for a read-only deployment
- Change `CWSO_WORKSPACE` (the in-container path) — only the host side is parameterized
- Touch application code beyond what config validation requires (prefer compose-level handling)

## File ownership

- **May create/modify:** `deploy/docker-compose.yml` (orchestrator volumes + env), `docs/user/installation-v3.md` (workspace section), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/*` code (unless config validation strictly requires it — justify in MR), `services/*`, `sample-workspace/*`

## Steps (execute in order)

1. Parameterize the volume mount with the `sample-workspace` default.
2. Add the path-exists validation.
3. Test default (no env): stack starts, `/workspace` contains `hello.txt`.
4. Test custom path: `CWSO_WORKSPACE_HOST=/tmp/some-repo` → mounted read-write.
5. Test missing path: clear failure message.
6. Docs + CHANGELOG.

## Expected outputs

- Parameterized mount with safe default
- Startup validation for the host path
- Docs section on pointing CWSO at a real repo
- CHANGELOG entry

## Acceptance criteria

1. No env set → `sample-workspace` mounted, stack healthy
2. `CWSO_WORKSPACE_HOST` set to a real repo → repo visible and writable at `/workspace`
3. Nonexistent path → clear startup failure, not a silent empty mount
4. Docs state the read-write nature and the `:ro` escape hatch

## Verification commands

```bash
docker compose -f deploy/docker-compose.yml up -d
docker exec cwso-orchestrator ls /workspace        # hello.txt
CWSO_WORKSPACE_HOST=/tmp/cwso-test-repo docker compose -f deploy/docker-compose.yml up -d
docker exec cwso-orchestrator touch /workspace/.cwso-write-test && echo "PASS: writable"
CWSO_WORKSPACE_HOST=/nonexistent docker compose -f deploy/docker-compose.yml up -d 2>&1 | grep -i "workspace"
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/devops-engineer/C015` from `develop` (rebased on merged C010)
- Commit: `feat(deploy): parameterize workspace mount via CWSO_WORKSPACE_HOST`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If read-write mounting conflicts with the `read_only: true` container rootfs or
`cap_drop` posture, do not weaken container hardening — report `technical` / `major`
with the exact mount error.

## Execution notes

<filled during execution>
