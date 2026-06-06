package rollout

import (
	"bytes"
	"context"
	"encoding/json"
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

	svc := NewService(broker, nil)
	sub, err := svc.SubmitTask(SubmitRequest{
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
	if err := svc.CompleteSession(sub.TaskID, &group); err != nil {
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

	h := NewHTTPHandler(NewService(broker, nil))

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
		}},
	})
	cbReq := httptest.NewRequest(http.MethodPost, "/callbacks/session_result", bytes.NewReader(cbBody))
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cbReq)
	if cbRec.Code != http.StatusOK {
		t.Fatalf("callback: %d", cbRec.Code)
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
