package rollout

import (
	"time"
)

const (
	// TopicReward is the memory-broker topic for merge-state-machine rewards (T136).
	TopicReward = "rollout/reward"
)

// RewardKind identifies the merge completion event that produced a reward.
type RewardKind string

const (
	RewardMergeSuccess  RewardKind = "merge_success"
	RewardMergeConflict RewardKind = "merge_conflict"
	RewardSyntaxFail    RewardKind = "syntax_fail"
)

// RewardEvent is published when merge_concurrent_results completes (rollout-architecture-v1 §7).
type RewardEvent struct {
	Kind              RewardKind `json:"kind"`
	Reward            float64    `json:"reward"`
	SessionID         string     `json:"session_id,omitempty"`
	Outcome           string     `json:"outcome"`
	MergedCount       int        `json:"merged_count"`
	ConflictCount     int        `json:"conflict_count"`
	FailureCount      int        `json:"failure_count"`
	TargetBranchRef   string     `json:"target_branch_ref,omitempty"`
	TimestampUnixNano int64      `json:"timestamp_ns"`
}

// Publisher publishes rollout reward events (implemented by memorybroker.Publisher).
type Publisher interface {
	Publish(topic string, payload any) error
}

// RewardEmitter publishes merge SM rewards when enabled.
type RewardEmitter struct {
	enabled   bool
	publisher Publisher
}

// NewRewardEmitter constructs an emitter; disabled emitters are no-ops.
func NewRewardEmitter(enabled bool, publisher Publisher) *RewardEmitter {
	if publisher == nil {
		enabled = false
	}
	return &RewardEmitter{enabled: enabled, publisher: publisher}
}

// Enabled reports whether reward emission is active.
func (e *RewardEmitter) Enabled() bool {
	return e != nil && e.enabled
}

// Emit publishes a reward for a completed merge invocation.
func (e *RewardEmitter) Emit(event RewardEvent) error {
	if e == nil || !e.enabled {
		return nil
	}
	if event.TimestampUnixNano == 0 {
		event.TimestampUnixNano = time.Now().UnixNano()
	}
	return e.publisher.Publish(TopicReward, event)
}

// ClassifyMergeOutcome maps aggregate merge tool output to a reward (architecture §7 table).
func ClassifyMergeOutcome(outcome string, results []MergeResultSummary) (RewardKind, float64) {
	switch outcome {
	case "success":
		return RewardMergeSuccess, 1.0
	case "conflict":
		return RewardMergeConflict, -1.0
	default:
		for _, r := range results {
			if r.ReasonCode == "invalid_engine_payload" {
				return RewardSyntaxFail, -1.0
			}
		}
		return RewardMergeConflict, -1.0
	}
}

// MergeResultSummary carries the per-file fields needed for reward classification.
type MergeResultSummary struct {
	Status     string
	ReasonCode string
}
