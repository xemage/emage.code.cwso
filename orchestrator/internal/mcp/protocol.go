// Package mcp implements the subset of the Model Context Protocol (spec 2025-03-26)
// required for Phase 1: JSON-RPC 2.0 envelopes, initialize handshake, tools/list,
// tools/call. Streaming notifications (SSE) are added in Phase 3.
//
// This is a hand-rolled minimal implementation rather than the official
// go-sdk to avoid network-dependent module fetches during the initial PoC
// build and to keep the dependency surface auditable. The official SDK
// will be adopted at T029 (PoC-debt remediation pass).
//
// POC-DEBT: Hand-rolled MCP subset; production must adopt
// github.com/modelcontextprotocol/go-sdk for full spec compliance and
// upstream maintenance. Tracked in POC-DEBT-SCORECARD-phase1.md.
package mcp

import (
	"encoding/json"
	"errors"
)

// JSON-RPC 2.0 envelope types.
const (
	JSONRPCVersion = "2.0"

	// Standard JSON-RPC error codes.
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603

	// MCP-specific error codes.
	ErrUnauthorized     = -32001
	ErrPermissionDenied = -32002
	ErrToolNotFound     = -32010
	ErrToolExecution    = -32011
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

// ParseRequest unmarshals and validates a JSON-RPC request envelope.
func ParseRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if req.JSONRPC != JSONRPCVersion {
		return nil, errors.New("invalid jsonrpc version")
	}
	if req.Method == "" {
		return nil, errors.New("missing method")
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

// SupportedProtocolVersion is the MCP spec version pinned per ADR-002.
const SupportedProtocolVersion = "2025-03-26"
