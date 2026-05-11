// Package transport implements stdio and Streamable HTTP transports for MCP.
//
// Transports operate on raw JSON bytes; protocol typing lives in package mcp
// and is handled by the server.
package transport

// Session is per-connection state carried through the handler.
type Session struct {
	// Role identifies the caller's permission tier ("orchestrator" or "worker").
	Role string
	// Subject is the authenticated principal (JWT sub claim) when present.
	Subject string
}
