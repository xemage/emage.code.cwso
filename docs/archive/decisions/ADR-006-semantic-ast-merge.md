# ADR-006: Semantic AST merge instead of line-based merge

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect

## Context
Line-based (Myers) merge corrupts code when parallel agents modify the same file in semantically orthogonal ways (e.g., one adds an import, another renames a function). The blueprint mandates conflict-free merging when intents are non-overlapping.

## Decision
The `cwso-merge-engine` Rust sidecar performs **AST-aware merge**:
1. Parse base, ours, theirs with tree-sitter.
2. Diff at the AST node level using a normalized Unified Symbol representation.
3. Auto-resolve when edits target disjoint nodes; serialize unified tree back to source.
4. On collision in the same scope, return a structured **conflict matrix** referencing AST node IDs — never produce corrupt output.

Heuristics: `ast_semantic_only` (default), `prefer_theirs`, `prefer_ours`, `fail_rapidly_on_conflict`.

## Consequences
- (+) Massively reduces merge interventions in parallel swarm work.
- (+) Conflict matrix gives the orchestrator LLM precise, actionable context.
- (−) Per-language semantic rules require maintenance — gated by per-language test corpus before enabling auto-merge for that language.
- (−) Some edits (e.g., trailing whitespace) bypass AST and are reconciled by a textual post-pass.

## Addendum (2026-05-17) — Node-level conflict payload staging

### Context
The current `merge_concurrent_results` tool contract surfaces normalized conflict metadata (`status`, `reason_code`, `escalation_class`, `escalation_action`) and does not yet expose AST node identifiers or node-level conflict coordinates in the outward payload.

### Decision
Node-level conflict payload detail is **explicitly deferred** for the current release line. For v0.1.x readiness, the conflict matrix contract is satisfied by stable class/reason semantics; node-level identifiers remain an internal merge-engine concern until a versioned payload extension is introduced.

### Scope boundary
- In scope now: deterministic conflict classification and escalation mapping at tool boundary.
- Deferred: node IDs, per-node ranges, and structured AST collision bundles in the tool response payload.

### Follow-up implementation target
Introduce a versioned payload extension in a follow-up architecture/implementation task that adds optional node-level fields while preserving backward compatibility for current clients.

### Rationale
This keeps the release contract stable, avoids late-breaking API surface expansion, and preserves a clean migration path to richer conflict diagnostics without regressing existing orchestrator behavior.
