# Artifact: user-guide-verification-v1.md

## Metadata
- **Producer agent**: qa-engineer
- **Task**: C054
- **Created**: 2026-08-28
- **Based on**: docs/user/README.md (as of commit `f08faa1`, develop tip after C050–C053)
- **Supersedes**: n/a (first version)

## Scope

Verifies **only** `docs/user/README.md` (the single guide produced by C050). Does **not**
cover `docs/user/deployment/**` (received by C052) — those guides have their own,
separately tracked provenance/validation status in `docs/user/deployment/README.md`, and
are explicitly out of scope for this task per the C054 delegation brief.

## Environment

- Host: Linux sandbox with Docker Engine 29.6.2, Docker Compose v5.3.1, bash 5.2.21,
  curl 8.5.0, python3 3.12.3, git 2.43.0 (all prerequisites from the guide's table present).
- **Clean-environment steps performed before starting** (all confirmed by direct
  inspection, not assumed):
  1. `docker rmi cwso/git-shadow:dev cwso/merge-engine:dev cwso/orchestrator:dev
     cwso/rollout:dev` — removed 4 pre-existing CWSO images left over from a prior
     session in this worktree.
  2. `docker system prune -f` — reclaimed 5.804GB of build cache, stopped containers,
     and orphan networks.
  3. Confirmed no `.env.jwt.dev` existed in the worktree before starting.
  4. Confirmed `docker ps -a --filter name=cwso` returned zero rows before starting.
  5. **Additionally**, rather than only relying on the pre-existing worktree checkout,
     performed a genuine `git clone` of `https://gitlab.com/em-age/emage.code.cwso.git`
     into a scratch directory (`/tmp/.../scratchpad/c054/emage.code.cwso`) — this
     succeeded and landed on the exact same commit (`f08faa1`) as the dispatched
     worktree, confirming the guide's own `git clone` command works and that the content
     under test is identical to what's on `develop`. **All command execution below was
     performed inside this freshly-cloned directory**, not the pre-existing worktree, to
     eliminate any doubt about prior-session contamination.

## Command-by-command log

Every fenced command block in `docs/user/README.md`, executed in document order, exactly
as written (only substituting the guide's own literal placeholder
`/absolute/path/to/your/repo` with a real directory, and the config-file example's literal
name `config.json`, both of which the guide itself instructs the reader to substitute).

### Prerequisites table ("Check" column)

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 1 | `docker --version && docker compose version` | PASS | `Docker version 29.6.2, build dfc4efb` / `Docker Compose version v5.3.1` |
| 2 | `bash --version` | PASS | `GNU bash, version 5.2.21(1)-release (x86_64-pc-linux-gnu)` |
| 3 | `curl --version` | PASS | `curl 8.5.0 (x86_64-pc-linux-gnu) ...` |
| 4 | `python3 --version` | PASS | `Python 3.12.3` |
| 5 | `git --version` | PASS | `git version 2.43.0` |

### Install

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 6 | `git clone https://gitlab.com/em-age/emage.code.cwso.git` | PASS | Cloned successfully; landed on commit `f08faa1edf30f8ae352ab8d7d5ddfbc8977cc8a0`, identical to the dispatched worktree's HEAD |
| 7 | `cd emage.code.cwso` | PASS | Directory contains expected repo tree (`.git`, `AGENTS.md`, `Makefile`, etc.) |
| 8 | `make up` | PASS | Ran all 5 documented steps in order (bootstrap secrets → build images → `docker compose up -d` → health-wait → token-mint). Exit code 0. Output ended with: `CWSO stack is healthy and ready.` followed by the `===== PASTE INTO YOUR MCP CLIENT =====` block with a real, freshly-minted JWT. `curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/healthz` independently confirmed `200`. `.env.jwt.dev` was created by the bootstrap step as documented (mode 600). First run took ~3 minutes (fresh image builds, including a ~2m Rust compile for `git-shadow`), consistent with the guide's "first run takes a few minutes" claim. |

### Configure your MCP client

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 9 | `claude mcp add cwso -- docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator` | PASS | `Added stdio MCP server cwso ... to local config`. `claude mcp list` subsequently showed `cwso: docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator - ✔ Connected`. Claude Code CLI (v2.1.220) was available and authenticated in this sandbox — real execution, not a substitution. |
| 10 | `TOKEN=$(scripts/cwso-token.sh)` + `claude mcp add --transport http cwso http://127.0.0.1:8080/mcp --header "Authorization: Bearer $TOKEN" --header "Origin: http://localhost"` | PASS | `Added HTTP MCP server cwso ...`. `claude mcp list` showed `cwso: http://127.0.0.1:8080/mcp (HTTP) - ✔ Connected`. |
| 11 | `npx @modelcontextprotocol/inspector@1.0.2 --cli docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator --method tools/list` | PASS | Exit 0. Returned a well-formed JSON `tools/list` payload (verified tool schemas present, e.g. `merge_shadow_workspaces`, `read_file_sync`). Pinned version `1.0.2` resolved and installed cleanly via `npx`. |
| 12 | `TOKEN=$(scripts/cwso-token.sh)` + `npx @modelcontextprotocol/inspector@1.0.2 --cli --transport http http://127.0.0.1:8080/mcp --header "Authorization: Bearer $TOKEN" --header "Origin: http://localhost" --method tools/list` | PASS | Exit 0. Returned the same `tools/list` JSON payload over HTTP (441 lines). Inspector printed its own (expected, documented-elsewhere) `v1 is deprecated` notice — informational only, not a failure; consistent with the guide's own rationale for pinning `1.0.2` ("since it's what CWSO's compatibility testing exercised"). |
| 13 | `config.json` (mcpServers JSON for wong2/mcp-cli) | PASS | File created verbatim as shown in the guide; used as-is in the next step. |
| 14 | `npx @wong2/mcp-cli -c config.json call-tool cwso:create_shadow_workspace` | PASS | Exit 0. Orchestrator's stdio startup/shutdown logs printed as expected (including the documented, non-fatal HAL-socket-absent warnings — see Known limitations in the guide), then returned `{"content":[{"type":"text","text":"{\"workspace_uuid\":\"97898b6a-e02a-497c-b7d3-2d692c148536\",\"base_tree_oid\":null}"}]}` — a real shadow workspace was created via this exact client and command. |

### Verify

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 15 | `make doctor` (with stack up) | PASS | `Summary: 7 OK, 2 WARN, 0 FAIL`, exit 0. The 2 WARNs (`/dev/kvm` and `/dev/vhost-net` absent) are expected/documented sandbox conditions ("gVisor-only operation is otherwise fully supported"), each carrying its own one-line fix suggestion as documented. |
| 16 | `make smoke` | PASS | Exit 0. All 7 stages passed in order: `health` → `create_shadow_workspace` → `write_shadow_file` → `query_ast` (found `SmokeGreet` definition) → `commit_shadow` → `merge_concurrent_results` (`outcome=success, status=merged`) → `teardown`. Final banner: `CWSO SMOKE TEST: ALL STAGES PASS`. The stack was torn down afterward via the documented `EXIT` trap (`docker compose down -v --remove-orphans`), ending with `[teardown] OK -- stack stopped, volumes removed, no orphans left`, exactly as the guide describes ("tears down the stack when it finishes ... run `make up` again afterward"). |

### Daily use

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 17 | `CWSO_WORKSPACE_HOST=<real-absolute-path> make up` | PASS | Substituted the guide's placeholder `/absolute/path/to/your/repo` with a real directory containing one file. All 5 steps succeeded; `docker compose logs workspace-check` confirmed: `workspace check OK: 1 entries found in workspace`, proving the mount pointed at the custom directory, not the default `sample-workspace/`. |
| 18 | `scripts/cwso-token.sh --role worker --ttl 900` | PASS | Returned a JWT. Decoded payload confirmed `"role":"worker"` and `exp - iat == 900` seconds exactly, matching the requested TTL. |
| 19 | `docker compose -f deploy/docker-compose.yml --profile rollout up -d` | PASS | Built `cwso/rollout:dev` (fresh Rust compile, ~2 min) and started it; `docker ps` confirmed `cwso-rollout ... Up ... (healthy)` alongside the rest of the stack. |

### Inline-mentioned commands (not fenced, but literal commands the guide tells the reader to use)

| # | Command | Result | Output excerpt |
|---|---|---|---|
| 20 | `make logs` | PASS | `docker compose -f deploy/docker-compose.yml logs -f`, streamed real container log lines (workspace-check, jwt-secret-fix, merge-engine, git-shadow) as expected. |
| 21 | `make down` | PASS | `docker compose -f deploy/docker-compose.yml down`; cleanly stopped and removed all containers and the network, exit 0. |

## Deliberate-failure / Troubleshoot-remedy test

**Trigger:** occupied host port 8080 with an unrelated, non-CWSO process (`python3 -m
http.server 8080 --bind 127.0.0.1`) before running `make up`, simulating the exact
scenario the C054 brief calls out as an example.

**Attempt 1 result:** `make up` failed cleanly and predictably at step 4/5:
```
==> [4/5] Waiting for http://127.0.0.1:8080/healthz (up to 120s)
make up: FAILED at step 4/5 -- http://127.0.0.1:8080/healthz did not return 200 within 120s (last status: 000)
---- last 50 lines of 'docker compose logs' ----
...
        run 'make logs' for the full stream, or 'make down' to stop the stack
make: *** [Makefile:47: up] Error 1
```
Exit code: 2. This matches the guide's own claim verbatim: *"If `make up` fails, it tells
you which of its 5 steps failed and why (bootstrap, build, start, health-wait, or
token-mint) and exits non-zero."*

**Troubleshoot-section validation:** the guide's Troubleshoot section opens with *"Start
with `make doctor` — every `[WARN]`/`[FAIL]` line it prints comes with its own one-line
fix."* Ran `make doctor` while port 8080 was still occupied:
```
[FAIL] Port 8080 is occupied by something other than cwso-orchestrator
       fix: Find and stop the process holding port 8080 (e.g. 'lsof -i :8080'), or 'make down' if a stale stack is still up.
...
Summary: 3 OK, 2 WARN, 1 FAIL
```
Exit code: 2, matching "[`make doctor`] exits non-zero only if a `[FAIL]` was printed."

**Remedy applied (exactly as the guide's own suggestion, and as printed by `make
doctor`'s fix line):** killed the process holding port 8080
(`kill -9 <pid>`, i.e. "stop the process holding the port").

**Recovery verification:**
- Re-ran `make doctor` → `Summary: 4 OK, 2 WARN, 0 FAIL`, exit 0 — port check now reports
  `[OK] Port 8080 is free`.
- Re-ran `make up` → completed all 5 steps successfully, `CWSO stack is healthy and
  ready.`, exit 0.

**Verdict:** the Troubleshoot section's implicit contract (start with `make doctor`,
follow its fix line) is accurate and sufficient to both diagnose and recover from this
failure. PASS.

**Bonus observation (same test session, not a separate deliberate-failure trigger, no
extra scope):** while investigating this scenario, an initial attempt at simulating "port
occupied" used a raw Python socket that called `.listen()` but never `.accept()`ed
connections. Against that specific (atypical) occupier, `bash`'s `/dev/tcp` probe inside
`cwso-doctor.sh`'s `port_in_use()` function hung indefinitely (>15s, would not return)
once its 5-connection backlog queue filled up — `make doctor` itself would not complete
in that case. This is *not* filed as a FAIL against the guide, because: (a) it did not
occur with a normal port occupier (`python3 -m http.server`, or the real `make up`
health-wait `curl` calls, which use `--max-time 3` and behave correctly), and (b) the
guide makes no explicit claim about the port check's behavior against a listener that
accepts TCP connections but never completes `accept()`. Flagging only as an observation
in case the orchestrator/technical-writer wants a follow-up hardening task
(`port_in_use()` in `scripts/cwso-doctor.sh` has no connect timeout) — **not** a defect in
`docs/user/README.md` itself.

**Additional (bonus) negative-path check performed while in this area, directly tied to
a Troubleshoot table row:** re-ran `make up` with `CWSO_WORKSPACE_HOST` pointed at a
nonexistent directory. Result: step 3/5 (`docker compose up -d`) failed as documented,
with the `workspace-check` container printing exactly the guide's referenced symptom:
```
FATAL: CWSO workspace path is missing or empty.
  CWSO_WORKSPACE_HOST (defaults to ../sample-workspace) resolves to an empty directory.
  ...
```
matching the Troubleshoot table row `CWSO workspace path is missing or empty` → cause →
fix, verbatim. PASS.

## Genuine command failures found

**None.** Every command in `docs/user/README.md`, executed exactly as written (with the
guide's own documented placeholder substitutions), passed.

## Environment-limited / not executed

**None.** All three MCP-client tool families named in the guide (Claude Code CLI, MCP
Inspector, `wong2/mcp-cli`) were genuinely installed/authenticated in this sandbox and
were executed for real — no substitution was needed for any command.

## Cleanup performed after testing

- `claude mcp remove cwso` (both times it was added) — confirmed via `claude mcp list`
  that no `cwso` entry remains.
- `docker compose -f deploy/docker-compose.yml --profile rollout down -v
  --remove-orphans` — confirmed via `docker ps -a --filter name=cwso` that zero CWSO
  containers remain running.
- Killed the ad hoc port-8080 occupier processes used for the deliberate-failure test.

## Acceptance criteria verification

| # | Criterion | Status |
|---|---|---|
| 1 | Every command in the guide executed on a clean host, logged | PASS — 21 commands/command-blocks covering all 13 fenced blocks plus the 5 prerequisite checks and the 2 inline-mentioned (`make logs`, `make down`) commands; clean environment confirmed before starting (image/container/cache prune, `.env.jwt.dev` absence, fresh `git clone` to a scratch directory) |
| 2 | Zero unexplained failures (any failure filed, not ignored) | PASS — zero genuine failures found; the one deliberate failure was intentional, expected, and its remedy verified |
| 3 | Troubleshoot section validated via one deliberate failure | PASS — port-8080-occupied scenario, `make doctor`'s FAIL diagnostic + fix line, remedy applied, full recovery confirmed; a second Troubleshoot row (`CWSO workspace path is missing or empty`) was validated as a bonus |
| 4 | Log complete and ready to attach to the MR | PASS — this document |

## VERDICT: PASS

No genuine command failures. The one deliberate failure (port 8080 occupied) behaved
exactly as `docs/user/README.md` documents, and its Troubleshoot-section remedy
(`make doctor`'s fix line: stop the process holding the port) fully resolved it. A second
Troubleshoot row (empty/missing `CWSO_WORKSPACE_HOST`) was validated as a bonus. One
non-blocking observation is noted above (`port_in_use()` lacking a connect timeout against
an atypical never-`accept()`ing occupier) for optional follow-up — it is not a defect in
the guide and does not affect this verdict.

## Consumed by

- C054 (qa-engineer) — this task
- CG4 (gate) — the last gate before v1.0.0's remaining C060–C063 tasks
