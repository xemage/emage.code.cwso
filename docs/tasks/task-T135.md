# Task T135 — KV-cache prefix router

> **ID note:** roadmap **placeholder T109**. Active **T135** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T132 (cwso-rollout sidecar)
- **Phase:** 9 — Rollout-as-a-Service (Feature F)
- **Based on:** `docs/artifacts/rollout-architecture-v1.md` §6, ADR-010

## Objective

Replace the synthetic `prefix-{workspace_id[:8]}` key on `POST /rollout/task/submit` with a
deterministic BLAKE3 prefix router keyed by shadow workspace base tree OID, system prompt hash,
and shared read-files manifest hash. Optionally prewarm the cwso-rollout LRU cache via UDS IPC.

## Deliverables

- **`orchestrator/internal/rollout/prefix_router.go`** — key computation + prewarm orchestration
- **`orchestrator/internal/rollout/workspace_resolver.go`** — shadow workspace metadata resolver
- **`orchestrator/internal/rollout/client.go`** — `prefix_prewarm` / `prefix_stats` IPC
- **`services/cwso-git-shadow`** — `get_workspace` IPC (base_tree_oid + file manifest)
- **`services/cwso-rollout`** — LRU prefix cache + IPC handlers
- **Config** — `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` (default false)
- **Tests** — Go unit tests; Rust cache/IPC tests

## Acceptance Criteria

- [x] Prefix key = `blake3(base_tree_oid || system_prompt_hash || shared_read_files_hash)`
- [x] `POST /rollout/task/submit` resolves workspace when router enabled
- [x] Optional prewarm via cwso-rollout `prefix_prewarm` when socket configured
- [x] Feature flag off by default; no synthetic prefix when disabled
- [x] `go test ./... -race` green locally
- [ ] CI green on MR

## Notes

- System prompt hash from `CWSO_ROLLOUT_SYSTEM_PROMPT_HASH` or BLAKE3 of `CWSO_ROLLOUT_SYSTEM_PROMPT`.
- Base OID change invalidates key automatically via new manifest/OID inputs.
- Proxy hot-path differential prompting deferred; v1 PoC covers keying + LRU prewarm only.
