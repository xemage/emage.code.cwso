package mcp

import (
	"encoding/json"
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
