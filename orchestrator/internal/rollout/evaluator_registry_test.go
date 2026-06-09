package rollout

import (
	"context"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func TestRegistryDisabledIsNoOp(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(RegistryConfig{Enabled: false})
	if registry.Enabled() {
		t.Fatal("expected disabled registry")
	}
	group := &TrajectoryGroup{SessionID: "s1"}
	out, err := registry.ApplyEvaluations(context.Background(), EvalRequest{Group: group})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out != group || len(group.Rewards) != 0 {
		t.Fatalf("group mutated: %+v", group)
	}
}

func TestSessionRewardPluginAggregatesRewards(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	plugin := NewSessionRewardPlugin(true, broker)
	_ = broker.Ingest(TopicReward, RewardEvent{
		Kind: RewardMergeSuccess, Reward: 1.0, SessionID: "sess-a", Outcome: "success",
	})
	_ = broker.Ingest(TopicReward, RewardEvent{
		Kind: RewardMergeConflict, Reward: -0.5, SessionID: "sess-a", Outcome: "conflict",
	})
	waitBrokerLen(t, broker, 2)

	result, err := plugin.Evaluate(context.Background(), EvalRequest{SessionID: "sess-a"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Reward != 0.5 {
		t.Fatalf("reward: %g", result.Reward)
	}
	if result.Metadata["merge_outcome"] != "conflict" {
		t.Fatalf("metadata: %+v", result.Metadata)
	}
}

func TestSWEBenchPluginStubMetadata(t *testing.T) {
	t.Parallel()
	plugin := NewSWEBenchPlugin(SWEBenchConfig{Enabled: true, InstanceID: "django-1234"})
	result, err := plugin.Evaluate(context.Background(), EvalRequest{SessionID: "s1"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Reward != 0 {
		t.Fatalf("reward: %g", result.Reward)
	}
	if result.Metadata["instance_id"] != "django-1234" || result.Metadata["status"] != "stub" {
		t.Fatalf("metadata: %+v", result.Metadata)
	}
}

func TestAttachEvaluationsPropagatesRewards(t *testing.T) {
	t.Parallel()
	group := &TrajectoryGroup{SessionID: "s1", Metadata: map[string]string{"builder": "v1"}}
	AttachEvaluations(group, []EvalResult{
		{EvaluatorID: EvaluatorSessionReward, Reward: 1.0, Metadata: map[string]string{"merge_outcome": "success"}},
		{EvaluatorID: EvaluatorSWEBench, Reward: 0, Metadata: map[string]string{"status": "stub"}},
	})
	if len(group.Rewards) != 2 || group.Rewards[0] != 1.0 {
		t.Fatalf("rewards: %+v", group.Rewards)
	}
	if group.Metadata["evaluator.session-reward.merge_outcome"] != "success" {
		t.Fatalf("metadata: %+v", group.Metadata)
	}
}

func TestRegistryRunsEnabledPluginsOnly(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	registry := NewRegistry(RegistryConfig{
		Enabled:              true,
		SessionRewardEnabled: true,
		SWEBenchEnabled:      false,
		Rewards:              broker,
	})
	_ = broker.Ingest(TopicReward, RewardEvent{
		Kind: RewardMergeSuccess, Reward: 1.0, SessionID: "sess-b",
	})
	waitBrokerLen(t, broker, 1)
	group := &TrajectoryGroup{SessionID: "sess-b"}
	_, err := registry.ApplyEvaluations(context.Background(), EvalRequest{
		SessionID: "sess-b",
		Group:     group,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(group.Rewards) != 1 || group.Rewards[0] != 1.0 {
		t.Fatalf("rewards: %+v", group.Rewards)
	}
	if _, ok := group.Metadata["evaluator.swe-bench.status"]; ok {
		t.Fatalf("swe-bench should be disabled: %+v", group.Metadata)
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	t.Parallel()
	registry := &Registry{enabled: true, plugins: make(map[EvaluatorID]Plugin)}
	plugin := NewSessionRewardPlugin(true, memorybroker.New())
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := registry.Register(plugin); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func waitBrokerLen(t *testing.T, broker *memorybroker.Broker, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broker.Len() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("broker has %d records, want >= %d", broker.Len(), want)
}
