# Task T144 — Polar harness adapters + runtime launcher

- **Status:** done
- **Owner:** backend-developer / integration-agent
- **Priority:** P1
- **Depends on:** T137, T142
- **Based on:** `polar-gap-analysis-v1.md`, Polar §3.2.1–3.2.2

## Objective

Launch external coding harnesses unchanged via cwso-rollout proxy and Docker runtime interface.

## Acceptance Criteria

- [x] Harness adapter registry with config + launch commands
- [x] Runtime interface: start, stop, exec, upload, download
- [x] At least one reference harness (shell-command) e2e against proxy capture
- [x] Documented in installation guide section

## Notes

Merged @ `50f3406` (MR !56). Stub adapters for codex/claude_code/qwen_code; shell-command is reference.
