# Task C013 — scripts/cwso-token.sh replaces the Python heredoc

**ID:** C013
**Owner:** devops-engineer
**Status:** in_progress
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

`docs/user/installation-v3.md` §3 mints a JWT via an inline Python heredoc — fragile,
unversioned, and the documented failure mode of the 7-step startup. Replace it with
`scripts/cwso-token.sh`: one command, prints a token, supports `--role`.

## Inputs

- `docs/user/installation-v3.md` §3 (the heredoc to replace — copy its exact JWT claims: alg HS256, issuer `cwso`, audience `cwso-mcp`)
- `orchestrator/internal/` auth code (confirm the exact claim names the server validates — do not guess)
- `.env.jwt.dev` (secret source; C012)

## Rails (read before starting)

### You MUST
- Implement `scripts/cwso-token.sh` with usage: `cwso-token.sh [--role orchestrator|worker] [--ttl <seconds>]`, defaults `--role orchestrator --ttl 3600`
- Read the secret from `.env.jwt.dev` (fail with a clear message pointing at `scripts/cwso-bootstrap-secrets.sh` if absent)
- Match the server's expected claims exactly (verify against the auth middleware; the compose env says HS256 / iss `cwso` / aud `cwso-mcp`)
- Print ONLY the token on stdout (status messages to stderr) so it composes: `TOKEN=$(scripts/cwso-token.sh)`
- Update `docs/user/installation-v3.md` §3 to call the script instead of the heredoc
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Hardcode a secret or embed one in the script
- Add a new dependency beyond `python3` + stdlib (or `openssl`; match what the heredoc already required)
- Change server-side auth validation
- Print the secret or the full decoded token payload to stdout

## File ownership

- **May create/modify:** `scripts/cwso-token.sh` (new), `docs/user/installation-v3.md` (§3 only), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/*`, `deploy/*`, other scripts

## Steps (execute in order)

1. Read the heredoc in installation-v3.md §3 and the server auth validation to pin the exact claims.
2. Write the script; `chmod +x`.
3. Mint a token; verify the server accepts it (`curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/healthz` or an authenticated endpoint).
4. Test `--role worker` and a bad-secret failure path.
5. Update installation-v3.md §3; CHANGELOG.

## Expected outputs

- `scripts/cwso-token.sh` (executable)
- installation-v3.md §3 simplified to one command
- CHANGELOG entry

## Acceptance criteria

1. `TOKEN=$(scripts/cwso-token.sh)` yields a token the running server accepts
2. `--role worker` produces a token with the worker role claim
3. Missing `.env.jwt.dev` fails with a pointer to the bootstrap script
4. stdout carries only the token

## Verification commands

```bash
bash scripts/cwso-bootstrap-secrets.sh
TOKEN=$(bash scripts/cwso-token.sh)
echo "$TOKEN" | grep -cE '^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$'   # = 1 (JWT shape)
curl -sS -o /dev/null -w '%{http_code}\n' -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/healthz
bash scripts/cwso-token.sh --role worker >/dev/null && echo "PASS: worker role"
```

## Git rails

- Branch: `agent/devops-engineer/C013` from `develop` (rebased on merged C010)
- Commit: `feat(scripts): add cwso-token.sh to replace JWT heredoc`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the server rejects a correctly-shaped token, do not loosen validation — capture the
server's rejection reason and report `technical` / `major`.

## Execution notes

<filled during execution>
