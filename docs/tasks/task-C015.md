# Task C015 — Mount the user's repository (CWSO_WORKSPACE_HOST)

**ID:** C015
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C010, C019, T193, T194
**Created:** 2026-08-12
**Completed:** 2026-08-19
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C015 row, open question 3); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Resumed 2026-08-16 — both blockers resolved

While investigating SEC-C019-01 (below), the assigned worker found that
`orchestrator/internal/tools/fs_tools.go`'s `pathGuard()` has a symlink-escape gap
for **new-file writes** (existing-file read/overwrite is correctly protected; new
files written through a symlinked intermediate directory are not). Per this task's
own instruction under SEC-C019-01 ("if your own pathGuard/fs_tools.go review surfaces
a genuine gap... STOP and report it as a blocker... this is a human re-decision
point"), the worker correctly stopped without committing rather than shipping this
task on top of an open path-confinement gap. The human decided: fix the gap first as
a blocking prerequisite.

**Both blockers are now resolved:**
- **T193** (symlink-escape fix): merged, MR !126, independent security-engineer
  review PASS on its own scope.
- **T194** (TOCTOU gap between `pathGuard()`'s check and each caller's later
  filesystem operation — surfaced by T193's own review): merged, MR !127,
  independent security-engineer review CONDITIONAL_PASS with no conditions blocking
  the merge. Fixed via an fd-anchored `openat`/`mkdirat` directory-chain walk on
  Linux (CWSO's only actual deployment target); a portable `!linux` fallback with a
  narrower guarantee is tracked separately as **T195** (P1, not blocking this task).

This task is **resumed**. Its SEC-C019-01 response (below) can now legitimately
point at both closed gaps (T193 + T194) as the primary basis for the read-write
mount's safety story, plus whatever residual scoped risk acceptance still makes
sense for defense-in-depth (e.g., T195's narrower non-Linux guarantee is worth
noting if relevant to any documented deployment path; T194's own MR review is worth
citing directly for how the in-process trust boundary was hardened, not just C019's
container-level P1-P4 evidence).

## Objective

The orchestrator mounts `../sample-workspace:/workspace:ro` — a demo, not a product.
Introduce `CWSO_WORKSPACE_HOST` so a developer points CWSO at **their own repository**,
defaulting to `sample-workspace` for the smoke test, and re-evaluate the `:ro` mount:
a shadow-workspace orchestrator that cannot write is a demo.

## Tracked security condition (SEC-C019-01, security-engineer review of MR !123, 2026-08-16)

**Medium severity, structural — do not close this task without addressing it.**
`docs/artifacts/sandbox-trustworthiness-v1.md` (C019) — the evidence artifact this
task is instructed to cite for its read-write default — only covers **container-level
sandbox tiering** (properties P1-P4: filesystem confinement via the container/volume
boundary, process isolation, resource limits, network policy). It does **not** cover
the `pathGuard`/`fs_tools.go` in-process trust boundary — i.e. the logic that decides
which paths *inside* an already-mounted, already-writable `/workspace` a given
sub-agent/tool call is allowed to touch. For a read-write mount of the user's real
repository, `pathGuard`/`fs_tools.go` is the actual exposure surface that matters day
to day — the container boundary (P1-P4) is necessary but not sufficient.

**Do not let this task cite "P1-P4 MET" as blanket justification for the read-write
default.** Before this task closes, it must do one of:

1. Commission or perform its own scoped review of `pathGuard`/`fs_tools.go` (path
   traversal handling, symlink handling, workspace-boundary enforcement) and document
   the result alongside the C019 citation in `docs/user/installation-v3.md`'s
   workspace section, **or**
2. Explicitly document a scoped risk acceptance if a full review is out of this
   task's budget — state in writing what is and isn't covered, so a reader of the
   installation docs isn't left assuming C019's P1-P4 evidence covers more than it
   does.

If a `pathGuard`/`fs_tools.go` review surfaces a genuine gap (e.g. a traversal or
symlink-escape path out of `/workspace`), treat it with the same seriousness as
C019's own hard-stop rule: do not ship a softened claim — escalate to the orchestrator
as `technical` / `critical` rather than proceeding with the read-write default
unresolved. This is a human re-decision point, same as C019's rail, not something to
quietly work around.

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
- **Satisfy SEC-C019-01 (see the tracked security condition above)**: address the `pathGuard`/`fs_tools.go` trust boundary explicitly (scoped review or documented risk acceptance) — do not cite C019's P1-P4 evidence as if it covers this
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

This task had two false-start recoveries, both handled with the same "treat inherited
work as unverified" discipline established for C019's earlier recovery.

**Recovery 1** (infra session limit killed the resumed worker mid-task, no commit
left): the fresh session independently re-reproduced the Docker silent-empty-directory
bind-mount behavior from scratch (not just cited the inherited claim), independently
re-read `orchestrator/internal/tools/fs_tools.go` to confirm T193/T194's fixes were
genuinely present at all three call sites (rather than trust the salvaged CHANGELOG
text asserting this), live-ran all four brief scenarios against the real stack
(default `sample-workspace` healthy; custom real-repo path genuinely writable;
nonexistent path caught by `workspace-check`'s `FATAL` exit with the orchestrator
never reaching `Up`; manual `:ro` override correctly rejects writes with "Read-only
file system"), and fixed a real section-numbering bug in the salvaged
`installation-v3.md` content (§11 had been skipped).

**Recovery 2** (a genuinely new, content-caused CI failure, not the T196 RUSTSEC
drift): `e2e:phase2` failed with `workspace-check` exiting 1 in the real GitLab
pipeline. Root-caused to this project's Docker-socket-binding CI runners: `docker
compose` runs inside the CI job container but talks to a daemon on a different
filesystem, so any relative bind-mount source (including the parameterized
`CWSO_WORKSPACE_HOST` default) silently resolves to an empty directory there — a
pre-existing, previously-invisible CI limitation (same root cause already solved once
for `.env.jwt.dev`) that `workspace-check` is simply the first thing to correctly
detect and fail loudly on. Confirmed no host-visible path exists in this runner's
config to work around it properly, confirmed `scripts/phase2-integration.py` never
reads/writes through the raw `/workspace` mount (all workspace interaction goes
through `git-shadow`'s isolated shadow-workspace RPCs), and fixed via a CI-only
`deploy/docker-compose.ci.yml` override — verified against the real pipeline (not
just local Docker, which can't reproduce this failure mode), with the real
`deploy/docker-compose.yml` left completely unweakened.

Independent security-engineer review (MR !129) returned **PASS, no conditions**:
every load-bearing claim re-derived live — Docker's bind-mount behavior reproduced
from scratch, all four scenarios re-run against the real stack including the
critical negative case, `fs_tools.go` read directly confirming T193/T194 present at
all three call sites, and SEC-C019-01 substantively verified satisfied (the docs cite
both trust boundaries — C019's container-level P1-P4 and T193/T194's in-process
fixes — separately with mechanism-level detail, not a blanket P1-P4 citation, per the
condition's explicit requirement). One Low, non-blocking finding: **SEC-C015-01** —
`workspace-check`'s non-emptiness signal is coarse (a stray file could pass without
being a real repo); a usability gap, not exploitable, optional follow-up only.

Merged to `develop` 2026-08-19 (squash), MR !129, after picking up `develop` twice
(once for the CI fix's own base, once more for T196's `h2` bump to clear an unrelated
`rust:audit` drift). **This closes the entire C019 → T193 → T194 → C015
security-hardening chain** for the in-process trust boundary that governs this
now-read-write, externally-managed repository mount.
