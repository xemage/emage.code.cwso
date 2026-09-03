# CWSO in emage.code: Overview, Deployment, and Agent Integration Guide

**Version:** 1
**Last updated:** 2026-08-06
**Based on:** `docs/plans/plan-018-cwso-agent-knowledge-awareness.md`, `implementation/runtime/cwso/README.md`
(task T319), `docs/artifacts/role-mapping-cwso-v1.md` (task T211), `docs/deployment/README.md`,
`docs/deployment/local-docker-desktop-guide.md`, `implementation/knowledge/skills/cwso-awareness/SKILL.md`
and the three updated agent files (task T327), `docs/tasks/task-T214.md`, `docs/tasks/task-T304.md`,
`docs/tasks/task-T319.md`

This is a single narrative walkthrough, written for a reader who knows plain emage.code but has never
touched CWSO: what it is and why it exists in this project, how to actually deploy it, how agent code
calls it at runtime, and whether the multi-agent knowledge base needed updating to reflect any of this.
It intentionally does not repeat the detail already written elsewhere — every section below links to the
canonical source and adds only enough summary to orient you.

---

## 1. Overview and goal

**Plain emage.code** coordinates a team of specialist AI agents (orchestrator, backend-developer,
devops-engineer, and so on) that work through tasks sequentially or in a delegated, one-agent-at-a-time
fashion, writing to a shared working tree.

**CWSO (Concurrent Workspace Orchestration)** is a separately-deployed service that emage.code
integrates with to go one step further: it lets *multiple* agents edit the same or related files at the
same time, safely. Each agent gets its own isolated, in-memory "shadow workspace" branched from a shared
base commit, writes and commits its changes independently, and then an orchestrator-role client runs an
AST-aware (semantic, not line-based) conflict pre-check before performing a structured merge
(`merge_concurrent_results`) that produces one integrated commit. This whole workflow is called
**Pattern A** inside this project.

The problem Pattern A solves: naive concurrent edits to the same file normally require serializing
agents (slow) or accepting textual merge conflicts (fragile, and produces false-positive conflicts when
two agents touch unrelated symbols in the same file). CWSO's AST-semantic merge heuristics let
independent-symbol edits merge cleanly and automatically, while still flagging genuine semantic
conflicts for a human or higher-tier agent to resolve.

CWSO is not a design proposal — it is a deployed runtime dependency with a real MCP server, real
role-based tool permissions, and a real Docker Compose stack (`deploy/docker-compose-t226.yml`). The
rest of this document walks through deploying it, calling it, and confirming the agent knowledge base
already reflects it correctly.

---

## 2. Deployment

CWSO deployment is documented independently and is **not re-derived here** — this section only tells
you which document to open and why.

- **[`docs/user/deployment/README.md`](README.md)** — the deployment index. Start here if you haven't chosen
  an environment yet; it compares Docker Desktop, Proxmox LXC, and GCP Cloud Run side by side (setup
  time, cost, scalability) and lists the shared CWSO architecture (orchestrator, rollout proxy, git
  shadow, merge engine, JWT auth) that is identical across all three targets.
- **[`docs/user/deployment/local-docker-desktop-guide.md`](local-docker-desktop-guide.md)** — the guide to
  actually use for local development. It is the only one of the three environment guides that has been
  validated end-to-end (the Proxmox and GCP guides are explicitly marked "not yet validated end-to-end"
  in their own headers); it walks through `bash scripts/deploy/cwso-docker-desktop.sh`, JWT secret setup,
  and the distinction between the public `/healthz` liveness route and the authenticated `/health` route.
- **`docs/user/deployment/cwso-emage-orchestrator-connection-guide.md`** — a companion guide (present in this
  working tree) for wiring an already-running local CWSO stack to emage.code runtime clients via
  `CWSO_BASE_URL` / `CWSO_JWT_SECRET`, with an MCP smoke check and a live functional test. Referenced
  here by name/path only; it is maintained separately from this document.

For local development, Docker Desktop is the practical default: it is the only environment with a
validated guide, and it is what T304 and the user's own 2026-08-06 session both exercised (see §4).

---

## 3. Runtime usage — how agent code actually calls CWSO

Once CWSO is deployed, agent code talks to it through `implementation/runtime/cwso/` — this section is a
brief orientation; the canonical, current, and only source you should follow for actual usage patterns
is:

**[`implementation/runtime/cwso/README.md`](../../../implementation/runtime/cwso/README.md)** (task T319)

That README documents `CwsoClient` (JWT minting, all 11 tools), `AstConflictChecker`,
`ConcurrentMergeOrchestrator`, a full worked three-agent Scenario 1 example, and a known
`write_shadow_file` response-shape quirk. Do not re-derive usage from first principles — read that file.

The one fact worth calling out here, because it is the single most important non-obvious thing about
Pattern A: **CWSO enforces a mandatory role split at the tool-call level.** A `worker`-role client can
create workspaces, write, commit, query AST, and drop workspaces, but is blocked from calling
`merge_concurrent_results`. An `orchestrator`-role client can create/drop workspaces and call
`merge_concurrent_results`, but is blocked from writing or committing. Calling a tool with the wrong role
fails with an HTTP 403. Every emage.code agent role maps to exactly one of these tiers (plus a `read`
tier for review-only roles); the full agent-to-tier mapping is not repeated here — see
`implementation/knowledge/skills/cwso-awareness/SKILL.md` (§4 below) or the source artifact
[`docs/artifacts/role-mapping-cwso-v1.md`](../../artifacts/role-mapping-cwso-v1.md) (task T211) for the
complete table and rationale.

---

## 4. Agent knowledge and skills — "do I need to update agent knowledge or skills?"

**Yes — and it was already done.** This was the objective of plan-018 (task T327), which ran before this
document. Nothing below is speculative; it is a report of what already exists in this branch.

Two things were added:

1. **A new skill**, `cwso-awareness`, so that any agent about to touch Pattern A work — deciding whether
   to use it, dispatching workers, writing CWSO client code, or reviewing a CWSO-related task — has a
   single place that explains what CWSO is, the mandatory worker/orchestrator role split, and the
   approved agent-to-tier mapping before it ever calls a tool:
   [`implementation/knowledge/skills/cwso-awareness/SKILL.md`](../../../implementation/knowledge/skills/cwso-awareness/SKILL.md)

2. **A new "CWSO Awareness" section added to the three agent role files** whose CWSO tier is
   operationally relevant (orchestrator-tier coordination, or worker-tier write/commit access), each
   stating that agent's specific CWSO permission tier and pointing back to the skill for the full rule:
   - [`implementation/knowledge/agents/orchestrator.md`](../../../implementation/knowledge/agents/orchestrator.md)
     — CWSO tier `orchestrator` (coordinates dispatch and workspace lifecycle; never delegates
     write/commit calls to an orchestrator-tier client)
   - [`implementation/knowledge/agents/backend-developer.md`](../../../implementation/knowledge/agents/backend-developer.md)
     — CWSO tier `worker` (produces and commits code in shadow workspaces; blocked from
     `merge_concurrent_results`)
   - [`implementation/knowledge/agents/devops-engineer.md`](../../../implementation/knowledge/agents/devops-engineer.md)
     — CWSO tier `worker` (writes CI/CD and infra artifacts requiring workspace mutation; blocked from
     `merge_concurrent_results`)

Both the skill and the three agent-file edits were reviewed by a solution-architect gate for faithfulness
to `docs/artifacts/role-mapping-cwso-v1.md`, with **VERDICT: PASS**, and have already been synced and
propagated into the live platform projections (`.claude/`, and the other seven per-platform directories)
via `make sync` + `scripts/install.sh`. You can confirm the projection landed yourself with:

```bash
grep -rl -i cwso .claude/skills .claude/agents
```

This returns the four projected files above (it returned nothing before plan-018).

### This is not new or unverified integration work

CWSO/Pattern A was already validated through manual/live testing well before this documentation task
existed — this guide, and the knowledge-base update it documents, describe an already-working system,
not an aspirational one:

- **T214** — the Pattern A end-to-end integration test (three concurrent agents, deterministic merge)
  passed against the live CWSO stack; this is also where the worker/orchestrator role-split rule and a
  `write_shadow_file` response-shape quirk were discovered empirically.
- **T304** — proved the `deploy/docker-compose-t226.yml` stack actually builds and runs for real, with no
  mocked substitute.
- **T319** — authored `implementation/runtime/cwso/README.md`, documenting the actual, current, post-fix
  runtime behavior referenced in §3 above.
- **User manual validation, 2026-08-06** — the user independently brought the Docker stack up via
  `deploy/docker-compose-t226.yml`, minted working tokens with `scripts/mint-cwso-jwt.py`, ran the
  `CWSO_LIVE_CONTRACT_TEST=1` live contract test successfully, and confirmed
  `CwsoClient.from_env().tools_list()` returns all 11 tools.

---

## Summary

| Question | Answer | Where to go next |
|---|---|---|
| What is CWSO and why does emage.code use it? | Concurrent multi-agent code editing (Pattern A) via AST-semantic shadow-workspace merges | §1 above |
| How do I deploy it locally? | `docs/deployment/local-docker-desktop-guide.md` (validated) | [local-docker-desktop-guide.md](local-docker-desktop-guide.md) |
| How does agent code call it? | `CwsoClient` with role-scoped (`worker`/`orchestrator`) JWTs; role split is mandatory | [`implementation/runtime/cwso/README.md`](../../../implementation/runtime/cwso/README.md) |
| Did agent knowledge/skills need updating? | Yes, and it was already done in T327 | [`cwso-awareness` skill](../../../implementation/knowledge/skills/cwso-awareness/SKILL.md), three agent files above |
