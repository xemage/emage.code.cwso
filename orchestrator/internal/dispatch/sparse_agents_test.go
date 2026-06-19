package dispatch

import (
	"encoding/json"
	"testing"
)

func TestParseAgentTelemetryResourceID(t *testing.T) {
	id, ok := ParseAgentTelemetryResourceID("cwso://agents/sa-001/telemetry")
	if !ok || id != "sa-001" {
		t.Fatalf("parse agent uri: ok=%v id=%q", ok, id)
	}
	if _, ok := ParseAgentTelemetryResourceID("cwso://spikes/x"); ok {
		t.Fatal("expected spike uri to fail agent parse")
	}
}

func TestAgentTelemetryFilterAllowsMatchingAgent(t *testing.T) {
	filter := &AgentTelemetryFilter{AgentID: "sa-abc"}
	payload, _ := json.Marshal(map[string]string{"wasm_agent_id": "sa-abc"})
	if !filter.Allow(TopicAgentTelemetry, payload) {
		t.Fatal("expected matching agent telemetry")
	}
	payload, _ = json.Marshal(map[string]string{"wasm_agent_id": "other"})
	if filter.Allow(TopicAgentTelemetry, payload) {
		t.Fatal("expected foreign agent to be filtered out")
	}
	if filter.Allow(TopicASTSpike, payload) {
		t.Fatal("expected wrong topic to be filtered out")
	}
}

func TestNormalizeQuantization(t *testing.T) {
	q, err := NormalizeQuantization("")
	if err != nil || q != "1.58-bit" {
		t.Fatalf("default quant: q=%q err=%v", q, err)
	}
	if _, err := NormalizeQuantization("int8"); err == nil {
		t.Fatal("expected int8 rejection")
	}
}

func TestSparseAgentRegistryRegisterList(t *testing.T) {
	reg := NewSparseAgentRegistry()
	rec := reg.Register("sa-1", "react-hooks", "Class: Foo")
	if rec.StreamURI != AgentTelemetryURI("sa-1") {
		t.Fatalf("unexpected stream uri %q", rec.StreamURI)
	}
	list := reg.List()
	if len(list) != 1 || list[0].ID != "sa-1" {
		t.Fatalf("list: %+v", list)
	}
}
