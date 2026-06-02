package dispatch

import (
	"context"
	"time"
)

// CapabilityFetcher returns the current live provider capabilities (e.g. sourced from the
// HAL sidecar). Implementations should be safe for concurrent use.
type CapabilityFetcher interface {
	FetchCapabilities() ([]ProviderCapability, error)
}

// CapabilitySyncer periodically refreshes a CapabilityRegistry from a live source so the
// policy engine routes against real adapter health/queue-depth/latency instead of a static
// catalog. It only upserts: providers that vanish from the source age out via the
// registry's staleness rule (healthy → degraded past TTL), and the CPU baseline — which the
// HAL always reports — stays fresh, preserving the terminal-safe fallback.
type CapabilitySyncer struct {
	fetcher  CapabilityFetcher
	registry *CapabilityRegistry
	interval time.Duration

	// OnError, when set, is invoked with any fetch error so operators get visibility
	// without coupling this package to a logger. Per-provider validation errors are
	// surfaced the same way but do not abort a sync.
	OnError func(error)
	// OnSync, when set, is invoked with the number of providers upserted on each tick.
	OnSync func(int)
}

// NewCapabilitySyncer builds a syncer. A non-positive interval defaults to 15s.
func NewCapabilitySyncer(fetcher CapabilityFetcher, registry *CapabilityRegistry, interval time.Duration) *CapabilitySyncer {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &CapabilitySyncer{fetcher: fetcher, registry: registry, interval: interval}
}

// SyncOnce fetches the live capabilities and upserts them into the registry. It returns the
// number of providers successfully upserted. A fetch error aborts the sync; individual
// invalid provider records are skipped (reported via OnError) so one bad entry cannot block
// the rest.
func (s *CapabilitySyncer) SyncOnce() (int, error) {
	if s == nil || s.fetcher == nil || s.registry == nil {
		return 0, nil
	}
	caps, err := s.fetcher.FetchCapabilities()
	if err != nil {
		s.reportError(err)
		return 0, err
	}
	synced := 0
	for _, cap := range caps {
		if cap.ContractVersion == "" {
			cap.ContractVersion = "dispatch.provider/v2"
		}
		// Force a fresh heartbeat timestamp so staleness is measured from this sync.
		cap.LastUpdatedAt = time.Time{}
		if upErr := s.registry.Upsert(cap); upErr != nil {
			s.reportError(upErr)
			continue
		}
		synced++
	}
	if s.OnSync != nil {
		s.OnSync(synced)
	}
	return synced, nil
}

// Start launches a background goroutine that performs an immediate sync and then refreshes
// on the configured interval until ctx is cancelled.
func (s *CapabilitySyncer) Start(ctx context.Context) {
	if s == nil {
		return
	}
	go func() {
		_, _ = s.SyncOnce()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.SyncOnce()
			}
		}
	}()
}

func (s *CapabilitySyncer) reportError(err error) {
	if s.OnError != nil {
		s.OnError(err)
	}
}
