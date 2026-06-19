package dispatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeFetcher struct {
	mu    sync.Mutex
	caps  []ProviderCapability
	err   error
	calls int
}

func (f *fakeFetcher) FetchCapabilities() ([]ProviderCapability, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.caps, nil
}

func liveCap(id, health string) ProviderCapability {
	return ProviderCapability{
		ProviderID:            id,
		ContractVersion:       "dispatch.provider/v2",
		HealthState:           health,
		LatencyClass:          "fast",
		CostClass:             "high",
		QueueDepth:            0,
		SupportedWorkloadTags: []string{"inference-heavy"},
		ReliabilityClass:      "gold",
	}
}

func TestCapabilitySyncerSyncOnceUpserts(t *testing.T) {
	reg := NewCapabilityRegistry(time.Minute)
	fetcher := &fakeFetcher{caps: []ProviderCapability{
		liveCap("cpu-baseline", HealthHealthy),
		liveCap("gpu-accelerated", HealthDegraded),
	}}
	syncer := NewCapabilitySyncer(fetcher, reg, time.Second)

	n, err := syncer.SyncOnce()
	if err != nil {
		t.Fatalf("sync once: %v", err)
	}
	if n != 2 {
		t.Fatalf("synced = %d, want 2", n)
	}
	snap := reg.Snapshot()
	if len(snap.Providers) != 2 {
		t.Fatalf("registry has %d providers, want 2", len(snap.Providers))
	}
	gpu, ok := reg.Get("gpu-accelerated")
	if !ok {
		t.Fatal("expected gpu-accelerated in registry")
	}
	if gpu.HealthState != HealthDegraded {
		t.Fatalf("gpu health = %q, want degraded (live health reflected)", gpu.HealthState)
	}
}

func TestCapabilitySyncerContractVersionDefaulted(t *testing.T) {
	reg := NewCapabilityRegistry(time.Minute)
	cap := liveCap("lpu-realtime", HealthHealthy)
	cap.ContractVersion = ""
	syncer := NewCapabilitySyncer(&fakeFetcher{caps: []ProviderCapability{cap}}, reg, time.Second)

	if _, err := syncer.SyncOnce(); err != nil {
		t.Fatalf("sync once: %v", err)
	}
	got, ok := reg.Get("lpu-realtime")
	if !ok {
		t.Fatal("expected lpu-realtime in registry")
	}
	if got.ContractVersion != "dispatch.provider/v2" {
		t.Fatalf("contract version = %q, want defaulted", got.ContractVersion)
	}
}

func TestCapabilitySyncerSkipsInvalidProvider(t *testing.T) {
	reg := NewCapabilityRegistry(time.Minute)
	bad := liveCap("", HealthHealthy) // missing provider_id → invalid
	good := liveCap("cpu-baseline", HealthHealthy)
	var errCount int
	syncer := NewCapabilitySyncer(&fakeFetcher{caps: []ProviderCapability{bad, good}}, reg, time.Second)
	syncer.OnError = func(error) { errCount++ }

	n, err := syncer.SyncOnce()
	if err != nil {
		t.Fatalf("sync once: %v", err)
	}
	if n != 1 {
		t.Fatalf("synced = %d, want 1 (invalid skipped)", n)
	}
	if errCount != 1 {
		t.Fatalf("OnError calls = %d, want 1", errCount)
	}
}

func TestCapabilitySyncerFetchErrorReported(t *testing.T) {
	reg := NewCapabilityRegistry(time.Minute)
	var gotErr error
	syncer := NewCapabilitySyncer(&fakeFetcher{err: errors.New("hal down")}, reg, time.Second)
	syncer.OnError = func(e error) { gotErr = e }

	n, err := syncer.SyncOnce()
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if n != 0 {
		t.Fatalf("synced = %d, want 0", n)
	}
	if gotErr == nil {
		t.Fatal("expected OnError to be invoked")
	}
}

func TestCapabilitySyncerStartSyncsAndStops(t *testing.T) {
	reg := NewCapabilityRegistry(time.Minute)
	fetcher := &fakeFetcher{caps: []ProviderCapability{liveCap("cpu-baseline", HealthHealthy)}}
	syncer := NewCapabilitySyncer(fetcher, reg, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	syncer.Start(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := reg.Get("cpu-baseline"); ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := reg.Get("cpu-baseline"); !ok {
		t.Fatal("expected background sync to populate registry")
	}

	cancel()
	time.Sleep(20 * time.Millisecond)
	fetcher.mu.Lock()
	callsAfterCancel := fetcher.calls
	fetcher.mu.Unlock()
	time.Sleep(30 * time.Millisecond)
	fetcher.mu.Lock()
	finalCalls := fetcher.calls
	fetcher.mu.Unlock()
	if finalCalls != callsAfterCancel {
		t.Fatalf("syncer kept running after cancel: %d → %d", callsAfterCancel, finalCalls)
	}
}
