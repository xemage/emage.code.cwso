# Task C020 — ADR-012: shadow-workspace filesystem projection decision

**ID:** C020
**Owner:** solution-architect
**Status:** in_progress
**Priority:** P0
**Depends on:** C010–C018 (gate CG1)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B2, §2.5 risk 1); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md

## Objective

Decide — with evidence, in an ADR — **which mechanism** makes shadow workspaces
reachable as real filesystem paths: **OverlayFS bind-mount** (the original P2-1 plan),
**FUSE**, or **materialise-to-tmpfs-on-open**. The ADR must record why the winner wins
on *this* host matrix, not in the abstract. Status `proposed`; **human approval is
required before C021 starts.**

> **Decision context (2026-08-13):** the human has ruled that the filesystem projection
> is **IN** v1.0 (roadmap Approval, decision 1). The *whether* is settled — ADR-012
> selects the *mechanism*. A NO-GO conclusion remains available **only** on documented
> evidence that all three mechanisms are infeasible on the host matrix; that outcome
> activates C025 and escalates to the human.

## Inputs

- `services/cwso-git-shadow/src/main.rs:11` (the P2-1 deferral marker)
- `input/CWSO_ Agentic AI Orchestration Blueprint.md` §2.3 (in-memory shadow workspaces) and §2.4 (tiered isolation)
- `sandbox/README.md`, `sandbox/router.go` (sandbox tiering and degraded mode)
- `docs/decisions/_template.md` and an existing ADR (e.g. ADR-010) for house format
- Host matrix: Linux+KVM, Linux without KVM (degraded), rootless Docker, SELinux-enforcing hosts

## Rails (read before starting)

### You MUST
- Evaluate all three options against all four host-matrix rows, with a per-cell feasibility note (works / needs privilege / impossible)
- Include a decision matrix weighted on: sub-agent compatibility (real path for `ls`/`cat`/`pytest`), crash safety (no leaked mounts), performance at v1.0 scale, implementation risk
- State the rejected options' fatal or acceptable flaws explicitly
- Name the GO/NO-GO conclusion plainly: if no option survives the host matrix, the ADR's conclusion is NO-GO and C025 (documented fallback) activates
- Follow the ADR house format (`docs/decisions/_template.md`); number it **ADR-012** (next free; ADR-008…011 exist)
- End with an explicit "Approval required" section addressed to the human

### You MUST NOT
- Write any implementation code — this is a decision record
- Pick a winner without scoring it on all four host-matrix rows
- Assume KVM is always present (the degraded path is a first-class row)
- Expand scope into Merkle indexing or performance design

## File ownership

- **May create/modify:** `docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md` (new)
- **Must NOT touch:** all code, all other docs

## Steps (execute in order)

1. Read the Blueprint sections, the P2-1 marker context, and the sandbox tiering code.
2. Build the option × host-matrix feasibility table.
3. Build the weighted decision matrix.
4. Write the ADR with a plain GO/NO-GO conclusion and the approval section.
5. Self-check: every host-matrix row scored for every option.

## Expected outputs

- `docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md` (status `proposed`)

## Acceptance criteria

1. All three options × four host-matrix rows scored
2. A single recommended option (or NO-GO) with explicit reasoning
3. Rejected options' flaws stated
4. ADR follows house format, numbered ADR-012, status `proposed`, with the approval section

## Verification commands

```bash
grep -c "OverlayFS\|FUSE\|tmpfs" docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md
grep -c "rootless\|SELinux\|KVM" docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md
grep -c "GO\|NO-GO" docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md
git diff --stat   # exactly 1 new file
```

## Git rails

- Branch: `agent/solution-architect/C020` from `develop`
- Commit: `docs(adr): propose shadow-workspace filesystem projection decision`
- MR target: `develop`, squash and merge, delete source branch
- **The MR description must end with the explicit GO/NO-GO recommendation for fast human review**

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the host matrix itself is unclear (e.g., whether rootless Docker is a supported
target), that is `unclear_requirements` / `critical` — stop and ask before writing.

## Execution notes

<filled during execution>
