# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Added (C018)
- **`test`**: Added `scripts/cwso-smoke-test.sh` -- the v1.0
  definition-of-done executable. From an already-running stack (`make up`),
  drives the full real flow over the MCP HTTP transport with a token minted
  fresh via `scripts/cwso-token.sh` (C013): `health` -> `create_shadow_workspace`
  -> `write_shadow_file` -> `query_ast` -> `commit_shadow` ->
  `merge_concurrent_results` -> `teardown` (`drop_shadow_workspace`), with a
  clear `[PASS]`/`[FAIL]` line per stage. Exits non-zero on the first failing
  stage and prints the failing response body -- no stage is mocked and no
  assertion is weakened to route around a real failure. An `EXIT` trap always
  tears the docker compose stack down (`down -v --remove-orphans`) on both
  the success and failure paths, so the host is never left with a stray
  container or volume. Added a `smoke` Makefile target (`make smoke`, depends
  on `up`) and a `e2e:smoke` CI job (stage `e2e`) that builds+starts the
  stack under CI's docker-socket-binding runner and runs the same script.
  Live-verified: a full pass against a healthy stack (`docker ps`/`docker
  volume ls` clean afterward), and a deliberately broken stage (git-shadow
  sidecar stopped mid-run) producing a non-zero exit with the failing
  response body printed and an equally clean teardown.
  Note for follow-up: `schemas/create_shadow_workspace.json` and
  `schemas/query_ast.json` have drifted from the real, currently-implemented
  MCP tool contract (verified against `orchestrator/internal/tools/shadow_tools.go`);
  this script follows the real live contract, not the stale schema files --
  worth a documentation-sync pass separate from this task.

### Added (C016)
- **`feat(make)`**: Added `make up` -- the one-command front door for the
  stack, collapsing the previously-manual 7-step startup (bootstrap secrets,
  build, start, wait for health, mint a token, hand-assemble an MCP client
  config) into a single invocation. Runs, in order and failing fast with a
  non-zero exit and a human-readable message at the first failing step (no
  half-started, silently-partial state): (1) `scripts/cwso-bootstrap-secrets.sh`
  (C012, called as-is, not reimplemented); (2) `docker compose build`; (3)
  `docker compose up -d`; (4) polls `http://127.0.0.1:8080/healthz` every 2s
  until it returns 200 or a 120s deadline is hit, on timeout printing the last
  50 lines of `docker compose logs` to stderr before exiting 1; (5) mints an
  MCP token via `scripts/cwso-token.sh` (C013, called as-is, at that script's
  own default role/TTL -- not hardcoded here). On success, prints a
  ready-to-paste MCP client config block (the `.vscode/mcp.json` /
  `.cursor/mcp.json` shape from `docs/user/ide-integration-v2.md`, token
  embedded directly rather than via `${env:CWSO_MCP_TOKEN}`) between
  `===== PASTE INTO YOUR MCP CLIENT =====` / `===== END =====` markers. Only
  the minted token is ever printed -- the underlying JWT signing secret
  (`.env.jwt.dev` / the staged `cwso-jwt-secret` volume, T191) is never
  written to stdout/stderr by this target, matching the same rule
  `scripts/cwso-bootstrap-secrets.sh` and `scripts/cwso-token.sh` already
  enforce.
  Also added `make down` (alias for the existing `make stop`, i.e. `docker
  compose down`) for naming symmetry with `make up`; `make logs` (`docker
  compose logs -f`) already existed and needed no change.
  This closes the release-gating condition tracked on this task since C010's
  review (chain of custody: C010's documented `docker compose up -d`
  quick-start failed at the orchestrator on a fresh clone missing
  `.env.jwt.dev` -> C012 built the bootstrap script but nothing called it yet
  -> this task is that caller). Verified live on a genuinely clean state
  (`rm -f .env.jwt.dev && make down && make up`): reaches a healthy
  orchestrator with zero manual steps, the printed config block is valid
  JSON, and the embedded token is accepted by an authenticated `tools/list`
  call against `/mcp`; `make down` cleanly stops the stack; a forced port
  8080 conflict makes `make up` fail fast at step 3/5 with a clear message
  and non-zero exit rather than hanging or partially starting.
- **`docs`**: Updated the shared quick-start command block (kept
  byte-identical between `README.md` and `docs/user/installation-v3.md` per
  the existing C002 convention) to call `make up` instead of the raw `make
  build` + `docker compose ... up -d` + `curl .../healthz` sequence it
  replaces; the `python3 scripts/phase2-integration.py` smoke-test line is
  unchanged.

### Deployment (T191 follow-up)
- **`fix(deploy)`**: Fixed a regression in the original T191 `jwt-secret-fix`
  helper found by Tech Lead review (MR !132) via live reproduction: the
  helper bind-mounted the secret *file* directly
  (`../.env.jwt.dev:/secret/jwt_secret`), and the `orchestrator` service
  separately consumed it via a top-level Compose `secrets: file:` stanza --
  both are implemented as host bind mounts outside Swarm mode (verified live
  against this stack's target Docker/Compose versions), and Docker Engine
  silently auto-creates an empty directory at a missing bind-mount *source*
  path before any in-container check can run. So a genuinely fresh clone
  (`.env.jwt.dev` absent, bootstrap not yet run) that ran `docker compose up`
  corrupted the host filesystem -- left a root-owned directory where a file
  belongs -- and broke `scripts/cwso-bootstrap-secrets.sh`'s recovery path
  (`... .env.jwt.dev: Is a directory`, hard failure). This affected *both*
  services independently: `depends_on: condition:
  service_completed_successfully` only gates *start* order, but Compose
  creates every service's containers (and materializes their mounts) up
  front for a single `up` invocation, so fixing only `jwt-secret-fix`'s own
  mount did not stop `orchestrator`'s separate `secrets:` mount from
  reproducing the identical bug.
  Fix: stop bind-mounting the host `.env.jwt.dev` path into any container.
  `jwt-secret-fix` now mounts the *parent* directory (the repo root, which
  always exists) read-write and tests for the named file inside that
  already-existing mount before touching anything -- a missing file inside
  an existing directory mount cannot trigger the Engine's auto-vivify
  behavior. If present, it copies (never moves -- the host original is left
  untouched) the secret into a new `cwso-jwt-secret` named Docker volume at
  `/run/secrets/jwt_secret`, owned by the looked-up `cwso` uid/gid, mode
  600. `orchestrator` now mounts that same named volume read-only at the
  identical in-container path `config.go` already expects (`config.go`
  itself is unmodified). Named volumes are managed entirely inside Docker's
  own storage, not sourced from a host path, so there is no
  missing-source-path for either service's mount to auto-vivify against --
  this removes the bug class entirely rather than only half of it.
  `jwt-secret-fix`'s capability set grew by one: `DAC_READ_SEARCH` (in
  addition to the existing `CHOWN`/`FOWNER`), because the helper now needs
  to *read* the host secret's content (to copy it) rather than only mutate
  its metadata in place; `CAP_DAC_OVERRIDE` (a much broader bypass of all
  read/write DAC checks) was deliberately not added -- the helper detects a
  stray leftover directory from a pre-fix run (or a host that never gets
  down to a clean state) and best-effort `rmdir`s it if empty, falling back
  to a clear one-line manual-cleanup instruction instead of a raw
  permission-denied error when that isn't possible without the broader
  capability.
  Verified live (see MR !132 discussion for full transcripts): (1) fresh
  clone, `.env.jwt.dev` absent, `docker compose up` without bootstrap --
  `jwt-secret-fix` no-ops cleanly, `orchestrator` fails closed with
  `config.go`'s original error, and `.env.jwt.dev` remains genuinely absent
  on the host afterward (not auto-vivified as a directory); bootstrap then
  runs and succeeds with zero manual intervention. (2) happy path (bootstrap
  first, then `up`) still reaches a healthy orchestrator (`/healthz` -> `ok`)
  with the staged secret at `mode 600 uid=100 gid=101` in the named volume,
  and the host `.env.jwt.dev` is left completely untouched (`mode 600`,
  original host owner, unchanged) throughout. (3) a directory manually
  planted at `.env.jwt.dev` (root-owned, mode 0755 -- matching what the
  Engine's own auto-vivify produces) is detected, refused, and either
  removed automatically or reported with an actionable manual-fix message,
  never chowned/treated as the secret.

### Deployment (T191)
- **`fix(deploy)`**: Resolved the `.env.jwt.dev` -> `/run/secrets/jwt_secret`
  permission mismatch that made the documented one-command `docker compose
  up` path fail to reach a healthy `orchestrator` on a genuinely fresh
  clone (discovered during C019's MR !123 verification; blocks C016's
  "zero manual steps" acceptance criterion). `scripts/cwso-bootstrap-secrets.sh`
  (C012) writes `.env.jwt.dev` `chmod 600`, owned by whatever host user ran
  it; the orchestrator's `secrets:` block bind-mounts that file as-is
  (Compose's `uid`/`gid`/`mode` secret attributes are silently ignored
  outside Swarm mode), and the orchestrator runs as the non-root `cwso`
  user (different uid) with `cap_drop: ["ALL"]` (C019), so it could not
  read the file without a manual `chmod` workaround.
  Added a new `jwt-secret-fix` service to `deploy/docker-compose.yml`: a
  throwaway, tightly-scoped container (`cap_drop: ["ALL"]` +
  `cap_add: ["CHOWN", "FOWNER"]` only, `network_mode: none`,
  `security_opt: no-new-privileges`) that runs before the orchestrator
  (`depends_on: condition: service_completed_successfully`, same pattern
  as C015's `workspace-check`) and `chown`s the host `.env.jwt.dev` to the
  orchestrator image's own `cwso` uid/gid (looked up live from the image's
  `/etc/passwd`, not hardcoded) so the orchestrator can read it via its
  *existing* owner-read permission bits. The file's mode stays `600`
  (never made group- or world-readable); only its owner changes, from the
  host user to the container's `cwso` uid/gid. Idempotent, and a no-op
  (exit 0) if `.env.jwt.dev` does not exist yet -- `config.go`'s existing
  fail-closed check remains the single source of truth for a missing/
  unreadable secret, and is unmodified. Verified live: fresh
  `rm -f .env.jwt.dev && bash scripts/cwso-bootstrap-secrets.sh && docker
  compose -f deploy/docker-compose.yml up -d --build orchestrator
  git-shadow merge-engine` reaches a healthy orchestrator
  (`/healthz` -> `ok`) with zero manual chmod; the secret file remains
  mode 600 (not world/group readable) throughout; and a genuinely missing
  `.env.jwt.dev` still fails closed with `config.go`'s original error
  (`JWT secret must be set via /run/secrets/jwt_secret or CWSO_JWT_SECRET
  when transport=http`), unchanged.
  Rejected alternatives (see MR description for full rationale): Swarm
  managed secrets (Compose's `uid`/`gid`/`mode` secret attributes are
  ignored for plain `docker compose up`, and switching to `docker stack
  deploy` is out of scope); host-side `chgrp`/`setfacl` from the
  unprivileged bootstrap script (verified live: `chgrp` fails with
  "Operation not permitted" for a non-member group, and `setfacl` is not
  installed/portable across dev hosts); an orchestrator-entrypoint
  root-then-drop-privileges fix (verified live: with `cap_drop: ["ALL"]`
  already in place from C019, root inside the container cannot bypass DAC
  checks either, since `CAP_DAC_OVERRIDE` is dropped -- weakening
  `cap_drop` was out of scope); running the orchestrator itself as the
  host's own uid (breaks the fixed-uid IPC peer-credential allowlist
  `CWSO_IPC_ALLOWED_UIDS`/`GIDS` that git-shadow/merge-engine already
  enforce against the orchestrator's `cwso` uid -- sandbox-tiering
  territory, out of scope).

### Security (T195)
- **`feat(tools)`**: Added `orchestrator/internal/tools/fs_tools_portable.go`
  (`//go:build !linux`), a portable counterpart to T194's Linux-only
  `fs_tools.go`. Fast-follow from T194's MR (!127) per the reviewing
  security engineer's platform-trade-off recommendation. Before this
  change, `fs_tools.go` being gated `//go:build linux` for its ENTIRE
  contents meant `internal/server/server.go` (which references
  `tools.ReadFileSync`/`tools.WriteFileSync`/`tools.ListDir` unconditionally,
  with no build tag) failed to compile the whole `orchestrator` module on
  any non-Linux `GOOS` — confirmed via `GOOS=darwin go build ./...` failing
  before this change and succeeding after — and, more importantly, the
  entire T193+T194 regression test suite silently vanished from
  `go test ./...` on non-Linux machines with zero visible warning. This
  file restores both: same exported surface (`ReadFileSync`,
  `WriteFileSync`, `ListDir`, same `Workspace string` struct shape, same
  tool names `read_file_sync`/`write_file_sync`/`list_dir`), so
  `server.go` compiles unmodified against either build tag. T193's
  symlink-resolution logic (`pathGuard`, `resolveNearestExistingAncestor`,
  `withinWorkspace`) is reused verbatim (it has no `syscall.Openat`/
  `Mkdirat` dependency and is fully portable), so the T193 fix is not
  regressed on non-Linux.
  **Explicit, deliberately NARROWER guarantee — read before relying on
  this build for anything security-sensitive:** this build does NOT have
  T194's kernel-enforced, `openat(2)`/`O_NOFOLLOW`-anchored atomicity
  available (Go's standard library only exposes `Openat`/`Mkdirat` for
  `GOOS=linux`). Instead it re-verifies path containment/symlink-safety a
  second time via `reverifyBeforeUse` (a second `pathGuard` call)
  immediately before each `os.Open`/`os.WriteFile`/`os.MkdirAll`/
  `os.ReadDir` call, minimizing — NOT eliminating — the check-then-use
  window. A symlink swap landing exactly inside that shortened window is
  still theoretically possible on this build. This is not just a
  theoretical caveat: a concurrent-race regression test
  (`TestPortableWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace`,
  ported from T194's Linux race test) was observed to actually reproduce
  an out-of-workspace write both with and without `-race` — independent
  security review reproduced it at a materially higher rate under `-race`
  (47% across 15 runs, vs. 7% across 15 runs without `-race` — the race
  detector's scheduler perturbation widens the window but does not create
  it) — concrete, empirical confirmation that the window is real,
  not hypothetical, even though CWSO's actual and only deployment target
  (`deploy/Dockerfile.orchestrator`, Alpine Linux) never builds or ships
  this file — `.gitlab-ci.yml`'s `go test ./... -race` job runs on Linux
  only, so it never compiles or exercises this build either. New
  `fs_tools_portable_test.go` ports the T193/T194 symlink-at-component and
  symlink-at-leaf rejection cases (plus the race stress test) to close the
  "test suite silently disappears on non-Linux" gap for `go test
  ./internal/tools/...`. See the package doc comment atop
  `fs_tools_portable.go` for the full narrower-guarantee writeup.

### Deployment (C015)
- **`feat(deploy)`**: Introduced `CWSO_WORKSPACE_HOST` so operators can point
  the orchestrator at their own repository instead of the bundled
  `sample-workspace/` demo, and changed the orchestrator's workspace mount
  from read-only to **read-write**
  (`${CWSO_WORKSPACE_HOST:-../sample-workspace}:/workspace:rw` in
  `deploy/docker-compose.yml`). `CWSO_WORKSPACE_HOST` is host-side only —
  the in-container path (`CWSO_WORKSPACE=/workspace`) is unchanged.
  `sample-workspace` remains the default when the variable is unset, so
  C018's smoke test needs no configuration. A read-only deployment is still
  supported by editing the mount suffix back to `:ro` (documented as an
  explicit escape hatch, not removed).
- **`feat(deploy)`**: Added a `workspace-check` pre-flight service to
  `deploy/docker-compose.yml` that verifies the resolved workspace path is a
  real, non-empty directory before the orchestrator container starts
  (`depends_on: workspace-check: condition: service_completed_successfully`).
  Docker Engine does not reject a bind mount whose host source is missing —
  it silently creates an empty, root-owned directory instead (verified live;
  also affects Compose's own `bind: create_host_path: false`, tracked
  upstream as ineffective for local `up`:
  docker/compose#13602) — so a successful mount alone does not prove the
  host path existed. `workspace-check` checks non-emptiness instead (true
  for any real repository or the `sample-workspace` default, never true for
  an auto-created empty directory), giving a clear `FATAL:` failure and a
  non-zero `docker compose up` exit instead of a silent empty mount.
- **`docs`**: Added `docs/user/installation-v3.md` §11 "Point CWSO at your
  own repository", documenting `CWSO_WORKSPACE_HOST`, the read-write default
  and what it does/doesn't mean (shadow workspaces, not the mount, are where
  agent edits land), host file-permission requirements for the non-root
  container user, the `:ro` escape hatch, and the startup validation above.
  Addresses the tracked security condition **SEC-C019-01** (security-engineer
  review of MR !123, 2026-08-16): the read-write default's safety story now
  cites both `docs/artifacts/sandbox-trustworthiness-v1.md` (C019 —
  container-level sandbox tiering, P1-P4) *and* the in-process
  `pathGuard`/`fs_tools.go` trust boundary closed by **T193** (symlink-escape
  fix, MR !126) and **T194** (TOCTOU fix, MR !127), rather than citing C019's
  P1-P4 evidence alone as if it covered the in-process boundary too. This
  task's own independent review of the current
  `orchestrator/internal/tools/fs_tools.go` (post-T193/T194, confirmed merged
  via `git log` on this branch) found both fixes correctly in place on every
  read/write/list call site and no further gap — verified by reading the
  code directly and by running `fs_tools_test.go`'s full suite (15 tests,
  including a live TOCTOU symlink-swap race exercise) with `go test
  ./internal/tools/... -race` in a `golang:1.25-alpine` container matching
  `deploy/Dockerfile.orchestrator`'s build stage; all pass, race detector
  clean. `orchestrator/*` code itself was not modified by this task.

### Dependencies (T196)
- **`chore(deps)`**: Bumped the transitive `h2` crate from `0.4.14` to `0.4.16`
  in `services/Cargo.lock` (lockfile-only; no `Cargo.toml` changes) to clear
  [RUSTSEC-2026-0258](https://rustsec.org/advisories/RUSTSEC-2026-0258)
  ("h2 unbounded empty DATA frames" — low-severity denial-of-service:
  h2 accepted and queued empty DATA frames without limit, risking unbounded
  memory growth or a panic-on-overflow if a stream wasn't actively drained;
  patched upstream in `h2` v0.4.16). `h2` is pulled in transitively via
  `hyper 1.10.1` / `hyper-util 0.1.20`, used by `services/cwso-rollout`;
  both are declared as semver-range constraints (`hyper = "1"`,
  `hyper-util = "0.1"`) in `services/cwso-rollout/Cargo.toml`, so
  `cargo update -p h2` alone resolved a patched version without widening
  those ranges. Verified with `cargo audit` (0 vulnerabilities; the 2
  pre-existing unmaintained-crate warnings for `fxhash`/`paste` are
  unaffected and out of scope) and the full Rust workspace test suite
  (`cwso-git-shadow`, `cwso-merge-engine`, `cwso-hal`, `cwso-sparse`,
  `cwso-rollout` — 146 tests passing, 0 regressions).

### Security (T194)
- **`fix(tools)`**: Closed the check-then-use (TOCTOU) window between
  `pathGuard()`'s symlink-safety check (fixed in T193) and the actual
  filesystem operation each caller performs afterward, as a separate step.
  `pathGuard()` only proves a path is safe **at the moment of the check**;
  `ReadFileSync.Execute`, `WriteFileSync.Execute`, and `ListDir.Execute`
  each then called `os.Stat`/`os.ReadFile`/`os.MkdirAll`/`os.WriteFile`/
  `os.ReadDir` by path string as a second, independently-timed step — if a
  symlink were swapped into place along that path in the gap, the OS's
  live symlink resolution at execution time (not the resolution
  `pathGuard()` saw a moment earlier) would decide the outcome. Not
  exploitable via CWSO's current tool surface (nothing can create a
  symlink at runtime today), but C015 (paused pending this fix) removes
  that precondition by mounting the user's real, externally-writable
  repository into the workspace. Fixed by adding `secureResolveDirs`/
  `secureOpenLeaf` to `orchestrator/internal/tools/fs_tools.go`: instead of
  a second path-string lookup, all three call sites now walk
  `pathGuard()`'s already-resolved, canonical path one component at a time
  via `openat(2)`, each hop anchored to the file descriptor obtained by
  the PREVIOUS hop and opened with `O_NOFOLLOW` (the final leaf hop too).
  A symlink swapped into any component after `pathGuard()`'s check is
  refused by the kernel at that exact hop (`ELOOP`/`ENOTDIR`) — there is no
  later, separately-timed name resolution left for a race to win against.
  This is the **strong (`*at()`-anchored) fix from all three call sites**
  (`ReadFileSync`, `WriteFileSync`, `ListDir`); no call site uses the
  weaker "re-verify then race a smaller window" fallback. Implemented with
  Go's standard `syscall` package only (no new dependency); scoped with
  `//go:build linux` since `syscall.Openat`/`Mkdirat` are Linux-only in Go's
  standard library and CWSO's sole deployment target
  (`deploy/Dockerfile.orchestrator`) is Linux (Alpine). **Known trade-off,
  flagged for reviewer sign-off in MR !<TBD>**: this constraint makes
  `GOOS=darwin`/`GOOS=windows` builds of the whole `orchestrator` module
  fail (confirmed via local cross-compile check; previously succeeded) —
  `internal/server/server.go` references the affected tool types and sits
  outside this task's file-ownership boundary, so it was not touched, but
  reviewers should be aware native (non-Docker) macOS/Windows dev builds
  are affected until a follow-up splits the Linux-only implementation
  behind a build tag with a portable fallback. T193's symlink-resolution
  fix and its regression tests are unmodified and still pass. New tests in
  `fs_tools_test.go` prove the new `*at()`-anchored code path is reachable
  and correctly rejects a pre-existing symlink at both the intermediate-
  directory and leaf levels, plus a best-effort (non-flaky-by-design)
  concurrent symlink-swap stress test as supporting evidence — see the MR
  description for the full written reasoning proof of why the window is
  closed, not just narrowed.

### Security (T193)
- **`fix(tools)`**: Closed a symlink-based workspace-escape gap in
  `orchestrator/internal/tools/fs_tools.go`'s `pathGuard()` for **new-file**
  writes. `pathGuard()` already resolved symlinks (via `filepath.EvalSymlinks`)
  and rejected escapes for paths that **exist** on disk, but silently fell
  through to `return clean, nil` — the unresolved, unchecked path — when the
  leaf didn't exist yet, because `EvalSymlinks` errors on a non-existent leaf.
  `write_file_sync` (the only write-capable tool) then wrote through that
  unresolved path via `os.MkdirAll`/`os.WriteFile`, so a new file targeted
  through a symlinked intermediate directory pointing outside the workspace
  root was actually written outside the workspace. `pathGuard()` now walks the
  target path upward (`filepath.Dir`) to find the nearest **existing**
  ancestor, resolves symlinks on that ancestor, verifies it's inside the
  workspace root, then rejoins the non-existent tail components and
  re-verifies the joined path is still inside the workspace before trusting
  it. Existing-file read/overwrite behavior (already correct) is unchanged.
  Discovered by the C015 worker while implementing the read-write workspace
  mount (blocked pending this fix); independently re-verified by the
  orchestrator before dispatch. New regression tests cover: a new file behind
  a workspace-external symlinked directory (rejected, and the filesystem is
  checked directly to confirm nothing was written outside), a deeply-nested
  variant (multiple non-existent path segments), and two positive cases (a
  symlink that stays inside the workspace, and a plain multi-level new-file
  write with no symlinks) to confirm the fix doesn't break legitimate writes.

### Deployment (C014)
- **`feat(deploy)`**: Folded every consumed variable from
  `scripts/cwso-enable-all-features.sh` into `deploy/docker-compose.yml`'s
  `environment:` blocks (20 vars onto `orchestrator`: `CWSO_HAL_SOCKET`,
  `CWSO_HHD_*` x10, `CWSO_SPARSE_*` x3, `CWSO_AST_SPIKE_*` x2, `CWSO_ROLLOUT_API_ENABLED`,
  `CWSO_ROLLOUT_REWARD_ENABLED`, `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED`,
  `CWSO_ROLLOUT_SOCKET`; 1 var — `CWSO_ROLLOUT_HTTP_BIND` — onto `rollout`), each
  verified against its consuming code path in `orchestrator/internal/config/config.go`
  or `services/cwso-rollout/src/config.rs` before moving. Boolean flags use
  `${VAR:-default}` so operators can still override; socket paths stay literal,
  matching the existing `CWSO_GIT_SHADOW_SOCKET`/`CWSO_MERGE_ENGINE_SOCKET` pattern
  since they're tied to the fixed `cwso-runtime` volume mount, not meaningfully
  user-overridable. Values are unchanged from
  `scripts/cwso-enable-all-features.env.example` — same behavior, new home. The
  script itself is marked deprecated (kept for reference, not deleted) and no
  longer needs to be sourced; `docs/user/installation-v3.md` §10's "Recommended
  daily workflow" no longer includes the `source` step. One variable from the
  script's env example — `CWSO_ROLLOUT_UPSTREAM_URL` — was already present on the
  `rollout` service from C011 and is unchanged here.

### Scripts (C013)
- **`feat(scripts)`**: Added `scripts/cwso-token.sh`, replacing the inline Python
  heredoc in `docs/user/installation-v3.md` §3 for minting the dev MCP JWT.
  Usage: `cwso-token.sh [--role orchestrator|worker] [--ttl <seconds>]` (defaults
  `--role orchestrator --ttl 3600`). Reads the signing secret from `.env.jwt.dev`
  in the repo root and fails with a pointer to `scripts/cwso-bootstrap-secrets.sh`
  if that file is missing. Prints only the signed token on stdout (all
  diagnostics go to stderr) so `TOKEN=$(scripts/cwso-token.sh)` composes cleanly.
  Claims (`alg` HS256, `iss` `cwso`, `aud` `cwso-mcp`, `role`) verified against
  `orchestrator/internal/transport/http.go` `verifyJWT()`/`authMiddleware()` and
  against a running `deploy/docker-compose.yml` orchestrator container: both
  `--role orchestrator` and `--role worker` tokens are accepted (200) on the
  auth-gated `/mcp` endpoint, while missing/garbage bearer tokens are rejected
  (401).

### Deployment (C017)
- **`feat(scripts)`**: Added `scripts/cwso-doctor.sh`, a pre-flight/post-flight
  diagnostic for the one-command stack. Checks, in order: `docker`/`docker compose`
  availability, port 8080 free-or-owned-by-`cwso-orchestrator`, `/dev/kvm` and
  vhost-net presence (mirroring the degraded-mode conclusion already computed by
  `orchestrator/internal/sandbox/router.go`'s `resolveFirecracker()` —
  `DEGRADED_FALLBACK_GVISOR` — without reimplementing it), `.env.jwt.dev`
  existence + gitignore status, sidecar UDS sockets, `/healthz`, and (when
  `scripts/cwso-token.sh` from C013 is present) freshly-minted-token acceptance
  against `/mcp`. Prints `[OK]`/`[WARN]`/`[FAIL]` per line with a one-line
  suggested fix after every `[WARN]`/`[FAIL]`; exits `0` unless a `[FAIL]` was
  printed. Runtime-only checks (sockets/healthz/token) degrade to a single
  informational `[OK]` line when the stack isn't running, so the script is
  always safe pre-flight on a clean host. Never prints secrets or tokens. Added
  a `make doctor` target that runs it.

### Deployment (C011)
- **`feat(deploy)`**: Added a `rollout` service to `deploy/docker-compose.yml`, built
  from `deploy/Dockerfile.rollout`, gated behind an **opt-in** `profiles: ["rollout"]`
  (default `docker compose up` does not start it). Mirrors the existing services'
  hardening posture (`read_only`, `cap_drop: ["ALL"]`, `security_opt:
  no-new-privileges`, non-root, shared `cwso-runtime` UDS volume) with one documented
  exception: no writable mount for the Parquet trajectory store, since it stays
  disabled (`CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED` unset) under this profile. Start
  with `docker compose -f deploy/docker-compose.yml --profile rollout up -d`.

### Deployment (C012)
- **`feat(scripts)`**: Added `scripts/cwso-bootstrap-secrets.sh` to generate the
  dev-only `.env.jwt.dev` file (`JWT_SECRET=<64 hex chars>`, `chmod 600`) consumed
  by `deploy/docker-compose.yml`'s `secrets: jwt_secret` mount, when it is absent.
  Previously a fresh checkout had no `.env.jwt.dev` and the orchestrator container
  failed to start with a JWT-secret config error until one was hand-created. Note
  (updated during C012's own Tech Lead review): this closes the *scripting* half of
  the release-gating condition tracked since C010's CONDITIONAL_PASS review (MR
  !113); the condition itself remains open and has moved to C016, since nothing
  calls this script yet. Idempotent on repeat runs (`[OK] .env.jwt.dev exists`,
  content unchanged), verified gitignored via the existing `.env*` pattern, and
  never prints the secret value to stdout/logs. Wiring this into `make up` is
  deferred to C016; run the script manually before `docker compose up` until then.

### Deployment (C010)
- **`feat(deploy)`**: Removed the stale `profiles: ["phase2"]` / `["phase4"]` gates on
  `git-shadow` and `merge-engine` in `deploy/docker-compose.yml` — both services are
  fully implemented and CI-built, so a bare `docker compose up` now starts the full
  stack (`orchestrator` + `git-shadow` + `merge-engine`) instead of the orchestrator
  alone. Dropped the now-unnecessary `--profile phase2 --profile phase4` flags from
  the quick-start blocks in `README.md` and `docs/user/installation-v3.md` (kept
  byte-identical). No compose hardening keys (environment, volumes, healthcheck,
  security_opt, tmpfs, read_only, cap_drop, secrets) were changed.

## v0.6.1 - 2026-08-08

### Technical Debt Remediation and Test Quality
- **Reduced function complexity** (TD-01 through TD-03, T180–T184): extracted
  `writeBrokerSSEFrame()`, `brokerSSETelemetryDefer()`, and `writeSSEFrame()` SSE
  helpers; `handleBrokerSSE()` reduced from 5 to 3 parameters via a `brokerSSEDeps`
  struct; `handleSSE()`, `handlePOST()`, and `publishSampleEvents()` all reduced to
  ≤3 parameters; introduced `HTTPHandlerConfig` struct for cleaner `RunHTTP()` and
  `newHTTPHandler()` signatures. All functions now comply with the project standard
  (≤50 lines, ≤4 parameters).
- **Race-free broker shutdown** (TD-07, T181): replaced racy `select` guard in
  `Broker.Close()` with `sync.Once`, eliminating concurrent-close goroutine races.
- **SSE connection pooling** (TD-09, T185): zero-count eviction in
  `sseConnectionStore.release()` prevents stale connection leaks on mid-transfer
  client disconnects.
- **New test helper** (TD-04, T186): `logging.NewWithWriter(levelStr, w)` enables
  deterministic, race-safe test log capture without OS-level redirection.

### Bug Fixes (Tests)
- **Fixed flaky tests** (TD-10, TD-11, T188–T189): `TestBrokerSSETelemetryLogOnClose`
  now uses `logging.NewWithWriter()` buffer injection and `httptest.Server.Close()`
  synchronization instead of racy `os.Stderr` redirect + `time.Sleep()`;
  `TestRetentionEvictionOldestFirst` gained a `waitForMaxSeq()` helper. All tests now
  pass under `go test -race -count=5`.

### Maintenance
- Removed `TECHNICAL-DEBT.md` (all 11 items resolved).
- No breaking changes: runtime behavior, API contracts, and CLI signatures are
  unchanged; drop-in replacement for v0.6.0.
- Release artifact: [`docs/artifacts/release-v0.6.1.md`](docs/artifacts/release-v0.6.1.md).

## v0.6.0 - 2026-08-06

### New Features (Operator Dashboard, T001–T010)
- Add operator dashboard with `GET /dashboard/status` (JSON) and `GET /dashboard`
  (embedded HTML polling every 10 s) endpoints — a read-only observability surface
  covering sidecar connectivity, config snapshot, job queue, client activity, and
  rollout pipeline state, with a top-level `overall` health field.
- Add `Stats()` method to `jobs.Manager` exposing live queue/worker/counter snapshot.
- Add `ClientMetrics` in-memory counters (requests, auth failures, rate-limit hits,
  tool calls).
- Add `SidecarChecker` with UDS connectivity probing and 5 s cache.
- Add `schemas/dashboard_status.json` JSON Schema with `additionalProperties: false`.
- Add `CWSO_DASHBOARD_TOKEN` env var support in orchestrator and `docker-compose.yml`;
  dashboard routes return `501 Not Implemented` when the token is unset and the token
  (SHA-256 hashed, constant-time compare) is separate from the MCP JWT secret.
- Add `ADR-011-operator-dashboard.md` documenting all architectural decisions.

### Bug Fixes / CI
- Complete registry publishing for all four CWSO service images (`fix(ci)` T178/T179
  — was in v0.5.x develop but now GA).

### No Breaking Changes
- All existing `/healthz`, `/mcp`, and `/rollout/*` routes are unchanged; no changes
  to MCP tool contracts or JWT auth flows. Dashboard routes are additive.
- Release artifact: [`docs/artifacts/release-v0.6.0.md`](docs/artifacts/release-v0.6.0.md).

## v0.5.2 - 2026-08-03

### CI and Registry Publishing
- Added `build:rollout` to CI so `deploy/Dockerfile.rollout` is built in MR/develop/main/tag pipelines.
- Expanded `deploy:registry` to process all four services: `orchestrator`, `git-shadow`, `merge-engine`, and `rollout`.
- Registry deploy now pushes `:latest` tags on `main` and also pushes `:$CI_COMMIT_TAG` tags on tag pipelines for all four services.

### Integration
- Unblocks downstream pull-only composition workflows by ensuring all required CWSO images are publishable from CI.

### Documentation
- Added release artifact: [docs/artifacts/release-v0.5.2.md](docs/artifacts/release-v0.5.2.md).

## v0.5.1 - 2026-08-01

### Bug Fixes (T170)
- **`fix(rollout)`**: Added a `GET /healthz` liveness route in `cwso-rollout`
  (`services/cwso-rollout/src/proxy.rs`), placed ahead of the existing global POST-only
  gate — pure static `200 {"status":"ok"}`, no upstream/provider dispatch.
  `deploy/Dockerfile.rollout` now carries a `HEALTHCHECK` instruction targeting it
  (`--interval=10s --timeout=3s --retries=5`). `/v1/models` behavior deliberately
  unchanged (still 405 GET / 404 POST — no route exists there).
- **`fix(rollout)`**: `StoreConfig::from_env` (`services/cwso-rollout/src/store.rs`) now
  resolves the trajectory store path via `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first,
  falling back to the canonical `CWSO_ROLLOUT_STORE_PATH`, then `./rollout_store` — fixes
  a name-drift bug where `deploy/Dockerfile.rollout`'s own env var was never read by the
  store.
- Both fixes carry new regression tests
  (`healthz_returns_200_and_v1_models_is_unchanged`;
  `from_env_prefers_trajectory_alias_then_canonical_then_default`) and were verified with
  real `cargo build`/`cargo test` (35/35 pass) and real `docker build`/`docker run`
  (sustained container `(healthy)`, 5/5 probes, `FailingStreak:0`).
- Root cause documented in T169:
  `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md`.

### Security
- **`memmap2`** 0.9.10 → 0.9.11 — resolves **RUSTSEC-2026-0186** (unchecked pointer
  offset), affects `cwso-sparse`.
- **`anyhow`** 1.0.102 → 1.0.104 — resolves **RUSTSEC-2026-0190**
  (`Error::downcast_mut()`), discovered live during T171, affects all Rust crates using
  `anyhow = "1"`.
- **`wasmtime`** 36.0.10 → 36.0.13 — resolves **RUSTSEC-2026-0222**, discovered live
  during T171, affects `cwso-sparse`.
- **`git2`** RUSTSEC-2026-0183/0184 (unsound `Remote::list()` / `BlameHunk` signature UB)
  were temporarily scoped-ignored in CI pending a Rust toolchain bump (blocked on Rust
  ≥1.87 MSRV for `git2 0.21.0`) — resolved fully by the toolchain bump below; no ignore
  remains on `develop`.
- Rust toolchain bumped **1.86 → 1.87** across all three Rust Dockerfiles
  (`git-shadow`, `merge-engine`, `rollout`) and all three Rust CI job images
  (`rust:lint`, `rust:test`, `rust:audit`).
- `git2` bumped 0.20.4 → 0.21.0 in `cwso-git-shadow`, resolving RUSTSEC-2026-0183/0184 and
  removing the scoped `cargo audit --ignore` flags added in T171.

### Operations
- `cargo audit` now exits 0 with zero ignore flags. Full workspace build+test verified
  clean across all 5 Rust crates; `cargo fmt --check` clean.

### Documentation
- Release artifact: [`docs/artifacts/release-v0.5.1.md`](docs/artifacts/release-v0.5.1.md).

## v0.5.0 - 2026-07-27

### Phase 3.1 and transport hardening
- **Executor node registry** with round-robin task assignment (Phase 3.1, T235.1). Nodes
  register and receive tasks distributed deterministically across available executors.
- Transport **SSE `WriteTimeout` disabled** — long-lived event streams no longer severed
  by the Go HTTP server timeout (fix(transport)).
- **MCP rate limiting** burst raised to 10 with localhost exemption; HTTP 429 documented.
- **Jobs manager close-path fix** — `Manager.Close()` drains the queued-job channel before
  cancelling the root context, so queued jobs reach `cancelled` reliably.
- **Deterministic round-robin** — node ordering is now stable across Go runs (T164).

### Security
- Go toolchain raised to **1.25.12** — remediates **GO-2026-5856** (`crypto/tls` ECH
  privacy leak). All three CI job images updated.
- `crossbeam-epoch` pinned to **0.9.20** — remediates **RUSTSEC-2026-0204** (invalid
  pointer dereference in `fmt::Pointer` for `Atomic`/`Shared`). Transitive path:
  `wasmtime → rayon-core → crossbeam-deque → crossbeam-epoch`.

### Operations
- `main` branch integrated into `develop` (MR !74); production and integration lines back
  in sync.

### Documentation
- Release artifact: [`docs/artifacts/release-v0.5.0.md`](docs/artifacts/release-v0.5.0.md).

## v0.4.0 - 2026-06-09

### Polar parity and operator readiness
- Polar **harness adapter registry** and Docker runtime launcher with shell-command reference
  harness (T144, carried from v0.3.0 scope).
- Rollout **`num_samples`** session fan-out (1–32) with per-session callbacks (T145).
- **Gateway async staging** — INIT/READY/RUNNING/POSTRUN worker pools, evaluator prewarm stub,
  and partial trace recovery on session timeout (T146).
- **Evaluator registry** with merge-SM session reward and SWE-bench stub hook (T148).
- **Trajectory builder v2** — `per_request` and `prefix_merge` strategies, EOT interstitial
  masking, partition-key chain splitting; per-task `trajectory_builder_strategy` on submit (T149).
- Comprehensive **installation guide v2** (`docs/user/installation-v2.md`) — architecture, full
  `CWSO_*` flag reference, MCP/rollout/gateway/evaluator workflows (T156).
- **IDE integration guide** for VS Code / Cursor MCP + rollout proxy routing (T154).
- **`scripts/cwso-enable-all-features.sh`** and env example for local PoC demos (T155).
- CI **tag pipeline deploy fix** — `deploy:registry` uses `needs:optional` on `e2e:phase2` (T153).

### Documentation
- Primary adoption doc for v0.4.0: [`docs/user/installation-v2.md`](docs/user/installation-v2.md).
- Release artifact: [`docs/artifacts/release-v0.4.0.md`](docs/artifacts/release-v0.4.0.md).

### Deferred post-GA
- T150 — KV differential prompting.
- T151 — Offline SFT data generation mode.

## v0.3.0 - 2026-06-07

### Post-RC hardening and operator readiness
- Installation and usage guide (`docs/user/installation-v1.md`) for Docker quick start,
  JWT, MCP HTTP, Phase 4/Next-Gen flags, and troubleshooting (T142).
- OpenAI **Responses API** route (`/v1/responses`) with provider-specific synthetic SSE
  and capture pipeline hardening (T147).
- Polar **harness adapter registry** and Docker runtime (start/stop/exec/upload/download)
  with shell-command reference harness and proxy-capture e2e (T144).
- CI e2e hardening: MCP RPC retry on transient connection errors in phase2 integration.

### Operations (carried from RC)
- KV prefix router (T135, default-off), blocking `go:audit` / `rust:audit` (T140).
- Phases 6–9 feature set unchanged from `v0.3.0-rc1`; see RC CHANGELOG for full scope.

### Deferred post-GA
- Polar parity T145–T151 (session fan-out, gateway staging, evaluators, trajectory parity,
  differential prompting, offline SFT).

## v0.3.0-rc1 - 2026-06-06

### Phase 6 — Heterogeneous Hardware Dispatcher (Feature A)
- Rust `cwso-hal` sidecar with `InferenceBackend` trait, CPU baseline, and optional GPU/LPU
  OpenAI-compatible adapters (`T082`–`T084`).
- Workload profiler and `dispatch_hardware_aware_job` MCP tool with live HAL execution,
  capability sync, and deterministic fallback chain (`T085`–`T087`).
- Context propagation, active health probing, TLS endpoint validation, and CI dependency
  audits (`T090`–`T094`, `T114`).
- Phase 6 gate **PASS/PASS** per `gate-phase6-feature-a-2026-06-02.md`.

### Phase 7 — Sparse Micro-Agents & Spiking Monitors (Features B + C)
- `cwso-sparse` sidecar: deterministic 1.58-bit ternary GEMM, `.cwsl` mmap loader,
  wasmtime agent lifecycle (`T119`–`T122`).
- `create_ephemeral_sparse_agent` MCP tool and quality-floor → dense GPU escalation (`T123`).
- AST write-spike monitor, semantic filter, conflict pre-warning, and `subscribe_ast_spikes`
  MCP resources with write-event feeder (`T115`–`T118`).
- Phase 7 gate **PASS/PASS** per `gate-phase7-feature-bc-2026-06-04.md`.

### Phase 8 — Semantic Sparse-Merging (Feature D)
- Sparse AST tensor encoding spec (ADR-009) and AVX2 sparse diff kernel (`T126`–`T127`).
- Sparse pre-filter in `merge_three_way` and sparse↔dense conformance suite (`T128`–`T129`).
- Large-repo merge benchmark and Phase 8 gate **PASS/PASS**
  (`gate-phase8-feature-d-2026-06-04.md`, `T130`).

### Phase 9 — Rollout-as-a-Service (Features E + F + G)
- `cwso-rollout` hyper reverse proxy with zero-copy capture (`T132`).
- Trajectory builder with prefix merging and Parquet/LZ4 trajectory store (`T133`–`T134`).
- Programmatic merge rewards (+1/−1) and Polar REST API for trainer e2e (`T136`–`T137`).
- Phase 9 integration QA and gate **PASS/PASS** (`qa-phase9-report-v1.md`,
  `gate-phase9-feature-efg-2026-06-05.md`, `T138`).

### Operations and Documentation
- Release readiness artifact: `docs/artifacts/release-v0.3.0-rc1.md`.
- Checkpoints 007–011 cover Phases 6–9 completion on `develop` @ `5d2cfca`.

### CI / Gates
- T138 merged via MR !47 (squash `011d8c8`); pipeline green on feature branch pre-merge.
- All new capabilities ship default-off behind `CWSO_*` flags.

### Known residual risk (RC)
- KV prefix router (T135) and trainer fleet proxy benchmark deferred.
- CI `govulncheck` / `cargo audit` remain `allow_failure: true` (T094 PoC posture).
- Orchestrator `/v1/chat/completions` is a 501 stub; transparent proxy on `cwso-rollout`.

## v0.2.0-rc1 - 2026-05-24

### Phase 5 Hardening Closure
- Closed all security hardening follow-ups from Phase 5 conditional pass:
  - T073: Wasm module integrity verification (SHA-256 pin + trusted path)
  - T074: Telemetry minimization/redaction policy (request ID and anomaly notes)
  - T075: eBPF latency semantics hardening (explicit advisory signaling)
- Updated dispatch telemetry and anomaly contracts to reduce false precision and
  sensitive-field exposure while preserving deterministic fallback behavior.

### Operations and Documentation
- Expanded hardware-aware operator guidance in README with:
  - mandatory Wasm integrity controls (`CWSO_HHD_WASM_SCORING_MODULE_SHA256`,
    `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`)
  - telemetry redaction controls (`CWSO_HHD_TELEMETRY_*`)
  - explicit advisory interpretation for `ebpf-hook` latency fields.
- Added release-candidate readiness artifact for v0.2.0-rc1.

### CI / Gates
- Release-candidate validation reached green pipeline on `develop` after the
  final hardening changes (`2548879153`).
- No open active tasks remain in Phase 5 scope.

## v0.1.1 - 2026-05-22

### Release Blockers Closed
- Closed all tracked post-v0.1.0 release blockers from the Phase 4 conditional pass:
  - T054: merge-engine unit-test CI gate requirement
  - T055: `merge_inputs` schema/runtime alignment
  - T056: ADR-006 reconciliation for node-level conflict-detail scope
  - T057: e2e policy-path validation for sidecar reason mapping
- Reconciled task board state to reflect blocker completion and current non-blocking deferrals.

### Documentation
- Updated [README.md](README.md) with a clearer "What CWSO is" overview and a
  practical "How to use CWSO" section covering startup, auth, MCP invocation,
  and validation commands.
- Added [release-v0.1.1 artifact](docs/artifacts/release-v0.1.1.md) with scope,
  validation, and release readiness summary.

### CI / Gates
- Release-ready baseline confirmed on `develop` with green lint/build/test/e2e
  pipeline status prior to release packaging.

## v0.1.0 - 2026-05-16

### Added
- Phase 1 foundation (T001-T011): requirements and architecture baselines, security baseline, Go orchestrator MCP server core, baseline filesystem tools, Streamable HTTP transport skeleton, and HS256 + Origin controls.
- Phase 2 shadow workspace + AST (T020, T022, T026, T028, T029): Rust `cwso-git-shadow` sidecar, UDS shadow client/tools, end-to-end integration harness, and PoC debt remediation pass.
- Phase 3 transport + concurrency (T030-T038): full-duplex SSE transport, async job runner pool, concurrent dispatch tool, event-sourced memory broker, telemetry throttling, and completed tech-lead/security gates.
- Phase 4 sandbox + merge pipeline (T040-T050): Docker/gVisor/Firecracker runner path, sandbox tier router, Rust merge engine, AST semantic merge flow, conflict-matrix escalation, and matrix-aware swarm e2e suite.

### Security
- Security gate T051 re-audit passed after remediation completion (see checkpoint-020).
- T058 hardened sidecar IPC socket permissions and Linux peer authorization.
- T059 added baseline HTTP security headers in transport middleware.
- T060 enforced `application/json` Content-Type for `POST /mcp`.
- T061 removed RS256 ambiguity by constraining current build/runtime to HS256.

### Testing and Validation
- Phase 1 review gate: PASS (checkpoint-001).
- Phase 2 integration validation: PASS for sidecar + shadow workspace + AST flows (checkpoint-002).
- Phase 3 tech-lead and security gates: PASS (checkpoint-008).
- Phase 4 quality gate: CONDITIONAL_PASS with tracked follow-up items (checkpoint-018).
- Security re-audit gate: PASS (checkpoint-020), with evidence:
  - `cargo test -p cwso-git-shadow -p cwso-merge-engine` (Rust sidecars): PASS.
  - `go test ./internal/config ./internal/transport` (orchestrator): PASS.

### Notes / Known Residual Risk
- Non-Linux peer-credential fallback remains permissive; acceptable for current Linux deployment scope, but must be revisited if portability scope expands.
- HSTS effectiveness depends on HTTPS termination configuration in deployment.
- T050 follow-up conditions remain tracked as open work for post-v0.1.0 hardening/alignment: T054, T055, T056, T057.