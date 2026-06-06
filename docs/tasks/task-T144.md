# Task T144 — Polar harness adapters + runtime launcher

- **Status:** pending
- **Owner:** backend-developer / integration-agent
- **Priority:** P1
- **Depends on:** T137, T142
- **Based on:** `polar-gap-analysis-v1.md`, Polar §3.2.1–3.2.2

## Objective

Launch external coding harnesses (codex, claude_code, qwen_code, shell-command) unchanged
by pointing their model `base_url` at cwso-rollout and providing runtime start/stop/exec
via a Polar-style runtime interface (Docker first).

## Acceptance Criteria

- [ ] Harness adapter registry with config + launch commands
- [ ] Runtime interface: start, stop, exec, upload, download
- [ ] At least one reference harness (shell-command) e2e against proxy capture
- [ ] Documented in installation guide v2 section
