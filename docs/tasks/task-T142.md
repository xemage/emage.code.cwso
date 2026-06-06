# Task T142 — Installation & usage documentation

- **Status:** done
- **Owner:** technical-writer / devops-engineer
- **Priority:** P0
- **Depends on:** T141 (RC published)
- **Based on:** `release-v0.3.0-rc1.md`, README.md, operator feedback for GA

## Objective

Ship operator-facing installation and usage documentation so adopters can run CWSO from
`v0.3.0-rc1` through GA without reading source code.

## Deliverables

- **`docs/user/installation-v1.md`** — prerequisites, compose quick start, JWT, MCP HTTP,
  Phase 4 / Next-Gen flags, troubleshooting
- README links to installation guide; status table updated for Phases 6–9

## Acceptance Criteria

- [x] Covers Docker-only path (no local Go/Rust required)
- [x] Documents JWT claims and MCP HTTP invocation
- [x] Lists Next-Gen feature flags with safe defaults
- [x] Troubleshooting section for common failures
- [x] Reviewed and merged; linked from README release section

## Notes

Extend with deployment topologies (K8s, multi-node) in a future v2 before production GA.
