---
name: "cwso-awareness"
description: "Use before designing or dispatching any Pattern A concurrent multi-agent code-editing work, before an agent calls implementation/runtime/cwso client code, or whenever it is unclear which CWSO permission tier an emage.code agent role holds. Explains what CWSO is, the mandatory worker/orchestrator role split enforced by the live server, and the approved agent-to-tier mapping."
---

# CWSO Awareness

## Purpose

CWSO (Concurrent Workspace Orchestration) is the deployed runtime that emage.code integrates
with to implement **Pattern A** — concurrent multi-agent code editing. Multiple agents each get
an isolated, in-memory "shadow workspace" branched from a shared base commit, write and commit
their changes independently, and an orchestrator-role client performs an AST-aware conflict
pre-check followed by a structured merge (`merge_concurrent_results`) to produce one integrated
commit. This lets N agents edit overlapping files concurrently without stepping on each other,
with semantic (not just textual) conflict detection.

This skill exists so that any agent touching Pattern A work — deciding whether to use it,
dispatching workers, writing CWSO client code, or reviewing a CWSO-related task — knows CWSO
exists, knows the one non-obvious rule that will otherwise cause a hard failure, and knows its
own permission tier before it ever calls a tool.

## When to Use

- Before designing or dispatching any Pattern A concurrent-editing workflow (multiple agents
  editing the same or related files in parallel).
- Before writing or modifying code that calls `implementation/runtime/cwso` (`CwsoClient`,
  `AstConflictChecker`, `ConcurrentMergeOrchestrator`).
- Whenever it is unclear which CWSO permission tier (`orchestrator`, `worker`, or `read`) an
  emage.code agent role should use for a given tool call.
- Before delegating a task that will need to bring up the CWSO stack or mint a CWSO JWT.

## What CWSO Is (and Why emage.code Integrates It)

CWSO is a separately-deployed MCP server (run via `deploy/docker-compose-t226.yml`) that exposes
11 tools for shadow-workspace lifecycle management, file writes, commits, AST queries, and
concurrent merges. emage.code integrates it as the concurrency engine behind Pattern A so that
parallel agent work (e.g. several `worker`-tier agents editing the same module) can be merged
deterministically using AST-semantic heuristics rather than line-based diff/patch, reducing
false-positive merge conflicts between agents working on independent symbols in the same file.

For full runtime usage, the worked Scenario 1 code example (three agents, independent edits,
clean merge), and known response-shape quirks (e.g. `write_shadow_file`'s plain-text response),
see the canonical runtime guide:
[`implementation/runtime/cwso/README.md`](../../../runtime/cwso/README.md). Do not re-derive
usage patterns from first principles — that guide is the accurate, current source.

For **how to stand up CWSO locally or connect it to the emage.code orchestrator**, see:
- [`docs/deployment/README.md`](../../../../docs/deployment/README.md) — deployment environment index
- [`docs/deployment/local-docker-desktop-guide.md`](../../../../docs/deployment/local-docker-desktop-guide.md) — local Docker Desktop setup
- `docs/deployment/cwso-emage-orchestrator-connection-guide.md` — connecting CWSO to the
  emage.code orchestrator (referenced by name/path only; this doc is maintained separately)

Do not duplicate the deployment steps here — link to those docs instead.

## The Mandatory Worker/Orchestrator Role Split

The live CWSO server enforces **role-based tool permissions** at the MCP boundary. This split is
**not documented anywhere in the CWSO server itself** — it was discovered empirically during
task `T214`. A single `CwsoClient` cannot run the full Pattern A flow; you need two role-scoped
clients built from the same shared JWT secret.

| Role | Allowed tools | Blocked tools |
|------|--------------|----------------|
| `worker` | `create_shadow_workspace`, `write_shadow_file`, `commit_shadow`, `query_ast`, `drop_shadow_workspace` | `merge_concurrent_results` |
| `orchestrator` | `create_shadow_workspace`, `merge_concurrent_results`, `drop_shadow_workspace` | `write_shadow_file`, `commit_shadow` |

**Rule:** use a `worker`-role client for workspace setup (create/write/commit/query_ast/drop) and
a separate `orchestrator`-role client for the final `merge_concurrent_results` call.

**Warning:** calling a tool with the wrong role produces an **HTTP 403** at the tool-call level.
This is the single most important, non-obvious fact about Pattern A — see
[`implementation/runtime/cwso/README.md`](../../../runtime/cwso/README.md) for the full code
example and citation of task `T214` as the empirical source.

## emage.code Agent -> CWSO Tier Mapping

The following table is embedded **verbatim** from the approved artifact
[`docs/artifacts/role-mapping-cwso-v1.md`](../../../../docs/artifacts/role-mapping-cwso-v1.md)
(task `T211`, solution-architect). Do not alter tier assignments or rationale here — if the
mapping needs to change, a new version of `role-mapping-cwso-v1.md` must be produced and this
skill updated to match.

| emage.code agent | CWSO tier | Rationale |
|---|---|---|
| `orchestrator` | `orchestrator` | Coordinates decomposition, dispatch, and lifecycle management; should not directly mutate task outputs. |
| `poc-orchestrator` | `orchestrator` | Same orchestration duties for PoC workflows; planning-scoped permissions only. |
| `backend-developer` | `worker` | Produces and commits code in shadow workspaces. |
| `frontend-developer` | `worker` | Produces and commits UI code changes in isolated workspaces. |
| `database-engineer` | `worker` | Writes schema/query changes and commits merge inputs. |
| `devops-engineer` | `worker` | Writes CI/CD and infra artifacts requiring workspace mutation. |
| `technical-writer` | `worker` | Updates documentation artifacts that require write capability. |
| `qa-engineer` | `worker` | Can author/update tests and helper fixtures as part of execution tasks. |
| `release-manager` | `worker` | May write release notes/metadata and merge release-oriented changes. |
| `tech-lead` | `read` | Review role; should inspect and validate without implementation writes during gates. |
| `security-engineer` | `read` | Audit role; requires read/query-only access to enforce separation of duties. |
| `product-owner` | `read` | Requirement and prioritization role; no code mutation required. |
| `solution-architect` | `read` | Architecture review/decision role; read-only for implementation boundaries. |
| `scrum-master` | `read` | Process coordination role; no direct code mutation. |
| `ux-designer` | `read` | Design feedback role; no direct code mutation in runtime execution. |

Note the tier naming distinction: emage.code's own `orchestrator` agent role maps to the CWSO
`orchestrator` tier, but every other write-capable emage.code role (including
`technical-writer`) maps to the CWSO `worker` tier — "worker" here is a CWSO permission-tier
name, not a statement about the agent's seniority or scope within emage.code.

## Status: Validated, Not Speculative

CWSO/Pattern A integration is a **deployed, tested runtime dependency**, not a paper design:

- `T214` — Pattern A end-to-end integration test (3 concurrent agents, deterministic merge)
  passed against the live stack; this is also where the worker/orchestrator role split and the
  `write_shadow_file` response-shape quirk were discovered.
- `T304` — proved the `deploy/docker-compose-t226.yml` stack actually builds and runs for real
  (no mocked substitute).
- `T319` — authored `implementation/runtime/cwso/README.md` documenting the actual, current,
  post-fix runtime behavior.
- User manual validation on 2026-08-06 additionally confirmed, live: the Docker stack coming up,
  JWT minting working, the live contract test passing, and `CwsoClient.from_env().tools_list()`
  returning all 11 tools.

Treat CWSO as a real dependency with real failure modes (notably the HTTP 403 role-split trap
above), not as an aspirational integration.

## Guidelines

- Always check this skill (or the linked runtime README) before writing new CWSO client code —
  do not assume tool permissions symmetrically apply to both roles.
- Never hold both a `worker` and `orchestrator` JWT for the same task step without a clear reason
  to switch roles mid-flow — least privilege applies per the approved mapping.
- If a tool call returns HTTP 403, check role assignment first before treating it as a CWSO bug.
- Never print, log, or commit the `CWSO_JWT_SECRET` or any minted JWT.
- If your agent role is not `orchestrator`, `worker`, or a role mapped to `worker`/`orchestrator`
  in the table above, you should be using a `read`-tier client (or no CWSO client at all).
