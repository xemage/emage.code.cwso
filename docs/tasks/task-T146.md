# Task T146 — Gateway async staging + partial trace recovery

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T132, T144
- **Based on:** Polar §3.3 INIT/RUNNING/POSTRUN pools, timeout partial traces

## Objective

Implement gateway worker pools (INIT, READY buffer, RUNNING, POSTRUN) so runtime prep and
evaluation do not block GPU-bound harness execution; recover partial trajectories on timeout.

## Acceptance Criteria

- [ ] Stage-isolated worker pools in cwso-rollout or orchestrator gateway layer
- [ ] Evaluator prewarm begins during agent run when configured
- [ ] Timeout still emits POSTRUN with partial captures + terminal status
