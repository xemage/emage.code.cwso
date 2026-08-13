# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T010 | SE: Security audit (auth, secret leakage) | security-engineer | in_review | P1 | T008 | 2026-08-06 |
| C001 | README version truth + CI drift guard | devops-engineer | in_progress | P0 | — | 2026-08-13 |
| C002 | Reconcile quick-start commands | technical-writer | in_progress | P0 | — | 2026-08-13 |
| C003 | Publish docs/DEBT-REGISTER.md | technical-writer | in_progress | P0 | — | 2026-08-13 |
| C004 | Reconcile task ledger with briefs | technical-writer | in_progress | P0 | — | 2026-08-13 |
| C005 | Publish docs/SCOPE-v1.0.md | technical-writer | in_progress | P0 | — | 2026-08-13 |
| C010 | Remove phase2/phase4 compose profile gates | devops-engineer | pending | P0 | C001–C005 (CG0) | 2026-08-12 |
| C011 | Add cwso-rollout behind opt-in profile | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C012 | Bootstrap .env.jwt.dev on first run | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C013 | scripts/cwso-token.sh replaces JWT heredoc | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C014 | Fold enable-all-features into compose defaults | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C015 | Mount user repo read-write (CWSO_WORKSPACE_HOST) | devops-engineer | pending | P0 | C010, C019 | 2026-08-13 |
| C016 | make up one-command target | devops-engineer | pending | P0 | C012, C013, C014, C015 | 2026-08-12 |
| C017 | scripts/cwso-doctor.sh diagnostics | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C018 | E2E smoke test (v1.0 DoD executable) | qa-engineer | pending | P0 | C016, C017 | 2026-08-12 |
| C019 | Sandbox trustworthiness, non-KVM default path | backend-developer | pending | P0 | C010 | 2026-08-13 |
| C020 | ADR-012: filesystem projection decision | solution-architect | pending | P0 | C010–C018 (CG1) | 2026-08-12 |
| C021 | Implement filesystem projection | backend-developer | pending | P0 | C020 (GO) | 2026-08-12 |
| C022 | Write-back into git ODB | backend-developer | pending | P0 | C021 | 2026-08-12 |
| C023 | Projection lifecycle + crash safety | backend-developer | pending | P0 | C021 | 2026-08-12 |
| C024 | Prove projection E2E in CI | qa-engineer | pending | P0 | C022, C023 | 2026-08-12 |
| C025 | CONDITIONAL: document IPC-only limitation | technical-writer | pending | P0 | C020 (NO-GO) | 2026-08-12 |
| C030 | MCP gap table (impl vs spec) | backend-developer | pending | P1 | C001–C005 (CG0) | 2026-08-12 |
| C031 | ADR-013: SDK vs conformance suite | solution-architect | pending | P1 | C030 | 2026-08-12 |
| C032 | Execute ADR-013 decision | backend-developer | pending | P1 | C031 | 2026-08-12 |
| C033 | Client compatibility matrix (3×2) | qa-engineer | pending | P1 | C032 | 2026-08-12 |
| C034 | Contract snapshot test in CI | qa-engineer | pending | P1 | C032 | 2026-08-12 |
| C040 | Scope/binding resolution for find_references | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C041 | Parent-commit tracking per workspace | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C042 | Three-way merge + conflict matrix | backend-developer | pending | P1 | C041 | 2026-08-12 |
| C043 | Connection pooling in shadow client | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C044 | UDS perms 0o660 or documented limitation | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C050 | Write the single user guide | technical-writer | pending | P1 | C040–C044 | 2026-08-12 |
| C051 | Delete the five superseded guides | technical-writer | pending | P1 | C050 | 2026-08-12 |
| C052 | Receive emage.code deployment docs (T403) | technical-writer | pending | P1 | C050, T403 | 2026-08-12 |
| C053 | Contributor vs user doc separation | technical-writer | pending | P1 | C050 | 2026-08-12 |
| C054 | Verify guide commands on clean machine | qa-engineer | pending | P1 | C050, C051, C052, C053 | 2026-08-12 |
| C060 | Debt register: zero unclassified rows | technical-writer | pending | P0 | C050–C054 (CG4) | 2026-08-12 |
| C061 | Security pass closing T010 | security-engineer | pending | P0 | C050–C054 (CG4) | 2026-08-12 |
| C062 | Release v1.0.0 | devops-engineer | pending | P0 | C060, C061, C063 | 2026-08-12 |
| C063 | Publish docs/LIMITATIONS.md | technical-writer | pending | P0 | C060 | 2026-08-12 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

> The C-series implements `docs/plans/plan-cwso-v1.0-roadmap.md` (**approved**
> 2026-08-13, incl. the three open-question decisions; C019 was added by decision 3).
> C025 activates only on an ADR-012 NO-GO. Gate dependencies (CG0–CG4) are noted inline.
> First dispatchable set: C001–C005 (parallel) and C030 (depends only on CG0).

Per-task briefs live alongside this file as `task-T001.md`, `task-T002.md`, …, `task-C001.md`, …
