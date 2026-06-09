package rollout

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGatewayStagePoolsProcessSession(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRewardReader{}, nil, nil)
	var initSeen, runSeen, postSeen atomic.Bool
	gw, err := NewGateway(GatewayConfig{
		InitWorkers:    1,
		ReadyBuffer:    2,
		RunningWorkers: 1,
		PostRunWorkers: 1,
		SessionTimeout: 50 * time.Millisecond,
		Hooks: SessionHooks{
			Init: func(context.Context, SessionState) error {
				initSeen.Store(true)
				return nil
			},
			Run: func(ctx context.Context, _ SessionState) error {
				runSeen.Store(true)
				<-ctx.Done()
				return ctx.Err()
			},
			PostRun: func(_ context.Context, state SessionState, timedOut bool) (*TrajectoryGroup, error) {
				postSeen.Store(true)
				if !timedOut {
					t.Fatal("expected timeout")
				}
				return &TrajectoryGroup{
					SessionID: state.SessionID,
					Chains:    []Chain{{ChainID: "partial", Steps: []Step{{LossMask: []uint8{1}}}}},
					Metadata:  map[string]string{"timeout": "true"},
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
		TaskSpec: TaskSpec{Description: "gateway", WorkspaceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		status, err := svc.GetTask(context.Background(), sub.TaskID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if status.Status == TaskFailed && len(status.Trajectories) == 1 {
			if !initSeen.Load() || !runSeen.Load() || !postSeen.Load() {
				t.Fatalf("stages init=%v run=%v post=%v", initSeen.Load(), runSeen.Load(), postSeen.Load())
			}
			if status.Error != "session timeout" {
				t.Fatalf("error: %q", status.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for gateway POSTRUN timeout result")
}

func TestStubEvaluatorPrewarmWithoutSidecar(t *testing.T) {
	t.Parallel()
	eval := NewStubEvaluator(nil, true)
	if err := eval.Prewarm(context.Background(), "sess-1"); err != nil {
		t.Fatalf("prewarm: %v", err)
	}
}

func TestGatewayConfigFromDefaults(t *testing.T) {
	t.Parallel()
	cfg := GatewayConfigFrom(nil, nil)
	if cfg.InitWorkers != 0 {
		t.Fatalf("expected zero init workers from nil config, got %d", cfg.InitWorkers)
	}
}

func TestApplySessionOutcomeMultiSample(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRewardReader{}, nil, nil)
	taskID := "task-1"
	svc.mu.Lock()
	svc.tasks[taskID] = &Task{
		ID: taskID, Status: TaskRunning, NumSamples: 2,
		SessionIDs: []string{"s1", "s2"}, SessionGroups: map[string]*TrajectoryGroup{},
	}
	svc.mu.Unlock()

	group := &TrajectoryGroup{SessionID: "s1", Chains: []Chain{{ChainID: "c1"}}}
	if err := svc.ApplySessionOutcome(taskID, "s1", SessionOutcome{
		Status: TaskCompleted, Group: group,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	status, _ := svc.GetTask(context.Background(), taskID)
	if status.SessionsCompleted != 1 || status.Status != TaskRunning {
		t.Fatalf("status after one session: %+v", status)
	}
}
