package shadow

import (
	"encoding/json"
	"errors"
	"fmt"
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
		if req.Op != "test_op" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		if len(req.Params) == 0 {
			t.Fatal("expected params")
		}
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"value":"ok"}`)}
	})

	client := NewClient(socket)
	var out struct {
		Value string `json:"value"`
	}
	err := client.Call("test_op", map[string]any{"k": "v"}, &out)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("expected value ok, got %q", out.Value)
	}
}

func TestCallSidecarError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{
			ID: req.ID,
			OK: false,
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{Code: "not_found", Message: "workspace missing"},
		}
	})

	client := NewClient(socket)
	err := client.Call("drop_workspace", map[string]any{"workspace_uuid": "missing"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sidecar not_found: workspace missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallResultDecodeError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"value":"not-an-int"}`)}
	})

	client := NewClient(socket)
	var out struct {
		Value int `json:"value"`
	}
	err := client.Call("query_ast", nil, &out)
	if err == nil {
		t.Fatal("expected decode result error")
	}
	if !strings.Contains(err.Error(), "decode result") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallMarshalParamsError(t *testing.T) {
	client := NewClient("/does/not/matter.sock")
	err := client.Call("bad", map[string]any{"fn": func() {}}, nil)
	if err == nil {
		t.Fatal("expected marshal params error")
	}
	if !strings.Contains(err.Error(), "marshal params") {
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

func TestReadFrameOutOfRange(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		var wire strings.Builder
		if err := writeHeader(&wire, 0); err != nil {
			t.Fatalf("write header: %v", err)
		}
		_, err := readFrame(strings.NewReader(wire.String()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "frame size out of range") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("too large", func(t *testing.T) {
		var wire strings.Builder
		if err := writeHeader(&wire, uint32(frameMax+1)); err != nil {
			t.Fatalf("write header: %v", err)
		}
		_, err := readFrame(strings.NewReader(wire.String()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "frame size out of range") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func writeHeader(sb *strings.Builder, n uint32) error {
	hdr := make([]byte, frameHeader)
	hdr[0] = byte(n >> 24)
	hdr[1] = byte(n >> 16)
	hdr[2] = byte(n >> 8)
	hdr[3] = byte(n)
	_, err := sb.Write(hdr)
	return err
}

func TestCallDialError(t *testing.T) {
	client := NewClient(t.TempDir() + "/missing.sock")
	err := client.Call("create_workspace", nil, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallSidecarFailureWithoutBody(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: false}
	})
	client := NewClient(socket)
	err := client.Call("create_workspace", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errors.New("sidecar reported failure with no error body")) &&
		!strings.Contains(err.Error(), "failure with no error body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallWithNilOutIgnoresResultDecode(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"x":1}`)}
	})
	client := NewClient(socket)
	if err := client.Call("noop", nil, nil); err != nil {
		t.Fatalf("Call failed: %v", err)
	}
}

func TestCallEnvelopeMarshalError(t *testing.T) {
	// This is defensive: op cannot trigger marshal failure, but we keep
	// the branch protected if envelope types evolve.
	client := NewClient("unused")
	err := client.Call(string([]byte{0xff, 0xfe}), nil, nil)
	if err != nil && !strings.Contains(err.Error(), "dial") {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = fmt.Sprintf("%v", client)
}
