package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

// startFakeHAL runs a minimal length-prefixed JSON UDS server that mimics cwso-hal.
// It signals on infered for every `infer` op it receives and always replies success.
func startFakeHAL(t *testing.T, socket string, infered chan<- map[string]any) {
	t.Helper()
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen fake hal: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(socket) })

	readFrame := func(c net.Conn) ([]byte, error) {
		hdr := make([]byte, 4)
		if _, err := readFull(c, hdr); err != nil {
			return nil, err
		}
		n := binary.BigEndian.Uint32(hdr)
		body := make([]byte, n)
		if _, err := readFull(c, body); err != nil {
			return nil, err
		}
		return body, nil
	}
	writeFrame := func(c net.Conn, body []byte) {
		hdr := make([]byte, 4)
		binary.BigEndian.PutUint32(hdr, uint32(len(body)))
		_, _ = c.Write(hdr)
		_, _ = c.Write(body)
	}

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
				var env struct {
					ID     string          `json:"id"`
					Op     string          `json:"op"`
					Params json.RawMessage `json:"params"`
				}
				_ = json.Unmarshal(body, &env)
				if env.Op == "infer" {
					var params map[string]any
					_ = json.Unmarshal(env.Params, &params)
					select {
					case infered <- params:
					default:
					}
				}
				result, _ := json.Marshal(map[string]any{
					"served_by":          "cpu-baseline",
					"requested_provider": "lpu-realtime",
					"fallback_count":     1,
					"completion": map[string]any{
						"provider_id": "cpu-baseline", "output": "ok",
						"tokens_in": 1, "tokens_out": 1, "latency_ms": 1, "deterministic": true,
					},
					"attempts": []any{},
				})
				resp, _ := json.Marshal(map[string]any{"id": env.ID, "ok": true, "result": json.RawMessage(result)})
				writeFrame(c, resp)
			}(conn)
		}
	}()
}

func readFull(c net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// TestHardwareAwareDispatchLiveHALIntegration exercises the full config → server →
// hal.Client → UDS path: a hardware-aware dispatch must register the tool and
// execute the dispatched job against the live (fake) HAL sidecar.
func TestHardwareAwareDispatchLiveHALIntegration(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "hal.sock")
	infered := make(chan map[string]any, 4)
	startFakeHAL(t, socket, infered)

	cfg := &config.Config{
		Transport:                "stdio",
		LogLevel:                 "error",
		Workspace:                dir,
		AllowedOrigins:           []string{"http://localhost"},
		HHDCapabilityRegistry:    true,
		HHDHardwareAwareDispatch: true,
		HHDPolicyEngineV2:        true,
		HHDPolicyMinConfidence:   0.2,
		HHDPolicyMaxQueueDepth:   16,
		HALSocket:                socket,
		HHDSnapshotTTLSeconds:    30,
		JobTimeoutSeconds:        30,
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// The tool must be registered when hardware-aware dispatch is enabled.
	listReq := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	out, err := s.Handle(context.Background(), &transport.Session{Role: "orchestrator"}, listReq)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if !strings.Contains(string(out), `"dispatch_hardware_aware_job"`) {
		t.Fatalf("expected dispatch_hardware_aware_job in tools list: %s", out)
	}

	callReq := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"dispatch_hardware_aware_job","arguments":{"task_description":"fix typo in handler","context_size_estimate":1000,"latency_requirement":"realtime"}}}`)
	out, err = s.Handle(context.Background(), &transport.Session{Role: "orchestrator"}, callReq)
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if !strings.Contains(string(out), "job_id") {
		t.Fatalf("expected job_id in dispatch result: %s", out)
	}

	select {
	case params := <-infered:
		if params["selected_provider"] != "lpu-realtime" {
			t.Fatalf("HAL infer selected_provider = %v, want lpu-realtime", params["selected_provider"])
		}
		req, _ := params["request"].(map[string]any)
		if req == nil || req["prompt"] != "fix typo in handler" {
			t.Fatalf("HAL infer request not forwarded faithfully: %+v", params["request"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for live HAL infer call")
	}
}
