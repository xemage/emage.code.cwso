# Task T191 — Fix `.env.jwt.dev` permission mismatch (chmod 600 vs non-root container user)

**ID:** T191
**Owner:** devops-engineer
**Status:** pending
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-16
**Completed:** —
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

<filled during execution>
