package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

type fakeSparseSidecarServer struct {
	createCalls int
}

func startFakeSparse(t *testing.T) string {
	t.Helper()
	fake := &fakeSparseSidecarServer{}
	ln, err := net.Listen("unix", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go fake.handle(conn)
		}
	}()
	return ln.Addr().String()
}

func (f *fakeSparseSidecarServer) handle(conn net.Conn) {
	defer conn.Close()
	body, err := readSparseUDSFrame(conn)
	if err != nil {
		return
	}
	var env struct {
		ID string `json:"id"`
		Op string `json:"op"`
	}
	_ = json.Unmarshal(body, &env)

	var resp []byte
	if env.Op == "create_agent" {
		f.createCalls++
		resp, _ = json.Marshal(map[string]any{
			"id": env.ID, "ok": true,
			"result": map[string]any{
				"wasm_agent_id":   "sa-integration",
				"skill_domain":    "react-hooks",
				"slice_sha256":    "abc",
				"state":           "ready",
				"cold_start_ms":   2.5,
				"resident_ram_mb": 8.0,
			},
		})
	} else {
		resp, _ = json.Marshal(map[string]any{
			"id": env.ID, "ok": false,
			"error": map[string]string{"code": "invalid_input", "message": "unsupported op"},
		})
	}
	_ = writeSparseUDSFrame(conn, resp)
}

func readSparseUDSFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	body := make([]byte, n)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeSparseUDSFrame(w io.Writer, body []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func TestCreateEphemeralSparseAgentServerIntegration(t *testing.T) {
	dir := t.TempDir()
	sparseSocket := startFakeSparse(t)

	cfg := &config.Config{
		Transport:           "stdio",
		LogLevel:            "error",
		Workspace:           dir,
		AllowedOrigins:      []string{"http://localhost"},
		SparseAgentsEnabled: true,
		SparseSocket:        sparseSocket,
		SparseHostRAMCapMB:  512,
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}

	listOut, err := s.Handle(context.Background(), &transport.Session{Role: "orchestrator"},
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(listOut), `"create_ephemeral_sparse_agent"`) {
		t.Fatalf("expected sparse tool registered: %s", listOut)
	}

	callOut, err := s.Handle(context.Background(), &transport.Session{Role: "orchestrator"},
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_ephemeral_sparse_agent","arguments":{"skill_domain":"react-hooks","max_ram_mb":128}}}`))
	if err != nil {
		t.Fatal(err)
	}
	toolText := extractToolResultText(t, callOut)
	if !strings.Contains(toolText, "sa-integration") {
		t.Fatalf("expected wasm_agent_id in result: %s", toolText)
	}
	if !strings.Contains(toolText, dispatch.AgentTelemetryURI("sa-integration")) {
		t.Fatalf("expected telemetry stream resource in result: %s", toolText)
	}
}

func TestSparseQualityFloorEscalationServerIntegration(t *testing.T) {
	dir := t.TempDir()
	halSocket := filepath.Join(dir, "hal.sock")
	sparseSocket := startFakeSparse(t)
	infered := make(chan map[string]any, 2)
	startFakeHAL(t, halSocket, infered)

	cfg := &config.Config{
		Transport:                     "stdio",
		LogLevel:                      "error",
		Workspace:                     dir,
		AllowedOrigins:                []string{"http://localhost"},
		SparseAgentsEnabled:           true,
		SparseSocket:                  sparseSocket,
		SparseHostRAMCapMB:            512,
		SparseQualityGuardrailEnabled: true,
		HHDCapabilityRegistry:         true,
		HHDHardwareAwareDispatch:      true,
		HHDPolicyEngineV2:             true,
		HHDQualityGuardrailMinScore:   0.98,
		HALSocket:                     halSocket,
		HHDSnapshotTTLSeconds:         30,
		JobTimeoutSeconds:             10,
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}

	callOut, err := s.Handle(context.Background(), &transport.Session{Role: "orchestrator"},
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_ephemeral_sparse_agent","arguments":{"skill_domain":"react-hooks","quality_floor":0.95,"task_description":"fix types","max_ram_mb":128}}}`))
	if err != nil {
		t.Fatal(err)
	}
	toolText := extractToolResultText(t, callOut)
	if !strings.Contains(toolText, `"escalated":true`) {
		t.Fatalf("expected escalation result: %s", toolText)
	}
	if strings.Contains(toolText, "sa-integration") {
		t.Fatalf("sparse agent must not be created on guardrail breach: %s", toolText)
	}
	if !strings.Contains(toolText, dispatch.ReasonQualityGuardrailAutodisable) {
		t.Fatalf("expected quality_guardrail_autodisable reason: %s", toolText)
	}

	select {
	case params := <-infered:
		if params["selected_provider"] != "gpu-accelerated" {
			t.Fatalf("HAL infer selected_provider = %v, want gpu-accelerated", params["selected_provider"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for dense GPU HAL infer on guardrail escalation")
	}
}

func extractToolResultText(t *testing.T, raw []byte) string {
	t.Helper()
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal tool response: %v (%s)", err, raw)
	}
	if len(env.Result.Content) == 0 {
		t.Fatalf("empty tool content: %s", raw)
	}
	return env.Result.Content[0].Text
}
