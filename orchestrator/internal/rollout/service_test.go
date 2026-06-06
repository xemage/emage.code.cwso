package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	svc := NewService(&stubRewardReader{}, nil, nil)
	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
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
	svc := NewService(reader, nil, nil)
	sub, _ := svc.SubmitTask(context.Background(), SubmitRequest{
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
	svc := NewService(&stubRewardReader{}, nil, nil)
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

func TestServiceSubmitWithPrefixRouter(t *testing.T) {
	t.Parallel()
	oid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	router := NewPrefixRouter(PrefixRouterConfig{
		Enabled:          true,
		SystemPromptHash: HashSystemPrompt(""),
		Resolver: &stubResolver{meta: WorkspaceMeta{
			BaseTreeOID: &oid,
			Files:       []WorkspaceFile{{Path: "main.go", BlobOID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		}},
	})
	svc := NewService(&stubRewardReader{}, nil, router)
	sub, err := svc.SubmitTask(context.Background(), SubmitRequest{
		TaskSpec: TaskSpec{Description: "train", WorkspaceID: "11111111-1111-1111-1111-111111111111"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.PrefixKey == "" || strings.HasPrefix(sub.PrefixKey, "prefix-") {
		t.Fatalf("expected BLAKE3 prefix key, got %q", sub.PrefixKey)
	}
}

func TestHTTPFleetStatus(t *testing.T) {
	svc := NewService(&stubRewardReader{}, nil, nil)
	h := NewHTTPHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/rollout/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code=%d", rec.Code)
	}
}
