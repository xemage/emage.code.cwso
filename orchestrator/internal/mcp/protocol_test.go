package mcp

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseRequestAndNotification(t *testing.T) {
	req, err := ParseRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"ok":true}}`))
	if err != nil {
		t.Fatalf("parse valid request: %v", err)
	}
	if req.Method != "ping" {
		t.Fatalf("unexpected method: %q", req.Method)
	}
	if req.IsNotification() {
		t.Fatal("request with id should not be notification")
	}

	notification, err := ParseRequest([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil {
		t.Fatalf("parse notification: %v", err)
	}
	if !notification.IsNotification() {
		t.Fatal("request without id should be notification")
	}

	reqNullID := &Request{JSONRPC: JSONRPCVersion, ID: json.RawMessage("null"), Method: "ping"}
	if !reqNullID.IsNotification() {
		t.Fatal("request with null id should be notification")
	}
}

func TestParseRequestValidationErrors(t *testing.T) {
	if _, err := ParseRequest([]byte(`{"jsonrpc":"1.0","method":"ping"}`)); err == nil {
		t.Fatal("expected jsonrpc version error")
	}
	if _, err := ParseRequest([]byte(`{"jsonrpc":"2.0"}`)); err == nil {
		t.Fatal("expected missing method error")
	}
	if _, err := ParseRequest([]byte(`{`)); err == nil {
		t.Fatal("expected invalid json error")
	}
}

// TestParseRequestErrorCodes is a conformance test (ADR-013 §3a "Misuse
// finding" — required fix, closing the mcp-gap-analysis-v1.md ErrInvalidRequest
// row): per JSON-RPC 2.0, malformed JSON that cannot be parsed at all is
// Parse error (-32700); a syntactically valid envelope with the wrong
// "jsonrpc" version or a missing "method" is Invalid Request (-32600). Each of
// ParseRequest's three failure branches must independently select the
// spec-correct code via the returned *RequestError, not collapse to -32700.
func TestParseRequestErrorCodes(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantCode int
	}{
		{"malformed json -> Parse error", `{`, ErrParse},
		{"wrong jsonrpc version -> Invalid Request", `{"jsonrpc":"1.0","id":1,"method":"ping"}`, ErrInvalidRequest},
		{"missing method -> Invalid Request", `{"jsonrpc":"2.0","id":1}`, ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRequest([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected an error")
			}
			var reqErr *RequestError
			if !errors.As(err, &reqErr) {
				t.Fatalf("expected *RequestError, got %T: %v", err, err)
			}
			if reqErr.Code != tc.wantCode {
				t.Fatalf("expected code %d, got %d", tc.wantCode, reqErr.Code)
			}
		})
	}
}

// TestErrUnauthorizedRemoved documents the C032 decision (ADR-013 §3b
// "Decision needed in C032"): ErrUnauthorized (-32001) was dead code (zero
// reachable call sites — auth failures are handled entirely at the HTTP
// transport layer as 401/403 before a JSON-RPC envelope is ever parsed) and
// was removed rather than wired to an invented JSON-RPC-level path. This test
// asserts the decision stuck: the remaining reserved-range constants keep
// their documented values and -32001 is not silently reintroduced.
func TestErrUnauthorizedRemoved(t *testing.T) {
	reserved := map[string]int{
		"ErrPermissionDenied": ErrPermissionDenied,
		"ErrToolNotFound":     ErrToolNotFound,
		"ErrToolExecution":    ErrToolExecution,
		"ErrResourceNotFound": ErrResourceNotFound,
	}
	want := map[string]int{
		"ErrPermissionDenied": -32002,
		"ErrToolNotFound":     -32010,
		"ErrToolExecution":    -32011,
		"ErrResourceNotFound": -32020,
	}
	for name, code := range want {
		if reserved[name] != code {
			t.Fatalf("%s: expected %d, got %d", name, code, reserved[name])
		}
	}
	for _, code := range reserved {
		if code < -32099 || code > -32000 {
			t.Fatalf("reserved-range code %d falls outside JSON-RPC's implementation-defined server-error range (-32000..-32099)", code)
		}
	}
}

func TestResponseAndErrorHelpers(t *testing.T) {
	id := json.RawMessage(`1`)
	okResp := OK(id, map[string]any{"pong": true})
	if okResp.JSONRPC != JSONRPCVersion || okResp.Error != nil || okResp.Result == nil {
		t.Fatalf("unexpected OK response: %+v", okResp)
	}

	errObj := NewError(ErrInvalidParams, "bad input")
	if errObj.Code != ErrInvalidParams || errObj.Message != "bad input" {
		t.Fatalf("unexpected error object: %+v", errObj)
	}

	errResp := ErrorResponse(id, errObj)
	if errResp.JSONRPC != JSONRPCVersion || errResp.Error == nil {
		t.Fatalf("unexpected error response: %+v", errResp)
	}
	if errResp.Error.Code != ErrInvalidParams {
		t.Fatalf("unexpected error code: %d", errResp.Error.Code)
	}
}

func TestToolResultHelpers(t *testing.T) {
	ok := TextResult("done")
	if ok.IsError {
		t.Fatal("expected non-error text result")
	}
	if len(ok.Content) != 1 || ok.Content[0].Type != "text" || ok.Content[0].Text != "done" {
		t.Fatalf("unexpected success content: %+v", ok.Content)
	}

	err := TextError("failed")
	if !err.IsError {
		t.Fatal("expected error text result")
	}
	if len(err.Content) != 1 || err.Content[0].Text != "failed" {
		t.Fatalf("unexpected error content: %+v", err.Content)
	}
}
