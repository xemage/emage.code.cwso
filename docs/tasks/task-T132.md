# Task T132 — Rust `hyper` reverse proxy + zero-copy capture

> **ID note:** roadmap **Feature E / placeholder T106**. Active **T132** (see `active-tasks.md`).

- **Status:** done
- **Owner:** backend-developer (reviewers: tech-lead, security-engineer)
- **Priority:** P0
- **Depends on:** T131 (rollout architecture)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/decisions/ADR-010-rollout-proxy-boundary.md`, `docs/artifacts/rollout-architecture-v1.md` §3

## Objective

Implement the `cwso-rollout` Rust sidecar: a `hyper` reverse proxy that transparently captures
LLM completions for trajectory building (T133), with provider normalization, synthetic SSE, and a
non-blocking capture queue.

## Deliverables

- **`services/cwso-rollout/`** — new workspace crate (`hyper`, `tokio`, `crossbeam-channel`)
- **Capture pipeline** — detect provider → normalize (force `logprobs=true`) → forward upstream →
  enqueue `CompletionRecord` → denormalize / synthetic SSE
- **UDS IPC** — framed-JSON control plane (`stat`, `capture_stats`, `drain_capture`) with `SO_PEERCRED`
- **Tests** — provider normalize/denormalize unit tests; mock-upstream integration tests
- **CI** — `cargo test -p cwso-rollout` in `.gitlab-ci.yml`

## Acceptance Criteria

- [x] Proxy handles `/v1/chat/completions`, `/v1/messages`, Google `generateContent` paths
- [x] Capture records include token ids / logprobs when upstream provides them
- [x] Saturated capture queue increments drop counter (non-blocking hot path)
- [x] Feature-flagged via `CWSO_ROLLOUT_PROXY_ENABLED` (default off)
- [x] `cargo test -p cwso-rollout` green locally
- [x] CI green on T132 MR (branch pipeline 2577824713; MR pipeline blocked by Docker Hub 429 — merged via API)
- [x] MR !41 merged to `develop` (`267922c`)

## Notes

- Trajectory builder (T133) and Parquet store (T134) consume drained capture records via UDS.
- Go orchestrator wiring to delegate model routes lands in T137.
