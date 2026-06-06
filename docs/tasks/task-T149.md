# Task T149 — Trajectory builder parity (per-request + full prefix merge)

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P2
- **Depends on:** T133
- **Based on:** Polar §3.4.1–3.4.2

## Objective

Add `per_request` builder strategy and upgrade prefix merging with message-level grouping,
EOT interstitial handling, and chain partitioning for sub-agents/compaction boundaries.

## Acceptance Criteria

- [ ] Configurable builder strategy per rollout task
- [ ] Golden tests against Polar Fig. 4 scenarios
- [ ] Prefix merge reduces trainer sample count vs per-request on fixture sessions
