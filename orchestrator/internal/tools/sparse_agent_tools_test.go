package tools

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/sparse"
)

type fakeSparseSidecar struct {
	mu       sync.Mutex
	createOK bool
}

func (f *fakeSparseSidecar) serve(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("unix", "")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.handle(conn)
		}
	}()
	return ln.Addr().String()
}

func (f *fakeSparseSidecar) handle(conn net.Conn) {
	defer conn.Close()
	body, err := readSparseFrame(conn)
	if err != nil {
		return
	}
	var env struct {
		ID string `json:"id"`
		Op string `json:"op"`
	}
	_ = json.Unmarshal(body, &env)

	var resp []byte
	if env.Op == "create_agent" && f.createOK {
		resp, _ = json.Marshal(map[string]any{
			"id": env.ID, "ok": true,
			"result": map[string]any{
				"wasm_agent_id":   "sa-test",
				"skill_domain":    "react-hooks",
				"slice_sha256":    "abc",
				"state":           "ready",
				"cold_start_ms":   3.5,
				"resident_ram_mb": 12.0,
				"tokens_per_sec":  0.0,
			},
		})
	} else {
		resp, _ = json.Marshal(map[string]any{
			"id": env.ID, "ok": false,
			"error": map[string]string{"code": "invalid_input", "message": "fail"},
		})
	}
	_ = writeSparseFrame(conn, resp)
}

func readSparseFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	body := make([]byte, n)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeSparseFrame(w io.Writer, body []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func TestCreateEphemeralSparseAgentSuccess(t *testing.T) {
	fake := &fakeSparseSidecar{createOK: true}
	socket := fake.serve(t)

	broker := memorybroker.New(memorybroker.WithCapacity(32))
	pub := memorybroker.NewTeePublisher(nil, broker)
	reg := dispatch.NewSparseAgentRegistry()
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient(socket), reg, pub, 512)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"skill_domain":"react-hooks","max_ram_mb":128}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", res.Content[0].Text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatal(err)
	}
	if out["wasm_agent_id"] != "sa-test" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out["stream_resource"] != dispatch.AgentTelemetryURI("sa-test") {
		t.Fatalf("stream resource: %+v", out)
	}
	records := waitBrokerRecords(broker, dispatch.TopicAgentTelemetry, 1)
	if len(records) != 1 {
		t.Fatalf("expected telemetry publish, got %d records", len(records))
	}
}

func waitBrokerRecords(broker *memorybroker.Broker, topic string, n int) []memorybroker.Record {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		recs := broker.Query(memorybroker.QueryOptions{Topics: []string{topic}})
		if len(recs) >= n {
			return recs
		}
		time.Sleep(10 * time.Millisecond)
	}
	return broker.Query(memorybroker.QueryOptions{Topics: []string{topic}})
}

func TestCreateEphemeralSparseAgentRejectsMissingDomain(t *testing.T) {
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient("/nonexistent"), dispatch.NewSparseAgentRegistry(), nil, 512)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for missing skill_domain")
	}
}

func TestCreateEphemeralSparseAgentRejectsRAMCap(t *testing.T) {
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient("/nonexistent"), dispatch.NewSparseAgentRegistry(), nil, 64)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"skill_domain":"x","max_ram_mb":512}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected max_ram_mb cap rejection")
	}
}
