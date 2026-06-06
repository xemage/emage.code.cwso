# Polar Gap Analysis — NVIDIA Polar vs CWSO Rollout Stack

**Based on:** `input/NVIDIA Polar.pdf` (arXiv:2605.24220), `rollout-architecture-v1.md`,  
`develop` implementation through T135/T137/T140  
**Date:** 2026-06-07  
**Purpose:** Map Polar paper features to CWSO status; drive tasks T144–T151.

## Summary

CWSO implements Polar's **core thesis** — black-box harness RL via an LLM API proxy, token-faithful
capture, prefix-merged trajectories, programmatic rewards, and async REST task API — as Phase 9
Features E+F+G. Gaps remain in **harness-native execution**, **gateway staging**, **evaluators**,
and **full trajectory / proxy parity**. Embedded GRPO training and gRPC are intentionally out of scope.

## Feature matrix

| Polar capability (paper §) | CWSO status | Task |
|---------------------------|-------------|------|
| Model API proxy (OpenAI Chat, Anthropic, Google) | **Done** — `cwso-rollout` T132 | — |
| OpenAI **Responses** API route | **Missing** | T147 |
| Synthetic SSE from buffered completion | **Done** (PoC) | T147 (harden) |
| Token IDs + logprobs capture | **Done** | — |
| Trajectory prefix merging | **Partial** — simplified Go builder T133 | T149 |
| Per-request trajectory builder | **Missing** | T149 |
| Message-level chain grouping + EOT interstitials | **Missing** | T149 |
| Parquet / Arrow trajectory store | **Done** T134 | — |
| Programmatic rewards | **Done** T136 (merge SM) | — |
| Rollout REST API (submit, poll, callbacks) | **Done** T137 | — |
| KV prefix router + LRU prewarm | **Done** T135 | T150 (differential prompt) |
| `num_samples` task fan-out | **Missing** | T145 |
| Harness adapters (codex, claude_code, …) | **Missing** | T144 |
| Runtime launcher (Docker/Apptainer) | **Partial** — sandbox tiers exist, not Polar harness launch | T144 |
| Gateway worker pools INIT/RUNNING/POSTRUN | **Missing** | T146 |
| Evaluator prewarm during agent run | **Missing** | T146 |
| Evaluator registry (SWE-bench, test-on-output) | **Missing** | T148 |
| Partial trace recovery on timeout | **Missing** | T146 |
| Offline SFT data generation mode | **Missing** | T151 |
| Embedded GRPO / trainer | **Out of scope** (external trainers) | — |
| gRPC rollout API | **Deferred** | — |
| ClickHouse warehouse | **Deferred** post-v0.3.0 | — |

## Recommended delivery order (post-RC)

1. **T142** — Installation & usage docs (GA blocker for adopters)
2. **T147** — OpenAI Responses + proxy hardening (trainer harness compatibility)
3. **T144** — Harness adapters + runtime launcher (Polar black-box promise)
4. **T145** — `num_samples` session fan-out
5. **T146** — Gateway async staging + timeout partial traces
6. **T149** — Full trajectory builder parity
7. **T148** — Evaluator registry + SWE-bench hook
8. **T150** — KV differential prompting
9. **T151** — Offline SFT data generation

## References

- Polar paper: `input/NVIDIA Polar.pdf`
- CWSO design: `docs/artifacts/rollout-architecture-v1.md`, ADR-010
- RC release: `docs/artifacts/release-v0.3.0-rc1.md`
