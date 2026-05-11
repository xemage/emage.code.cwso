# Sandbox runners (Phase 4)

This directory will host the sandbox runner integrations:

| Runner | File | Phase |
|--------|------|-------|
| Docker (trusted) | `runner_docker.go` | 4 (T041) |
| gVisor / runsc | `runner_gvisor.go` | 4 (T042) |
| Firecracker microVM + snapshots | `runner_firecracker.go` | 4 (T043) |

A common `RunnerInterface` lives at `orchestrator/internal/sandbox/runner.go` (to be created in T041).
