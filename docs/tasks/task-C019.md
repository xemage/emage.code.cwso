# Task C019 — Sandbox trustworthiness for the non-KVM default path

**ID:** C019
**Owner:** backend-developer
**Status:** done
**Priority:** P0
**Depends on:** C010
**Created:** 2026-08-13
**Completed:** 2026-08-16
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

A first session on this task was killed mid-work by an infra-level API session limit
(not a content failure) after producing an uncommitted draft artifact and compose
diff. A second session treated that draft as **unverified input, not a trusted
starting point**: independently re-derived the real sandbox code location
(`orchestrator/internal/sandbox/*`, correcting the brief's `sandbox/router.go`
assumption), re-checked every citation against current source, fixed several (wrong
line ranges, wrong function attribution for hardening defaults, a duplicate
list-numbering bug, a miscounted test total), replaced a synthetic/reconstructed
verification transcript with a real live `docker compose up`/`inspect`/`logs` run,
and independently re-derived the `network_mode: "none"` safety claim from
`Cargo.toml` dependency lists and source greps (zero networking crates, zero
`std::net::*` usage, no remote-git operations) rather than trusting the inherited
reasoning comments. All four required properties (P1-P4) verified MET; no hard-stop
condition triggered.

Two pre-existing, unrelated defects were incidentally discovered during live
verification (both reproduced identically with this task's compose diff stashed out,
confirming no regression): a `.env.jwt.dev` permission mismatch (C012's `chmod 600`
vs. the orchestrator container's non-root user) and a JWT 401 mismatch in
`scripts/phase2-integration.py`'s smoke test. Neither was fixed here (out of file
ownership) — logged as **T191** (P0) and **T192** (P1) respectively.

Independent security-engineer review (MR !123, required by this task's own brief)
returned **CONDITIONAL_PASS**: 0 critical, 0 high, 1 medium, 2 low findings. Every
independently-checkable claim reproduced cleanly against live source/containers,
including re-confirming `network_mode: none`'s safety via the reviewer's own
independent crate-source grep (matching this task's method rather than just trusting
its conclusion). Notably, the review found the artifact **under**-claims rather than
overclaims (§7/§8 actively narrow scope rather than soften a gap) — a good sign for a
safety-critical document. The one Medium finding, **SEC-C019-01**, is structural
rather than a defect in this task: this artifact covers container-level sandbox
tiering (P1-P4) only, not the `pathGuard`/`fs_tools.go` in-process trust boundary,
which is the actual exposure surface for C015's read-write mount decision — tracked
forward onto `docs/tasks/task-C015.md` (not closed here; C015 must not cite "P1-P4
MET" as blanket justification). The two Low findings (rollout's less-rigorous
`pids_limit` rationale comment; confirming the pipeline was fully green before merge)
were resolved/accepted as noted in `completed-tasks.md`.

Merged to `develop` 2026-08-16 (squash), MR !123 — closes the sequential
`deploy/docker-compose.yml` chain (C011 → C014 → C019).
