# Task T145 — Rollout `num_samples` session fan-out

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T137
- **Based on:** Polar §3.1 TaskRequest → num_samples sessions

## Objective

Expand `POST /rollout/task/submit` to accept `num_samples` and schedule independent sessions
with shared task spec, distinct session IDs, and aggregated task status.

## Acceptance Criteria

- [x] Schema + API accept `num_samples` (default 1, max 32)
- [x] Per-session callbacks and terminal result persistence
- [x] Integration test: submit N=3, poll until all complete

## Notes

Single-sample tasks preserve backward-compatible `session_id == task_id` reward tagging.
