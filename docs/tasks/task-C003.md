# Task C003 — Publish docs/DEBT-REGISTER.md

**ID:** C003
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (§1.3, B1–B13); docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md

## Objective

Create `docs/DEBT-REGISTER.md`: the single live register consolidating both archived
phase scorecards and every in-code `POC-DEBT` marker, each row carrying a disposition
(`v1.0-blocker` / `v1.1` / `wontfix` / `fixed`) and, for v1.0-blockers, the C-task that
closes it. Today the debt is scattered across `docs/archive/debt/` and code comments;
after this task there is one place to look.

## Inputs

- `docs/archive/debt/POC-DEBT-SCORECARD-phase1.md`
- `docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`
- `docs/archive/debt/README.md`
- Output of `grep -rn "POC-DEBT" . --exclude-dir=.git` (run it yourself; do not rely on this brief's list alone)
- Roadmap §1.3 blocker table (B1–B13) for the pre-decided dispositions

## Rails (read before starting)

### You MUST
- Account for **every** in-code `POC-DEBT` hit (code files, not docs/archive) as its own register row
- Use these pre-decided dispositions from the roadmap — do not invent alternatives:
  | Marker | Location | Disposition | Closed by |
  |---|---|---|---|
  | B1 hand-rolled MCP subset | `orchestrator/internal/mcp/protocol.go:10` | v1.0-blocker | C030–C032 |
  | B2 OverlayFS deferred | `services/cwso-git-shadow/src/main.rs:11` | v1.0-blocker | C020–C025 |
  | B6 find_references text matching | scorecard P2-7 | v1.0-blocker | C040 |
  | B7 orphan commits | `services/cwso-git-shadow/src/repo.rs:180` | v1.0-blocker | C041 |
  | B12 UDS perms 0o666 | scorecard P2-5 | v1.0-blocker | C044 |
  | B13 no connection pooling | `orchestrator/internal/shadow/client.go:5` | v1.0-blocker | C043 |
  | B11 SWE-bench evaluator stub | `evaluator_swebench.go:64` | v1.1 | — |
  | P2-2 Merkle incremental indexer | scorecard | v1.1 | — |
  | File-based JWT secret | `deploy/docker-compose.yml:6` | v1.0-blocker (document) | C063 |
  | T029 Vault/SOPS | scorecard / compose comment | v1.1 | — |
- Verify before marking anything `fixed`: P2-3 (grammar coverage) claims closed — confirm `services/Cargo.toml` wires `tree-sitter-go`, `tree-sitter-python`, `tree-sitter-rust`, `tree-sitter-typescript` before writing `fixed`; phase-1 D6 (rate limiting) claims implemented — confirm in `orchestrator/internal/transport/http.go`
- Include both phase scorecards as consolidated historical sections (mark each row carried-forward or closed)

### You MUST NOT
- Modify any code file, scorecard, or archived document — the register is a new file
- Delete or edit the archived scorecards (they are history)
- Mark a row `fixed` without the verification evidence above
- Leave any row's disposition empty

## File ownership

- **May create/modify:** `docs/DEBT-REGISTER.md` (new file only)
- **Must NOT touch:** everything else

## Steps (execute in order)

1. Run `grep -rn "POC-DEBT" . --exclude-dir=.git --exclude-dir=docs` and record every code hit.
2. Read both scorecards and `docs/archive/debt/README.md`.
3. Verify the two `fixed` claims (P2-3 grammars, D6 rate limiting) per the rails.
4. Write `docs/DEBT-REGISTER.md` with this header structure: purpose paragraph → live register table (ID | source `file:line` | category | description | status | disposition | closing task) → historical scorecard sections → footer noting the register is updated by C060 at release.
5. Cross-check: every grep hit from step 1 appears as a row.

## Expected outputs

- `docs/DEBT-REGISTER.md` — complete, every row dispositioned

## Acceptance criteria

1. Every in-code `POC-DEBT` marker appears in the register with a disposition
2. Both scorecards are represented; carried-forward rows keep their original IDs (P1-x, P2-x, D6)
3. The two `fixed` claims are either confirmed with evidence (file reference quoted in the row) or downgraded to `v1.0-blocker` with a note
4. No empty disposition cells

## Verification commands

```bash
grep -rn "POC-DEBT" . --exclude-dir=.git --exclude-dir=docs | wc -l   # count code markers
grep -c "v1.0-blocker\|v1.1\|wontfix\|fixed" docs/DEBT-REGISTER.md    # every row dispositioned
grep "tree-sitter-go\|tree-sitter-python\|tree-sitter-rust\|tree-sitter-typescript" services/Cargo.toml
grep -n "rate" orchestrator/internal/transport/http.go | head -5
```

## Git rails

- Branch: `agent/technical-writer/C003` from `develop`
- Commit: `docs: publish consolidated debt register`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a scorecard row is unintelligible or contradicts the code, do not resolve it by
guess — include the row with disposition `unclear` and flag `unclear_requirements` / `minor`.

## Execution notes

<filled during execution>
