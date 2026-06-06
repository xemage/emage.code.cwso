# Task T145 — Rollout `num_samples` session fan-out

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T137
- **Based on:** Polar §3.1 TaskRequest → num_samples sessions

## Objective

Expand `POST /rollout/task/submit` to accept `num_samples` and schedule independent sessions
with shared task spec, distinct session IDs, and aggregated task status.

## Acceptance Criteria

- [ ] Schema + API accept `num_samples` (default 1)
- [ ] Per-session callbacks and terminal result persistence
- [ ] Integration test: submit N=3, poll until all complete
