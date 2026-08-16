# Task C012 — Bootstrap .env.jwt.dev on first run

**ID:** C012
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-12
**Completed:** 2026-08-16
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

Today a developer must hand-create `.env.jwt.dev` before `docker compose up` works
(compose `secrets:` block references it). Generate it automatically on first run when
absent, with a cryptographically random value, so the one-command stack needs zero
manual file creation.

## Release-gating condition (tracked, from C010's CONDITIONAL_PASS review, 2026-08-16)

**This is not backlog — it blocks the v1.0 GA/release cut.** Tech Lead's review of
C010 (MR !113, `deploy/docker-compose.yml` profile-gate removal) confirmed C010's own
diff is correct and complete, but returned CONDITIONAL_PASS on the following tracked
condition, which attaches to this task rather than to C010:

> C012 ("Bootstrap `.env.jwt.dev` on first run") must land and be verified against a
> truly fresh clone before the v1.0 GA/release cut. As of C010, the documented
> `docker compose up -d` quick-start in `README.md` / `docs/user/installation-v3.md`
> fails at the orchestrator container with a JWT-secret config error on any checkout
> lacking a manually-created `.env.jwt.dev`.

Do not close the v1.0 release gate (see `docs/tasks/task-C062.md`, "Release v1.0.0")
without confirming this task is `done` and its acceptance criteria (below) have been
verified on a genuinely fresh clone — not just re-using a developer machine that
already has `.env.jwt.dev` from earlier work.

**Update (2026-08-16, Tech Lead review of MR !115 — CONDITIONAL_PASS):** this task is
now `done` and merged; the script itself is verified correct (secret never printed,
real entropy, mode 600, idempotent, `.gitignore` coverage independently confirmed).
**The condition is NOT closed** — the review found nothing in the documented
quick-start (or anywhere else) actually calls `scripts/cwso-bootstrap-secrets.sh` yet;
that wiring is **C016**'s job (this task's own rails forbid touching the `Makefile`).
**The tracked release-gating condition has moved to `docs/tasks/task-C016.md` §
"Release-gating condition"** and to the `active-tasks.md` footnote ¹, now attached to
C016's row. This section is left in place for history; do not re-attach the condition
here.

## Inputs

- `deploy/docker-compose.yml` (`secrets:` block, lines 3–6)
- `.gitignore` (must already exclude `.env.jwt.dev` — verify)
- `Makefile` (for where the bootstrap hook lives)

## Rails (read before starting)

### You MUST
- Implement bootstrap as `scripts/cwso-bootstrap-secrets.sh`: if `.env.jwt.dev` is absent, generate `JWT_SECRET=<64 hex chars from openssl rand -hex 32>` (or `/dev/urandom` equivalent), write it with `chmod 600`, and print `[OK] generated .env.jwt.dev`; if present, print `[OK] .env.jwt.dev exists` and exit 0
- Verify `.env.jwt.dev` is gitignored **before** generating; if it is not, add it to `.gitignore` first and say so in the MR
- Call the script from the `Makefile` `up` path (C016) and document that `docker compose up` alone still requires it to have run once — OR make the compose `secrets:` file reference tolerant (e.g., document the one-time `make up` as the entry point). Choose the Makefile-hook approach; do not weaken the compose secret mechanism
- Never print the secret value to stdout/logs — only the `[OK]` status lines
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Commit `.env.jwt.dev` or any generated value
- Reuse a hardcoded/example secret as the generated value
- Change how the orchestrator reads the secret (it stays a compose `secrets:` file mount)
- Touch application code

## File ownership

- **May create/modify:** `scripts/cwso-bootstrap-secrets.sh` (new), `.gitignore` (only if `.env.jwt.dev` missing), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `deploy/docker-compose.yml` secret semantics, `orchestrator/*`, `services/*`, `Makefile` (C016 owns the `up` target; coordinate via the brief dependency)

## Steps (execute in order)

1. Check `.gitignore` for `.env.jwt.dev`; add if missing.
2. Write the bootstrap script per the rails; `chmod +x`.
3. Test: remove any existing `.env.jwt.dev`, run the script, confirm file created with mode 600 and a 64-char hex value.
4. Test: run again, confirm idempotent `[OK] exists` and unchanged content.
5. Test: `git status` shows `.env.jwt.dev` as ignored.
6. CHANGELOG entry.

## Expected outputs

- `scripts/cwso-bootstrap-secrets.sh` (executable)
- `.gitignore` verified/updated
- CHANGELOG entry

## Acceptance criteria

1. First run generates `.env.jwt.dev` (mode 600, 64-char hex secret)
2. Second run is idempotent (content unchanged)
3. `git check-ignore .env.jwt.dev` succeeds
4. Secret value never appears in script output

## Verification commands

```bash
rm -f .env.jwt.dev
bash scripts/cwso-bootstrap-secrets.sh
stat -c '%a' .env.jwt.dev          # = 600
grep -c '^JWT_SECRET=[0-9a-f]\{64\}$' .env.jwt.dev   # = 1
bash scripts/cwso-bootstrap-secrets.sh   # idempotent
git check-ignore .env.jwt.dev && echo "PASS: ignored"
```

## Git rails

- Branch: `agent/devops-engineer/C012` from `develop` (rebased on merged C010)
- Commit: `feat(scripts): bootstrap dev JWT secret on first run`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If `openssl` is unavailable on the host matrix, fall back to `head -c 32 /dev/urandom | od -A n -t x1 | tr -d ' \n'` — and note the fallback in the script header.

## Execution notes

Implemented `scripts/cwso-bootstrap-secrets.sh` per rails: generates a 64-hex-char
secret via `openssl rand -hex 32` (documented `/dev/urandom` fallback), writes at mode
600 via umask (no world/group-readable window), idempotent on re-run, never prints the
secret value. `.gitignore` already covered `.env.jwt.dev` via the existing `.env*`
pattern — no `.gitignore` change was needed.

Independent Tech Lead review (MR !115) returned **CONDITIONAL_PASS**: the script
itself passed every check (no leakage to stdout/logs at any code path, real entropy,
correct permissions, idempotency, `.gitignore` coverage independently confirmed via
`git check-ignore -v`, file ownership clean). The condition was not closed by this
task, though — nothing yet calls the script (no `make up` target exists), so a fresh
clone following today's docs still hits the JWT-secret error. The release-gating
condition originated on C010's review and has now moved to **C016** (see this brief's
"Release-gating condition" section above and `docs/tasks/task-C016.md`). Merged to
`develop` 2026-08-16 (squash).
