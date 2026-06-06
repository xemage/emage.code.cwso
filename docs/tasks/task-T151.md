# Task T151 — Offline SFT data generation mode

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P2
- **Depends on:** T134, T144
- **Based on:** Polar §4.2

## Objective

Repurpose rollout infrastructure for fixed-checkpoint offline trace generation: fan-out harness
sessions, journal to disk, filter/post-process for SFT datasets.

## Acceptance Criteria

- [ ] CLI or REST mode for batch offline generation (no trainer callback)
- [ ] Output compatible with Parquet store layout
- [ ] Documented workflow in user guide v2
