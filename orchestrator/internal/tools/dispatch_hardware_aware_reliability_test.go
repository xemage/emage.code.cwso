package tools

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/hal"
	"github.com/emage/cwso/orchestrator/internal/jobs"
)

// Phase 6 reliability budgets (T088). These guard the deterministic control-plane
// overhead and the fallback path; they are intentionally generous relative to the
// in-memory cost so they stay stable across CI runners.
const (
	dispatchOverheadBudget = 10 * time.Millisecond
	fallbackLatencyBudget  = 2 * time.Second
)

// latencyFakeInferrer is a configurable HAL stand-in: it can simulate a provider
// round-trip delay, report which provider ultimately served, and surface an error.
type latencyFakeInferrer struct {
	mu       sync.Mutex
	delay    time.Duration
	servedBy string
	err      error
	chains   [][]string
	calls    int
	done     chan struct{}
}

func newLatencyFake(delay time.Duration, servedBy string) *latencyFakeInferrer {
	return &latencyFakeInferrer{delay: delay, servedBy: servedBy, done: make(chan struct{}, 64)}
}

func (f *latencyFakeInferrer) Infer(providerID string, fallbackChain []string, _ hal.InferenceRequest) (*hal.InferResult, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.calls++
	f.chains = append(f.chains, fallbackChain)
	servedBy := f.servedBy
	if servedBy == "" {
		servedBy = providerID
	}
	err := f.err
	f.mu.Unlock()
	select {
	case f.done <- struct{}{}:
	default:
	}
	if err != nil {
		return nil, err
	}
	return &hal.InferResult{ServedBy: servedBy, Completion: hal.Completion{ProviderID: servedBy}}, nil
}

func newHWAwareToolWithHAL(t *testing.T, mgr *jobs.Manager, client halInferrer) *DispatchHardwareAwareJob {
	t.Helper()
	policyCfg := dispatch.DefaultPolicyV2Config()
	policyCfg.Enabled = true
	return NewDispatchHardwareAwareJobWithHAL(
		mgr, 300, &recordingDispatchEmitter{},
		stubCapabilitySnapshotReader{snapshot: hwAwareSnapshot()},
		policyCfg, client,
	)
}

func waitJobTerminal(t *testing.T, mgr *jobs.Manager, id string, timeout time.Duration) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if job, ok := mgr.Get(id); ok {
			switch job.State {
			case jobs.StateCompleted, jobs.StateFailed, jobs.StateCancelled:
				return job
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job %s did not reach a terminal state within %v", id, timeout)
	return jobs.Job{}
}

// TestDispatchOverheadBudget asserts the deterministic profiling + policy selection +
// enqueue path (the synchronous control-plane work the model waits on) stays within
// the Phase 6 dispatch-overhead budget. Execution itself is fire-and-forget.
func TestDispatchOverheadBudget(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 4, QueueSize: 2048}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	tool := newHWAwareToolWithHAL(t, mgr, newLatencyFake(0, ""))

	const iters = 300
	args := json.RawMessage(`{"task_description":"fix typo in handler","context_size_estimate":1500,"latency_requirement":"realtime"}`)
	durations := make([]time.Duration, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		res, err := tool.Execute(context.Background(), args)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("execute[%d]: %v", i, err)
		}
		if res.IsError {
			t.Fatalf("execute[%d] tool error: %+v", i, res)
		}
		durations = append(durations, elapsed)
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	median := durations[len(durations)/2]
	p95 := durations[int(float64(len(durations))*0.95)]
	if median > dispatchOverheadBudget {
		t.Fatalf("median dispatch overhead %v exceeds budget %v (p95=%v)", median, dispatchOverheadBudget, p95)
	}
	t.Logf("dispatch overhead: median=%v p95=%v over %d iters (budget %v)", median, p95, iters, dispatchOverheadBudget)
}

// TestFallbackLatencyBudget simulates the selected backend failing over to the
// CPU baseline (the terminal-safe provider) and asserts the dispatched job still
// completes, end-to-end, within the Phase 6 fallback budget — guarding against
// hangs or missing completions on the fallback path.
func TestFallbackLatencyBudget(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 2, QueueSize: 16}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	// Simulate a failed-primary round-trip before the baseline serves.
	fake := newLatencyFake(50*time.Millisecond, "cpu-baseline")
	tool := newHWAwareToolWithHAL(t, mgr, fake)

	start := time.Now()
	res, err := tool.Execute(context.Background(),
		json.RawMessage(`{"task_description":"fix typo","context_size_estimate":1000,"latency_requirement":"realtime"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := decodeHWAwareResult(t, res)

	job := waitJobTerminal(t, mgr, out.JobID, fallbackLatencyBudget)
	elapsed := time.Since(start)
	if job.State != jobs.StateCompleted {
		t.Fatalf("job state = %q (err=%q), want completed", job.State, job.Error)
	}
	if elapsed > fallbackLatencyBudget {
		t.Fatalf("fallback end-to-end latency %v exceeds budget %v", elapsed, fallbackLatencyBudget)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.calls != 1 {
		t.Fatalf("expected one HAL dispatch, got %d", fake.calls)
	}
	// The ranked fallback chain must be forwarded so the HAL can fall back deterministically.
	if len(fake.chains) == 0 || len(fake.chains[0]) < 2 {
		t.Fatalf("expected a ranked fallback chain with a baseline tail, got %+v", fake.chains)
	}
	tail := fake.chains[0][len(fake.chains[0])-1]
	if tail != dispatch.DefaultBaselineProviderID {
		t.Fatalf("fallback chain must terminate at %q, got tail %q (chain=%+v)",
			dispatch.DefaultBaselineProviderID, tail, fake.chains[0])
	}
	t.Logf("fallback end-to-end latency: %v (budget %v)", elapsed, fallbackLatencyBudget)
}

// TestDispatchFailurePropagatesWithinBudget asserts that when every backend is down
// the job fails fast (no hang) and the failure is recorded — reliability of error
// propagation on the dispatch path.
func TestDispatchFailurePropagatesWithinBudget(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 2, QueueSize: 16}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	fake := newLatencyFake(0, "")
	fake.err = &hal.SidecarError{Code: "unavailable", Message: "all backends down"}
	tool := newHWAwareToolWithHAL(t, mgr, fake)

	start := time.Now()
	res, err := tool.Execute(context.Background(),
		json.RawMessage(`{"task_description":"do work","context_size_estimate":500,"latency_requirement":"batch"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := decodeHWAwareResult(t, res)

	job := waitJobTerminal(t, mgr, out.JobID, fallbackLatencyBudget)
	if job.State != jobs.StateFailed {
		t.Fatalf("job state = %q, want failed", job.State)
	}
	if job.Error == "" {
		t.Fatalf("expected a recorded failure reason")
	}
	if elapsed := time.Since(start); elapsed > fallbackLatencyBudget {
		t.Fatalf("failure propagation latency %v exceeds budget %v", elapsed, fallbackLatencyBudget)
	}
}
