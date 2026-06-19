# ADR-003: Tiered sandbox strategy (Docker / gVisor / Firecracker)

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect + security-engineer

## Context
LLM-generated code is non-deterministic and may include malicious payloads. A single sandbox tech cannot satisfy both rapid cold-start (planning agents) and cryptographic isolation (untrusted code execution).

## Decision
Adopt a **3-tier sandbox model** routed by the orchestrator based on declared trust:
- `docker-trusted` — internal tooling only (orchestration helpers).
- `gvisor-fast-ephemeral` — benign, ephemeral sub-agent logic (~10 ms cold start).
- `firecracker-secure-isolation` — all untrusted/LLM-generated code execution; snapshot CoW for sub-millisecond clone.

Tier selection is performed server-side; the LLM cannot escalate.

## Consequences
- (+) Defense in depth; matches threat model.
- (+) Snapshotted Firecracker yields high density even with hardware virtualization.
- (−) KVM required on host for Firecracker — validated in T040; if absent, system runs in degraded gVisor-only mode.
- (−) Three runtimes increase ops surface — single `RunnerInterface` abstraction caps complexity.
