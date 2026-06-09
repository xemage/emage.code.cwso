# Task T156 — Comprehensive installation & usage guide (v0.4.0)

- **Status:** in_review
- **Owner:** technical-writer
- **Priority:** P0
- **Depends on:** T142, T146, T148, T154
- **Based on:** `installation-v1.md`, post-GA Polar features on `develop`

## Objective

Produce a **detailed** operator and integrator guide covering full v0.4.0 capabilities:
architecture, configuration reference, end-to-end workflows (MCP, rollout, gateway, evaluators),
IDE integration, and troubleshooting — suitable as the primary adoption document for v0.4.0 GA.

## Acceptance Criteria

- [x] `docs/user/installation-v2.md` — complete guide (not a stub)
- [x] Covers T146 gateway staging, T148 evaluator registry, T145 num_samples, harness adapters
- [x] Configuration tables for all `CWSO_*` rollout/orchestrator flags used in v0.4.0
- [x] Linked from README and `installation-v1.md`
- [ ] Reviewed against live `develop` after T149–T151 land (conditional update before GA tag)

## Notes

`installation-v1.md` remains for v0.3.0 reference; v2 is canonical for v0.4.0 adopters.
