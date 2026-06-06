package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

type stubRewardReader struct {
	records []memorybroker.Record
}

func (s *stubRewardReader) Query(opts memorybroker.QueryOptions) []memorybroker.Record {
	return s.records
}

func TestServiceSubmitAndGetTask(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRewardReader{}, nil)
	sub, err := svc.SubmitTask(SubmitRequest{
		TaskSpec: TaskSpec{Description: "train", WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.TaskID == "" || sub.Status != TaskRunning {
		t.Fatalf("submit resp: %+v", sub)
	}
	status, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status.TaskID != sub.TaskID {
		t.Fatalf("status: %+v", status)
	}
}

func TestServiceGetTaskIncludesRewards(t *testing.T) {
	t.Parallel()
	ev := RewardEvent{
		Kind: RewardMergeSuccess, Reward: 1.0, SessionID: "sess-1", Outcome: "success",
	}
	raw, _ := json.Marshal(ev)
	reader := &stubRewardReader{records: []memorybroker.Record{{Topic: TopicReward, Payload: raw}}}
	svc := NewService(reader, nil)
	sub, _ := svc.SubmitTask(SubmitRequest{
		TaskSpec: TaskSpec{Description: "x", WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	})
	svc.mu.Lock()
	svc.tasks[sub.TaskID].SessionID = "sess-1"
	svc.mu.Unlock()

	status, err := svc.GetTask(context.Background(), sub.TaskID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(status.PartialResults) != 1 || status.PartialResults[0].Reward != 1.0 {
		t.Fatalf("partial: %+v", status.PartialResults)
	}
}

func TestHTTPSubmitAndPoll(t *testing.T) {
	svc := NewService(&stubRewardReader{}, nil)
	h := NewHTTPHandler(svc)

	body, _ := json.Marshal(SubmitRequest{
		TaskSpec: TaskSpec{Description: "demo", WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	})
	subReq := httptest.NewRequest(http.MethodPost, "/rollout/task/submit", bytes.NewReader(body))
	subRec := httptest.NewRecorder()
	h.ServeHTTP(subRec, subReq)
	if subRec.Code != http.StatusAccepted {
		t.Fatalf("submit code=%d body=%s", subRec.Code, subRec.Body.String())
	}
	var sub SubmitResponse
	if err := json.Unmarshal(subRec.Body.Bytes(), &sub); err != nil {
		t.Fatalf("decode submit: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/rollout/task/"+sub.TaskID, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get code=%d", getRec.Code)
	}
}

func TestHTTPFleetStatus(t *testing.T) {
	svc := NewService(&stubRewardReader{}, nil)
	h := NewHTTPHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/rollout/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code=%d", rec.Code)
	}
}
