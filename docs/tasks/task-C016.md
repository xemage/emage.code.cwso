# Task C016 — `make up`: one command to a working stack

**ID:** C016
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C012, C013, C014, C015, T191 (all satisfied)
**Created:** 2026-08-12
**Completed:** 2026-08-20
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

Create the `make up` target that collapses the 7-step startup into one command:
bootstrap secrets → build → start → wait for health → mint a token → print the
ready-to-paste MCP client config block. This is the front door of v1.0.

## Release-gating condition (tracked, refined from C010/C012's CONDITIONAL_PASS reviews, 2026-08-16)

**This is not backlog — it blocks the v1.0 GA/release cut, same as it did when attached
to C012.** Chain of custody: Tech Lead's review of C010 (MR !113) found the documented
`docker compose up -d` quick-start fails at the orchestrator container
(`.env.jwt.dev` missing) on a fresh clone. C012 (MR !115) built a correct, independently
verified bootstrap script (`scripts/cwso-bootstrap-secrets.sh` — secret never printed,
real entropy, mode 600, idempotent) to fix this, but its own review found **nothing
calls the script yet** — this task, C016, is that caller. The condition therefore
attaches here now, not to C012 (C012 is done and merged on its own correct merits).

Do not close the v1.0 release gate (see `docs/tasks/task-C062.md`, "Release v1.0.0")
until **all three** of the following are true:

1. `make up`'s first step is `scripts/cwso-bootstrap-secrets.sh` (already required by
   this brief's own "You MUST" list below — this condition just makes explicit that
   it is release-gating, not merely a nice-to-have ordering).
2. Independent re-verification, on a genuinely fresh clone (not a dev machine that
   already has `.env.jwt.dev` from earlier work), that the **documented** quick-start
   — whatever the docs say to run at that point — succeeds with zero manual file
   creation.
3. A follow-up update to `README.md` and `docs/user/installation-v3.md`'s quick-start
   sections to reference `make up` (or otherwise make clear the bootstrap step is
   automatic), since neither currently mentions `scripts/cwso-bootstrap-secrets.sh` or
   any one-command target. **This brief's file-ownership rail below has been amended
   (2026-08-16, by the orchestrator) to explicitly permit this narrow quick-start edit**
   — the original rail reserved all docs for C050 ("write the single user guide"), but
   C050 depends on C040–C044 and sits much further downstream than v1.0's release gate
   can safely wait for; C002/C010/C014 already established the precedent of touching
   just the quick-start command blocks (not the whole guide) task-by-task.

**Cross-reference (2026-08-16, discovered incidentally during C019's audit, MR !123
§6; RESOLVED 2026-08-19):** even with `make up` correctly calling
`scripts/cwso-bootstrap-secrets.sh` first (item 1 above), acceptance criterion #1
("`make up` from clean state reaches healthy with zero manual steps") would have
**still failed on a genuinely fresh clone**, for an unrelated reason: the bootstrap
script's `chmod 600` on `.env.jwt.dev` left the file unreadable by the
`orchestrator` container's non-root `cwso` user (different UID than the host user
that ran the bootstrap script), because the compose `secrets:` block was a plain
bind mount, not a Swarm-managed secret. Tracked and **fixed as T191** (MR !132,
merged 2026-08-19, CONDITIONAL_PASS after one Tech Lead re-review round — a first
fix attempt introduced its own regression, live-caught by review, before the final
parent-dir-mount + named-volume-staging design closed it cleanly). **This is now a
satisfied prerequisite, not an open risk** — C016 does not need to rediscover or
work around this bug; `deploy/docker-compose.yml`'s current state already has the
fix in place (a `jwt-secret-fix` pre-flight service stages the secret into a named
Docker volume the orchestrator reads, with no host bind-mount of the secret file
anywhere). C016 should still include `T191` in its own verification pass (the
existing acceptance criterion #1 already covers this — no brief change needed there
beyond this note), but should not expect to find or need to fix this class of issue
itself.

## Inputs

- `Makefile` (existing targets, e.g. `build`)
- `scripts/cwso-bootstrap-secrets.sh` (C012)
- `scripts/cwso-token.sh` (C013)
- `deploy/docker-compose.yml` (post-C010/C014/C015 state)
- The MCP client config shape the orchestrator expects (check `docs/user/ide-integration-v2.md` for the current JSON shape)

## Rails (read before starting)

### You MUST
- Implement `make up` to run, in order: bootstrap secrets → `docker compose build` → `docker compose up -d` → poll `http://127.0.0.1:8080/healthz` until 200 or a 120s timeout → mint a token via `scripts/cwso-token.sh` → print the MCP client config block with the token embedded
- Print the config block between clear markers, e.g. `===== PASTE INTO YOUR MCP CLIENT =====` … `===== END =====`
- Add `make down` (compose down) and `make logs` (compose logs -f) for symmetry
- Fail fast with a non-zero exit and a human-readable message if any step fails (no half-started silent states)
- Add a CHANGELOG `## Unreleased` entry
- **(Added 2026-08-16, orchestrator amendment — closes this task's tracked
  release-gating condition, item 3):** update the quick-start code block in
  `README.md` and `docs/user/installation-v3.md` to reference `make up` (dropping the
  raw `docker compose ... up -d` invocation), keeping the two blocks byte-identical to
  each other per the C002 convention already established for that pair of files

### You MUST NOT
- Print the JWT secret (the token is fine; the secret is not)
- Background-and-forget: `make up` must not exit 0 until the stack is healthy
- Hardcode the token TTL beyond the C013 default
- Modify the scripts from C012/C013 — call them
- Touch application code
- Rewrite or restructure the guides beyond the quick-start command block itself — that
  remains C050's job ("write the single user guide")

## File ownership

- **May create/modify:** `Makefile` (add `up`, `down`, `logs` targets), `CHANGELOG.md`
  (Unreleased), `README.md` (quick-start block only — amended 2026-08-16, see
  "Release-gating condition" above), `docs/user/installation-v3.md` (quick-start block
  only — same amendment)
- **Must NOT touch:** `scripts/*`, `deploy/*`, `orchestrator/*`, `services/*`, any part
  of the docs beyond the quick-start command blocks (C050 owns the rest of the guide)

## Steps (execute in order)

1. Read the existing Makefile and the ide-integration doc for the config JSON shape.
2. Implement `up`, `down`, `logs`.
3. Test from a simulated clean state: `rm -f .env.jwt.dev && make down && make up`.
4. Confirm the printed config block is valid JSON and carries a working token.
5. Update the quick-start blocks in `README.md` and `docs/user/installation-v3.md` to
   call `make up`; confirm the two blocks are still byte-identical (`diff`).
6. CHANGELOG.

## Expected outputs

- `Makefile` with `up` / `down` / `logs`
- `README.md` / `docs/user/installation-v3.md` quick-start blocks updated and
  byte-identical
- CHANGELOG entry

## Acceptance criteria

1. `make up` from clean state reaches healthy with zero manual steps
2. The printed config block pastes into a client unmodified and works
3. A failed step (e.g., port 8080 occupied) exits non-zero with a clear message
4. `make down` cleanly stops the stack
5. **(Release-gating condition item 3):** `README.md` and
   `docs/user/installation-v3.md` quick-starts reference `make up`, are byte-identical
   to each other, and following them on a genuinely fresh clone requires zero manual
   file creation

## Verification commands

```bash
rm -f .env.jwt.dev
make down 2>/dev/null; make up
curl -sS http://127.0.0.1:8080/healthz
make up | sed -n '/PASTE INTO YOUR MCP CLIENT/,/END/p' | grep -c '"'
make down
```

## Git rails

- Branch: `agent/devops-engineer/C016` from `develop` (rebased on merged C012–C015)
- Commit: `feat(make): add one-command up/down/logs targets`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the health endpoint never returns 200 within timeout on a healthy-looking stack,
capture `make logs` output and report `technical` / `major` — do not just raise the timeout.

## Execution notes

Implemented `make up`/`down`/`logs` calling C012's `scripts/cwso-bootstrap-secrets.sh`
and C013's `scripts/cwso-token.sh` as black boxes (neither modified), with a bounded
120s `/healthz` poll and fail-fast, non-zero-exit behavior on any step failure. Did
not need to rediscover or route around the T191 (`.env.jwt.dev` permission) or C015
(`workspace-check` empty-mount) bug classes — both fixes held cleanly under this
task's own live testing, exactly as expected. Updated `README.md`/
`docs/user/installation-v3.md`'s quick-start blocks to call `make up`, confirmed
byte-identical to each other with `diff`.

Independent Tech Lead review (MR !135) returned **PASS, no conditions**: every
functional claim independently live-reproduced rather than trusted — clean-state
`make up` reached a healthy, token-authenticated stack in ~7.7s with zero manual
steps; the printed config block's token was verified against a real live `POST /mcp`
`tools/list` call (200, real 15-tool list); a secret-leak grep across the *entire*
transcript (not just the final printed block) found zero occurrences of the raw
secret or `JWT_SECRET=`; `make down` confirmed clean teardown; the failure path
(pre-occupied port 8080) failed in 6.4s at step 3, before the health poll even
started. Reviewer also explicitly confirmed the Tech-Lead-only review routing was
correct (zero diff in any security-sensitive path — `scripts/*`, `deploy/*`,
`orchestrator/*`, `services/*` all untouched; pure orchestration glue over
already-independently-reviewed components).

Merged to `develop` 2026-08-20 (squash), MR !135. **Closes the C010 → C012 → C016
(+ T191) release-gating condition, tracked since 2026-08-16** — see the
`active-tasks.md` footnote ¹ for the full chain history. `docs/tasks/task-C062.md`
("Release v1.0.0") can now cite a working one-command install path without this
being an open risk.

<filled during execution>
