package tools

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/sparse"
)

// Phase 7 sparse micro-agent budgets (T124). The cold-start SLO (< 10 ms) is enforced on the
// sidecar measurement in cwso-sparse; here we guard the orchestrator control-plane overhead and
// verify cold_start_ms is faithfully forwarded from the sidecar response.
const sparseControlPlaneBudget = 10 * time.Millisecond

func TestSparseAgentColdStartReportedUnderBudget(t *testing.T) {
	fake := &fakeSparseSidecar{createOK: true}
	socket := fake.serve(t)
	reg := dispatch.NewSparseAgentRegistry()
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient(socket), reg, nil, 512)

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
	cold, ok := out["cold_start_ms"].(float64)
	if !ok || cold <= 0 {
		t.Fatalf("expected positive cold_start_ms, got %+v", out["cold_start_ms"])
	}
	if cold >= 10 {
		t.Fatalf("cold_start_ms %v exceeds 10 ms budget (fake sidecar reports 3.5)", cold)
	}
}

func TestSparseAgentControlPlaneOverheadBudget(t *testing.T) {
	fake := &fakeSparseSidecar{createOK: true}
	socket := fake.serve(t)
	broker := memorybroker.New(memorybroker.WithCapacity(64))
	pub := memorybroker.NewTeePublisher(nil, broker)
	reg := dispatch.NewSparseAgentRegistry()
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient(socket), reg, pub, 512)

	const iters = 100
	args := json.RawMessage(`{"skill_domain":"react-hooks","max_ram_mb":128}`)
	durations := make([]time.Duration, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		res, err := tool.Execute(context.Background(), args)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("execute[%d]: %v", i, err)
		}
		if res.IsError {
			t.Fatalf("execute[%d] tool error: %s", i, res.Content[0].Text)
		}
		durations = append(durations, elapsed)
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	median := durations[len(durations)/2]
	p95 := durations[int(float64(len(durations))*0.95)]
	if median > sparseControlPlaneBudget {
		t.Fatalf("median sparse control-plane overhead %v exceeds budget %v (p95=%v)",
			median, sparseControlPlaneBudget, p95)
	}
	t.Logf("sparse control-plane overhead: median=%v p95=%v over %d iters (budget %v)",
		median, p95, iters, sparseControlPlaneBudget)
}
