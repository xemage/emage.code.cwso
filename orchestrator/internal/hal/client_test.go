package hal

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeHAL is a minimal length-prefixed JSON UDS server that mimics cwso-hal for
// one request/response per connection.
func fakeHAL(t *testing.T, handler func(op string, params json.RawMessage) (any, *struct {
	Code, Message string
})) string {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "hal.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(socket) })

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
				var env envelope
				if err := json.Unmarshal(body, &env); err != nil {
					return
				}
				result, errObj := handler(env.Op, env.Params)
				resp := map[string]any{"id": env.ID, "ok": errObj == nil}
				if errObj != nil {
					resp["error"] = map[string]any{"code": errObj.Code, "message": errObj.Message}
				} else if result != nil {
					rb, _ := json.Marshal(result)
					resp["result"] = json.RawMessage(rb)
				}
				rb, _ := json.Marshal(resp)
				hdr := make([]byte, frameHeader)
				binary.BigEndian.PutUint32(hdr, uint32(len(rb)))
				_, _ = c.Write(hdr)
				_, _ = c.Write(rb)
			}(conn)
		}
	}()
	return socket
}

func TestClientInferRoundTrip(t *testing.T) {
	var gotOp string
	var gotParams inferParams
	socket := fakeHAL(t, func(op string, params json.RawMessage) (any, *struct{ Code, Message string }) {
		gotOp = op
		_ = json.Unmarshal(params, &gotParams)
		return InferResult{
			ServedBy:          "lpu-realtime",
			RequestedProvider: "lpu-realtime",
			FallbackCount:     0,
			Completion:        Completion{ProviderID: "lpu-realtime", Output: "ok", TokensOut: 3, Deterministic: true},
			Attempts:          []Attempt{{ProviderID: "lpu-realtime", Outcome: "served"}},
		}, nil
	})

	client := NewClient(socket)
	res, err := client.Infer(context.Background(), "lpu-realtime", []string{"lpu-realtime", "cpu-baseline"}, InferenceRequest{
		Prompt:        "fix typo",
		ContextTokens: 1000,
		WorkloadTags:  []string{"realtime"},
		LatencyClass:  "realtime",
	})
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if gotOp != "infer" {
		t.Fatalf("op = %q, want infer", gotOp)
	}
	if gotParams.SelectedProvider != "lpu-realtime" {
		t.Fatalf("selected = %q, want lpu-realtime", gotParams.SelectedProvider)
	}
	if gotParams.Request.Prompt != "fix typo" || gotParams.Request.ContextTokens != 1000 {
		t.Fatalf("request not forwarded faithfully: %+v", gotParams.Request)
	}
	if res.ServedBy != "lpu-realtime" || res.Completion.Output != "ok" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestClientInferStructuredError(t *testing.T) {
	socket := fakeHAL(t, func(op string, params json.RawMessage) (any, *struct{ Code, Message string }) {
		return nil, &struct{ Code, Message string }{Code: "unavailable", Message: "all backends down"}
	})
	client := NewClient(socket)
	_, err := client.Infer(context.Background(), "gpu-accelerated", nil, InferenceRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *SidecarError
	if !errors.As(err, &se) {
		t.Fatalf("expected *SidecarError, got %T: %v", err, err)
	}
	if se.Code != "unavailable" {
		t.Fatalf("code = %q, want unavailable", se.Code)
	}
}

func TestClientDialFailure(t *testing.T) {
	client := NewClient(filepath.Join(t.TempDir(), "does-not-exist.sock"))
	_, err := client.Infer(context.Background(), "cpu-baseline", nil, InferenceRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestClientInferContextCancelled(t *testing.T) {
	// A server that accepts but never replies, so the call blocks until ctx cancels it.
	dir := t.TempDir()
	socket := filepath.Join(dir, "hal.sock")
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
			_ = conn // hold the connection open without replying
		}
	}()

	client := NewClient(socket)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err = client.Infer(ctx, "cpu-baseline", nil, InferenceRequest{Prompt: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
}

func TestClientInferContextDeadline(t *testing.T) {
	client := NewClient(filepath.Join(t.TempDir(), "hal.sock"))
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := client.Infer(ctx, "cpu-baseline", nil, InferenceRequest{Prompt: "x"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}
