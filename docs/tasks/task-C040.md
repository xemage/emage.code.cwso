# Task C040 — Scope/binding resolution for find_references

**ID:** C040
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B6, P2-7); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

`find_references` currently matches identifier **text** only — no scope/binding
analysis — so it returns false positives across shadowed names and methods on
different receivers. `query_ast` is the most-called tool; silently wrong answers are
worse than errors. Implement real scope/binding resolution for the four wired grammars.

## Inputs

- `services/cwso-git-shadow/src/ast.rs` (`find_references` implementation)
- `services/Cargo.toml` (the four wired grammars: tree-sitter-go, -python, -rust, -typescript)
- Scorecard P2-7 (`docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`)

## Rails (read before starting)

### You MUST
- Implement binding resolution using tree-sitter's scope model (locals/definitions), for the four wired grammars only
- Build a shadowed-name fixture set covering all four grammars: same identifier in nested scopes, same method name on different receivers/types, shadowed imports
- Return an honest "unresolved" (or exclude with a documented reason) for references that cannot be resolved within a single file — never guess
- Remove the P2-7 marker and update `docs/DEBT-REGISTER.md` (B6 → `fixed`, closing task C040)
- Add regression tests: the fixture set must produce zero false positives

### You MUST NOT
- Attempt cross-file or type-inference resolution — that is v1.1; single-file scope/binding only
- Change the `query_ast` tool signature or response shape (clients depend on it)
- Add new grammars
- Touch the merge-engine or orchestrator

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `docs/DEBT-REGISTER.md` (B6 row)
- **Must NOT touch:** `orchestrator/*`, other services, `schemas/*`

## Steps (execute in order)

1. Read the current `find_references` and the P2-7 scorecard entry.
2. Build the shadowed-name fixture set (4 grammars).
3. Implement scope/binding resolution.
4. Tests: zero false positives on fixtures; existing tests stay green.
5. Remove marker; update DEBT-REGISTER.

## Expected outputs

- Resolver in `ast.rs` + fixture set + tests
- P2-7 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. Zero false positives on the shadowed-name fixture set (all four grammars)
2. Unresolvable references return honest "unresolved", not guesses
3. `cargo test -p cwso-git-shadow` passes
4. DEBT-REGISTER B6 = `fixed` / C040

## Verification commands

```bash
cargo test -p cwso-git-shadow find_references
grep -rn "P2-7" services/cwso-git-shadow/src/   # = no hits
```

## Git rails

- Branch: `agent/backend-developer/C040` from `develop`
- Commit: `fix(git-shadow): resolve scope and binding in find_references`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a grammar's tree-sitter bindings make scope analysis impractical, report
`technical` / `major` naming the grammar — do not ship text-matching for that grammar
disguised as resolution.

## Execution notes

<filled during execution>
