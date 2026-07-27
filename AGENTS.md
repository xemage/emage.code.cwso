# emage.code v3 — Workspace Conventions

## Team Model
- Two orchestration tracks: Production (`@orchestrator`) and PoC (`@poc-orchestrator`)
- Only orchestrators are user-invocable. All other agents are subagents.
- Orchestrators delegate via structured briefs. Agents do not self-activate.

## Lifecycle States
```
pending → in_progress → blocked → in_review → done | cancelled
```

## Task Protocol
- Task list: `docs/tasks/active-tasks.md` (table: ID, status, owner, dependencies)
- Task briefs: `docs/tasks/task-<ID>.md` (objective, inputs, outputs, acceptance criteria)
- Only orchestrators create/transition tasks. Agents report completion and blockers.
- Sequential IDs: `T001`, `T002`, … Priorities: `P0` (critical path), `P1`, `P2`.

## Artifact Versioning
- Immutable artifacts: `<type>-v<N>.md` (e.g. `requirements-v1.md`, `architecture-v1.md`)
- Every artifact references its inputs: `Based on: requirements-v1.md`
- Revisions create new versions, never overwrite.

## Checkpoint Protocol
- Checkpoints: `docs/checkpoints/checkpoint-<SEQ>-<phase>.md`
- Written at every phase boundary by the orchestrator.
- Include: completed tasks, key decisions, blockers, token metrics, next steps.
- Agents receive latest checkpoint + task brief (not full history).

## Decision Log
- ADRs: `docs/decisions/ADR-<NNN>-<slug>.md`
- Each decision references requirements and task IDs.
- Immutable once accepted; superseded decisions link to replacement.

## Delegation Brief
1. **Objective** — clear, one-paragraph description
2. **Context** — current phase + latest checkpoint reference
3. **Inputs** — specific artifact versions to reference
4. **Constraints** — token budget, file ownership boundaries, technology constraints
5. **Expected Outputs** — artifact names and format
6. **Acceptance Criteria** — specific, testable conditions
7. **Blocker Protocol** — reminder to report blockers with type and severity

## Blocker Protocol
- Types: `technical` | `dependency` | `unclear_requirements` | `external`
- Severities: `critical` | `major` | `minor`
- Max 2 retries before user escalation. Agents MUST NOT silently fail.

## Validation Gates
- VERDICTS: `PASS` | `CONDITIONAL_PASS` | `FAIL`
- Review agents (Tech Lead, QA, Security) MUST NOT modify code during review.
- `FAIL` blocks progression. Orchestrator creates fix tasks and re-routes.
- `CONDITIONAL_PASS` proceeds with tracked conditions added to the task list.

## Skill Workflow (mandatory)

Before implementation, debugging, review response, or completion claims, agents
MUST check applicable skills in the active platform projection (`.github/skills/`, `.cursor/skills/`, `.gemini/skills/`, `.opencode/skills/`, `.pi/skills/`) (or invoke `/discover-skills`).

| Situation | Required skill |
|-----------|----------------|
| Any bug, test failure, unexpected behavior | `systematic-debugging` |
| Code review feedback to implement | `receiving-code-review` |
| Marking work done, commit, MR, release | `verification-before-completion` |
| Phase or validation transition | `validation-gates`, `checkpoint-protocol` |

Skipping a mandatory skill requires orchestrator approval and a logged exception
in the task brief or checkpoint.

## Code Standards
- See platform instruction projections: `.github/instructions/coding-standards.instructions.md`, `.cursor/rules/coding-standards.mdc`, `.gemini/instructions/coding-standards.md`, `.opencode/instructions/coding-standards.md`, `.pi/instructions/coding-standards.md`.
- Max 50-line functions, max 4 parameters, early returns.
- Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.
- GitFlow: `main`, `develop`, `feature/*`, `bugfix/*`, `release/*`, `hotfix/*`.

## Security
- See platform security instruction projections: `.github/instructions/security-guidelines.instructions.md`, `.cursor/rules/security-guidelines.mdc`, `.gemini/instructions/security-guidelines.md`, `.opencode/instructions/security-guidelines.md`, `.pi/instructions/security-guidelines.md`, and `implementation/SECURITY.md`.
- OWASP Top 10 compliance required.
- No secrets in code — use environment variables or vault. **Never commit live API keys.**
- Parameterized queries only.
- Input validation at all system boundaries.

## Knowledge Base
- Source knowledge lives in the emage.code repository under `implementation/knowledge/`; this target uses installed platform projections.
- Per-platform folders (`.github/`, `.gemini/`, `.opencode/`, `.cursor/`, `.pi/`) are **generated** by `scripts/sync.mjs`. **Do not edit them by hand.**
- Use the installed platform folders (`.github/`, `.cursor/`, `.gemini/`, `.opencode/`, `.pi/`) as runtime references in this target project.

## MCP Servers
Declared in platform MCP configs (`.vscode/mcp.json`, `.cursor/mcp.json`, `.gemini/settings.json`, `.opencode/opencode.json`, `.pi/mcp.json`). Each server is tagged:

| Tag | Emitted to |
|-----|-----------|
| `core` | every platform |
| `extended` | platforms whose manifest opts in (`gemini`, `opencode`, `cursor`) |

### Core servers (always available)

| Server | Purpose | Used By |
|--------|---------|---------|
| `gitlab` | Repository, issues, MRs, pipelines | Orchestrator, Scrum Master, DevOps, Release Manager |
| `playwright` | Browser automation, E2E testing | QA, Frontend, Demo Agent |
| `memory` | Persistent knowledge graph | Orchestrator (team memory) |
| `sequential-thinking` | Complex problem decomposition | Architect, Orchestrator |
| `fetch` | Web content fetching | Architect, Tech Lead, Developers |
| `brave` | Technology discovery | Technology Scout, Feasibility |
| `context7` | Framework/API documentation | Technology Scout, Integration |

### Extended servers (opt-in)
`hf-mcp-server`, `filesystem`, `github`, `git`, `supabase`, `e2b`, `docker`, `redis`, `postgresql`, `figma`, `notion`, `toolradar`. Most require additional credentials — see `servers.yaml` for the env-var contract.

## Token Governance
| Phase | Budget |
|-------|--------|
| Planning | ≤ 80k tokens |
| Architecture | ≤ 80k tokens |
| Implementation | ≤ 120k tokens |
| QA / Security / Release | ≤ 60k tokens |

Track usage in checkpoints. Compress context and start fresh delegation when approaching budget.
