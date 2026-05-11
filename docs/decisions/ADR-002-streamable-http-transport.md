# ADR-002: MCP Streamable HTTP transport (spec 2025-03-26)

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect

## Context
The MCP spec offers stdio, SSE-only, and Streamable HTTP transports. A swarm orchestrator needs (a) network reachability for distributed clients, (b) unidirectional server→client telemetry without polling, (c) non-blocking tool dispatch. The newer 2025-11-25 spec adds task envelopes; adoption is still early.

## Decision
Implement **both stdio and Streamable HTTP** transports, pinning to MCP spec **2025-03-26**. Streamable HTTP uses POST for client→server JSON-RPC and a persistent SSE stream for server→client notifications. The 2025-11-25 task semantics will be evaluated as ADR-007 during Phase 4.

## Consequences
- (+) stdio retained for `mcp-inspector` and Claude Desktop local use.
- (+) Streamable HTTP enables async dispatch with HTTP 202 + UUIDs.
- (+) SSE eliminates polling and keeps LLM context window clean.
- (−) DNS-rebinding risk on HTTP — mitigated via mandatory `Origin` validation (FR-7.1).
- (−) Spec churn risk — mitigated by version pinning and ADR-007 follow-up.
