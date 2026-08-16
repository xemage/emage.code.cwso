# Task C019 — Sandbox trustworthiness for the non-KVM default path

**ID:** C019
**Owner:** backend-developer
**Status:** in_progress
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-13
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (Approval, decision 3); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

The human approved read-write mounting of the user's repository (C015) **on the
condition** that the sandbox tiering is trustworthy for the default (non-KVM) path.
Most v1.0 users will run without KVM — the degraded path *is* the product for them.
Audit the degraded sandbox path, define "trustworthy" as concrete isolation properties,
close the gaps that are closable at v1.0 scope, and produce the evidence artifact that
C015's read-write default cites.

## Inputs

- `sandbox/router.go` (existing tier routing + degraded-mode detection)
- `sandbox/README.md`, `sandbox/probe/`
- `deploy/docker-compose.yml` (container hardening posture: `read_only`, `cap_drop`, `security_opt`, tmpfs)
- `input/CWSO_ Agentic AI Orchestration Blueprint.md` §2.4 (tiered isolation intent)

## Rails (read before starting)

### You MUST
- Define "trustworthy" as an explicit, testable property list for the non-KVM tier, covering at minimum: filesystem confinement (what a sub-agent process can reach outside its workspace), process isolation, resource limits (CPU/memory/pids), and network policy
- Audit the current degraded path against each property: met / partially met / not met, with code references
- Close gaps that are closable within the existing container/compose posture (e.g., `no-new-privileges`, dropped caps, tmpfs limits, network policy) — small, reviewable hardening changes
- Produce `docs/artifacts/sandbox-trustworthiness-v1.md`: the property list, the audit result per property with evidence (command + output), the hardening changes made, and a plain verdict per property
- State plainly what the non-KVM tier does **not** guarantee (this feeds C063's LIMITATIONS.md)

### You MUST NOT
- Promote or extend the Firecracker microVM tier — v1.0 ships the fallback path, documented (roadmap §2.4)
- Weaken any existing hardening to make a property pass
- Claim a property is met without executable evidence in the artifact
- Change the MCP tool surface or the orchestrator's dispatch semantics
- **If a required property cannot be met on the non-KVM path: STOP.** Do not ship a softened claim. Report `technical` / `critical` with the evidence — the read-write default then requires a human re-decision (orchestrator escalates)

## File ownership

- **May create/modify:** `sandbox/**`, `deploy/docker-compose.yml` (sandbox-relevant hardening keys only, with justifying comments), `docs/artifacts/sandbox-trustworthiness-v1.md` (new)
- **Must NOT touch:** `orchestrator/*` (except reading), `services/*`, other docs

## Steps (execute in order)

1. Read the sandbox tiering code and Blueprint §2.4.
2. Write the property list (agree it in the MR description before deep work).
3. Audit each property against the degraded path; capture evidence.
4. Implement closable hardening gaps.
5. Re-run evidence; write the artifact with per-property verdicts.

## Expected outputs

- `docs/artifacts/sandbox-trustworthiness-v1.md` (property list + evidence + verdicts)
- Minimal hardening diffs in `sandbox/**` / compose (if gaps found)

## Acceptance criteria

1. Every property has a verdict backed by executable evidence in the artifact
2. Hardening changes (if any) keep the smoke test green
3. The artifact's "not guaranteed" section is written plainly (feeds C063)
4. C015 can cite this artifact for its read-write default

## Verification commands

```bash
bash scripts/cwso-smoke-test.sh   # still green after any hardening
grep -c "met\|partially met\|not met" docs/artifacts/sandbox-trustworthiness-v1.md
git diff --stat
```

## Git rails

- Branch: `agent/backend-developer/C019` from `develop` (rebased on merged C010)
- Commit: `feat(sandbox): audit and harden non-KVM sandbox tier trustworthiness`
- MR target: `develop`, squash and merge, delete source branch
- Request security-engineer review on the MR (read-only reviewer per permission classification)

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
Remember the hard rail: an unmeetable property is a `critical` escalation, not a
documentation task.

## Execution notes

<filled during execution>
