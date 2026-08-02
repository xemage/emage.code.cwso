# Task T169 — Root-cause investigation: rollout healthcheck 405 and trajectory store path mismatch

**ID:** T169
**Owner:** backend-developer
**Status:** done
**Priority:** P1
**Depends on:** —
**Created:** 2026-07-31
**Completed:** 2026-08-01
**Based on:** docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md

## Objective
Determine the actual, source-verified root cause(s) of two defects observed in the `rollout`
service during an independent Docker Compose build/run by a consuming project (emage.code): (1)
the documented healthcheck `curl -f http://127.0.0.1:8787/v1/models` fails every probe with HTTP
405, so the container never becomes `healthy`; and (2) the trajectory store writer logs
`"error":"create rollout store \"./rollout_store\""` and exits at startup despite
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store` being set and mounted as a writable
volume. Read the relevant `cwso-rollout` Rust source (the `/v1/models` HTTP handler and the
trajectory store writer's path resolution/config wiring) and confirm, refute, or refine the two
candidate causes already logged in the originating evidence report. Do not implement a fix in
this task — investigation and findings only.

## Inputs
- `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md` (full evidence:
  commands, logs, `docker inspect` output, environment)
- `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md` (Goal, Scope, Risks)
- `deploy/Dockerfile.rollout` (build context/entrypoint for the rollout binary)
- `cwso-rollout` source tree (locate via the crate/binary referenced in `deploy/Dockerfile.rollout`)

## Expected outputs
- `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` containing:
  - For the healthcheck issue: which handler serves `/v1/models`, what HTTP methods it actually
    accepts, why a bare GET returns 405, and whether other callers/tests currently depend on its
    current method contract.
  - For the trajectory store issue: where `./rollout_store` is set (hardcoded literal vs.
    resolved from config), how `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` is meant to be wired in, and
    at what point the wiring is missing or overridden.
  - An explicit statement of whether each of the two originally-logged candidate causes is
    CONFIRMED, REFUTED, or NEEDS-REFINEMENT, with source file/line references as evidence.
  - A recommended fix direction for each confirmed issue (not the implementation itself — that is
    T170's job), including any compatibility risks identified (e.g. existing callers of
    `/v1/models` with the current method contract).

## Acceptance criteria
1. Both candidate causes from `emagecode-integration-defect-cwso-rollout-unhealthy-v1.md` are
   explicitly addressed with source-grounded evidence (file path + line reference), not
   re-asserted from the log alone.
2. If either candidate is refuted, the actual cause is documented with the same level of
   source-grounded evidence.
3. `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` is produced and contains no invented
   or unverified claims — every root-cause statement cites a specific source location.
4. Existing callers/tests of `/v1/models` (if any) are enumerated, so T170 knows whether a method
   contract change is safe.
5. No source code is modified in this task — investigation and reporting only.

## Blocker protocol
Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Execution notes
Completed 2026-08-01. Output: `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md`.

Verdicts:
- Issue 1 (healthcheck 405): NEEDS-REFINEMENT. Confirmed the 405 mechanism (global POST-only
  gate, `services/cwso-rollout/src/proxy.rs:46-51`, runs before path parsing), but refined the
  framing — `/v1/models` is not a route under any method (`provider.rs:41-54`'s
  `detect_provider()` has no branch for it; would 404 even under POST). No liveness/health route
  exists in the crate at all. No existing callers/tests depend on `/v1/models`'s current
  behavior (repo-wide grep, zero hits). Recommended: add a dedicated `GET /healthz` route rather
  than repurposing `/v1/models`.
- Issue 2 (trajectory store path): CONFIRMED. `services/cwso-rollout/src/store.rs:46` reads
  `CWSO_ROLLOUT_STORE_PATH` (matching the source's own architecture doc,
  `docs/artifacts/rollout-architecture-v1.md:194`), never `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`,
  which this repo's own `deploy/Dockerfile.rollout:27` sets as an image-level default — a
  name-drift bug in the Dockerfile, not a hardcoded-literal or ignored-env-var bug. No other
  in-repo consumer of either name found.

No source code was modified during this task. Reviewed and accepted by orchestrator before
T170 was allowed to proceed.
