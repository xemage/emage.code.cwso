package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
)

func TestSubscribeASTSpikesReturnsHandle(t *testing.T) {
	reg := dispatch.NewSpikeSubscriptionRegistry()
	tool := NewSubscribeASTSpikes(reg)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"pkg/*.go","semantic_threshold":"any","workspace_scope":["ws1"]}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content[0].Text)
	}
	var out struct {
		SubscriptionID    string   `json:"subscription_id"`
		StreamResource    string   `json:"stream_resource"`
		SemanticThreshold string   `json:"semantic_threshold"`
		Topics            []string `json:"topics"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("unmarshal result: %v (%s)", err, res.Content[0].Text)
	}
	if out.SubscriptionID == "" {
		t.Fatal("expected non-empty subscription_id")
	}
	if out.StreamResource != dispatch.SpikeResourcePrefix+out.SubscriptionID {
		t.Fatalf("unexpected stream_resource %q", out.StreamResource)
	}
	if out.SemanticThreshold != "any" {
		t.Fatalf("expected threshold any, got %q", out.SemanticThreshold)
	}
	if len(out.Topics) != 3 {
		t.Fatalf("expected 3 spike topics, got %v", out.Topics)
	}
	if _, ok := reg.Get(out.SubscriptionID); !ok {
		t.Fatal("subscription should be registered")
	}
}

func TestSubscribeASTSpikesRejectsBadThreshold(t *testing.T) {
	tool := NewSubscribeASTSpikes(dispatch.NewSpikeSubscriptionRegistry())
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"semantic_threshold":"nonsense"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "semantic_threshold") {
		t.Fatalf("expected threshold validation error, got: %+v", res)
	}
}

func TestSubscribeASTSpikesDisabled(t *testing.T) {
	tool := NewSubscribeASTSpikes(nil)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "not enabled") {
		t.Fatalf("expected disabled error, got: %+v", res)
	}
}

func TestSubscribeASTSpikesMetadata(t *testing.T) {
	tool := NewSubscribeASTSpikes(dispatch.NewSpikeSubscriptionRegistry())
	if tool.Name() != "subscribe_ast_spikes" {
		t.Fatalf("unexpected name %q", tool.Name())
	}
	roles := tool.AllowedRoles()
	if len(roles) != 2 {
		t.Fatalf("expected orchestrator+worker roles, got %v", roles)
	}
	if _, ok := tool.InputSchema()["properties"]; !ok {
		t.Fatal("expected input schema properties")
	}
}
