# Host Readiness v1

Based on: docs/tasks/task-T040.md, docs/plans/plan-T040-phase4-host-validation.md, docs/decisions/ADR-003-tiered-sandbox-strategy.md

## Purpose

This artifact records read-only host capability checks required before enabling Firecracker workloads in Phase 4. The probe is designed for containerized execution only.

## Probe Contract

Probe path: `sandbox/probe/host_probe.sh`

JSON fields emitted:
- `kvm` (bool)
- `vhost_net` (bool)
- `kernel_version` (string)
- `kernel_gte_5_10` (bool)
- `runsc_ok` (bool)
- `firecracker_ok` (bool)

Decision rule:
- Firecracker-ready host: `firecracker_ok=true`
- Degraded host (gVisor-only): `firecracker_ok=false`

## Containerized Invocation

```bash
docker build -t cwso-host-probe:local -f sandbox/probe/Dockerfile sandbox/probe

docker run --rm --privileged --cap-add=SYS_PTRACE -v /:/host:ro cwso-host-probe:local
```

Notes:
- Probe is read-only and does not mutate host settings.
- Capability absence is reported as `false`; probe still exits successfully.

## Capability Matrix By Host Class

| Host class | kvm | vhost_net | kernel >= 5.10 | runsc_ok | firecracker_ok | Routing recommendation |
|---|---|---|---|---|---|---|
| Developer laptop (Docker Desktop/WSL/common local VM) | Often false | Often false | Usually true | Varies | false | Degraded gVisor-only |
| CI runner (shared, nested virt disabled) | Usually false | Usually false | Usually true | Varies | false | Degraded gVisor-only |
| Bare-metal Linux with KVM enabled and Firecracker installed | true | true | true | true/false | true | Full tiered routing |

## Local Validation Snapshot (2026-05-15)

Sample output from containerized probe run on the local dev host:

```json
{
  "kvm": false,
  "vhost_net": false,
  "kernel_version": "6.6.114.1-microsoft-standard-WSL2",
  "kernel_gte_5_10": true,
  "runsc_ok": false,
  "firecracker_ok": false
}
```

Assessment:
- Local host is not Firecracker-ready.
- Local host is valid for degraded gVisor-only development and validation.

## Operational Handoff

- Consumers: T041, T042, T043, T044
- If target production host reports `firecracker_ok=false`, escalate blocker:
  - type: `external`
  - severity: `major`
  - rationale: host virtualization capability is outside application code remediation
