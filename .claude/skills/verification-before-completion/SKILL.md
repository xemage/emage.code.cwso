---
name: "verification-before-completion"
description: "Evidence before success claims. Use before marking tasks done, committing, opening MRs, or declaring fixes complete."
---

# Verification Before Completion

## Purpose

Prevent false completion claims. Every status assertion must cite fresh command output.

## When to Use

- Before marking a task `done` in `docs/tasks/active-tasks.md`
- Before commit, push, or MR creation
- Before validation gate `PASS` verdicts
- After bug fixes or refactors
- Before release or deployment steps

## Iron Rule

```
NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE
```

## Gate Procedure

```
1. IDENTIFY — what command proves the claim?
2. RUN     — full command in current workspace state
3. READ    — exit code + complete output
4. VERIFY  — output supports the claim?
5. CLAIM   — state result with evidence (command + outcome)
```

## Claim → Evidence Map

| Claim | Required evidence |
|-------|-------------------|
| Tests pass | Test runner: 0 failures |
| Linter clean | Linter: 0 errors on changed paths |
| Build succeeds | Build command: exit 0 |
| Bug fixed | Original reproduction now passes |
| No drift | `verify.mjs`: OK |
| Release ready | `verify-release-docs.py --tag X`: passed |
| Gate PASS | Checklist filled with command output |

## Red Flags — Stop

- "Should work", "probably fixed", "seems fine"
- Satisfaction before running verification
- Trusting agent self-report without VCS or test output
- Citing a previous run from an earlier session

## emage.code Standard Commands

```bash
# Validation super-gate (implementation changes)
python3 implementation/scripts/check.py --root implementation --required

# Projection drift
node implementation/scripts/verify.mjs --root implementation

# Repository test suite
python3 tests/run.py

# Release docs (maintainers)
python3 scripts/verify-release-docs.py --tag vX.Y.Z
```

## Orchestrator Enforcement

The orchestrator must reject `done` transitions when the task brief lists
verification steps that lack recorded evidence in the checkpoint or task comment.
