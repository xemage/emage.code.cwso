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

**Status:** done. Implemented on branch `agent/backend-developer/C040` in worktree
`/home/emage/Code/emage/worktrees/agent-backend-developer-C040`, checked out from
`origin/develop` at `0bec0f7`. Not pushed/merged — orchestrator will handle that.

### What was done

`find_references` in `services/cwso-git-shadow/src/ast.rs` previously matched any node of
kind `identifier`/`type_identifier` whose text equalled `target_symbol` — no notion of
scope or binding at all. Replaced that with a real per-file lexical scope resolver
(`resolve_references` / `resolve_references_walk`, plus supporting `is_scope_boundary`,
`is_method_like`, `definition_name_node`, `contributes_to_enclosing_scope`,
`is_member_access_name`, `collect_bindings`, `add_binding_from_node` and per-language
binding-extraction helpers), all added to `services/cwso-git-shadow/src/ast.rs`. No other
query type (`find_definition`, `extract_signature`, `list_exports`,
`detect_entrypoints`), the `query_ast` wire protocol (`proto::Request::QueryAst`), or the
response shape (`{language, query_type, target_symbol, hits}`) was touched.

**Design** (documented in full in the `// --- Scope/binding resolution ---` comment block
in `ast.rs`, immediately above the new code):
- The wire protocol passes only a bare `target_symbol: String`, no source position, so a
  *specific* binding can never be selected by the caller — that's an existing, unchangeable
  constraint, not something this task could resolve differently. What real scope resolution
  *can* do honestly: build an actual lexical scope tree per grammar (nearest-enclosing-scope
  binding lookup, correctly modelling shadowing) and only report an occurrence as a
  reference when it resolves to *some* real, in-scope declaration of that name. An
  occurrence whose text matches but which has no visible binding in its own scope chain
  (e.g. the same name used in an unrelated sibling function) is now excluded rather than
  reported — that is the concrete "false positive across shadowed names" this closes.
- Definition sites (function/type/class/method declarations) are always reported when their
  name matches — they're unambiguous, not a guess, regardless of how many other same-named
  definitions exist elsewhere (e.g. two unrelated types each defining a method of the same
  name). Method-like definitions (receiver methods, impl methods, class methods) are
  reported but never contribute their name to the *enclosing* scope, since `Get`/`get`/
  `foo` isn't a bare-callable name outside `receiver.Get()`.
- Member/attribute/field/method-call-site access (`obj.foo`, `self.x`, `f.Get()`) is a
  separate resolution problem requiring the receiver's runtime type — i.e. type inference,
  explicitly out of scope for v1.0 per the brief. Verified via direct tree-sitter
  S-expression dumps (not assumed) that Go's selector field and method name, Rust's
  field/method access, and TypeScript's member/property access and method name all use
  `field_identifier`/`property_identifier` node kinds, distinct from `identifier`/
  `type_identifier` — so they're structurally excluded by the existing kind check with no
  extra logic. Python is the one grammar where attribute access reuses the plain
  `identifier` kind, so it needs (and got) an explicit `is_member_access_name` exclusion
  check.
- Scope boundaries and binding-introducing constructs are deliberately minimal per grammar
  (functions/methods + their params, blocks/closures, module/program, `let`/`var`/`const`/
  assignment, imports/`use`) — enough to model the fixture scenarios correctly, consistent
  with "single-file scope/binding only, no cross-file or type-inference resolution" per the
  brief. No cross-file resolution was attempted anywhere.

### Fixture scenarios built (all four grammars, `services/cwso-git-shadow/src/ast.rs`
`#[cfg(test)] mod tests`)

Each of `GO_SHADOW_FIXTURE`, `RUST_SHADOW_FIXTURE`, `PYTHON_SHADOW_FIXTURE`,
`TS_SHADOW_FIXTURE` covers, per language: an identifier orphaned outside the scope of its
only declaration; the same identifier declared in nested scopes (shadowing, with every
legitimate occurrence still resolving — proving no false negatives either); the same method
name defined on two different receiver/container types (`Foo.Get`/`Bar.Get`,
`impl Foo`/`impl Bar`, two Python classes' `foo`, two TS classes' `get`); and an imported
name shadowed by a local re-import/re-declaration. Python additionally gets a dedicated
`self.x`-style attribute-access exclusion test. 17 new tests in total (4 languages × 4
scenarios + 1 Python-specific attribute test), each with hand-derived expected hit counts
verified against the actual tree-sitter parse structure (via throwaway S-expression dumps,
not assumptions) before being encoded as assertions.

### Verification (run for real, from `services/`, using the `1.87.0` toolchain — this repo
requires ≥1.87 for `git2 0.21.0`; see `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`,
`rustup toolchain install 1.87.0` was already available locally)

```
$ cargo +1.87.0 test -p cwso-git-shadow find_references
running 17 tests
test ast::tests::go_find_references_excludes_orphan_reference_outside_declaring_scope ... ok
test ast::tests::go_find_references_local_variable_shadows_import_alias_within_its_scope ... ok
test ast::tests::go_find_references_method_definitions_on_different_receivers_not_conflated_with_calls ... ok
test ast::tests::go_find_references_resolves_nested_shadowed_variable_without_conflation ... ok
test ast::tests::python_find_references_attribute_access_is_never_guessed ... ok
test ast::tests::python_find_references_excludes_orphan_reference_outside_declaring_scope ... ok
test ast::tests::python_find_references_local_import_alias_shadows_module_import_within_its_scope ... ok
test ast::tests::python_find_references_method_definitions_on_different_receivers_not_conflated_with_calls ... ok
test ast::tests::python_find_references_resolves_nested_shadowed_variable_without_conflation ... ok
test ast::tests::rust_find_references_excludes_orphan_reference_outside_declaring_scope ... ok
test ast::tests::rust_find_references_local_binding_shadows_use_alias_within_its_scope ... ok
test ast::tests::rust_find_references_method_definitions_on_different_receivers_not_conflated_with_calls ... ok
test ast::tests::rust_find_references_resolves_nested_shadowed_variable_without_conflation ... ok
test ast::tests::typescript_find_references_excludes_orphan_reference_outside_declaring_scope ... ok
test ast::tests::typescript_find_references_local_const_shadows_import_alias_within_its_scope ... ok
test ast::tests::typescript_find_references_method_definitions_on_different_receivers_not_conflated_with_calls ... ok
test ast::tests::typescript_find_references_resolves_nested_shadowed_variable_without_conflation ... ok

test result: ok. 17 passed; 0 failed; 0 ignored; 0 measured; 36 filtered out; finished in 0.00s
```

```
$ cargo +1.87.0 test -p cwso-git-shadow
...
test result: ok. 53 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 3.05s
     Running tests/signal_shutdown.rs ...
test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.04s
```

55/55 passing — 36 pre-existing lib tests + 2 signal-shutdown integration tests, both
unchanged from the pre-change baseline (confirmed by running the full suite before making
any change), plus the 17 new tests above. Zero regressions.

```
$ grep -rn "P2-7" services/cwso-git-shadow/src/
(no output)
```

`cargo +1.87.0 fmt -p cwso-git-shadow -- --check` and `cargo +1.87.0 clippy -p cwso-git-shadow --all-targets`
were also run; `fmt` found formatting drift in the new code (fixed by running `cargo fmt`,
then re-verified clean) and `clippy` reported only pre-existing warnings in `main.rs`/
`repo.rs`/the original `walk` helper, none in the new code.

### Acceptance criteria

1. **Zero false positives on the shadowed-name fixture set (all four grammars)** — met.
   Each language's "excludes orphan reference outside declaring scope" test proves a
   concrete case the old text-match algorithm would have wrongly included is now excluded;
   the "resolves nested shadowed variable" tests prove shadowing is handled without losing
   legitimate occurrences either; the "method definitions on different receivers" tests
   prove call sites for a same-named method on two different receiver types are never
   guessed at either receiver (excluded, not merged); the "shadowed import" tests prove a
   local re-import/re-declaration correctly shadows the outer one within its own scope only.
2. **Unresolvable references return honest "unresolved", not guesses** — met. Orphaned
   identifiers (no visible binding) and all member/attribute/method-call-site access are
   excluded from `hits` rather than guessed at a specific binding/receiver; the rationale
   for each exclusion path is documented in the code comment block and in per-test doc
   comments.
3. **`cargo test -p cwso-git-shadow` passes** — met, see verbatim output above (55/55).
4. **`docs/DEBT-REGISTER.md` B6 = `fixed` / C040** — met, updated (live register row + the
   historical P2-7 cross-reference row for consistency).

### Assumptions / scope decisions

- Scope boundaries and binding rules are intentionally minimal per grammar (enough to model
  the fixture scenarios correctly: functions/methods+params, blocks/closures, module-level
  var/const/let/assignment, imports/`use`), not exhaustive language-construct coverage —
  consistent with "single-file scope/binding only" per the brief. Go's `type_declaration`
  find_definition/find_references interaction was found (via a `child_by_field_name("name")`
  check on `type_declaration` returning `None`, since the name field actually lives one
  level down on `type_spec`) to already be a pre-existing gap in `find_definition` unrelated
  to this task; not touched, and fixtures avoided relying on it.
- Type/value namespaces are merged into a single per-scope name-set (not separated) as a
  deliberate simplification; documented as a simplification, not expected to cause
  incorrect results for the fixture set or typical single-file usage.
- Unaliased Go imports and Rust glob (`use foo::*`)/grouped (`use foo::{a, b}`) imports are
  left unbound (occurrences of those names elsewhere are marked unresolved) rather than
  parsing an import path string or enumerating a glob's members, per "never guess" — a
  documented, deliberate limitation, not a silent gap.

### Blocker status

None. All four wired grammars (Go, Python, Rust, TypeScript) had genuinely implementable
scope/binding resolution; no grammar required text-matching disguised as resolution.
