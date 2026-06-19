# Task T155 — Enable-all-features script

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T142
- **Based on:** `orchestrator/internal/config/config.go`, `installation-v1.md` §6

## Objective

Provide a single script that exports all Phase 6–9 feature flags (safe local defaults)
and prints the matching Docker Compose command for phase2+phase4 stacks.

## Acceptance Criteria

- [x] `scripts/cwso-enable-all-features.sh` — idempotent, sources env file
- [x] `scripts/cwso-enable-all-features.env.example` — documented variables
- [x] Linked from installation guide
- [ ] Sidecar compose extension documented (HAL/sparse/rollout run separately today)

## Notes

Orchestrator flags only; Rust sidecars (hal, sparse, rollout) require additional
containers — script documents socket paths for when sidecars are attached.
