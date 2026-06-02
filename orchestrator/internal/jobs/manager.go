package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/eventbus"
)

const (
	defaultWorkers   = 4
	defaultQueueSize = 64
)

var (
	ErrInvalidConfig = errors.New("invalid jobs manager config")
	ErrQueueFull     = errors.New("job queue is full")
	ErrJobNotFound   = errors.New("job not found")
	ErrInvalidJob    = errors.New("invalid job request")
	ErrClosed        = errors.New("jobs manager is closed")
)

// State is a job lifecycle state.
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// Publisher is a minimal event publication hook.
type Publisher interface {
	Publish(topic string, payload any) error
}

// Config controls worker and queue limits.
type Config struct {
	Workers   int
	QueueSize int
	Now       func() time.Time
}

// Request defines a job submission. Exactly one of Run or RunResult must be set:
// Run is the classic side-effect-only body; RunResult additionally returns a result
// payload captured into Job.Result and published on completion.
type Request struct {
	Name      string
	Timeout   time.Duration
	Run       func(context.Context) error
	RunResult func(context.Context) (string, error)
}

// Job is an immutable job snapshot returned by manager APIs.
type Job struct {
	ID         string
	Name       string
	State      State
	Error      string
	Result     string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Timeout    time.Duration
}

type record struct {
	job       Job
	run       func(context.Context) error
	runResult func(context.Context) (string, error)
	ctx       context.Context
	cancel    context.CancelFunc
}

// Manager executes jobs using a bounded queue and bounded worker pool.
type Manager struct {
	queue     chan *record
	publisher Publisher
	now       func() time.Time

	rootCtx    context.Context
	rootCancel context.CancelFunc
	closed     atomic.Bool
	idSeq      atomic.Uint64

	mu   sync.RWMutex
	jobs map[string]*record

	wg sync.WaitGroup
}

// NewManager constructs and starts a manager with fixed workers.
func NewManager(cfg Config, publisher Publisher) (*Manager, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = defaultWorkers
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.Workers <= 0 || cfg.QueueSize <= 0 {
		return nil, fmt.Errorf("%w: workers and queue size must be > 0", ErrInvalidConfig)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	m := &Manager{
		queue:      make(chan *record, cfg.QueueSize),
		publisher:  publisher,
		now:        cfg.Now,
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
		jobs:       make(map[string]*record),
	}

	for i := 0; i < cfg.Workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}

	return m, nil
}

// Close cancels in-flight work and stops workers.
func (m *Manager) Close() {
	if m.closed.Swap(true) {
		return
	}
	m.rootCancel()
	m.wg.Wait()
}

// Enqueue submits a job for asynchronous execution.
func (m *Manager) Enqueue(req Request) (Job, error) {
	if m.closed.Load() {
		return Job{}, ErrClosed
	}
	if req.Run == nil && req.RunResult == nil {
		return Job{}, fmt.Errorf("%w: run function is required", ErrInvalidJob)
	}
	if req.Run != nil && req.RunResult != nil {
		return Job{}, fmt.Errorf("%w: set exactly one of Run or RunResult", ErrInvalidJob)
	}
	if req.Timeout < 0 {
		return Job{}, fmt.Errorf("%w: timeout must be >= 0", ErrInvalidJob)
	}

	now := m.now()
	id := m.nextID(now)
	jobCtx := m.rootCtx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		jobCtx, cancel = context.WithTimeout(m.rootCtx, req.Timeout)
	} else {
		jobCtx, cancel = context.WithCancel(m.rootCtx)
	}

	r := &record{
		job: Job{
			ID:        id,
			Name:      req.Name,
			State:     StateQueued,
			CreatedAt: now,
			Timeout:   req.Timeout,
		},
		run:       req.Run,
		runResult: req.RunResult,
		ctx:       jobCtx,
		cancel:    cancel,
	}

	m.mu.Lock()
	m.jobs[id] = r
	snapshot := r.job
	m.mu.Unlock()

	select {
	case m.queue <- r:
		m.publishQueued(snapshot)
		return snapshot, nil
	default:
		m.mu.Lock()
		delete(m.jobs, id)
		m.mu.Unlock()
		cancel()
		return Job{}, ErrQueueFull
	}
}

// Cancel cancels a queued or running job by ID.
func (m *Manager) Cancel(id string) error {
	if id == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidJob)
	}

	m.mu.Lock()
	r, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}
	prev := r.job.State
	if isTerminal(prev) {
		m.mu.Unlock()
		return nil
	}
	r.cancel()
	if prev == StateQueued {
		now := m.now()
		r.job.State = StateCancelled
		r.job.Error = context.Canceled.Error()
		r.job.FinishedAt = &now
		snapshot := r.job
		m.mu.Unlock()
		m.publishTransition(snapshot, prev)
		return nil
	}
	m.mu.Unlock()
	return nil
}

// Get returns a point-in-time snapshot for a job ID.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return r.job, true
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.rootCtx.Done():
			return
		case r := <-m.queue:
			if r == nil {
				continue
			}
			m.runRecord(r)
		}
	}
}

func (m *Manager) runRecord(r *record) {
	if ok := m.transition(r.job.ID, StateRunning, ""); !ok {
		return
	}

	var result string
	var err error
	if r.runResult != nil {
		result, err = r.runResult(r.ctx)
	} else {
		err = r.run(r.ctx)
	}
	// Capture the context error before cancel(): the post-run cancel() below would
	// otherwise make ctx.Err() report Canceled for every job, masking genuine
	// failures as cancellations and discarding the real error reason.
	ctxErr := r.ctx.Err()
	r.cancel()

	if err == nil {
		m.transitionWithResult(r.job.ID, StateCompleted, "", result)
		return
	}

	if errors.Is(err, context.Canceled) || errors.Is(ctxErr, context.Canceled) ||
		errors.Is(ctxErr, context.DeadlineExceeded) {
		m.transition(r.job.ID, StateCancelled, context.Canceled.Error())
		return
	}

	m.transition(r.job.ID, StateFailed, err.Error())
}

func (m *Manager) transition(id string, next State, errMsg string) bool {
	return m.transitionWithResult(id, next, errMsg, "")
}

func (m *Manager) transitionWithResult(id string, next State, errMsg, result string) bool {
	m.mu.Lock()
	r, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return false
	}
	prev := r.job.State
	if !canTransition(prev, next) {
		m.mu.Unlock()
		return false
	}
	now := m.now()
	if next == StateRunning {
		r.job.StartedAt = &now
	}
	if isTerminal(next) {
		r.job.FinishedAt = &now
	}
	r.job.State = next
	r.job.Error = errMsg
	if result != "" {
		r.job.Result = result
	}
	snapshot := r.job
	m.mu.Unlock()

	m.publishTransition(snapshot, prev)
	return true
}

func (m *Manager) publishTransition(job Job, previous State) {
	payload := map[string]any{
		"job_id":         job.ID,
		"name":           job.Name,
		"state":          job.State,
		"previous_state": previous,
		"created_at":     job.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if job.StartedAt != nil {
		payload["started_at"] = job.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if job.FinishedAt != nil {
		payload["finished_at"] = job.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if job.Error != "" {
		payload["error"] = job.Error
	}
	if job.Result != "" {
		payload["result"] = job.Result
	}
	m.publish(eventbus.TopicNotificationsJobState, payload)
}

func (m *Manager) publishQueued(job Job) {
	payload := map[string]any{
		"job_id":     job.ID,
		"name":       job.Name,
		"state":      job.State,
		"created_at": job.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	m.publish(eventbus.TopicNotificationsJobState, payload)
}

func (m *Manager) publish(topic string, payload any) {
	if m.publisher == nil {
		return
	}
	_ = m.publisher.Publish(topic, payload)
}

func (m *Manager) nextID(now time.Time) string {
	seq := m.idSeq.Add(1)
	return fmt.Sprintf("job-%d-%d", now.UTC().UnixNano(), seq)
}

func isTerminal(s State) bool {
	switch s {
	case StateCompleted, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

func canTransition(from, to State) bool {
	switch from {
	case StateQueued:
		return to == StateRunning || to == StateCancelled
	case StateRunning:
		return to == StateCompleted || to == StateFailed || to == StateCancelled
	default:
		return false
	}
}
