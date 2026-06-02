# Checkpoint <SEQ> — <phase>

> Filename: `checkpoint-<SEQ>-<phase>.md` (e.g. `checkpoint-002-architecture.md`).
> Written by the orchestrator at every phase boundary.

## Phase summary
One paragraph: what phase produced, whether it met its acceptance criteria.

## Completed tasks (this phase)
| ID | Title | Owner | Outcome |
|----|-------|-------|---------|

## Open / carried over
| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|

## Key decisions
- ADR-001 — … (see `docs/decisions/ADR-001-*.md`)

## Artifacts produced
- `requirements-v1.md`
- `architecture-v1.md`
- …

## Blockers (active)
| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|

## Token usage
| Phase | Budget | Spent | % |
|-------|--------|-------|---|
|       |        |       |   |

## Next steps
- Phase: …
- Tasks: T0xx, T0yy
- Inputs to delegate forward: <artifact-vN>, this checkpoint.

## Compression note
This checkpoint is the canonical handoff for the next phase. Subsequent agents receive **only**: this checkpoint + their task brief + referenced artifact versions.
