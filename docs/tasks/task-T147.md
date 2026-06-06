# Task T147 — OpenAI Responses API + proxy hardening

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T132
- **Based on:** Polar §3.2 (OpenAI Responses route), gap analysis

## Objective

Add OpenAI Responses API detection/normalize/denormalize path to cwso-rollout proxy;
harden synthetic SSE and logprob capture for all four provider families.

## Acceptance Criteria

- [ ] `/v1/responses` (or equivalent) routed and captured
- [ ] Unit tests for normalize + capture fields
- [ ] Documented provider matrix in rollout architecture v2
