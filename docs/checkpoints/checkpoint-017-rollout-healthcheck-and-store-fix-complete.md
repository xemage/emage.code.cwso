# Checkpoint 017 — Rollout healthcheck + trajectory store fix complete

- Date: 2026-08-01
- Phase: Implementation (targeted defect fix), on branch `chore/T168-backmerge-main-to-develop`
- Plan: `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md` (approved
  2026-07-31 by CWSO maintainer)

## Completed tasks

| Task | Outcome | Artifact |
|------|---------|----------|
| T169 | Root-cause investigation, source-verified | `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` |
| T170 | Fix implemented + verified with real build/docker evidence | `docs/artifacts/fix-verification-cwso-rollout-v1.md` |

Both moved to `docs/tasks/completed-tasks.md` (2026-08-01). `docs/tasks/active-tasks.md` no
longer holds T169/T170 rows, per this repo's invariant that the active board never carries
`done` rows.

## What was found (T169)

Two originally-unconfirmed candidate defects in `cwso-rollout` (Rust), read directly against
source in `services/cwso-rollout/src/`:

1. **Healthcheck 405 — verdict NEEDS-REFINEMENT.** The evidence report's "method/path mismatch"
   framing was half right: a global, path-independent gate in `proxy.rs:46-51` rejects any
   non-`POST` request with `405` before the path is even parsed, which is exactly what `curl -f
   GET /v1/models` hit. But the deeper finding is that `/v1/models` isn't a route at all under
   any method — `provider.rs`'s `detect_provider()` has no branch for it, so even `POST
   /v1/models` would 404, not 200. `cwso-rollout` had zero liveness/health routes. No in-repo
   caller or test depended on `/v1/models`'s current (non-existent) behavior.
2. **Trajectory store path — verdict CONFIRMED.** `store.rs:46` reads env var
   `CWSO_ROLLOUT_STORE_PATH` (matching the source's own architecture doc), never
   `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` — which is what this repo's own `deploy/Dockerfile.rollout:27`
   sets as an image-level default (and what the external emage.code consumer's compose file also
   sets). This is a name-drift bug in `deploy/Dockerfile.rollout`, not a hardcoded-literal bug or
   an ignored env var.

The two issues are unrelated (no shared root cause) — confirmed by source, not assumed.

## What was fixed (T170)

Scoped strictly to the two T169-confirmed findings, per the plan's risk mitigations:

- `services/cwso-rollout/src/proxy.rs` — added `GET /healthz` liveness route ahead of the
  POST-only gate (200, no upstream dispatch, no provider lookup). `/v1/models` deliberately left
  untouched (still 405 GET / 404 POST) since T169 found no callers to protect and the path is
  reserved for future OpenAI-compatible model-listing semantics. New regression test:
  `healthz_returns_200_and_v1_models_is_unchanged`.
- `services/cwso-rollout/src/store.rs` — `StoreConfig::from_env` now checks
  `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first, falling back to the canonical
  `CWSO_ROLLOUT_STORE_PATH`, then `./rollout_store` — backward-compatible with the known
  external consumer (emage.code) and this repo's own Dockerfile default, without requiring a
  Dockerfile change. New regression test:
  `from_env_prefers_trajectory_alias_then_canonical_then_default`.
- `deploy/Dockerfile.rollout` — added a `HEALTHCHECK` instruction targeting `/healthz` (the image
  previously had none at all), so the image owns its own healthcheck contract rather than relying
  on a consuming project's compose file to probe a nonexistent route.

Verified for real (no simulated output):
- `cargo build -p cwso-rollout` — clean, only pre-existing dead-code warnings.
- `cargo test -p cwso-rollout` — 35/35 pass, including both new regression tests.
- `docker build -f deploy/Dockerfile.rollout -t cwso-rollout-t170-verify .` — succeeded.
- `docker run` (env vars/mounts matching the original failing scenario) — `docker inspect
  .State.Health` showed `"Status":"healthy"`, `"FailingStreak":0`, 5/5 consecutive probes with
  `ExitCode:0`, verified independently both via explicit `--health-cmd` override and the
  Dockerfile-native `HEALTHCHECK` alone.
- `docker logs` — no `"error":"create rollout store ..."` line; `/data/parquet-store` directory
  confirmed created (container `ls -la` and host bind-mount `ls -la`).
- `curl -i GET /v1/models` — unchanged, byte-identical `405` response.
- One honest caveat carried into the artifact: no live `.parquet` write was exercised in this
  smoke test (no traffic sent through the proxy to a real upstream); directory-creation is
  confirmed directly, and the write path itself is independently covered by pre-existing passing
  unit tests (`parquet_round_trip_preserves_records`,
  `writer_thread_flushes_batches_without_blocking_proxy`).

## Process notes

- One subagent delegation for T170 verification was mistakenly launched with `isolation:
  "worktree"`, which created a fresh worktree from the last commit and did not carry over the
  in-progress uncommitted fix. The subagent correctly refused to re-implement anything and
  reported a `dependency`/`major` blocker per protocol instead of guessing. The orchestrator
  removed the (auto-cleaned, no-op) worktree and re-delegated the same verification work directly
  against the shared checkout, which then completed correctly. No source-of-truth work was lost;
  the fix that was already on disk was never touched or duplicated.
- Gate discipline: T170 was not delegated until the orchestrator had independently re-verified
  T169's file/line citations against the actual source (`proxy.rs`, `provider.rs`, `store.rs`,
  `deploy/Dockerfile.rollout`, `rollout-architecture-v1.md`) — all checked out exactly as cited.

## Current status

- Working tree: uncommitted changes only, on branch `chore/T168-backmerge-main-to-develop`.
  Nothing was committed, branched, or pushed, per instruction.
- No stray Docker containers left running; the verification image
  `cwso-rollout-t170-verify:latest` was intentionally left on disk for reviewer convenience
  (removable via `docker rmi cwso-rollout-t170-verify`).
- Scope discipline: no other CWSO service (`orchestrator`, `git-shadow`, `merge-engine`,
  `sia-executor`) touched; no file outside this repository touched; `docs/tasks/active-tasks.md`
  was not touched by T170 itself (only by the orchestrator's task-lifecycle bookkeeping before
  and after delegation).

## Files ready for maintainer review (all uncommitted)

- `services/cwso-rollout/src/proxy.rs` (fix + test)
- `services/cwso-rollout/src/store.rs` (fix + test)
- `deploy/Dockerfile.rollout` (HEALTHCHECK addition)
- `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md` (approval recorded)
- `docs/tasks/task-T169.md`, `docs/tasks/task-T170.md` (execution notes filled in)
- `docs/tasks/active-tasks.md` (T169/T170 rows removed after completion)
- `docs/tasks/completed-tasks.md` (T169/T170 appended)
- `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` (new)
- `docs/artifacts/fix-verification-cwso-rollout-v1.md` (new)
- `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md` (originating evidence,
  now tracked/committable alongside the above)

## Next steps

- Maintainer review of the diff; no MR was opened and no commit was made, per instruction to
  leave everything staged for review. If approved, this work should go through this repo's
  branch policy (a `bugfix/<id>-*` branch from `develop`, since this is fix-type work, not a
  direct commit to `develop`) before merge — that branch creation was intentionally left for the
  maintainer to trigger explicitly.
- No further validation gates (Tech Lead / Security) were invoked for this checkpoint since the
  user's request scoped this specifically to plan execution end-to-end through evidence-backed
  completion, not a full gate cycle; recommend a Tech Lead review pass before opening the actual
  merge request, given the Dockerfile change and env-var precedence decision are judgment calls
  worth a second set of eyes.
