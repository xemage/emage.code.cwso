# Task T147 — OpenAI Responses API + proxy hardening

- **Status:** done
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T132
- **Based on:** Polar §3.2 (OpenAI Responses route), gap analysis

## Objective

Add OpenAI Responses API detection/normalize/denormalize path to cwso-rollout proxy;
harden synthetic SSE and logprob capture for all four provider families.

## Acceptance Criteria

- [x] `/v1/responses` (or equivalent) routed and captured
- [x] Unit tests for normalize + capture fields
- [x] Documented provider matrix in rollout architecture v2
