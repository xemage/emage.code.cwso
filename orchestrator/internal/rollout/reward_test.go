package rollout

import (
	"testing"
)

func TestClassifyMergeOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		outcome string
		results []MergeResultSummary
		kind    RewardKind
		score   float64
	}{
		{"success", nil, RewardMergeSuccess, 1.0},
		{"conflict", nil, RewardMergeConflict, -1.0},
		{"error", []MergeResultSummary{{ReasonCode: "invalid_engine_payload"}}, RewardSyntaxFail, -1.0},
		{"error", []MergeResultSummary{{ReasonCode: "merge_engine_unavailable"}}, RewardMergeConflict, -1.0},
	}
	for _, tc := range cases {
		kind, score := ClassifyMergeOutcome(tc.outcome, tc.results)
		if kind != tc.kind || score != tc.score {
			t.Fatalf("outcome=%q: got (%s, %v) want (%s, %v)", tc.outcome, kind, score, tc.kind, tc.score)
		}
	}
}

func TestRewardEmitterDisabledNoOp(t *testing.T) {
	t.Parallel()
	e := NewRewardEmitter(false, &recordingPublisher{})
	if err := e.Emit(RewardEvent{Kind: RewardMergeSuccess, Reward: 1.0}); err != nil {
		t.Fatalf("disabled emit: %v", err)
	}
}

func TestRewardEmitterPublishes(t *testing.T) {
	t.Parallel()
	rec := &recordingPublisher{}
	e := NewRewardEmitter(true, rec)
	ev := RewardEvent{
		Kind:            RewardMergeSuccess,
		Reward:          1.0,
		SessionID:       "sess-1",
		Outcome:         "success",
		MergedCount:     2,
		TargetBranchRef: "main",
	}
	if err := e.Emit(ev); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if len(rec.topics) != 1 || rec.topics[0] != TopicReward {
		t.Fatalf("topics: %v", rec.topics)
	}
	payload, ok := rec.payloads[0].(RewardEvent)
	if !ok || payload.SessionID != "sess-1" || payload.Reward != 1.0 {
		t.Fatalf("payload: %+v", rec.payloads[0])
	}
	if payload.TimestampUnixNano == 0 {
		t.Fatal("expected timestamp to be set")
	}
}

type recordingPublisher struct {
	topics   []string
	payloads []any
}

func (r *recordingPublisher) Publish(topic string, payload any) error {
	r.topics = append(r.topics, topic)
	r.payloads = append(r.payloads, payload)
	return nil
}
