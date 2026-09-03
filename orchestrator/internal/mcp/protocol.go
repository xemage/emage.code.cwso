// Package mcp implements the CWSO JSON-RPC 2.0 / MCP protocol kernel: envelope
// types, MCP payload structs, and error-code constants for the spec version
// pinned in ADR-002 (2025-03-26).
//
// This is a hand-rolled implementation rather than the official go-sdk. That
// choice was revisited for v1.0 and reaffirmed in
// docs/decisions/ADR-013-mcp-protocol-path.md: the kernel's synchronous,
// single-path dispatch (Server.Handle in
// orchestrator/internal/server/server.go) is a deliberate determinism
// property the SDK's async-per-session model could not be verified to
// preserve. Rather than adopting the SDK, CWSO backs the hand-rolled kernel
// with a conformance suite (this package's tests plus
// orchestrator/internal/server/mcp_conformance_test.go) asserting spec-shaped
// request/response/error behavior for every method the server implements,
// and a correct, spec-shaped "not supported" error for every method it does
// not. The full method/notification/error-code inventory the suite covers is
// docs/artifacts/mcp-gap-analysis-v1.md.
package mcp

import (
	"encoding/json"
	"errors"
)

// JSON-RPC 2.0 envelope types.
const (
	JSONRPCVersion = "2.0"

	// Standard JSON-RPC error codes (JSON-RPC 2.0 base spec, adopted by MCP).
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603

	// CWSO-specific error codes, in the JSON-RPC 2.0 reserved
	// implementation-defined server-error range (-32000..-32099).
	//
	// -32001 (ErrUnauthorized) was removed here (C032, per ADR-013's
	// required decision on this dead constant): authentication failures are
	// handled entirely at the HTTP transport layer (401/403 — see
	// transport/http.go) before a JSON-RPC envelope is ever parsed, so a
	// JSON-RPC-level auth error code had no reachable call site. Removing
	// the dead constant (rather than inventing a new JSON-RPC-level
	// auth-failure path solely to give it one) leaves wire behavior
	// unchanged and avoids implying a code path that does not exist.
	ErrPermissionDenied = -32002
	ErrToolNotFound     = -32010
	ErrToolExecution    = -32011
	ErrResourceNotFound = -32020
)

// Request is an inbound JSON-RPC message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether this message has no ID (no response expected).
func (r *Request) IsNotification() bool { return len(r.ID) == 0 || string(r.ID) == "null" }

// Response is an outbound JSON-RPC message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewError constructs a new JSON-RPC error.
func NewError(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

// ErrorResponse builds a JSON-RPC error response.
func ErrorResponse(id json.RawMessage, err *Error) *Response {
	return &Response{JSONRPC: JSONRPCVersion, ID: id, Error: err}
}

// OK builds a successful response.
func OK(id json.RawMessage, result any) *Response {
	return &Response{JSONRPC: JSONRPCVersion, ID: id, Result: result}
}

// RequestError wraps a JSON-RPC error code together with the underlying
// cause, letting callers (Server.Handle) select the spec-correct JSON-RPC
// error code for a ParseRequest failure instead of collapsing every failure
// mode to Parse error (-32700).
//
// Per JSON-RPC 2.0, -32700 (Parse error) is reserved for JSON that cannot be
// parsed at all; a syntactically valid envelope with the wrong protocol
// version or a missing method is Invalid Request (-32600). This distinction
// was previously not made (all three ParseRequest failure branches mapped to
// -32700) — a spec-correctness bug identified in the C030 gap analysis and
// fixed here in C032 per ADR-013.
type RequestError struct {
	Code int
	Err  error
}

func (e *RequestError) Error() string { return e.Err.Error() }
func (e *RequestError) Unwrap() error { return e.Err }

// ParseRequest unmarshals and validates a JSON-RPC request envelope.
func ParseRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, &RequestError{Code: ErrParse, Err: err}
	}
	if req.JSONRPC != JSONRPCVersion {
		return nil, &RequestError{Code: ErrInvalidRequest, Err: errors.New("invalid jsonrpc version")}
	}
	if req.Method == "" {
		return nil, &RequestError{Code: ErrInvalidRequest, Err: errors.New("missing method")}
	}
	return &req, nil
}

// InitializeParams matches the MCP initialize request.
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      ClientInfo     `json:"clientInfo,omitempty"`
}

// ClientInfo identifies the calling client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is sent by the server in response to initialize.
type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
}

// ServerInfo identifies the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool describes a registered tool returned by tools/list.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolsListResult is the response payload for tools/list.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams is the inbound params for tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult is the response payload for tools/call.
// Per spec, content is a list of typed blocks; we use simple text blocks.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a typed result item ("text" only for now).
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// TextResult builds a single-text-block successful tool result.
func TextResult(text string) *ToolCallResult {
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: text}}}
}

// TextError builds a single-text-block error tool result.
func TextError(text string) *ToolCallResult {
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: text}}, IsError: true}
}

// --- MCP Resources (spec 2025-03-26 §Resources) ---
//
// Added in Phase 7 (T117) to expose AST write-spike streams as subscribable
// resources under the cwso:// scheme. Resource contents are JSON text blocks
// (a replay snapshot from the broker); live updates are delivered over the
// existing SSE transport scoped to the subscription id.

// Resource is a concrete resource returned by resources/list.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceTemplate is a parameterized resource URI returned by resources/templates/list.
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult is the response payload for resources/list.
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceTemplatesListResult is the response payload for resources/templates/list.
type ResourceTemplatesListResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

// ResourceURIParams is the inbound params for resources/read|subscribe|unsubscribe.
type ResourceURIParams struct {
	URI string `json:"uri"`
}

// ResourceContents is a single content item returned by resources/read.
type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ResourceReadResult is the response payload for resources/read.
type ResourceReadResult struct {
	Contents []ResourceContents `json:"contents"`
}

// SupportedProtocolVersion is the MCP spec version pinned per ADR-002.
const SupportedProtocolVersion = "2025-03-26"
