package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/eventbus"
)

func TestLifecycleCompleted(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	job, err := m.Enqueue(Request{
		Name: "ok",
		Run: func(context.Context) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	waitForState(t, m, job.ID, StateCompleted, 500*time.Millisecond)
	snapshot, ok := m.Get(job.ID)
	if !ok {
		t.Fatal("expected job snapshot")
	}
	if snapshot.StartedAt == nil || snapshot.FinishedAt == nil {
		t.Fatalf("expected timestamps, got %+v", snapshot)
	}
}

func TestLifecycleFailedPreservesError(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	wantErr := errors.New("backend unavailable")
	job, err := m.Enqueue(Request{
		Name: "boom",
		Run: func(context.Context) error {
			return wantErr
		},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	waitForState(t, m, job.ID, StateFailed, 500*time.Millisecond)
	snapshot, ok := m.Get(job.ID)
	if !ok {
		t.Fatal("expected job snapshot")
	}
	// Regression guard: a genuine run error must be classified as failed (not
	// cancelled) and must preserve the original error reason for diagnostics.
	if snapshot.State != StateFailed {
		t.Fatalf("state = %q, want failed", snapshot.State)
	}
	if snapshot.Error != wantErr.Error() {
		t.Fatalf("error = %q, want %q", snapshot.Error, wantErr.Error())
	}
}

func TestLifecycleRunResultCaptured(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	job, err := m.Enqueue(Request{
		Name: "with-result",
		RunResult: func(context.Context) (string, error) {
			return `{"served_by":"lpu-realtime"}`, nil
		},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitForState(t, m, job.ID, StateCompleted, 500*time.Millisecond)
	snapshot, ok := m.Get(job.ID)
	if !ok {
		t.Fatal("expected job snapshot")
	}
	if snapshot.Result != `{"served_by":"lpu-realtime"}` {
		t.Fatalf("result = %q, want captured payload", snapshot.Result)
	}
}

func TestEnqueueRejectsBothRunAndRunResult(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()
	_, err = m.Enqueue(Request{
		Run:       func(context.Context) error { return nil },
		RunResult: func(context.Context) (string, error) { return "", nil },
	})
	if !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("expected ErrInvalidJob, got %v", err)
	}
}

func TestBoundedConcurrency(t *testing.T) {
	const workers = 2
	const totalJobs = 6

	m, err := NewManager(Config{Workers: workers, QueueSize: totalJobs}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	var running int32
	var peak int32
	release := make(chan struct{})

	for i := 0; i < totalJobs; i++ {
		_, err := m.Enqueue(Request{Run: func(context.Context) error {
			current := atomic.AddInt32(&running, 1)
			for {
				max := atomic.LoadInt32(&peak)
				if current <= max || atomic.CompareAndSwapInt32(&peak, max, current) {
					break
				}
			}
			<-release
			atomic.AddInt32(&running, -1)
			return nil
		}})
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&peak) == workers {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&peak); got > workers {
		t.Fatalf("running workers exceeded limit: got %d want <= %d", got, workers)
	}

	closeOnce(release)
}

func TestQueueOverloadIsDeterministic(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 1}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	runStarted := make(chan struct{})
	release := make(chan struct{})
	defer closeOnce(release)

	_, err = m.Enqueue(Request{Run: func(ctx context.Context) error {
		closeOnce(runStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	}})
	if err != nil {
		t.Fatalf("enqueue #1: %v", err)
	}

	<-runStarted

	_, err = m.Enqueue(Request{Run: func(context.Context) error {
		return nil
	}})
	if err != nil {
		t.Fatalf("enqueue #2: %v", err)
	}

	_, err = m.Enqueue(Request{Run: func(context.Context) error { return nil }})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestCancelQueuedAndRunning(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	runStarted := make(chan struct{})
	releaseRun := make(chan struct{})
	runningJob, err := m.Enqueue(Request{Run: func(ctx context.Context) error {
		closeOnce(runStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseRun:
			return nil
		}
	}})
	if err != nil {
		t.Fatalf("enqueue running job: %v", err)
	}

	<-runStarted
	queuedJob, err := m.Enqueue(Request{Run: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatalf("enqueue queued job: %v", err)
	}

	if err := m.Cancel(queuedJob.ID); err != nil {
		t.Fatalf("cancel queued job: %v", err)
	}
	waitForState(t, m, queuedJob.ID, StateCancelled, 500*time.Millisecond)

	if err := m.Cancel(runningJob.ID); err != nil {
		t.Fatalf("cancel running job: %v", err)
	}
	waitForState(t, m, runningJob.ID, StateCancelled, 500*time.Millisecond)

	close(releaseRun)
}

func TestPublishLifecycleEvents(t *testing.T) {
	bus := eventbus.New()
	sub := bus.Subscribe()
	defer sub.Close()

	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, bus)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	job, err := m.Enqueue(Request{Run: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	states := map[State]bool{}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && !(states[StateQueued] && states[StateRunning] && states[StateCompleted]) {
		select {
		case msg := <-sub.Messages():
			if msg.Topic != eventbus.TopicNotificationsJobState {
				continue
			}
			var payload struct {
				JobID string `json:"job_id"`
				State State  `json:"state"`
			}
			if payloadErr := json.Unmarshal(msg.Payload, &payload); payloadErr != nil {
				t.Fatalf("unmarshal payload: %v", payloadErr)
			}
			if payload.JobID == job.ID {
				states[payload.State] = true
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	if !states[StateQueued] || !states[StateRunning] || !states[StateCompleted] {
		t.Fatalf("missing lifecycle notifications: %+v", states)
	}
}

func TestRaceSafeParallelGetCancel(t *testing.T) {
	m, err := NewManager(Config{Workers: 4, QueueSize: 32}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	ids := make([]string, 0, 16)
	for i := 0; i < 16; i++ {
		job, enqueueErr := m.Enqueue(Request{Run: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return nil
			}
		}})
		if enqueueErr != nil {
			t.Fatalf("enqueue %d: %v", i, enqueueErr)
		}
		ids = append(ids, job.ID)
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := ids[idx%len(ids)]
			_, _ = m.Get(id)
			_ = m.Cancel(id)
		}(i)
	}
	wg.Wait()
}

func TestCloseCancelsQueuedJobs(t *testing.T) {
	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	runningStarted := make(chan struct{})
	_, err = m.Enqueue(Request{Run: func(ctx context.Context) error {
		closeOnce(runningStarted)
		<-ctx.Done()
		return ctx.Err()
	}})
	if err != nil {
		t.Fatalf("enqueue running: %v", err)
	}
	<-runningStarted

	queued, err := m.Enqueue(Request{Run: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatalf("enqueue queued: %v", err)
	}

	m.Close()

	snapshot, ok := m.Get(queued.ID)
	if !ok {
		t.Fatal("expected queued job snapshot")
	}
	if snapshot.State != StateCancelled {
		t.Fatalf("queued state = %q, want cancelled", snapshot.State)
	}
	if snapshot.FinishedAt == nil {
		t.Fatal("expected queued finished_at on close cancellation")
	}
}

func TestPublishLifecycleErrorIsRedacted(t *testing.T) {
	bus := eventbus.New()
	sub := bus.Subscribe()
	defer sub.Close()

	m, err := NewManager(Config{Workers: 1, QueueSize: 2}, bus)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer m.Close()

	job, err := m.Enqueue(Request{Run: func(context.Context) error {
		return fmt.Errorf("upstream returned bearer token abc123")
	}})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case msg := <-sub.Messages():
			if msg.Topic != eventbus.TopicNotificationsJobState {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if payload["job_id"] != job.ID {
				continue
			}
			if payload["state"] != string(StateFailed) {
				continue
			}
			if got := payload["error"]; got != "error details redacted" {
				t.Fatalf("broadcast error = %v", got)
			}
			return
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("did not receive failed redacted notification")
}

func waitForState(t *testing.T, m *Manager, id string, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, ok := m.Get(id)
		if ok && job.State == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := m.Get(id)
	t.Fatalf("job %s did not reach %q, last=%q", id, want, job.State)
}

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
		return
	default:
		close(ch)
	}
}
