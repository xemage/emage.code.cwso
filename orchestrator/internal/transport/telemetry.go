package transport

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

const (
	defaultLogThrottleWindow      = 250 * time.Millisecond
	defaultJobStateThrottleWindow = 75 * time.Millisecond
)

type topicThrottlePolicy struct {
	Window time.Duration
	Bypass func(payload json.RawMessage) bool
}

type throttleCounters struct {
	Emitted    int
	Suppressed int
}

type topicThrottleState struct {
	lastEmitAt time.Time
	counters   throttleCounters
}

type telemetryThrottle struct {
	now      func() time.Time
	policies map[string]topicThrottlePolicy
	states   map[string]*topicThrottleState
}

func newTelemetryThrottle() *telemetryThrottle {
	return &telemetryThrottle{
		now: time.Now,
		policies: map[string]topicThrottlePolicy{
			eventbus.TopicNotificationsLog: {
				Window: defaultLogThrottleWindow,
			},
			eventbus.TopicNotificationsJobState: {
				Window: defaultJobStateThrottleWindow,
				Bypass: isTerminalJobStatePayload,
			},
		},
		states: make(map[string]*topicThrottleState),
	}
}

func (t *telemetryThrottle) Allow(record memorybroker.Record) bool {
	policy, ok := t.policies[record.Topic]
	if !ok || policy.Window <= 0 {
		state := t.stateFor(record.Topic)
		state.counters.Emitted++
		state.lastEmitAt = eventTime(record.At, t.now)
		return true
	}

	if policy.Bypass != nil && policy.Bypass(record.Payload) {
		state := t.stateFor(record.Topic)
		state.counters.Emitted++
		state.lastEmitAt = eventTime(record.At, t.now)
		return true
	}

	state := t.stateFor(record.Topic)
	at := eventTime(record.At, t.now)
	if state.lastEmitAt.IsZero() || !at.Before(state.lastEmitAt.Add(policy.Window)) {
		state.lastEmitAt = at
		state.counters.Emitted++
		return true
	}

	state.counters.Suppressed++
	return false
}

func (t *telemetryThrottle) Snapshot() map[string]throttleCounters {
	out := make(map[string]throttleCounters, len(t.states))
	for topic, state := range t.states {
		out[topic] = state.counters
	}
	return out
}

func (t *telemetryThrottle) stateFor(topic string) *topicThrottleState {
	state, ok := t.states[topic]
	if ok {
		return state
	}
	state = &topicThrottleState{}
	t.states[topic] = state
	return state
}

func eventTime(at time.Time, fallback func() time.Time) time.Time {
	if !at.IsZero() {
		return at.UTC()
	}
	if fallback == nil {
		return time.Time{}
	}
	return fallback().UTC()
}

func isTerminalJobStatePayload(payload json.RawMessage) bool {
	if len(payload) == 0 {
		return false
	}
	var body struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(body.State)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}
