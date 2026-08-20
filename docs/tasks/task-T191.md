# Task T191 — Fix `.env.jwt.dev` permission mismatch (chmod 600 vs non-root container user)

**ID:** T191
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-16
**Completed:** 2026-08-19
**Based on:** Discovered incidentally during C019 (sandbox trustworthiness audit), MR !123 §6 — not part of C019's scope, logged separately per the orchestrator's defect-disposition call.

## Objective

`scripts/cwso-bootstrap-secrets.sh` (C012) writes `.env.jwt.dev` with `chmod 600`,
restricting read access to the file's owning host user. `deploy/docker-compose.yml`'s
`secrets:` block bind-mounts `../.env.jwt.dev` directly into
`/run/secrets/jwt_secret` (this is a plain bind mount, not a Swarm-managed secret), so
the file's **host** permissions are exactly what the container sees. The
`orchestrator` container runs as a non-root, different-UID `cwso` user
(`deploy/Dockerfile.orchestrator:17-18,21`), so it cannot read a mode-600 file owned
by the host user that ran the bootstrap script. `orchestrator/internal/config/config.go:127-131,242-244`
correctly fails closed (refuses to start without a readable secret) rather than
degrading silently — that part is working as intended — but the practical effect is
that **the documented one-command path does not actually reach a healthy stack on a
genuinely fresh clone** without a manual `chmod` workaround.

**This directly threatens C016's acceptance criterion #1** ("`make up` from clean
state reaches healthy with zero manual steps") — C016 has not yet been dispatched,
but when it is, this defect will reproduce identically for the same root-cause reason
unless resolved first or as part of C016. See `docs/tasks/task-C016.md` §
"Release-gating condition" for the cross-reference.

## Evidence

Captured live during C019's MR !123 verification (`docs/artifacts/sandbox-trustworthiness-v1.md`
§6, reverted immediately after capture — not committed):

```
$ chmod 644 .env.jwt.dev   # local-only workaround, reverted after
$ docker compose -f deploy/docker-compose.yml up -d --build orchestrator git-shadow merge-engine
 Container cwso-git-shadow Started
 Container cwso-merge-engine Started
 Container cwso-orchestrator Started
$ curl -sf http://localhost:8080/healthz
ok
$ docker compose -f deploy/docker-compose.yml down -v
$ chmod 600 .env.jwt.dev   # workaround reverted
```

With `.env.jwt.dev` left at its bootstrap-script-default `chmod 600`, the
`cwso-orchestrator` container fails to start (JWT secret unreadable, config
fail-closed per `config.go:127-131`).

## Inputs

- `scripts/cwso-bootstrap-secrets.sh` (C012 — sets the `chmod 600` mode)
- `deploy/docker-compose.yml` `secrets:` block (bind-mount semantics, lines 3–6)
- `deploy/Dockerfile.orchestrator:17-18,21` (non-root `cwso` user, different UID than
  the host user running the bootstrap script)
- `orchestrator/internal/config/config.go:127-131,242-244` (fail-closed secret read —
  correct behavior, do not weaken)

## Rails (read before starting)

### You MUST
- Fix the permission mismatch so a freshly-bootstrapped `.env.jwt.dev` is readable by
  the `orchestrator` container's actual runtime user, without making the secret file
  world-readable on the host
- Preserve `config.go`'s fail-closed behavior — do not make secret-read failures
  silent or permissive
- Preserve the "never print the secret value" invariant established by C012/C013
- Choose and justify an approach (candidates to evaluate, not a prescription): mount
  the secret via Docker's actual Swarm-secret mechanism instead of a raw bind mount;
  have the bootstrap script `chmod` to a mode the container's UID/GID can read (e.g.
  group-readable with a shared group, or a mode that matches the container's declared
  UID); or have the orchestrator's entrypoint fix ownership/mode at container-start
  time before dropping privileges. Do not just chmod 644 as a blanket fix without
  considering the host-side exposure that creates for other local users
- Add a CHANGELOG `## Unreleased` entry
- Re-verify with a genuinely fresh `.env.jwt.dev` (delete, re-bootstrap, do not
  manually chmod) that the stack reaches healthy with zero manual steps

### You MUST NOT
- Weaken `config.go`'s fail-closed secret validation
- Make `.env.jwt.dev` world-readable as the fix (that trades one problem for a worse
  one)
- Touch application logic beyond what's needed to resolve the permission mismatch

## File ownership

- **May create/modify:** `scripts/cwso-bootstrap-secrets.sh`, `deploy/docker-compose.yml`
  (`secrets:` block / relevant service `user:`/mount config only), `deploy/Dockerfile.orchestrator`
  (only if the fix requires an entrypoint-level permission fix), `CHANGELOG.md`
  (Unreleased)
- **Must NOT touch:** `orchestrator/internal/config/config.go`'s fail-closed
  validation logic itself (read-only, to confirm behavior, not modify), MCP tool
  surface, sandbox tiering (`sandbox/**`, C019's territory)

## Acceptance criteria

1. `rm -f .env.jwt.dev && bash scripts/cwso-bootstrap-secrets.sh && docker compose -f deploy/docker-compose.yml up -d` reaches a healthy `orchestrator` with **zero** manual permission changes
2. `config.go`'s fail-closed behavior on a genuinely unreadable/missing secret is unchanged (still refuses to start, does not degrade silently)
3. The secret file is not made world-readable on the host as part of the fix
4. `git diff --stat` touches only the files listed under "File ownership"

## Verification commands

```bash
rm -f .env.jwt.dev
bash scripts/cwso-bootstrap-secrets.sh
stat -c '%a %U' .env.jwt.dev
docker compose -f deploy/docker-compose.yml up -d --build orchestrator git-shadow merge-engine
curl -sf http://127.0.0.1:8080/healthz && echo "PASS: healthy with zero manual chmod"
docker compose -f deploy/docker-compose.yml down -v
```

## Git rails

- Branch: `agent/devops-engineer/T191` from `develop`
- Commit: `fix(deploy): resolve .env.jwt.dev permission mismatch for non-root orchestrator user`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If no approach can satisfy both "container-readable" and "not world-readable on
host" simultaneously given the current bind-mount architecture, escalate as
`technical` / `major` with the constraint you hit — this may require revisiting the
bind-mount-vs-Swarm-secret architecture decision, which is bigger than this task's
scope alone.

## Execution notes

**First commit**: added a `jwt-secret-fix` pre-flight helper service, gated into
orchestrator's `depends_on` (same pattern as C015's `workspace-check`). It looked up
the orchestrator image's live `cwso` uid/gid (not hardcoded) and `chown`ed the host
`.env.jwt.dev` to match, mode staying `600`. All three brief-listed candidate
approaches (Swarm secrets, host chgrp/ACLs, root-then-drop-privileges) were tested
live and rejected for verified reasons, documented in the MR.

**Independent Tech Lead review FAILED this first commit**: live-reproduced that the
helper's bind mount of the secret file caused Docker to silently auto-vivify an
empty, root-owned directory at `.env.jwt.dev`'s host path whenever the file was
genuinely absent (before any in-container check could run) — corrupting the host
filesystem on a fresh clone and breaking `scripts/cwso-bootstrap-secrets.sh`'s
recovery path (`Is a directory` hard failure). This was a real regression versus
pre-task behavior for that exact scenario.

**Second commit** found a deeper root cause than the first review flagged: `orchestrator`'s
own top-level Compose `secrets: file:` stanza *also* independently bind-mounted the
same host path, and Compose materializes every service's mounts up front for a
single `up` invocation (not gated by `depends_on` start-order) — so fixing only
`jwt-secret-fix`'s mount would not have stopped `orchestrator`'s own mount from
reproducing the identical bug. Fix: stop bind-mounting the host path into any
container. `jwt-secret-fix` now mounts the always-existing parent directory, checks
for the named file inside it, and (if present) copies — never mutates in place — the
secret into a new named Docker volume (`cwso-jwt-secret`) at `/run/secrets/jwt_secret`,
owned by the looked-up `cwso` uid/gid, mode 600. `orchestrator` mounts that same
named volume read-only at the identical in-container path `config.go` already
expects (`config.go` itself untouched throughout). Named volumes have no host source
path, closing the auto-vivification bug class for both services at once, not just
half of it. Also detects a stray leftover directory (from a pre-fix run) and refuses
to treat it as the secret, with best-effort cleanup.

Independent Tech Lead re-review returned **CONDITIONAL_PASS**: every claim
independently live-reproduced on the actual host filesystem — the exact check the
first review's own value came from, deliberately repeated rather than trusted this
time. Missing-secret scenario: `.env.jwt.dev` genuinely stays absent (not a
directory), `config.go`'s original fail-closed error unchanged, clean bootstrap
recovery afterward. Happy path: host file mode/owner/content unchanged, and — going
further than the fix's own claim — byte-identical content confirmed between the host
file and the staged volume copy. Stray-directory path: a manually planted root-owned
directory correctly refused, not corrupted further. New `DAC_READ_SEARCH` capability
grant (needed because the helper now reads file content to copy it, not just mutate
metadata) confirmed minimal — no `DAC_OVERRIDE`. `config.go` diff-confirmed
untouched. Two small process conditions, both resolved before merge: the MR
description's `git diff --stat` claim was stale (said 2 files, actually 4 including
the two orchestrator-owned ledger files) — corrected directly; and confirming the
correct MR-gating pipeline (not a redundant duplicate that had separately hit an
unrelated runner infra flake) actually finished green — it did, after one transient
runner hang (`check:version-drift` produced zero trace output for 20+ minutes,
resolved cleanly on retry, consistent with infra flakiness rather than content).

Incidentally found and independently re-verified during this task (not fixed here,
logged separately as **T197**, P2): `CWSO_IPC_ALLOWED_GIDS` is hardcoded to a gid
that doesn't match the orchestrator's actual live gid. Confirmed **latent, not an
active access-control gap** — the parallel `CWSO_IPC_ALLOWED_UIDS` allowlist already
matches the orchestrator's real uid, and the allowlist check is `uid OR gid`.

Merged to `develop` 2026-08-19 (squash), MR !132 — unblocks **C016**.

<filled during execution>
