---
name: "systematic-debugging"
description: "Four-phase root cause process for bugs, test failures, and unexpected behavior. Use before proposing any fix."
---

# Systematic Debugging

## Purpose

Prevent symptom-chasing and rework. Find root cause with evidence before changing code.

## When to Use

- Test failures or flaky tests
- Production or CI bugs
- Unexpected agent or build behavior
- Performance regressions
- Integration failures across components

**Especially when** under time pressure, after a failed fix attempt, or when the issue "looks obvious."

## Iron Rule

```
NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST
```

## Four Phases

Complete each phase before the next.

### Phase 1 — Investigate

1. Read error messages and stack traces completely
2. Reproduce reliably; record exact steps
3. Inspect recent changes (`git log`, `git diff`, dependency updates)
4. At component boundaries, gather evidence (logs, inputs/outputs) before guessing

### Phase 2 — Hypothesize

1. List plausible causes ranked by likelihood
2. State what evidence would confirm or reject each
3. Pick the cheapest test that eliminates the most uncertainty

### Phase 3 — Experiment

1. Change one variable at a time
2. Run the smallest command that validates the hypothesis
3. Document result: confirmed / rejected / inconclusive

### Phase 4 — Fix and Verify

1. Implement the minimal fix for the confirmed root cause
2. Re-run the original reproduction steps
3. Run related regression tests
4. If fix fails, return to Phase 1 — do not stack patches

## Multi-Component Systems

For pipelines (CI → build → deploy, API → service → DB):

```
FOR EACH boundary:
  - What enters?
  - What exits?
  - Where does it diverge from expected?
```

## Escalation

If root cause is unclear after two structured cycles:

1. Report blocker type `technical`, severity `major`
2. Attach evidence gathered so far
3. Request orchestrator routing (e.g. specialist agent or human decision)

## Anti-Patterns

- Changing code before reproduction
- Multiple unrelated fixes in one commit
- Declaring fixed without re-running the failing command
- Ignoring warnings that accompany the error
