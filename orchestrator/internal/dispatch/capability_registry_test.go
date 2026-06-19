package dispatch

import (
	"testing"
	"time"
)

func TestCapabilityRegistryUpsertAndSnapshot(t *testing.T) {
	r := NewCapabilityRegistry(30 * time.Second)
	base := time.Date(2026, time.May, 22, 10, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }

	if err := r.Upsert(ProviderCapability{
		ProviderID:            "gpu-a",
		ContractVersion:       "dispatch.provider/v1.0",
		HealthState:           HealthHealthy,
		LatencyClass:          "fast",
		CostClass:             "high",
		QueueDepth:            2,
		SupportedWorkloadTags: []string{"inference-heavy", "merge-assist", "inference-heavy"},
		ReliabilityClass:      "gold",
		FeatureFlags:          []string{"hhd.shadow", "hhd.shadow"},
	}); err != nil {
		t.Fatalf("upsert capability: %v", err)
	}

	snapshot := r.Snapshot()
	if snapshot.Epoch != 1 {
		t.Fatalf("expected epoch=1, got %d", snapshot.Epoch)
	}
	if len(snapshot.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(snapshot.Providers))
	}
	cap := snapshot.Providers[0]
	if cap.ProviderID != "gpu-a" {
		t.Fatalf("unexpected provider id: %s", cap.ProviderID)
	}
	if cap.HealthState != HealthHealthy {
		t.Fatalf("expected healthy provider, got %s", cap.HealthState)
	}
	if len(cap.SupportedWorkloadTags) != 2 {
		t.Fatalf("expected deduped workload tags, got %+v", cap.SupportedWorkloadTags)
	}
	if len(cap.FeatureFlags) != 1 || cap.FeatureFlags[0] != "hhd.shadow" {
		t.Fatalf("expected deduped feature flags, got %+v", cap.FeatureFlags)
	}
}

func TestCapabilityRegistryStaleHealthDegradesInSnapshot(t *testing.T) {
	r := NewCapabilityRegistry(5 * time.Second)
	base := time.Date(2026, time.May, 22, 10, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }

	if err := r.Upsert(ProviderCapability{
		ProviderID:            "cpu-baseline",
		ContractVersion:       "dispatch.provider/v1.0",
		HealthState:           HealthHealthy,
		LatencyClass:          "baseline",
		CostClass:             "low",
		QueueDepth:            0,
		SupportedWorkloadTags: []string{"default"},
		ReliabilityClass:      "standard",
		LastUpdatedAt:         base.Add(-10 * time.Second),
	}); err != nil {
		t.Fatalf("upsert capability: %v", err)
	}

	r.now = func() time.Time { return base.Add(20 * time.Second) }
	cap, ok := r.Get("cpu-baseline")
	if !ok {
		t.Fatal("expected provider to exist")
	}
	if cap.HealthState != HealthDegraded {
		t.Fatalf("expected stale provider to degrade health, got %s", cap.HealthState)
	}
}
