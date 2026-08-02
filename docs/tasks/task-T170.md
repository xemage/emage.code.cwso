# Task T170 — Implement and verify fix for confirmed rollout defect(s)

**ID:** T170
**Owner:** backend-developer
**Status:** done
**Priority:** P1
**Depends on:** T169
**Created:** 2026-07-31
**Completed:** 2026-08-01
**Based on:** docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md

## Objective
Implement a fix for whichever root cause(s) T169 confirms (healthcheck method/path mismatch,
trajectory store path wiring, both, or a different cause found during investigation), scoped
strictly to what T169's findings actually support — do not implement a fix for a candidate cause
T169 refutes. Then verify the fix by rebuilding the `rollout` image and re-running the exact
verification sequence from the originating defect report against `deploy/docker-compose-t226.yml`
(or an equivalent local compose invocation of this repo's own `deploy/Dockerfile.rollout`),
confirming the container reaches and sustains `(healthy)` status and the trajectory store writer
starts without error.

## Inputs
- `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` (T169 output — confirmed cause(s) and
  recommended fix direction)
- `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md` (original evidence
  and exact verification commands to re-run)
- `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md` (Scope, Risks & mitigations)
- `deploy/Dockerfile.rollout`

## Expected outputs
- Source code changes in `cwso-rollout` implementing only the fix(es) T169 confirmed as root
  cause, each change referencing the corresponding finding in `root-cause-analysis-cwso-rollout-v1.md`.
- `docs/artifacts/fix-verification-cwso-rollout-v1.md` containing:
  - The exact rebuild/verification commands run.
  - `docker inspect ... .State.Health` output showing sustained `healthy` status (at least 5
    consecutive successful probes, no `FailingStreak` growth).
  - Trajectory store writer log output showing successful startup (no `"error":"create rollout
    store ..."` line).
  - Confirmation that any `/v1/models` existing callers/tests enumerated by T169 still pass, if
    the fix touched that endpoint's method contract.

## Acceptance criteria
1. Only the root cause(s) confirmed by T169 are fixed — no speculative changes to code paths
   T169 did not implicate.
2. Rebuilding `cwso-rollout` and running it via the documented compose healthcheck produces
   `(healthy)` status sustained across at least 5 consecutive probes.
3. Trajectory store writer logs show successful startup with no path-creation error, and writes
   land at the path configured via `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`.
4. If the fix changes `/v1/models`'s method contract, all existing callers/tests enumerated by
   T169 are confirmed still passing (or updated, with that update called out explicitly).
5. `docs/artifacts/fix-verification-cwso-rollout-v1.md` is produced with verbatim command output
   as evidence — no unverified completion claims.
6. This task does not schedule itself or its outputs into `docs/tasks/active-tasks.md` — merging
   and release scheduling remain CWSO maintainer decisions taken after normal review.

## Blocker protocol
Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Execution notes
Completed 2026-08-01. Output: `docs/artifacts/fix-verification-cwso-rollout-v1.md`.

Fix scoped strictly to T169's two confirmed/refined findings:
- `services/cwso-rollout/src/proxy.rs` — added `GET /healthz` liveness route ahead of the global
  POST-only gate, returns `200 {"status":"ok"}`, no upstream/provider dispatch. `/v1/models`
  left untouched (still 405 GET / 404 POST), per T169's recommendation and confirmed no-caller
  finding. Added regression test `healthz_returns_200_and_v1_models_is_unchanged`.
- `services/cwso-rollout/src/store.rs` — `StoreConfig::from_env` now checks
  `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first (backward-compatible with the known external
  emage.code consumer and this repo's own Dockerfile default), falling back to the canonical
  `CWSO_ROLLOUT_STORE_PATH`, then `./rollout_store`. Added regression test
  `from_env_prefers_trajectory_alias_then_canonical_then_default`.
- `deploy/Dockerfile.rollout` — added a `HEALTHCHECK` instruction targeting `/healthz` (the image
  previously had none), so the image owns its own healthcheck contract instead of depending on a
  consuming project's compose file to probe a nonexistent route.

Verification (real, not simulated): `cargo build -p cwso-rollout` and `cargo test -p cwso-rollout`
(35/35 pass, including both new tests); real `docker build -f deploy/Dockerfile.rollout` and
`docker run` against the same env vars/mounts as the original failing scenario — sustained
`(healthy)` / `FailingStreak:0` across 5/5 consecutive probes (verified two ways: explicit
`--health-cmd` override and the Dockerfile-native `HEALTHCHECK` alone); trajectory store
directory created cleanly with no `"error":"create rollout store ..."` line;
`/v1/models` confirmed unchanged (405, identical body). One honest caveat: no live
`/v1/chat/completions` traffic was sent through the proxy in this smoke test, so no `.parquet`
data file was produced — directory creation (the actual T169 root cause) was verified directly;
the write mechanism itself is separately covered by pre-existing passing unit tests
(`store::tests::parquet_round_trip_preserves_records`,
`store::tests::writer_thread_flushes_batches_without_blocking_proxy`).

`docs/tasks/active-tasks.md` was not modified by this task. Test containers and scratch dirs were
cleaned up; no stray Docker resources left running.
