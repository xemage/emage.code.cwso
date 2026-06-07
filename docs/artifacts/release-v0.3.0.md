# Artifact: release-v0.3.0 (GA)

## Metadata
- Producer agent: release-manager
- Created: 2026-06-07
- Based on: `release-v0.3.0-rc1.md`, `checkpoint-012-nextgen-ga-prep.md`, T142/T147/T144
- **develop tip:** `50f3406` (pre-T152 packaging)
- **Prior RC tag:** `v0.3.0-rc1` @ `2032b33`

## Release intent

**v0.3.0** is the general-availability release for the Next-Gen line (Phases 6–9), incorporating
post-RC hardening, operator documentation, OpenAI Responses proxy parity, and Polar harness adapters.

## Scope vs v0.3.0-rc1

| Item | Task | GA status |
|------|------|-----------|
| Installation & usage docs | T142 | **Included** |
| OpenAI Responses API + proxy hardening | T147 | **Included** |
| Polar harness adapters + Docker runtime | T144 | **Included** |
| CI e2e MCP retry hardening | !55 | **Included** |
| KV prefix router | T135 | Included (default-off) |
| Blocking CI audits | T140 | Included |
| Polar T145–T151 | backlog | Deferred post-GA |

## Validation and CI evidence

- Phases 6–9 gates: **PASS/PASS** (unchanged from RC)
- `develop` CI green post T144 merge
- Installation guide: `docs/user/installation-v1.md`
- Harness tests: `go test ./internal/harness/...` 12/12 PASS

## GA verdict

**CONDITIONAL_PASS (GA_READY)**

Rationale:
- All RC scope plus post-RC deliverables merged and CI-validated.
- Stakeholder RC sign-off on `v0.3.0-rc1` remains external; GA proceeds with documented delta.

### Conditions
- Capture stakeholder feedback; open patch release if critical issues found.
- Polar parity backlog (T145–T151) tracked for v0.3.x follow-on.

## Release actions

1. Merge T152 MR after CI green.
2. Tag **`v0.3.0`** on `develop`.
3. Publish GitLab release with CHANGELOG excerpt.
