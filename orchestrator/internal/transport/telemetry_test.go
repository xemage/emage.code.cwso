package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func TestTelemetryThrottleSuppressesWithinTopicWindow(t *testing.T) {
	throttle := newTelemetryThrottle()

	base := time.Date(2026, time.May, 14, 18, 0, 0, 0, time.UTC)
	first := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsLog,
		At:      base,
		Payload: mustJSON(t, map[string]any{"idx": 1}),
	}
	second := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsLog,
		At:      base.Add(50 * time.Millisecond),
		Payload: mustJSON(t, map[string]any{"idx": 2}),
	}
	third := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsLog,
		At:      base.Add(defaultLogThrottleWindow),
		Payload: mustJSON(t, map[string]any{"idx": 3}),
	}

	if !throttle.Allow(first) {
		t.Fatal("expected first record to be emitted")
	}
	if throttle.Allow(second) {
		t.Fatal("expected second record in same topic window to be suppressed")
	}
	if !throttle.Allow(third) {
		t.Fatal("expected record at next topic window boundary to be emitted")
	}

	counters := throttle.Snapshot()[eventbus.TopicNotificationsLog]
	if counters.Emitted != 2 || counters.Suppressed != 1 {
		t.Fatalf("unexpected counters: %+v", counters)
	}
}

func TestTelemetryThrottleBypassesTerminalJobState(t *testing.T) {
	throttle := newTelemetryThrottle()

	base := time.Date(2026, time.May, 14, 18, 0, 0, 0, time.UTC)
	running := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsJobState,
		At:      base,
		Payload: mustJSON(t, map[string]any{"job_id": "job-1", "state": "running"}),
	}
	failed := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsJobState,
		At:      base.Add(10 * time.Millisecond),
		Payload: mustJSON(t, map[string]any{"job_id": "job-1", "state": "failed"}),
	}

	if !throttle.Allow(running) {
		t.Fatal("expected first running record to be emitted")
	}
	if !throttle.Allow(failed) {
		t.Fatal("expected terminal job-state to bypass throttle")
	}
}

func TestTelemetryThrottleIsTopicAware(t *testing.T) {
	throttle := newTelemetryThrottle()
	base := time.Date(2026, time.May, 14, 18, 0, 0, 0, time.UTC)

	logRecord := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsLog,
		At:      base,
		Payload: mustJSON(t, map[string]any{"idx": 1}),
	}
	jobStateRecord := memorybroker.Record{
		Topic:   eventbus.TopicNotificationsJobState,
		At:      base.Add(20 * time.Millisecond),
		Payload: mustJSON(t, map[string]any{"job_id": "job-1", "state": "running"}),
	}

	if !throttle.Allow(logRecord) {
		t.Fatal("expected first log record to be emitted")
	}
	if !throttle.Allow(jobStateRecord) {
		t.Fatal("expected different topic to use an independent throttle window")
	}
}

func mustJSON(t *testing.T, payload any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return encoded
}
