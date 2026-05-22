package dispatch

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	HealthHealthy     = "healthy"
	HealthDegraded    = "degraded"
	HealthUnavailable = "unavailable"
)

// ProviderCapability is the policy-facing capability record for one backend.
type ProviderCapability struct {
	ProviderID            string    `json:"provider_id"`
	ContractVersion       string    `json:"contract_version"`
	HealthState           string    `json:"health_state"`
	LatencyClass          string    `json:"latency_class"`
	CostClass             string    `json:"cost_class"`
	QueueDepth            int       `json:"queue_depth"`
	SupportedWorkloadTags []string  `json:"supported_workload_tags"`
	ReliabilityClass      string    `json:"reliability_class"`
	FeatureFlags          []string  `json:"feature_flags"`
	LastUpdatedAt         time.Time `json:"last_updated_at"`
}

// CapabilitySnapshot is a deterministic, replay-friendly view for policy usage.
type CapabilitySnapshot struct {
	Epoch      uint64               `json:"epoch"`
	CapturedAt time.Time            `json:"captured_at"`
	Providers  []ProviderCapability `json:"providers"`
}

// CapabilityRegistry stores provider capabilities and emits deterministic snapshots.
type CapabilityRegistry struct {
	mu    sync.RWMutex
	now   func() time.Time
	ttl   time.Duration
	epoch uint64
	items map[string]ProviderCapability
}

func NewCapabilityRegistry(snapshotTTL time.Duration) *CapabilityRegistry {
	if snapshotTTL <= 0 {
		snapshotTTL = 30 * time.Second
	}
	return &CapabilityRegistry{
		now:   time.Now,
		ttl:   snapshotTTL,
		items: make(map[string]ProviderCapability),
	}
}

func (r *CapabilityRegistry) Upsert(cap ProviderCapability) error {
	if err := validateCapability(cap); err != nil {
		return err
	}
	now := r.now().UTC()
	if cap.LastUpdatedAt.IsZero() {
		cap.LastUpdatedAt = now
	} else {
		cap.LastUpdatedAt = cap.LastUpdatedAt.UTC()
	}
	cap.SupportedWorkloadTags = dedupeAndSort(cap.SupportedWorkloadTags)
	cap.FeatureFlags = dedupeAndSort(cap.FeatureFlags)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.epoch++
	r.items[cap.ProviderID] = cap
	return nil
}

func (r *CapabilityRegistry) Get(providerID string) (ProviderCapability, bool) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return ProviderCapability{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	cap, ok := r.items[providerID]
	if !ok {
		return ProviderCapability{}, false
	}
	return applyStaleness(cap, r.now().UTC(), r.ttl), true
}

func (r *CapabilityRegistry) Snapshot() CapabilitySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]ProviderCapability, 0, len(r.items))
	now := r.now().UTC()
	for _, cap := range r.items {
		providers = append(providers, applyStaleness(cap, now, r.ttl))
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ProviderID < providers[j].ProviderID
	})
	return CapabilitySnapshot{
		Epoch:      r.epoch,
		CapturedAt: now,
		Providers:  providers,
	}
}

func validateCapability(cap ProviderCapability) error {
	cap.ProviderID = strings.TrimSpace(cap.ProviderID)
	cap.ContractVersion = strings.TrimSpace(cap.ContractVersion)
	cap.HealthState = strings.ToLower(strings.TrimSpace(cap.HealthState))
	cap.LatencyClass = strings.TrimSpace(cap.LatencyClass)
	cap.CostClass = strings.TrimSpace(cap.CostClass)
	cap.ReliabilityClass = strings.TrimSpace(cap.ReliabilityClass)

	if cap.ProviderID == "" {
		return fmt.Errorf("provider_id is required")
	}
	if cap.ContractVersion == "" {
		return fmt.Errorf("contract_version is required")
	}
	if cap.LatencyClass == "" {
		return fmt.Errorf("latency_class is required")
	}
	if cap.CostClass == "" {
		return fmt.Errorf("cost_class is required")
	}
	if cap.ReliabilityClass == "" {
		return fmt.Errorf("reliability_class is required")
	}
	if cap.QueueDepth < 0 {
		return fmt.Errorf("queue_depth must be >= 0")
	}
	switch cap.HealthState {
	case HealthHealthy, HealthDegraded, HealthUnavailable:
		return nil
	default:
		return fmt.Errorf("health_state must be one of: %s, %s, %s", HealthHealthy, HealthDegraded, HealthUnavailable)
	}
}

func applyStaleness(cap ProviderCapability, now time.Time, ttl time.Duration) ProviderCapability {
	if ttl <= 0 || cap.LastUpdatedAt.IsZero() {
		return cap
	}
	if now.Sub(cap.LastUpdatedAt.UTC()) > ttl && strings.EqualFold(cap.HealthState, HealthHealthy) {
		cap.HealthState = HealthDegraded
	}
	return cap
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
