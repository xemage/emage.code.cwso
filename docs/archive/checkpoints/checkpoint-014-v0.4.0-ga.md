# Checkpoint 014 — v0.4.0 GA

**Date:** 2026-06-09  
**Phase:** GA release (Polar parity)  
**develop tip:** `adf34d2` (post-!66 board sync)  
**GA tag:** `v0.4.0` (published)

## Completed since v0.3.0

| ID | Title | Merge |
|----|-------|-------|
| T153 | Tag pipeline deploy fix | !58 |
| T145 | Rollout `num_samples` fan-out | !59 |
| T154 | IDE integration guide | !60 |
| T155 | Enable-all-features script | !60 |
| T146 | Gateway async staging + partial traces | !61 |
| T148 | Evaluator registry + SWE-bench hook | !62 |
| T156 | Installation guide v2 | !64 |
| T149 | Trajectory builder Polar parity | !65 |
| — | Board sync + install guide trajectory section | !66 |

## Gate summary

| Gate | Status |
|------|--------|
| Phases 6–9 | PASS/PASS (v0.3.0 gates) |
| develop CI | GREEN @ `adf34d2` (pipeline `#2588782604`) |
| Operator docs | `installation-v2.md` (primary for v0.4.0) |
| GA artifact | `release-v0.4.0.md` |
| Polar parity (T144–T149) | Merged; T150/T151 deferred |

## Blockers

None.

## Next steps

1. Track post-GA backlog T150/T151 (differential prompting, offline SFT).
2. Continue v0.4.x hardening for local operability and debt remediation.
