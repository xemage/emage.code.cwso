# Task T040 — KVM / Firecracker host validation

- Phase: **4 (Production)** · Owner: **devops-engineer** · Priority: **P0**
- Depends on: T037 · Blocks: T041, T042, T043
- Status: pending

## Objective
Validate up-front that the target host(s) for Phase 4 swarm execution have the kernel features required for Firecracker microVMs: KVM, vhost-net, kernel ≥ 5.10, and unprivileged user-namespace support for gVisor. Produce a host-readiness report and a documented "degraded mode" fallback path for hosts without KVM.

## Inputs
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §6
- [ADR-003](../decisions/ADR-003-tiered-sandbox-strategy.md)
- [security-baseline-v1.md](../artifacts/security-baseline-v1.md)

## Constraints
- No host kernel changes performed by this task — read-only assessment + scripts.
- Local dev host (Docker available, no guarantee of KVM) MUST be supported in degraded gVisor-only mode.
- All probing scripts must run inside a privileged container with `--cap-add=SYS_PTRACE`; never run on bare host.
- Output must be machine-readable so the orchestrator can self-select tier router defaults at boot.

## Expected outputs
- `sandbox/probe/host_probe.sh` — bash script printing JSON: `{kvm: bool, vhost_net: bool, kernel_version, kernel_gte_5_10: bool, runsc_ok: bool, firecracker_ok: bool}`
- `sandbox/probe/Dockerfile` — minimal probe image
- `docs/artifacts/host-readiness-v1.md` — report with capability matrix per known host class (laptop, CI runner, bare-metal)
- `docs/artifacts/degraded-mode-v1.md` — operator runbook for gVisor-only deployments

## Acceptance criteria
1. `host_probe.sh` produces valid JSON with all six fields populated on the local dev host.
2. Probe distinguishes KVM-capable from non-KVM hosts without false positives (verified by toggling `/dev/kvm` permission).
3. Degraded-mode runbook explains exactly which orchestrator config flags activate gVisor-only routing.
4. Probe image scanned with `trivy`; zero HIGH/CRITICAL CVEs.
5. Probe script passes `shellcheck` with no warnings.

## Blocker protocol
Same as T020. If host lacks KVM and target deployment requires it, escalate as `external` blocker (cannot be remediated in code).
