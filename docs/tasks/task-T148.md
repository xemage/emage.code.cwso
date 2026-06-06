# Task T148 — Evaluator registry + SWE-bench hook

- **Status:** pending
- **Owner:** backend-developer / qa-engineer
- **Priority:** P2
- **Depends on:** T146, T144
- **Based on:** Polar §3.5

## Objective

Registry-backed evaluators run after trajectory construction: session reward, test-on-output,
and SWE-bench/SWE-Gym patch scoring in a fresh runtime.

## Acceptance Criteria

- [ ] Evaluator plugin interface + built-in session reward
- [ ] SWE-bench harness evaluator PoC (single instance)
- [ ] Rewards attach to trajectory traces per Polar propagation rules
