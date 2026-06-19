package rollout

import (
	"encoding/json"
	"testing"
)

func TestDrainCaptureRequestEnvelopeShape(t *testing.T) {
	env := requestEnvelope{ID: "1", Op: "drain_capture", Limit: 32}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["op"] != "drain_capture" {
		t.Fatalf("op = %v", doc["op"])
	}
	if doc["limit"] != float64(32) {
		t.Fatalf("limit = %v", doc["limit"])
	}
	if _, ok := doc["params"]; ok {
		t.Fatal("params must be flattened for cwso-rollout IPC")
	}
}
