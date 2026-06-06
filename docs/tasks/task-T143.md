# Task T143 — Root hygiene & PoC debt archive

- **Status:** done
- **Owner:** tech-lead
- **Priority:** P2
- **Depends on:** T141
- **Based on:** workspace audit 2026-06-07

## Objective

Clean repository root: archive historical PoC artifacts, relocate superseded blueprint,
keep active `TECHNICAL-DEBT.md` register at root.

## Deliverables

- Move `POC-DEBT-SCORECARD-phase{1,2}.md` → `docs/archive/debt/`
- Move `CWSO_ Agentic AI Orchestration Blueprint.md` → `input/` (PDF already present;
  superseded by `docs/artifacts/cwso-nextgen-blueprint-v1.md`)
- Update README and MR template links
- `TECHNICAL-DEBT.md` retained — active TD-01..TD-09 register

## Acceptance Criteria

- [x] Root contains only operational files (README, CHANGELOG, LICENSE, AGENTS.md, Makefile, …)
- [x] Historical scorecards archived, not deleted
- [x] No broken links in README
