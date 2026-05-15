# Degraded Mode v1 (gVisor-only)

Based on: docs/tasks/task-T040.md, docs/plans/plan-T040-phase4-host-validation.md, docs/decisions/ADR-003-tiered-sandbox-strategy.md

## Purpose

Define operator actions when Firecracker prerequisites are absent on a host. In degraded mode, sandbox routing is forced to gVisor and Firecracker workloads are not scheduled.

## Activation Conditions

Run the host probe in containerized mode:

```bash
docker run --rm --privileged --cap-add=SYS_PTRACE -v /:/host:ro cwso-host-probe:local
```

Activate degraded mode when either condition is true:
- `firecracker_ok=false`
- `kvm=false` or `vhost_net=false` or `kernel_gte_5_10=false`

## Routing Controls (Current Repository State)

Current codebase does not yet expose dedicated environment flags for tier-router default selection (planned for T044).

Use these existing routing controls to force gVisor-only behavior:
- Request schema field `sandbox_profile` in `schemas/create_shadow_workspace.json`.
- Allowed value for degraded mode: `gvisor-fast-ephemeral`.
- Do not dispatch `firecracker-secure-isolation` requests while host is degraded.

Required operator rule in degraded mode:
- Every orchestrator request that sets sandbox profile must set `sandbox_profile=gvisor-fast-ephemeral`.

## Step-by-Step Operator Runbook

1. Build and run the host probe container.
2. Store emitted JSON with deployment metadata (host name, timestamp, kernel).
3. If `firecracker_ok=true`, continue with full tier routing.
4. If `firecracker_ok=false`, enforce `sandbox_profile=gvisor-fast-ephemeral` for all dispatches.
5. Mark host as degraded in deployment records.
6. Re-run probe after host-level virtualization remediation.

## Guardrails

- Do not perform host kernel modifications from orchestrator deployment automation.
- Do not fail orchestrator startup solely due to missing KVM on dev/CI hosts.
- Treat Firecracker absence as a routing decision, not a probe failure.

## Exit Criteria

Degraded mode can be removed only when a new probe output shows:
- `kvm=true`
- `vhost_net=true`
- `kernel_gte_5_10=true`
- `firecracker_ok=true`

Then Firecracker scheduling may be re-enabled by allowing `firecracker-secure-isolation` requests again.
