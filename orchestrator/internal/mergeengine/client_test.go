package mergeengine

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
)

func startTestSidecar(t *testing.T, handler func(envelope) response) string {
	t.Helper()
	socket := t.TempDir() + "/sidecar.sock"

	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				body, err := readFrame(c)
				if err != nil {
					return
				}
				var req envelope
				if err := json.Unmarshal(body, &req); err != nil {
					return
				}
				resp := handler(req)
				respBody, err := json.Marshal(resp)
				if err != nil {
					return
				}
				_ = writeFrame(c, respBody)
			}(conn)
		}
	}()

	return socket
}

func TestCallSuccess(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		if req.Op != "merge_three_way" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"merged_b64":"b2s="}`)}
	})

	client := NewClient(socket)
	var out struct {
		MergedB64 string `json:"merged_b64"`
	}
	err := client.Call("merge_three_way", map[string]any{"language": "go"}, &out)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if out.MergedB64 != "b2s=" {
		t.Fatalf("expected merged payload, got %q", out.MergedB64)
	}
}

func TestCallSidecarError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: false, Error: &struct {
			Code       string `json:"code"`
			Class      string `json:"class,omitempty"`
			ReasonCode string `json:"reason_code,omitempty"`
			Message    string `json:"message"`
		}{
			Code:       "merge_conflict",
			Class:      "semantic_conflict",
			ReasonCode: "ast_overlap_conflict",
			Message:    "AST semantic overlap conflict",
		}}
	})

	client := NewClient(socket)
	err := client.Call("merge_three_way", map[string]any{"language": "go"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var sidecarErr *SidecarError
	if !errors.As(err, &sidecarErr) {
		t.Fatalf("expected SidecarError, got %T (%v)", err, err)
	}
	if sidecarErr.Code != "merge_conflict" {
		t.Fatalf("unexpected sidecar code: %q", sidecarErr.Code)
	}
	if sidecarErr.Class != "semantic_conflict" {
		t.Fatalf("unexpected sidecar class: %q", sidecarErr.Class)
	}
	if sidecarErr.ReasonCode != "ast_overlap_conflict" {
		t.Fatalf("unexpected reason code: %q", sidecarErr.ReasonCode)
	}
}

func TestCallDecodeResultError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"merged_b64":10}`)}
	})

	client := NewClient(socket)
	var out struct {
		MergedB64 string `json:"merged_b64"`
	}
	err := client.Call("merge_three_way", nil, &out)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode result") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	payload := make([]byte, frameMax+1)
	err := writeFrame(&strings.Builder{}, payload)
	if err == nil {
		t.Fatal("expected frame too large error")
	}
	if !strings.Contains(err.Error(), "frame too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallDialError(t *testing.T) {
	client := NewClient(t.TempDir() + "/missing.sock")
	err := client.Call("merge_three_way", nil, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatalf("unexpected error: %v", err)
	}
}
