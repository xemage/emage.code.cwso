# Task T150 — KV differential prompting (proxy hot path)

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P2
- **Depends on:** T135, T147
- **Based on:** `rollout-architecture-v1.md` §6 step 3

## Objective

On cache hit, strip cached prefix tokens from upstream prompts (differential prompting) using
T135 prefix keys; integrate with HAL/vLLM `cache_salt`.

## Acceptance Criteria

- [ ] Proxy removes prefix token span when prefix_key matches LRU entry
- [ ] Integration test with mock upstream verifying shortened prompt
- [ ] Feature-flagged; default off
