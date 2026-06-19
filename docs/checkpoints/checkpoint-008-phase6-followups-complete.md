# Checkpoint 008 — Phase 6 Follow-ups Complete (Gate T089 actions closed)

## Summary
All five non-blocking follow-ups from the Phase 6 validation gate (T089) are **implemented,
CI-green, and merged to `develop`**, plus one additional CI-hygiene chore. The Heterogeneous
Hardware Dispatcher (Feature A) is now hardened: job context is cancellable through the HAL,
hardware-aware jobs return a retrievable completion summary, accelerator health is actively
probed and live in capability snapshots, remote accelerator endpoints are TLS-enforced, and
the dependency-audit stage runs clean.

## Completed tasks (this checkpoint)
| ID | Title | Owner | MR | Outcome |
|----|-------|-------|----|---------|
| T090 | Thread job context into `hal.Client.Infer` (cancellation) | backend-developer | !20 | `Call`/`Infer` take `ctx`; cancel/deadline closes conn, returns ctx error |
| T092 | Hardware-aware job result retrieval | backend-developer | !20 | `jobs.RunResult` variant → `Job.Result`; compact completion summary on job-state |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | !20 | New `audit` stage, non-blocking during PoC |
| T091 | Active HAL health probing → live `health_state` | backend-developer | !21 | `probe()` + cached health; background prober + reactive infer; `queue_depth` plumbed (0) |
| T093 | TLS for non-loopback HAL accelerator endpoints | devops-engineer | !21 | `security::validate_endpoint`; rejects plaintext-http to remote; documented in `SECURITY.md` |
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | !22 | go.mod + CI + Dockerfile → Go 1.25; `govulncheck` clean |

## CI / verification
- MRs **!20, !21, !22** each merged with a **fully green** pipeline
  (lint, 3 builds, go:test, rust:test, go:audit, rust:audit, e2e:phase2, e2e:phase4-swarm).
- After T114, **`go:audit` is green** (was non-blocking-red): empirically, go1.23.12 → 18
  stdlib advisories, go1.24.13 → 7, **go1.25.10 → none**.
- Go suite green under `-race`; `cwso-hal` `cargo test` green (**46 tests**, up from 31).

## Key decisions
- **Health caching split:** `health()` stays cheap (cached, hot-path); `probe()` does active
  I/O off the hot path (background prober every `CWSO_HAL_HEALTH_PROBE_SECONDS`, default 10s),
  and every `infer` updates the cache reactively. Capability snapshots now carry live health.
- **TLS posture:** plaintext `http` to non-loopback hosts is refused (bearer key protection);
  loopback `http` and `https` allowed; `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true` overrides (warned).
- **Audit stays `allow_failure: true`** during the PoC phase (a future stdlib advisory should
  surface without wedging delivery); flip to a hard gate at production hardening (see task-T094).
- **`queue_depth`** is plumbed but 0: no standard OpenAI endpoint; a provider-metrics scrape
  (e.g. vLLM `/metrics`) is deferred future work.

## ID reconciliation (important for the next agent)
The Go-toolchain chore was authored/merged as **T095** (MR !22) before noticing that the
roadmap (`plan-cwso-nextgen-phase6plus.md`) reserves **T095 = eBPF AST write-spike monitor**
(Phase 7). It has been **renumbered to T114** in the board and brief (`task-T114.md`); the
merged commit message still says T095. **T095 remains the Phase 7 eBPF monitor and is NOT
started.**

## Artifacts produced (this checkpoint)
- Go: `hal/client.go` (ctx), `jobs/manager.go` (`RunResult`/`Result`),
  `tools/dispatch_hardware_aware_tools.go` (result summary + ctx), tests across `hal`/`jobs`/`tools`.
- Rust: `cwso-hal/src/backend.rs` (`probe()` + health mapping), `openai.rs` (cached health,
  reactive infer), `registry.rs` (`probe_all`), `security.rs` (TLS policy), `main.rs` (prober + TLS gate).
- CI/infra: `.gitlab-ci.yml` (`audit` stage, Go 1.25), `orchestrator/go.mod` (Go 1.25),
  `deploy/Dockerfile.orchestrator` (Go 1.25). `SECURITY.md` (HAL TLS section).
- Docs: `task-T090/T091/T092/T093/T094/T114.md`, this checkpoint.

## Blockers (active)
None.

## Next steps
- **Phase 6 Feature A: fully closed** (feature + gate + all follow-ups).
- Per the roadmap, **Phase 7 begins at T095** (eBPF AST write-spike monitor / Feature C),
  through T099; Phase 8 = T100–T104; Phase 9 = T105–T113.
- Inputs to delegate forward: this checkpoint, `cwso-nextgen-blueprint-v1.md`,
  `plan-cwso-nextgen-phase6plus.md`.
