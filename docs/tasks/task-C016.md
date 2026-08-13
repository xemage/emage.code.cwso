# Task C016 — `make up`: one command to a working stack

**ID:** C016
**Owner:** devops-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C012, C013, C014, C015
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

Create the `make up` target that collapses the 7-step startup into one command:
bootstrap secrets → build → start → wait for health → mint a token → print the
ready-to-paste MCP client config block. This is the front door of v1.0.

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

### You MUST NOT
- Print the JWT secret (the token is fine; the secret is not)
- Background-and-forget: `make up` must not exit 0 until the stack is healthy
- Hardcode the token TTL beyond the C013 default
- Modify the scripts from C012/C013 — call them
- Touch application code

## File ownership

- **May create/modify:** `Makefile` (add `up`, `down`, `logs` targets), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `scripts/*`, `deploy/*`, `orchestrator/*`, `services/*`, docs (C050 owns the guide)

## Steps (execute in order)

1. Read the existing Makefile and the ide-integration doc for the config JSON shape.
2. Implement `up`, `down`, `logs`.
3. Test from a simulated clean state: `rm -f .env.jwt.dev && make down && make up`.
4. Confirm the printed config block is valid JSON and carries a working token.
5. CHANGELOG.

## Expected outputs

- `Makefile` with `up` / `down` / `logs`
- CHANGELOG entry

## Acceptance criteria

1. `make up` from clean state reaches healthy with zero manual steps
2. The printed config block pastes into a client unmodified and works
3. A failed step (e.g., port 8080 occupied) exits non-zero with a clear message
4. `make down` cleanly stops the stack

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

<filled during execution>
