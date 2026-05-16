# Task T054 — CI gate: merge-engine unit tests required

- Phase: **4 (Quality Follow-up)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T050 · Blocks: none
- Status: pending

## Objective
Add required CI test coverage for `cwso-merge-engine` unit tests so merge-engine classification and protocol regressions are caught in the test stage.

## Inputs
- [task-T050.md](./task-T050.md)
- [completed-tasks.md](./completed-tasks.md)
- [.gitlab-ci.yml](../../.gitlab-ci.yml)
- [services/Cargo.toml](../../services/Cargo.toml)

## Constraints
- Keep existing pipeline behavior stable.
- Ensure merge-engine tests are required (non-optional) in CI test stage.
- Keep job runtime reasonable.

## Expected outputs
- CI update adding merge-engine unit tests to required test execution.
- Evidence of local/CI-equivalent successful run.

## Acceptance criteria
1. CI pipeline executes `cargo test -p cwso-merge-engine` (or equivalent required scope).
2. Job fails pipeline on merge-engine test failure.
3. Existing Rust/Go test jobs remain green.

## Blocker protocol
If CI runtime impact is unacceptable, report measured impact and propose split strategy with required gates preserved.
