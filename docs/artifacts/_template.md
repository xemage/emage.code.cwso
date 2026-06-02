# Artifact: <type>-v<N>.md

> Filename: `<type>-v<N>.md` (e.g. `requirements-v1.md`, `architecture-v2.md`).
> Artifacts are **immutable** once produced. Revisions bump `<N>`; never overwrite.

## Metadata
- **Producer agent**: <agent-name>
- **Task**: T0xx
- **Created**: YYYY-MM-DD
- **Based on**: requirements-v1.md, architecture-v1.md
- **Supersedes**: <type>-v(N-1).md  ← if revision

## Body
The artifact's content. Format depends on type:

- `requirements-vN.md` → user stories, acceptance criteria, NFRs
- `architecture-vN.md` → context / container / component diagrams, key decisions
- `test-report-vN.md` → pass/fail counts per suite, validation gate verdict
- `release-notes-vN.md` → changelog entries, breaking changes

## Consumed by
List the downstream tasks/agents that should reference this artifact version explicitly.

- T0xx (qa-engineer)
- T0yy (release-manager)
