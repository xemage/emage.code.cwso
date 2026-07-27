package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// TestPhase9TrainerE2EFlow exercises submit → reward → poll → callback (T138).
func TestPhase9TrainerE2EFlow(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	svc := NewService(broker, nil, nil)
	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		TaskSpec: TaskSpec{
			Description: "integration rollout",
			WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	ev := RewardEvent{
		Kind:      RewardMergeSuccess,
		Reward:    1.0,
		SessionID: sub.TaskID,
		Outcome:   "success",
	}
	if !broker.Ingest(TopicReward, ev) {
		t.Fatal("broker ingest failed")
	}
	waitBrokerRecords(t, broker, 1)

	status, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(status.PartialResults) != 1 || status.PartialResults[0].Reward != 1.0 {
		t.Fatalf("partial results: %+v", status.PartialResults)
	}

	group := TrajectoryGroup{
		SessionID: sub.TaskID,
		Chains: []Chain{{
			ChainID:        "chain-1",
			PrefixTokenIDs: []uint32{1, 2},
			Steps:          []Step{{TokenIDs: []uint32{3}, LossMask: []uint8{1}, Logprobs: []float64{-0.1}}},
		}},
		Rewards: []float64{1.0},
	}
	if err := svc.CompleteSession(sub.TaskID, "", &group); err != nil {
		t.Fatalf("complete: %v", err)
	}

	done, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get done: %v", err)
	}
	if done.Status != TaskCompleted {
		t.Fatalf("status: %q", done.Status)
	}
	if len(done.Trajectories) != 1 || done.Trajectories[0].LossMaskTokenCount != 1 {
		t.Fatalf("trajectories: %+v", done.Trajectories)
	}
}

// TestPhase9RESTTrainerE2E runs the HTTP surface used by external trainers.
func TestPhase9RESTTrainerE2E(t *testing.T) {
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	h := NewHTTPHandler(NewService(broker, nil, nil))

	subBody, _ := json.Marshal(SubmitRequest{
		TaskSpec: TaskSpec{
			Description: "http e2e",
			WorkspaceID: "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
		},
	})
	subReq := httptest.NewRequest(http.MethodPost, "/rollout/task/submit", bytes.NewReader(subBody))
	subRec := httptest.NewRecorder()
	h.ServeHTTP(subRec, subReq)
	if subRec.Code != http.StatusAccepted {
		t.Fatalf("submit: %d %s", subRec.Code, subRec.Body.String())
	}
	var sub SubmitResponse
	if err := json.Unmarshal(subRec.Body.Bytes(), &sub); err != nil {
		t.Fatalf("decode: %v", err)
	}

	_ = broker.Ingest(TopicReward, RewardEvent{
		Kind: RewardMergeConflict, Reward: -1.0, SessionID: sub.TaskID, Outcome: "conflict",
	})
	waitBrokerRecords(t, broker, 1)

	getReq := httptest.NewRequest(http.MethodGet, "/rollout/task/"+sub.TaskID, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: %d", getRec.Code)
	}
	var status TaskStatusResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(status.PartialResults) != 1 || status.PartialResults[0].MergeOutcome != "conflict" {
		t.Fatalf("status: %+v", status)
	}

	cbBody, _ := json.Marshal(map[string]any{
		"task_id": sub.TaskID,
		"trajectories": []TrajectoryGroup{{
			SessionID: sub.TaskID,
			Chains:    []Chain{{ChainID: "c1", Steps: []Step{{LossMask: []uint8{1}}}}},
			Metadata: map[string]string{
				"partial_result": `{"step":0,"reward":0.75,"merge_outcome":"completed"}`,
			},
		}},
	})
	cbReq := httptest.NewRequest(http.MethodPost, "/callbacks/session_result", bytes.NewReader(cbBody))
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cbReq)
	if cbRec.Code != http.StatusOK {
		t.Fatalf("callback: %d", cbRec.Code)
	}

	getReq = httptest.NewRequest(http.MethodGet, "/rollout/task/"+sub.TaskID, nil)
	getRec = httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get after callback: %d", getRec.Code)
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status after callback: %v", err)
	}
	if len(status.PartialResults) != 1 || status.PartialResults[0].MergeOutcome != "completed" {
		t.Fatalf("expected callback-derived partial result, got %+v", status)
	}
}

func TestNumSamplesSessionFanOut(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)
	svc := NewService(broker, nil, nil)

	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		NumSamples: 3,
		TaskSpec: TaskSpec{
			Description: "fan-out",
			WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.NumSamples != 3 || len(sub.SessionIDs) != 3 {
		t.Fatalf("submit resp: %+v", sub)
	}

	for i, sid := range sub.SessionIDs {
		_ = broker.Ingest(TopicReward, RewardEvent{
			Kind: RewardMergeSuccess, Reward: 1.0, SessionID: sid, Outcome: "success",
		})
		group := TrajectoryGroup{
			SessionID: sid,
			Chains:    []Chain{{ChainID: fmt.Sprintf("chain-%d", i), Steps: []Step{{LossMask: []uint8{1}}}}},
		}
		if err := svc.CompleteSession(sub.TaskID, sid, &group); err != nil {
			t.Fatalf("complete session %d: %v", i, err)
		}
	}
	waitBrokerRecords(t, broker, 3)

	status, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status.Status != TaskCompleted {
		t.Fatalf("status: %q", status.Status)
	}
	if status.SessionsCompleted != 3 || len(status.PartialResults) != 3 {
		t.Fatalf("status: %+v", status)
	}
	if len(status.Trajectories) != 3 {
		t.Fatalf("trajectories: %+v", status.Trajectories)
	}
}

// TestEvaluatorRegistryIntegration verifies post-run rewards attach to trajectories (T148).
func TestEvaluatorRegistryIntegration(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	svc := NewService(broker, nil, nil)
	svc.SetEvaluatorRegistry(NewRegistry(RegistryConfig{
		Enabled:              true,
		SessionRewardEnabled: true,
		SWEBenchEnabled:      true,
		SWEBenchInstance:     "django-1234",
		Rewards:              broker,
	}))

	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		TaskSpec: TaskSpec{
			Description: "evaluator registry",
			WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if !broker.Ingest(TopicReward, RewardEvent{
		Kind: RewardMergeSuccess, Reward: 1.0, SessionID: sub.TaskID, Outcome: "success",
	}) {
		t.Fatal("broker ingest failed")
	}
	waitBrokerRecords(t, broker, 1)

	group := TrajectoryGroup{
		SessionID: sub.TaskID,
		Chains: []Chain{{
			ChainID:        "chain-1",
			PrefixTokenIDs: []uint32{1},
			Steps:          []Step{{TokenIDs: []uint32{2}, LossMask: []uint8{1}}},
		}},
	}
	if err := svc.CompleteSession(sub.TaskID, "", &group); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if len(group.Rewards) != 2 {
		t.Fatalf("rewards: %+v", group.Rewards)
	}
	if group.Rewards[0] != 1.0 {
		t.Fatalf("session reward: %g", group.Rewards[0])
	}
	if group.Metadata["evaluator.session-reward.merge_outcome"] != "success" {
		t.Fatalf("session metadata: %+v", group.Metadata)
	}
	if group.Metadata["evaluator.swe-bench.instance_id"] != "django-1234" {
		t.Fatalf("swe-bench metadata: %+v", group.Metadata)
	}

	status, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status.Status != TaskCompleted {
		t.Fatalf("status: %q", status.Status)
	}
	if len(status.Trajectories) != 1 || status.Trajectories[0].TotalReward != 1.0 {
		t.Fatalf("trajectories: %+v", status.Trajectories)
	}
}

func waitBrokerRecords(t *testing.T, broker *memorybroker.Broker, want int) {
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

// TestTrajectoryBuilderIntegration verifies per-task strategy and prefix merge drain (T149).
func TestTrajectoryBuilderIntegration(t *testing.T) {
	t.Parallel()
	broker := memorybroker.New()
	t.Cleanup(broker.Close)

	svc := NewService(broker, nil, nil)
	svc.SetTrajectoryBuilder(BuilderConfig{
		Active:           true,
		DefaultStrategy:  StrategyPrefixMerge,
		InterstitialEOTs: true,
	})

	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		TrajectoryBuilderStrategy: StrategyPerRequest,
		TaskSpec: TaskSpec{
			Description: "builder integration",
			WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	svc.mu.Lock()
	task := svc.tasks[sub.TaskID]
	svc.mu.Unlock()
	if task.TrajectoryBuilderStrategy != StrategyPerRequest {
		t.Fatalf("task strategy = %q", task.TrajectoryBuilderStrategy)
	}

	records := polarFig4Fixture()
	group := assembleTrajectoryGroup(sub.TaskID, records, svc.builder, task.TrajectoryBuilderStrategy)
	if err := ValidateTrajectoryGroup(group); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(group.Chains) != 5 {
		t.Fatalf("per_request integration chains = %d", len(group.Chains))
	}

	merged := assembleTrajectoryGroup(sub.TaskID, records, svc.builder, StrategyPrefixMerge)
	if len(merged.Chains) != 3 {
		t.Fatalf("prefix_merge integration chains = %d", len(merged.Chains))
	}
	if trainerSampleCount(merged) >= trainerSampleCount(group) {
		t.Fatalf("prefix_merge should reduce trainer samples")
	}
}

// TestGatewayTimeoutPartialTraceRecovery verifies POSTRUN emits partial trajectories (T146).
func TestGatewayTimeoutPartialTraceRecovery(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRewardReader{}, nil, nil)
	gw, err := NewGateway(GatewayConfig{
		InitWorkers: 1, ReadyBuffer: 1, RunningWorkers: 1, PostRunWorkers: 1,
		SessionTimeout: 30 * time.Millisecond,
		Hooks: SessionHooks{
			Run: func(ctx context.Context, _ SessionState) error {
				select {
				case <-time.After(200 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			PostRun: func(_ context.Context, state SessionState, timedOut bool) (*TrajectoryGroup, error) {
				return &TrajectoryGroup{
					SessionID: state.SessionID,
					Chains:    []Chain{{ChainID: "timeout-partial", Steps: []Step{{LossMask: []uint8{1, 1}}}}},
					Metadata:  map[string]string{"timeout": "true", "gateway_stage": "postrun"},
				}, nil
			},
		},
	}, svc)
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	t.Cleanup(gw.Close)
	svc.AttachGateway(gw)

	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		TaskSpec: TaskSpec{Description: "timeout", WorkspaceID: "cccccccc-dddd-eeee-ffff-000000000001"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, err := svc.GetTask(context.Background(), sub.TaskID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if status.Status == TaskFailed && len(status.Trajectories) == 1 {
			if status.Trajectories[0].ChainID != "timeout-partial" {
				t.Fatalf("trajectories: %+v", status.Trajectories)
			}
			if status.Trajectories[0].LossMaskTokenCount != 2 {
				t.Fatalf("loss mask count: %d", status.Trajectories[0].LossMaskTokenCount)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("gateway did not recover partial trace on timeout")
}
