# Technical Debt Register

Active engineering debt tracked for production remediation. Historical PoC-phase scorecards
(Phase 1–2 hypotheses and inventories) live in `docs/archive/debt/`. PoC `POC-DEBT` tags in
code should be scanned periodically and promoted here or closed.

This file tracks known technical debt items that are not blocking but must be remediated before production.

| ID | File | Location | Category | Description | Effort | Tracked since |
|----|------|----------|----------|-------------|--------|---------------|
| TD-01 | internal/transport/http.go | handleBrokerSSE (64 lines), handleSSE (59 lines) | Style | Both functions exceed the ≤50-line limit. Extract SSE loop body and frame-flush logic into helpers. | S | 2026-05-15 |
| TD-02 | internal/transport/http.go | RunHTTP (7 params), newHTTPHandler (7 params) | Style | Both exceed the ≤4-parameter project standard. Introduce a `HTTPHandlerConfig` struct to group related options. | S | 2026-05-15 |
| TD-03 | internal/transport/http.go | handlePOST (5 params), handleSSE (5 params), publishSampleEvents (6 params) | Style | Multiple internal helpers exceed the 4-parameter limit. Group into options structs. | S | 2026-05-15 |
| TD-04 | internal/transport/http_test.go | handleBrokerSSE deferred log path | Testing | Telemetry-on-close log path (defer in handleBrokerSSE) has no dedicated unit test. Covered indirectly by integration tests only. | S | 2026-05-15 |
| TD-05 | internal/jobs/manager.go | publish() ~line 320 | Observability | Publish errors silently discarded (not even DEBUG logged). Add DEBUG-level log on publish failure. | S | 2026-05-15 |
| TD-06 | internal/jobs/manager.go | Close() | Correctness | Jobs queued but not yet dequeued at Close() time are stranded in StateQueued. Their contexts are cancelled but no StateCancelled transition is published. Add drain loop in Close() to transition remaining queued jobs. | M | 2026-05-15 |
| TD-07 | internal/memorybroker/broker.go | Close() | Correctness | Double-close guard uses select idiom rather than sync.Once. Replace with sync.Once for race-free protection. | S | 2026-05-15 |

| TD-08 | orchestrator/internal/jobs/manager.go | publishTransition | Security | Job error strings (from Run func) are broadcast verbatim in SSE job-state notifications. If a tool's run function logs sensitive data (file paths, env variables), it is exposed to all SSE subscribers. Consider scrubbing error messages before broadcast. | M | 2026-05-15 |
| TD-09 | orchestrator/internal/transport/http.go | SSE connection store | Reliability | sseConns map is never evicted. IPs that disconnect without incrementing beyond the cap still hold a slot until release() is called. Under normal operation the defer release() ensures correctness; the map grows O(unique IPs) but entries are released immediately — acceptable for PoC. | S | 2026-05-15 |
## Legend
- Effort: `S` = small (<1h), `M` = medium (1–4h), `L` = large (>4h)
- Category: `Style` | `Testing` | `Observability` | `Correctness` | `Security` | `Reliability`
