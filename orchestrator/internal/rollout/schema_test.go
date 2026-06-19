package rollout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the CWSO repository root (parent of orchestrator/).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../orchestrator/internal/rollout/schema_test.go -> repo root (3 levels up)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestRolloutSchemasAreValidJSON(t *testing.T) {
	root := repoRoot(t)
	names := []string{
		"rollout_task_submit.json",
		"rollout_task_status.json",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "schemas", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parse schema JSON: %v", err)
			}
			if doc["$schema"] == nil {
				t.Error("missing $schema")
			}
			if doc["title"] == nil {
				t.Error("missing title")
			}
		})
	}
}

func TestRolloutTaskSubmitRequiredFields(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "schemas", "rollout_task_submit.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Required) != 1 || doc.Required[0] != "task_spec" {
		t.Errorf("required = %v, want [task_spec]", doc.Required)
	}
}
