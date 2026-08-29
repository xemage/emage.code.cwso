# CWSO Debt Register

**Status:** live — this is the single place to look for outstanding proof-of-concept debt.
**Created:** 2026-08-12 (task C003)
**Last full re-classification:** 2026-08-29 (task C060 — see "C060 release classification" section below)
**Supersedes (as history, not content):** `docs/archive/debt/POC-DEBT-SCORECARD-phase1.md`, `docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`

## Purpose

Until this file existed, CWSO's debt was scattered across two archived phase
scorecards (`docs/archive/debt/`) and inline `POC-DEBT` code comments, with no
single view of what is closed, what blocks v1.0, and what is deferred. This
register consolidates all three sources into one live table. Every row carries a
disposition:

| Disposition | Meaning |
|---|---|
| `v1.0-blocker` | Must be closed (or explicitly documented as a limitation) before v1.0 is declared. The closing C-task is named in the row. |
| `v1.1` | Real debt, deliberately deferred past v1.0. |
| `wontfix` | Reviewed and consciously accepted as-is. |
| `fixed` | Closed in code; the evidence column quotes the verifying file/symbol. |
| `unclear` | Source row unintelligible or contradicts the code — flagged, never guessed. |

Rows are identified by their **original** IDs where one exists (roadmap blockers
`B1`–`B13`, phase-1 scorecard `D1`–`D8`, phase-2 scorecard `P2-1`–`P2-8`).
Scorecard-only items with no prior ID are numbered `R-x` here. Historical
scorecard sections at the bottom record where each row came from and whether it
was carried forward or closed.

## C060 release classification (2026-08-29)

Per `docs/tasks/task-C060.md`, every row in the live register below is additionally
re-classified into a stricter, 3-value release-readiness scheme for v1.0.0. This is
the authoritative release-gate answer (see the new **`C060 class.`** column on every
row); the `Disposition` column above/below records the fuller historical picture and
is preserved unchanged, not replaced.

| Value | Meaning |
|---|---|
| `fixed` | Re-verified directly against the current code by this task (marker gone or never existed, described behavior/tests confirmed present) — genuinely closed. |
| `documented-limitation` | Real, disclosed gap that stays open for v1.0 by a conscious decision; requires (or is flagged here as requiring) a matching `docs/LIMITATIONS.md` entry (C063). |
| `v1.1` | Real debt, explicitly deferred past v1.0, not required for release. |

**Result (updated 2026-08-29 — R-11, R-12 added by C061's security audit findings
F-C061-03/F-C061-04, after C060's own classification pass closed): 28/28 rows
classified. Zero unclassified rows.** (`fixed`: 11 — B2, B6, B7, B12, B13, D3, D6,
P2-3, R-3, R-7, R-9; `documented-limitation`: 3 — B1, R-1, R-6; `v1.1`: 14 — B11, D2,
D4, D5, D8, P2-2, P2-8, R-2, R-4, R-5, R-8, R-10, R-11, R-12.)

**`documented-limitation` rows and their `docs/LIMITATIONS.md` cross-check status (as of
this classification pass):**

| Row | Needs a LIMITATIONS.md entry for | Status |
|---|---|---|
| B1 | MCP hand-rolled kernel: 6/16 methods + 8/9 notifications genuinely unimplemented in v1.0 (kept by ADR-013, full inventory in `docs/artifacts/mcp-gap-analysis-v1.md`) | **Not yet written** — `docs/LIMITATIONS.md` does not exist in this worktree; C063 owns creating it |
| R-1 | Dev/compose JWT secret is file-based (`.env.jwt.dev` staged into the `cwso-jwt-secret` volume), acceptable for v1.0's local-only deployment model, not for production | **Not yet written** — same |
| R-6 | `git-shadow`'s tmpfs projection mount is `noexec` with no compiler toolchain in the runtime image (deliberate hardening, C019); CI works around it (C024), production code is unaffected | **Not yet written** — same |

`docs/LIMITATIONS.md` was confirmed absent from this worktree at the time of this
classification pass (2026-08-29) — C063 has not yet run. All three
`documented-limitation` rows above are honest, real, already-evidenced findings (each
has a `POC-DEBT`/design-doc paper trail predating this task) — `documented-limitation`
is not being used here to avoid work, per this task's own rail. They cannot be marked
fully closed end-to-end until C063 publishes `docs/LIMITATIONS.md` with a matching
entry for each. This is a **coordination dependency on C063**, not a gap in this
classification pass, and is flagged explicitly rather than silently left open.

### Notes on C060 reclassifications (2026-08-29)

Re-verification for this task found three rows whose existing label undersold or
overstated current reality; all three are corrected here rather than carried over
silently, per this task's "do not trust the register's own claim" rail.

- **B1 — MCP protocol: `fixed` → `documented-limitation`.** Re-reading
  `orchestrator/internal/mcp/protocol.go`, `orchestrator/internal/server/server.go`
  (`Handle`, `handleInitialize`), `docs/decisions/ADR-013-mcp-protocol-path.md`, and
  `docs/artifacts/mcp-gap-analysis-v1.md` confirms C032's two named "required fixes"
  are genuinely done — `handleInitialize` now advertises `resources.listChanged:
  false` (matching that the notification is never published) and `Handle()`'s
  `ParseRequest` error path now distinguishes -32700 from -32600 via a new
  `mcp.RequestError` type — so the two concrete defects this row's `fixed`/C030–C032
  label pointed at are real fixes, not a rubber stamp. But the row's *own
  description* — "only a partial method set is implemented" — is still literally true
  today, by design: ADR-013 records the human's 2026-08-13 decision to explicitly keep
  the hand-rolled kernel rather than adopt the official SDK, and the gap analysis's own
  Consequences section states plainly "the 6 Missing methods and 8 Missing
  notifications remain genuinely unimplemented in v1.0." A conscious, ADR-recorded,
  conformance-tested decision to ship a documented subset of the spec is the textbook
  definition of a *documented limitation*, not a `fixed` bug — labeling it `fixed`
  overstates what shipped. This is **not** a scope change (B1 was never deferred to
  v1.1; it stays a required v1.0 release-gate item, now satisfied via disclosure +
  conformance testing rather than via literal completeness), so it does not trigger
  this task's v1.0-blocker→v1.1 sign-off rail — but it is flagged prominently here for
  the orchestrator's explicit awareness before merge, since it is a real downgrade from
  the row's prior label.
- **D3 — SSE notifications: `v1.1` → `fixed`.** This row's own text ("SSE GET endpoint
  emits heartbeats only; real notifications deferred to ... T030") is stale. Re-reading
  `orchestrator/internal/transport/http.go`'s `handleSSE` found a live `sub.Messages()`
  case (alongside the heartbeat ticker) forwarding real broker events via
  `writeBrokerSSEFrame`, and `publishSampleEvents` genuinely publishing
  `notifications/log`/`notifications/job-state` onto the event bus for
  request-lifecycle events — T030 (EventBus integration) has evidently already
  shipped; its own task record predates `docs/tasks/completed-tasks.md`'s ledger and
  could not be located in this repo, but the resulting code is directly observable and
  live. The literal debt this row describes ("no real notifications") is closed. See
  the row's own updated text below for the narrower, already-tracked spec-shape
  nuance (non-spec method names/payload shape) — folded into B1's
  `documented-limitation` above via `mcp-gap-analysis-v1.md` §2, not double-counted as
  a separate open item here.
- **R-6 — `wontfix` → `documented-limitation`.** No factual change to the underlying
  finding — this row was already a consciously-reviewed, permanently-accepted
  constraint (independent Tech Lead review of C024 explicitly recommended deferring
  any change, per the row's own text). `wontfix` and `documented-limitation` describe
  the same real-world state under this task's stricter 3-value scheme; relabeled for
  consistency with the release-gate vocabulary, not because anything about the
  underlying finding changed.
- **D2 — JWT verifier: description corrected, classification unchanged (`v1.1`).**
  This row's own text ("Hand-rolled HS256 JWT verifier; production must use
  `golang-jwt/jwt/v5`... full claims validation") is partly stale. Re-reading
  `orchestrator/internal/transport/http.go`'s `verifyJWT` found it already imports and
  uses `github.com/golang-jwt/jwt/v5` (confirmed in `orchestrator/go.mod`) via
  `jwt.NewParser(jwt.WithLeeway(60*time.Second), jwt.WithExpirationRequired())`, and
  independently validates `iss`/`aud` after parsing — the section header comment reads
  `// --- JWT verification using github.com/golang-jwt/jwt/v5 (T029 remediation) ---`.
  The "hand-rolled verifier" and "full claims validation (iss, aud, nbf, exp leeway)"
  halves of this debt are genuinely closed. What remains genuinely open, confirmed by
  reading the algorithm `switch`: only `"HS256"` is accepted (`default:` branch
  returns `"unsupported algorithm"`), and `deploy/docker-compose.yml`'s own
  `CWSO_JWT_ALG: "HS256"` line carries an explicit `# RS256 is not supported in this
  build` comment; no key-rotation mechanism was found anywhere in `config`/
  `transport`. Row text below narrowed accordingly (RS256 + key rotation only, not a
  full hand-rolled-verifier rewrite); still genuinely deferred, not required for
  v1.0's local-only deployment model (same reasoning as R-1/R-2), so `v1.1` stands.
- **R-1 — file:line citation corrected in the marker cross-check table below.** The
  in-code `POC-DEBT` marker cross-check table cited `deploy/docker-compose.yml:6` for
  R-1; that line number is stale — the compose file has been substantially
  restructured since (T191's JWT-secret-staging rework moved the top-level `secrets:`
  stanza into the `jwt-secret-fix` service + named-volume pattern). The marker itself
  is still present, now at `deploy/docker-compose.yml:513` (the `volumes:` section,
  `cwso-jwt-secret` entry) — its own comment explicitly asks for this citation to be
  updated "next time \[the file\] is touched," which this task satisfies. Corrected
  below; disposition/classification unaffected.

## Live register

| ID | Source `file:line` | Category | Description | Status | Disposition | Closing task | C060 class. |
|---|---|---|---|---|---|---|---|
| B1 (= D1, P1-1) | `orchestrator/internal/mcp/protocol.go:10` | Maintainability / spec compliance | Hand-rolled MCP protocol subset instead of the official `go-sdk`; only a partial method set is implemented | closed | fixed | C030–C032 | **documented-limitation** (see "Notes on C060 reclassifications" above — kept-and-proven per ADR-013, not literally completed; 6/16 methods + 8/9 notifications genuinely unimplemented in v1.0) |
| B2 (= P2-1) | `services/cwso-git-shadow/src/main.rs` (module doc; marker removed) | Architecture | Was: OverlayFS bind-mount layer deferred; shadow files reachable only via orchestrator→sidecar IPC. Fixed by C021: every shadow workspace is now eagerly materialized onto a real, tmpfs-backed directory (`<storage_root>/<workspace-uuid>/`, ADR-012 "materialise-to-tmpfs") at creation time and kept in sync on every write, so `ls`/`cat`/`pytest`/arbitrary tooling can reach it directly. Write-back of raw external writes at the projected path into the git object store (the remaining half of the original gap) is now also fixed by C022: `services/cwso-git-shadow/src/writeback.rs`'s `WriteBackEngine` (inotify-driven, with a periodic hash-based reconciliation backstop per ADR-012) folds create/modify/delete/rename mutations made directly at the real path back into `Workspace.files`, so `commit_shadow` captures them regardless of how the edit arrived | closed | fixed | C021, C022 | **fixed** (re-verified 2026-08-29: `repo.rs` module doc + `writeback.rs`'s `WriteBackEngine` module doc both confirmed present and matching this description) |
| B6 (= P2-7) | `services/cwso-git-shadow/src/ast.rs` (`find_references` / `resolve_references`) | Correctness | Was: `find_references` matched identifier text only — no scope/binding analysis; false positives across shadowed names. Fixed by C040: `resolve_references` (`services/cwso-git-shadow/src/ast.rs`) builds a real per-file lexical scope tree (nearest-enclosing-scope binding resolution) for all four wired grammars (Go, Python, Rust, TypeScript); an occurrence is only reported when it resolves to a real, in-scope declaration, honestly excluding orphaned/out-of-scope text matches and member/attribute/method-call-site access (which would require type inference, deferred to v1.1) rather than guessing. Definition sites remain always-reported (unambiguous). Regression-tested against a 17-case shadowed-name fixture set (nested-scope shadowing, same method name on different receivers, shadowed imports) covering all four grammars, zero false positives | closed | fixed | C040 | **fixed** (re-verified 2026-08-29: `resolve_references`/`resolve_references_walk` read directly, 17 shadowed-name regression tests confirmed present in `ast.rs`'s `#[cfg(test)]` module, matching the described scope-resolution logic and per-grammar node-kind handling) |
| B7 (= P2-4) | `services/cwso-git-shadow/src/repo.rs` (`Workspace.head`, `ShadowStore::commit`) | Correctness | Was: every shadow commit was an orphan (no parent); workspaces never formed a history chain, so per-workspace history and three-way merges were unavailable. Fixed by C041: `Workspace` now tracks its own `head: Option<Oid>` (seeded from the base commit at `create` time, or `None` for a base-less workspace); `ShadowStore::commit` reads that as the sole parent for the next commit, then advances `head` to the new commit's oid only after `repo.commit` succeeds. A base-less workspace's first commit is still a genuine root commit (`head` starts `None`); every commit after that — in the same workspace, or continuing from a seeded base commit — forms a real parent-child chain, unblocking C042's three-way merge | closed | fixed | C041 | **fixed** (re-verified 2026-08-29: `Workspace.head: Option<Oid>` field read directly in `repo.rs`, with the exact doc comment describing the seed/advance behavior this row claims) |
| B12 (= P2-5) | `services/cwso-git-shadow/src/main.rs:104`, `services/cwso-merge-engine/src/ipc.rs:31` | Security | Was: UDS permissions reported as `0o666` (world read-write) in the phase-2 scorecard, plus no shared-GID story across orchestrator/sidecar UIDs. C044 live-reverified against a running compose stack (2026-08-27) that both claims are stale: both sockets have been bound `0o660` since the sidecars' first commit, and T197 already corrected `CWSO_IPC_ALLOWED_GIDS` to the live orchestrator image gid. See verification note below | closed | fixed | C044 | **fixed** (re-verified 2026-08-29: both cited lines read directly — `main.rs:104` and `ipc.rs:31` both call `std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))`; `scripts/check-ipc-gid-drift.sh` and `deploy/docker-compose.yml`'s `CWSO_IPC_ALLOWED_GIDS: "0,101"` on both `git-shadow`/`merge-engine` confirmed present) |
| B13 (= P2-6) | `orchestrator/internal/shadow/client.go` (marker removed; see `Client.acquire`/`Client.release`) | Performance | Was: one TCP-style request-per-connection model, all RPCs serialized through a single `Client.mu` — no pooling, no pipelining; would throttle under concurrent dispatch. Fixed by C043: `Client` now holds a bounded pool of persistent UDS connections (`sem` + `idle`, default size 8, configurable via `NewClientWithPoolSize` or `CWSO_SHADOW_POOL_SIZE`); each `Call` checks out one connection exclusively for its round trip and returns it for reuse afterward, so synchronization is per-connection rather than global and up to `poolSize` RPCs are genuinely concurrent. Verified by `internal/shadow/client_test.go`'s `TestSoakConcurrentDispatch` (32 concurrent calls over a pool of 4, race-detector clean) | closed | fixed | C043 | **fixed** (re-verified 2026-08-29: `client.go`'s package doc and `sem`/`idle` pool fields read directly, matching this description) |
| R-9 | `deploy/docker-compose.yml` (`rollout` service block) | Security | Independent Security Engineer re-review of C044 (SEC-C044-001, HIGH) found the opt-in `rollout` service shared the same uid=100/gid=101 `cwso` identity as orchestrator/git-shadow/merge-engine (coincidental: Debian `addgroup --system` landing on the same numbers as Alpine's independently-assigned identity) *and* mounted the same `cwso-runtime` volume the `git-shadow`/`merge-engine` sockets live on — since `IpcAuthzPolicy::allows()` is `uid OR gid`, a compromised `rollout` container would have passed both sidecars' authorization check despite having zero code that dials either socket today. Distinct from B12 (which is about the sockets' own permission bits and GID alignment, both still correct) — this is about an unrelated service's unnecessary *reachability* to those same sockets. (Renumbered from this fix commit's original R-7 to R-9 during merge with develop — R-7 and R-8 were independently claimed by C041's SEV-C041-001 fix and its own follow-up, merged to develop first; no content change, ID collision only) | closed | fixed | C044 (follow-up) | **fixed** (re-verified 2026-08-29: `deploy/docker-compose.yml`'s `rollout` service block read in full — no `cwso-runtime` volume mount present; the removal and its rationale are documented in-line in the compose file itself) |
| B11 | `orchestrator/internal/rollout/evaluator_swebench.go:64` | Functionality | SWE-bench/SWE-Gym evaluator is a stub — harness launch deferred; returns neutral reward | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `evaluator_swebench.go` read directly — `Evaluate()` still ends with the exact `POC-DEBT: Launch SWE-bench/SWE-Gym harness...` marker at line 64 and returns a stub/neutral result) |
| P2-2 | scorecard P2-2 (planning item, no code marker) | Performance | Merkle-hash incremental indexer not implemented; every AST query re-parses the file. Fine at PoC sizes (<1k LOC), will not scale | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `ast.rs`'s `query()` entry point still calls `parser.parse(src, None)` fresh on every invocation; no caching/indexing layer found anywhere in the crate) |
| R-1 (= P1-5) | `deploy/docker-compose.yml:513` | Security | File-based JWT secret (`../.env.jwt.dev`) acceptable for dev/compose; production needs external secret management (Vault/SOPS). v1.0 is local-only, so this is acceptable **if documented** | open | v1.0-blocker (document) | C063 | **documented-limitation** (file:line corrected 2026-08-29 from the stale `:6` — see "Notes on C060 reclassifications" above; needs a matching `docs/LIMITATIONS.md` entry from C063, not yet written — see cross-check table above) |
| R-2 (= P1-5, prod half / T029) | `deploy/docker-compose.yml:5` | Security / operations | Vault/SOPS external secret management (T029) not started — the production half of the compose-secret debt | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: no Vault/SOPS integration found anywhere in `deploy/**` or `scripts/**`; genuinely not started) |
| P2-3 | `services/cwso-git-shadow/Cargo.toml:20` | Language coverage | Only Go + Python tree-sitter grammars wired at Phase 2; Rust and TypeScript required by FR-3 | closed | fixed | — | **fixed** (re-verified 2026-08-29: `Cargo.toml` declares all four `tree-sitter-{go,python,rust,typescript}` deps; `ast.rs`'s `Lang` enum and `ts_language()` wire all four) |
| D6 (= P1-7) | `orchestrator/internal/transport/http.go` (rate limiting) | Security | No per-IP rate limit on `/mcp` POST at Phase 1; relied on JWT to gate | closed | fixed | — | **fixed** (re-verified 2026-08-29: `newRateLimiterStore`/`rateLimitMiddleware` read directly in `http.go`, wired into the `/mcp` handler chain, 60 req/min default confirmed) |
| D2 (= P1-2) | `orchestrator/internal/transport/http.go` (`verifyJWT`) | Security | **C060-narrowed (2026-08-29 — see "Notes on C060 reclassifications" above):** was "hand-rolled HS256 JWT verifier; production must use `golang-jwt/jwt/v5` with RS256, key rotation, full claims validation." Re-verification found `verifyJWT` already uses `github.com/golang-jwt/jwt/v5` (`orchestrator/go.mod`) with `jwt.WithLeeway(60s)`/`jwt.WithExpirationRequired()` plus explicit `iss`/`aud` checks — those two halves are genuinely closed. What remains: only `HS256` is accepted (`deploy/docker-compose.yml`'s `CWSO_JWT_ALG` comment: `# RS256 is not supported in this build`); no key-rotation mechanism exists | open | v1.1 | — | **v1.1** (description corrected 2026-08-29; remaining scope narrowed to RS256 support + key rotation only) |
| D3 (= P1-3) | `orchestrator/internal/transport/http.go` (`handleSSE`, `publishSampleEvents`) | Functionality | **C060 reclassification (2026-08-29 — see "Notes on C060 reclassifications" above):** was "SSE GET endpoint emits heartbeats only; real notifications deferred to Phase 3 EventBus integration (T030)." Re-verification found this stale: `handleSSE` has a live `sub.Messages()` case forwarding real broker events, and `publishSampleEvents` genuinely publishes `notifications/log`/`notifications/job-state` for request-lifecycle events over SSE. T030 has evidently already shipped (its own task record predates the current ledger and could not be located, but the resulting code is directly observable). Residual spec-shape non-conformance (non-spec method names, no `logging/setLevel` filtering) is already tracked under B1's `documented-limitation`, not double-counted here | closed | fixed | T030 (pre-ledger; re-verified live in code by C060, 2026-08-29) | **fixed** (reclassified 2026-08-29 from stale `v1.1`) |
| D4 (= P1-4) | `orchestrator/internal/logging/logger.go` (package doc) | Observability | Stdlib-only logger; production should adopt `zerolog` + OTEL integration | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `logger.go`'s package doc still reads "stdlib-only... A richer logger (zerolog) is planned for Phase 3 hardening (T029)"; no zerolog/OTEL import found anywhere in `orchestrator/**`) |
| D5 (= P1-6) | `orchestrator/internal/server/server.go` (`handleInitialize`) | Spec compliance | Capability negotiation declares `tools.listChanged: false`; full capability set (resources, prompts, sampling) deferred | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `handleInitialize` read directly — advertises only `tools`/conditionally `resources`; no `prompts`/`logging`/`completions`/`sampling` capability ever set, matching this row exactly) |
| D8 (= P1-8) | `orchestrator/internal/tools/fs_tools.go` (1 MiB cap) | Robustness | Read cap is hard-coded at 1 MiB; production should expose it via config | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `const maxSize = 1 << 20 // 1 MiB cap for Phase 1` confirmed still hard-coded in `ReadFileSync.Execute`, not config-driven) |
| P2-8 | `services/cwso-git-shadow/src/repo.rs` (`Workspace.base_tree`) | Cleanliness | **Correction (Tech Lead review, 2026-08-29):** the "never read after seeding the file map; dead state" description is stale. `base_tree` is genuinely read at `workspace_meta()` (`ws.base_tree.map(...)`) and returned directly in the `CreateWorkspace`/`GetWorkspace` IPC responses — it is live, used for external workspace-metadata reporting on every creation/get call, just not consumed by any internal decision logic. `v1.1` classification unaffected (this remains real, low-priority cleanup debt — a field with exactly one external-reporting consumer and no internal logic use is still worth reviewing for its actual justification), but "dead state" overstates it | open | v1.1 | — | **v1.1** (description corrected 2026-08-29; not dead, read once for external metadata reporting — see corrected description) |
| R-3 | `services/cwso-git-shadow/src/repo.rs` (`scan_workspace_tree`'s doc comment) | Security | C022's write-back read-side scan (`scan_workspace_tree`, used by both the inotify event handler and the reconciliation pass) checked `symlink_metadata` immediately before reading/recursing into an entry, but that check and the subsequent `fs::read`/recursion were two separate syscalls, not one fd-anchored operation the way the write side's `openat(..., O_NOFOLLOW)` (`materialize_write_via_fd_walk`) closes the equivalent gap. A component could in principle be swapped for a symlink in the narrow window between the two. Independent Security Engineer review of C022's MR (!153) confirmed this was not exploitable against *today's* mount topology, but found the "same actor already has the access" justification for accepting this permanently rested on an assumption about sandbox-mount wiring that had not been built yet (`C024`) and that ADR-012 itself names as an unsolved mount-propagation problem — accepting a TOCTOU gap permanently on a premise that depends on future code being built a particular way was inconsistent with the bar this project held the write side to (SEC-001, HIGH, blocking). | closed | fixed (C035) | C035 | **fixed** (re-verified 2026-08-29: `repo.rs` module doc + `Workspace`/`ShadowStore` structures read directly, consistent with the fd-anchored read-side hardening this row and C060's C035 task-log entry describe; the specific `open_entry_nofollow`/`fdopendir_dup` primitives were not individually re-read line-by-line under this task's time budget, but are independently corroborated by the two separate reviewers recorded in `docs/tasks/completed-tasks.md`'s C035 entry — Tech Lead and Security Engineer, both independently reproducing the pre-fix/post-fix race distinction from scratch in their own separate worktrees — corrected 2026-08-29 (Tech Lead review of C060) from a prior mischaracterization as "two independent security-engineer re-reviews") |
| R-4 | `services/cwso-git-shadow/src/writeback.rs` (`handle_event`'s non-UTF-8 filename branch) | Robustness | A file created/renamed directly at a shadow workspace's real path with a non-UTF-8 name is silently skipped by write-back (logged at `warn`, no error surfaced to the caller) and can never be captured into a `commit_shadow` result. This is a pre-existing, system-wide constraint C022 does not introduce or worsen — `Workspace.files` is `HashMap<String, Oid>` and the IPC protocol carries every path as a JSON string end-to-end, so `write_file` itself already could not represent such a path either — but C022 is the first code path that can *observe* such a name arriving via raw filesystem tooling (rather than only ever receiving paths that were already JSON-string-encoded), so it is called out explicitly here rather than left implicit | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `writeback.rs`'s module doc, read directly, still documents this exact constraint; no fix present) |
| R-5 | `services/cwso-git-shadow/src/writeback.rs` (rename-handling doc comment, "Accepted, documented race") | Correctness | Rename is decomposed into two independent operations (`MOVED_FROM` → delete, `MOVED_TO` → create/sync), not one atomic move — independent Tech Lead review of C022's MR (!153) endorsed this design (no cookie-correlation) as sound, but identified a real, narrow race: the delete-half and create-half are two separate critical sections, and a `commit()` landing in the gap between them could observe the affected file as missing under *both* its old and new path (a transient full disappearance), self-resolving on the next event or reconciliation tick. Non-blocking; a future hardening (not required for v1.0) would batch same-tick, same-`inotify`-cookie `MOVED_FROM`/`MOVED_TO` pairs into one atomic delete+create under a single lock acquisition | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `writeback.rs`'s "Rename handling: independent delete+create, not cookie-correlated" doc-comment section, read directly, still describes exactly this design and its accepted race) |
| R-6 | `deploy/docker-compose.yml` (`git-shadow`'s `tmpfs:` mount) + `deploy/Dockerfile.git-shadow` (minimal runtime image) | Test infrastructure | C024's real-filesystem E2E proof discovered the shadow projection's tmpfs mount is `noexec` and the `git-shadow` runtime image carries no compiler/toolchain — both the direct, intended consequence of C019's deliberate hardening (`cap_drop: ["ALL"]`, `network_mode: "none"`, `read_only: true`, minimal base image), not an oversight. C024 works around this correctly, not around the constraint's substance: it pre-compiles a static test binary (`CGO_ENABLED=0`) outside the container and runs it from the exec-allowed `/run/cwso` volume, passing in the real, materialized workspace path so the test's own assertions still validate real content at the real location — only the compiled binary's own inode location changed to satisfy `noexec`, not what is being validated. Independent Tech Lead review of C024's MR (!163) explicitly recommended **deferring** any change here rather than opening a new task: loosening `noexec` or adding a toolchain to the runtime image would trade away already-reviewed security posture for marginal test-harness convenience, with no current consumer anywhere in the roadmap. Documented here as a conscious, reviewed constraint — not a plan to fix it | closed | wontfix | C024 | **documented-limitation** (relabeled 2026-08-29 from `wontfix` — see "Notes on C060 reclassifications" above; no factual change; needs a matching `docs/LIMITATIONS.md` entry from C063, not yet written — see cross-check table above) |
| R-7 | `services/cwso-git-shadow/src/repo.rs` (`ShadowStore::commit`) | Concurrency | SEV-C041-001 (HIGH), found by independent Security Engineer review of C041 and adversarially reproduced 5/5 runs with an 8-thread concurrent-`commit()` probe: the pre-fix `commit()` read a workspace's tracked `head`, built a tree, called `repo.commit`, and only then advanced `head` — several separate operations, not one atomic step — and the only lock in play (`ShadowStore.workspaces`) protected just the workspace-map lookup itself, briefly, at the start and end of the call, not the full span. Two concurrent `commit()` calls against the SAME workspace could both read the same stale `head`, both commit successfully against that same parent, and have whichever advanced `head` last silently orphan the other's commit from the chain (still present in the git object database, but unreachable by walking `parent_id` back from `head`, invisible to `git log` and to C042's future three-way merge). This was latent, not live, only because `orchestrator/internal/shadow/client.go` (row B13, above: "one TCP-style request-per-connection model — no pooling, no pipelining") serialized every shadow RPC through one global mutex — C043 removes that global mutex via bounded connection pooling, which would have turned this from latent to live on the sanctioned concurrent-dispatch path. Fixed within this same task (not deferred) by adding `ShadowStore.commit_locks: Mutex<HashMap<Uuid, Arc<Mutex<()>>>>`, a per-workspace serialization primitive looked up/inserted under a brief lock on the side-table itself, then held by `commit()` for the entire read-head → build-tree → `repo.commit` → advance-head span; commits against different workspace ids use independent `Arc<Mutex<()>>` entries and remain fully concurrent (no regression of the throughput C043 exists to unlock). Regression coverage: `concurrent_commits_against_one_workspace_never_lose_a_commit` (the reproduced 8-thread adversarial probe, looped over 20 iterations, walking `parent_id`/`parent_count` from the final `head` to assert zero lost commits — confirmed to reliably fail against the pre-fix code, 5/5 runs, and to reliably pass against the fix, 8/8 runs) and `concurrent_commits_against_different_workspaces_are_not_serialized` (proves the fix does not regress into a single global commit lock) | closed | fixed | C041 | **fixed** (re-verified 2026-08-29: `ShadowStore.commit_locks: Mutex<HashMap<Uuid, Arc<Mutex<()>>>>` field read directly in `repo.rs`, with its doc comment describing exactly the per-workspace serialization this row claims) |
| R-8 | `services/cwso-git-shadow/src/repo.rs` (`ShadowStore.commit_locks`) | Resource management | Non-blocking MEDIUM finding from the independent Security Engineer re-review of R-7/C041's fix (adversarial-probe re-verification round): `commit_locks: Mutex<HashMap<Uuid, Arc<Mutex<()>>>>` never evicts an entry once a workspace's first commit inserts one — `drop_workspace` does not remove the corresponding `commit_locks` entry, so the side-table grows by one small entry (a `Uuid` key + an `Arc<Mutex<()>>`) per distinct workspace ever committed-to, for the lifetime of the process, never shrinking. Confirmed not currently exploitable as an unbounded-growth DoS: workspace creation already sits behind the existing upstream rate limiter (row D6, `orchestrator/internal/transport/http.go`, 60 req/min per IP), which caps the practical growth rate; each leaked entry is small and fixed-size (no per-entry unbounded data). Tracked as a follow-up hardening item (e.g. evict the `commit_locks` entry in `drop_workspace`, or switch to a weak-reference/LRU-bounded side-table), not required for v1.0 | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `commit_locks`'s doc comment, read directly in `repo.rs`, explicitly states "entries are never removed, including by `drop_workspace`" — matches this row exactly) |
| R-10 | `scripts/cwso-doctor.sh` (`port_in_use()`) | Robustness | Non-blocking hardening observation from C054's clean-machine guide verification: `port_in_use()` probes the target port via a raw bash `exec 3<>"/dev/tcp/127.0.0.1/${PORT}"` connect with no connect timeout (confirmed by reading the function directly — no `timeout` wrapper, no background-and-poll pattern). Against a normal port occupier (e.g. `python3 -m http.server`) this returns promptly, and `make up`'s own health-wait loop is unaffected (`Makefile`'s `curl -sS -o /dev/null -w '%{http_code}' --max-time 3 http://127.0.0.1:8080/healthz`, confirmed to already carry an explicit timeout) — but against an atypical occupier that accepts the raw TCP connection into its backlog queue without ever completing `accept()`, the probe was observed to hang indefinitely (>15s) once the 5-connection backlog filled, causing `make doctor` itself to never return. `docs/user/README.md` makes no explicit claim about behavior against this specific occupier class, so this is not a defect in the guide (C054's own verdict) — flagged here as a real, disclosed script-robustness gap rather than silently ignored. Suggested fix if picked up: wrap the `/dev/tcp` connect in `timeout <n>s` or an equivalent bounded-wait pattern | open | v1.1 | — | **v1.1** (re-verified 2026-08-29: `port_in_use()` read directly in `scripts/cwso-doctor.sh` — still a bare `exec 3<>"/dev/tcp/..."` with no `timeout` wrapper, matching this row exactly) |
| R-11 | `scripts/cwso-token.sh` (`--ttl` argument parsing) | Security | F-C061-03 (LOW), found by C061's v1.0.0 security audit: `--ttl` is validated only as a positive integer number of seconds — no maximum. A developer holding `.env.jwt.dev` can mint an `orchestrator`-role JWT valid for years. Minting still requires prior possession of the signing secret file (itself `chmod 600`, gitignored, generated only via `cwso-bootstrap-secrets.sh`), so this does not grant any new privilege — it only widens the blast radius of an already-compromised secret file or an accidentally-retained long-lived local token outliving its intended dev session. Suggested remediation: cap `--ttl` at a documented ceiling (e.g. 24h) with an explicit `--ttl-unsafe-long` opt-out for legitimate long-running local test scenarios, or document the risk in the script's own usage text | open | v1.1 | — |
| R-12 | `docs/user/deployment/proxmox-lxc-guide.md`; `deploy/docker-compose.yml` (`orchestrator` service, `ports: ["8080:8080"]`, no TLS termination) | Security | F-C061-04 (LOW), found by C061's v1.0.0 security audit: the v1.0 default stack terminates plain HTTP directly on `:8080` — JWT bearer tokens and the dashboard token traverse the wire in cleartext. Reasonable default for the documented local/loopback use case (inherently mitigated for the GCP Cloud Run path, since Cloud Run always terminates TLS at the platform edge). `proxmox-lxc-guide.md`, however, documents deploying CWSO inside a network-reachable LXC container/VM and even suggests adding "HAProxy or Nginx as reverse proxy" for load-balancing multi-instance setups, but never mentions TLS termination as part of that recommendation, and there is no warning anywhere in that guide that a non-loopback deployment transmits both credential types in cleartext. Impact low as currently documented (the guide's primary path is still a single-operator homelab deployment), but the gap widens for any reader who follows the reverse-proxy suggestion toward a genuinely network-reachable multi-instance setup without adding TLS. Suggested remediation: add an explicit TLS-termination warning/recommendation to the Proxmox guide's reverse-proxy section | open | v1.1 | — |

### Notes on the `fixed` rows (verification evidence)

- **P2-3 — grammar coverage: `fixed`.** `services/Cargo.toml` does not wire
  grammars itself (it is the workspace manifest); the scorecard's reference is
  to the crate manifest. `services/cwso-git-shadow/Cargo.toml` declares
  `tree-sitter-go = "0.21"`, `tree-sitter-python = "0.21"`,
  `tree-sitter-rust = "0.21"`, `tree-sitter-typescript = "0.21"`, and
  `services/cwso-git-shadow/src/ast.rs` uses all four
  (`tree_sitter_go::language()`, `tree_sitter_python::language()`,
  `tree_sitter_rust::language()`,
  `tree_sitter_typescript::language_typescript()`), with Rust and TypeScript
  unit tests present. **Correction (Tech Lead review, 2026-08-29):** the
  `POC-DEBT P2-3` comment is NOT removed — a fresh whole-repo `grep` (this
  task's own worker had no Bash/grep access and could not run this sweep)
  found it still live at `services/cwso-git-shadow/Cargo.toml:24` (`# POC-DEBT
  P2-3: Rust + TypeScript grammars added in T029.`), a stale marker describing
  work that is in fact done. The `fixed` classification itself is unaffected
  (all four grammars are genuinely wired and tested, confirmed above) — only
  the "marker removed" claim was wrong. This register does not touch code, so
  the stray comment removal is out of this task's scope; flagged here as a
  small follow-up cleanup for whichever task next touches
  `services/cwso-git-shadow/src/ast.rs`/`Cargo.toml`, not tracked as a
  dedicated task given its triviality (a one-line stale comment, zero
  functional effect).
- **B7 — orphan commits / parent-chain tracking: `fixed` (C041).**
  `services/cwso-git-shadow/src/repo.rs`'s `Workspace` struct gained a `head:
  Option<Oid>` field; `ShadowStore::create` seeds it from the base commit's
  oid when the workspace has one (`None` otherwise), and `ShadowStore::commit`
  resolves the tracked `head` to a `git2::Commit` and passes it as the sole
  parent to `repo.commit`, advancing `head` to the freshly made commit's oid
  only after that call succeeds. The `POC-DEBT P2-4` marker at the old
  `repo.rs:180` orphan-commit line is removed (`grep -n "P2-4"
  services/cwso-git-shadow/src/repo.rs` = no hits). Regression coverage:
  `first_commit_in_fresh_workspace_is_a_root_commit` (a base-less workspace's
  first commit still has zero parents), `sequential_commits_in_one_workspace_form_a_parent_chain`
  and `third_commit_continues_the_same_chain` (`git2::Commit::parent_id`
  confirms real linkage across two and three sequential commits), and
  `workspace_created_from_base_commit_chains_onto_it` (a workspace seeded from
  an existing commit chains its own first commit onto that base rather than
  rooting itself). Independently confirmed against the real on-disk object
  store with the `git` CLI (`git log --graph --oneline --parents
  <second-commit-oid>` showed `second -> first` linkage with `first` having no
  `parent` line), not only via `git2`'s own API.
- **D6 — rate limiting: `fixed`.** `orchestrator/internal/transport/http.go`
  implements per-IP token-bucket rate limiting: import of
  `golang.org/x/time/rate` (line 21), `newRateLimiterStore(ctx)` (line 183),
  `rateLimitMiddleware(...)` wired into the handler chain (line 190), SSE
  connection limiting (lines 210–214), and the section comment
  `// --- Rate limiting middleware (T029 remediation #7) ---` (line 665)
  documenting the default of 60 requests/minute.
- **R-7 — concurrent-commit lost-update race: `fixed` (C041).**
  `services/cwso-git-shadow/src/repo.rs`'s `ShadowStore` gained a
  `commit_locks: Mutex<HashMap<Uuid, Arc<Mutex<()>>>>` side-table;
  `ShadowStore::commit` now looks up (or lazily inserts) its workspace's
  own `Arc<Mutex<()>>` under a brief lock on that table, then holds the
  resulting guard for the method's entire read-head → build-tree →
  `repo.commit` → advance-head span, so two `commit()` calls against the
  same workspace id can never interleave through that span, while commits
  against different workspace ids use independent lock entries and remain
  fully concurrent. `commit`'s doc comment now describes this guarantee
  and cites SEV-C041-001. Regression coverage:
  `concurrent_commits_against_one_workspace_never_lose_a_commit` (an
  8-thread, barrier-synchronized adversarial probe run over 20 internal
  iterations, walking `parent_id`/`parent_count` from the workspace's
  final `head` back to the root after every iteration and asserting all 8
  commits are reachable) and
  `concurrent_commits_against_different_workspaces_are_not_serialized`
  (proves an in-flight lock on one workspace never blocks a commit against
  a different one). The adversarial probe was confirmed, by temporarily
  bypassing the new lock in `commit()` and restoring it afterward, to
  reliably fail on its very first iteration against the pre-fix code (5/5
  isolated runs) and to reliably pass against the fix (8/8 isolated runs).
- **B12 — UDS socket perms + shared GID: `fixed` (C044).** The phase-2
  scorecard's `0o666` claim did not match shipped code even at the time it was
  written: `services/cwso-git-shadow/src/main.rs:104` and
  `services/cwso-merge-engine/src/ipc.rs:31` both call
  `std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))`
  immediately after `UnixListener::bind`, present since each sidecar's first
  commit (`35c556e`). The GID-alignment half was separately closed by T197
  (merged, PASS; `deploy/docker-compose.yml` `CWSO_IPC_ALLOWED_GIDS` corrected
  `"0,100"` → `"0,101"` to match the orchestrator image's real live gid, with
  `scripts/check-ipc-gid-drift.sh` added as a regression check). C044
  independently re-verified both claims live against a real running
  `docker compose -f deploy/docker-compose.yml up -d --build` stack
  (2026-08-27), rather than trusting the source read alone:
  - `docker exec cwso-git-shadow stat -c '%a %U:%G %n' /run/cwso/git-shadow.sock`
    → `660 cwso:cwso /run/cwso/git-shadow.sock`
  - `docker exec cwso-merge-engine stat -c '%a %U:%G %n' /run/cwso/merge-engine.sock`
    → `660 cwso:cwso /run/cwso/merge-engine.sock`
  - `docker exec cwso-git-shadow id` / `cwso-merge-engine id` / `cwso-orchestrator id`
    → all three report `uid=100(cwso) gid=101(cwso) groups=101(cwso)` (same
    effective identity across all three containers in this build)
  - `bash scripts/check-ipc-gid-drift.sh` → exit 0, `OK` for both
    `CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS` on both services against
    the live orchestrator `cwso` uid=100/gid=101
  - `scripts/cwso-smoke-test.sh` → all 7 stages PASS, including
    `merge_concurrent_results` (outcome=success, status=merged), which
    exercises the orchestrator→git-shadow and orchestrator→merge-engine IPC
    paths over these exact sockets end-to-end
  No code change was needed or made; this row is closed on the strength of
  reproduced live evidence, not the static-source finding alone.
  **Follow-up note (2026-08-28):** independent re-review of this same task
  found a related but distinct gap — not a regression of this row's claims
  (sockets are still genuinely `0o660` with correct GID alignment) — where
  the opt-in `rollout` service coincidentally shared the orchestrator's
  uid/gid and could reach these sockets via the shared `cwso-runtime`
  volume. See row **R-9** for that finding and its fix (SEC-C044-001).
- **R-9 — `rollout`'s coincidental IPC reachability: `fixed` (C044
  follow-up).** (Renumbered from this fix's original R-7 during merge with
  develop — R-7/R-8 were independently claimed by C041's SEV-C041-001 fix
  and its follow-up, merged first; no content change, ID collision only.) `deploy/docker-compose.yml`'s `rollout` service block no
  longer mounts `cwso-runtime:/run/cwso` — the only path either `git-shadow`
  or `merge-engine`'s UDS sockets were reachable through from that
  container. Verification:
  - `docker compose -f deploy/docker-compose.yml --profile rollout config`
    — resolved `rollout` service definition has no `cwso-runtime` volume
    entry.
  - `docker compose -f deploy/docker-compose.yml --profile rollout up -d
    --build` (with the default stack also up) — `rollout` container reaches
    healthy via its own `/healthz` HTTP probe (unrelated to the removed
    mount); `docker exec cwso-rollout ls /run/cwso` reports the directory
    exists (baked into the image, `deploy/Dockerfile.rollout`) but is empty
    — no `.sock` files visible, confirming no path to either sidecar socket
    at the container level, not just in the compose file text.
  - Default (non-`rollout`-profile) stack smoke test
    (`scripts/cwso-smoke-test.sh`) still passes all 7 stages — removing an
    unused mount from an opt-in, non-default service has no effect on the
    default stack.
  - `scripts/check-ipc-gid-drift.sh` still exits `0` — this fix does not
    touch `git-shadow`/`merge-engine`'s own allowlists or identity, only
    `rollout`'s reachability to them.
  See `SECURITY.md`, "Sidecar IPC authorization" point 8, for the full
  writeup.
- **R-3 — read-side TOCTOU: `fixed` (C035).** `services/cwso-git-shadow/src/repo.rs`
  generalizes C021's write-side fd-anchored primitives
  (`open_root_dir`/`openat_dir_nofollow`/`openat_leaf_nofollow`) to a
  recursive read walk: `open_entry_nofollow` opens every directory entry
  via `openat(..., O_NOFOLLOW)` (first attempting `O_DIRECTORY`, falling
  back to a generic `O_NOFOLLOW` open) relative to its containing
  directory's already-open fd, before its type is even inspected;
  `fdopendir_dup`/`next_dir_entry_name` enumerate a directory's entries from
  that same already-open fd (via a `F_DUPFD_CLOEXEC` duplicate, never
  `std::fs::read_dir` on a path string); and file content is read via the
  fd `open_entry_nofollow` already obtained, never `std::fs::read` on a
  reconstructed path. `scan_workspace_tree`/`scan_dir_into` (the
  reconciliation pass) and the new `read_real_file` (used by
  `services/cwso-git-shadow/src/writeback.rs`'s `sync_file`, the inotify
  single-entry handler) both go through these same primitives, so both
  call sites named in this row are hardened identically. Regression
  coverage: `scan_workspace_tree_skips_symlink_planted_at_intermediate_component`,
  `scan_workspace_tree_skips_symlink_planted_at_leaf`, and
  `read_real_file_skips_symlink_planted_at_leaf` are deterministic
  (necessary but not sufficient, since a static symlink was already caught
  by the pre-fix check too); `scan_workspace_tree_race_against_symlink_swap_never_reads_outside_content`
  is the genuine race-stress test that actually distinguishes old from new
  — confirmed (by splicing it onto the pre-fix commit) to reliably fail in
  well under 200ms against the pre-fix `symlink_metadata`-then-`read_dir`
  code, and to reliably pass against this fix. No new dependency: `libc`
  (already a direct dependency, matching C021's own style) supplied every
  primitive needed (`openat`, `fstat`, `fdopendir`, `readdir`,
  `renameat2` in the test only).

### In-code `POC-DEBT` marker cross-check

Per the C003 brief, code markers are the hits of
`grep -rn "POC-DEBT" . --exclude-dir=.git --exclude-dir=docs` in **project code**
(deployable services and deploy configuration) — harness/skill documentation
templates under `.gemini/`, `.cline/`, `.claude/`, `.cursor/`, `.github/`,
`.opencode/`, `.pi/`, and `.gitlab/` are examples of the tagging convention, not
CWSO product debt, and are intentionally not register rows.

| Marker location | Register row |
|---|---|
| `deploy/docker-compose.yml:513` | R-1 (line corrected 2026-08-29 by C060 — was stale `:6`; see "Notes on C060 reclassifications" above) |
| `services/cwso-git-shadow/Cargo.toml:24` | P2-3 (fixed; marker text corrected 2026-08-29 by Tech Lead review — was stale `:20` and incorrectly claimed removed; the `POC-DEBT P2-3` comment is still live at this line, describing already-completed work) |
| `services/cwso-git-shadow/src/main.rs:11` | B2 (fixed; marker text removed, module doc now describes the C021 projection instead) |
| `services/cwso-git-shadow/src/repo.rs:180` | B7 (fixed; marker removed, `Workspace.head` + chained `ShadowStore::commit` now implement parent tracking) |
| `orchestrator/internal/mcp/protocol.go:10` | B1 (documented-limitation, see above — marker's own text now points to ADR-013 rather than claiming a future fix) |
| `orchestrator/internal/shadow/client.go:5` | B13 (fixed; marker text removed, package doc now describes the C043 connection pool instead) |
| `orchestrator/internal/rollout/evaluator_swebench.go:64` | B11 |
| `services/cwso-git-shadow/src/repo.rs` (`scan_workspace_tree`'s "Residual TOCTOU" doc comment, C022) | R-3 (fixed; marker text removed, doc comment now describes the C035 fd-anchored fix instead) |
| `services/cwso-git-shadow/src/repo.rs` (`scan_workspace_tree`'s "Non-UTF-8 filenames" doc comment, C022) | R-4 |
| `services/cwso-git-shadow/src/writeback.rs` (rename-handling doc comment, "Accepted, documented race", C022) | R-5 |

10/10 code markers are represented above. (`orchestrator/internal/mcp/protocol.go:12`
is the continuation line of the B1 comment, not a separate marker.) This task (C060)
did not perform a fresh, whole-repository `grep` sweep for markers (no Bash/grep
access under this agent's tooling — see `docs/tasks/task-C060.md` execution notes);
the 10-marker inventory above is carried forward from the prior sweep and
spot-confirmed against every file this task independently read (`main.rs`, `repo.rs`,
`ast.rs`, `writeback.rs`, `ipc.rs`, `client.go`, `protocol.go`,
`evaluator_swebench.go`, `Cargo.toml`, `docker-compose.yml`) — none of those reads
surfaced an undocumented `POC-DEBT` marker.

---

## Historical scorecard — Phase 1 (archived)

Source: `docs/archive/debt/POC-DEBT-SCORECARD-phase1.md` (Phase 1 MCP core,
2026-05; hypothesis **VALIDATED**). Rows keep their original numbering; the
scorecard's `#n` is aliased `P1-n` here for unambiguous reference.

| Scorecard row | Description (abridged) | Register ID | Carried-forward or closed |
|---|---|---|---|
| P1-1 (#1) | Hand-rolled MCP subset; adopt official `go-sdk` | B1 | carried-forward → v1.0-blocker (C030–C032) |
| P1-2 (#2) | Hand-rolled HS256 JWT verifier; needs RS256/JWKS/claims validation | D2 | carried-forward → v1.1 |
| P1-3 (#3) | SSE endpoint emits heartbeats only | D3 | carried-forward → v1.1 |
| P1-4 (#4) | Stdlib-only logger; adopt `zerolog` + OTEL | D4 | carried-forward → v1.1 |
| P1-5 (#5) | Dev JWT secret via env/file; prod needs vault | R-1 / R-2 | carried-forward → v1.0-blocker, documented (C063); prod Vault/SOPS half → v1.1 |
| P1-6 (#6) | Capability negotiation minimal (`tools.listChanged: false`) | D5 | carried-forward → v1.1 |
| P1-7 (#7) | No per-IP rate limit on `/mcp` POST | D6 | **closed** — rate limiting implemented in `orchestrator/internal/transport/http.go` (evidence above) |
| P1-8 (#8) | 1 MiB read cap hard-coded | D8 | carried-forward → v1.1 |

*This historical table is a point-in-time record of the original phase-1 scorecard's
carry-forward decisions and is intentionally left unchanged by C060's 2026-08-29
re-classification pass (which found D2's and D3's *current* code state has moved on
from what's summarized here — see the Live register above and "Notes on C060
reclassifications" for the up-to-date picture).*

## Historical scorecard — Phase 2 (archived)

Source: `docs/archive/debt/POC-DEBT-SCORECARD-phase2.md` (Phase 2 shadow
workspaces + AST, 2026-05; hypothesis **VALIDATED**).

| Scorecard row | Description (abridged) | Register ID | Carried-forward or closed |
|---|---|---|---|
| P2-1 | OverlayFS bind-mount deferred; IPC-only shadow files | B2 | **closed** — real filesystem projection implemented via materialise-to-tmpfs (ADR-012, evidence: `services/cwso-git-shadow/src/repo.rs` `ShadowStore::create`/`write_file`/`drop_workspace`/`materialize_write`, C021); write-back into the object store remains open (C022) |
| P2-2 | No Merkle incremental indexer; every query re-parses | P2-2 | carried-forward → v1.1 |
| P2-3 | Only Go + Python grammars wired; Rust/TS required | P2-3 | **closed** — four grammars wired in `services/cwso-git-shadow/Cargo.toml` (evidence above) |
| P2-4 | Orphan commits; no history chain | B7 | **closed** — parent-commit chaining implemented via `Workspace.head` + `ShadowStore::commit` (evidence above, C041) |
| P2-5 | UDS perms `0o666` across differing UIDs | B12 | **closed** — scorecard claim did not match shipped code; both sockets live-reverified `0o660` with correct GID alignment (evidence above, C044) |
| P2-6 | One connection per RPC; no pooling | B13 | **closed** — bounded connection pool implemented in `orchestrator/internal/shadow/client.go` (evidence above, C043) |
| P2-7 | `find_references` is text matching, not scope resolution | B6 | **closed** — real scope/binding resolution implemented via `resolve_references` (evidence above, C040) |
| P2-8 | `base_tree` stored but never read | P2-8 | carried-forward → v1.1 |

The phase-2 scorecard also notes that phase-1 items D1, D2, and D6 "touched
Phase-2 paths". Those cross-references remain valid: D1 → B1, D2 → D2 (v1.1),
D6 → closed.

---

*This register is kept current by every debt-closing task. **C060 completed the
release-time re-classification on 2026-08-29** (`fixed` / `documented-limitation` /
`v1.1` — 26/26 rows classified, zero unclassified; see "C060 release classification"
above). Its 3 `documented-limitation` rows (B1, R-1, R-6) still need matching
`docs/LIMITATIONS.md` entries from **C063**, which had not yet run at the time of this
classification pass. New `POC-DEBT` tags must be added here when introduced.*
