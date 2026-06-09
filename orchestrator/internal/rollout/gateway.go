package rollout

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Stage names Polar gateway worker pools (§3.3).
type Stage string

const (
	StageInit    Stage = "init"
	StageReady   Stage = "ready"
	StageRunning Stage = "running"
	StagePostRun Stage = "postrun"
)

// GatewayConfig controls staged worker pools and session timeout (T146).
type GatewayConfig struct {
	InitWorkers    int
	ReadyBuffer    int
	RunningWorkers int
	PostRunWorkers int
	SessionTimeout time.Duration
	Evaluator      Evaluator
	Hooks          SessionHooks
	Client         *Client
}

// SessionHooks inject stage work for tests and future harness wiring.
type SessionHooks struct {
	Init    func(ctx context.Context, state SessionState) error
	Run     func(ctx context.Context, state SessionState) error
	PostRun func(ctx context.Context, state SessionState, timedOut bool) (*TrajectoryGroup, error)
}

// SessionState tracks one rollout session through gateway stages.
type SessionState struct {
	TaskID    string
	SessionID string
	Spec      TaskSpec
	TimedOut  bool
}

// SessionOutcome is applied to the rollout Service after POSTRUN.
type SessionOutcome struct {
	Status   TaskStatus
	Error    string
	Group    *TrajectoryGroup
	TimedOut bool
}

// Gateway runs INIT → READY → RUNNING → POSTRUN with isolated worker pools.
type Gateway struct {
	cfg        GatewayConfig
	svc        *Service
	initPool   *stagePool
	readyQueue chan sessionJob
	runPool    *stagePool
	postPool   *stagePool
	rootCtx    context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup
	closed     atomic.Bool
}

type sessionJob struct {
	state SessionState
}

// NewGateway constructs staged pools. Caller must call Close on shutdown.
func NewGateway(cfg GatewayConfig, svc *Service) (*Gateway, error) {
	if svc == nil {
		return nil, errors.New("rollout service is required")
	}
	normalizeGatewayConfig(&cfg)
	rootCtx, rootCancel := context.WithCancel(context.Background())
	g := &Gateway{
		cfg:        cfg,
		svc:        svc,
		initPool:   newStagePool(StageInit, cfg.InitWorkers, cfg.ReadyBuffer),
		readyQueue: make(chan sessionJob, cfg.ReadyBuffer),
		runPool:    newStagePool(StageRunning, cfg.RunningWorkers, cfg.ReadyBuffer),
		postPool:   newStagePool(StagePostRun, cfg.PostRunWorkers, cfg.ReadyBuffer),
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
	}
	g.wg.Add(1)
	go g.dispatchReady()
	return g, nil
}

func normalizeGatewayConfig(cfg *GatewayConfig) {
	if cfg.InitWorkers <= 0 {
		cfg.InitWorkers = 2
	}
	if cfg.ReadyBuffer <= 0 {
		cfg.ReadyBuffer = 4
	}
	if cfg.RunningWorkers <= 0 {
		cfg.RunningWorkers = 4
	}
	if cfg.PostRunWorkers <= 0 {
		cfg.PostRunWorkers = 2
	}
	if cfg.SessionTimeout <= 0 {
		cfg.SessionTimeout = 5 * time.Minute
	}
}

// StartSession enqueues async gateway processing for one session.
func (g *Gateway) StartSession(taskID, sessionID string, spec TaskSpec) {
	if g == nil || g.closed.Load() {
		return
	}
	job := sessionJob{state: SessionState{TaskID: taskID, SessionID: sessionID, Spec: spec}}
	g.initPool.submit(g.rootCtx, func() { g.runInit(job) })
}

// Close stops gateway workers.
func (g *Gateway) Close() {
	if g == nil || g.closed.Swap(true) {
		return
	}
	g.rootCancel()
	g.initPool.close()
	g.runPool.close()
	g.postPool.close()
	close(g.readyQueue)
	g.wg.Wait()
}

func (g *Gateway) dispatchReady() {
	defer g.wg.Done()
	for {
		select {
		case <-g.rootCtx.Done():
			return
		case job, ok := <-g.readyQueue:
			if !ok {
				return
			}
			j := job
			g.runPool.submit(g.rootCtx, func() { g.runRunning(j) })
		}
	}
}

// PoolDepths exposes queue depths for observability tests.
func (g *Gateway) PoolDepths() map[Stage]int {
	if g == nil {
		return nil
	}
	return map[Stage]int{
		StageInit:    g.initPool.depth(),
		StageReady:   len(g.readyQueue),
		StageRunning: g.runPool.depth(),
		StagePostRun: g.postPool.depth(),
	}
}

type stagePool struct {
	stage   Stage
	workers int
	queue   chan func()
	wg      sync.WaitGroup
	closed  atomic.Bool
}

func newStagePool(stage Stage, workers, queueSize int) *stagePool {
	if queueSize < workers {
		queueSize = workers
	}
	p := &stagePool{stage: stage, workers: workers, queue: make(chan func(), queueSize)}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *stagePool) worker() {
	defer p.wg.Done()
	for fn := range p.queue {
		if fn != nil {
			fn()
		}
	}
}

func (p *stagePool) submit(ctx context.Context, fn func()) {
	if p == nil || fn == nil || p.closed.Load() {
		return
	}
	select {
	case p.queue <- fn:
	case <-ctx.Done():
	}
}

func (p *stagePool) close() {
	if p == nil || p.closed.Swap(true) {
		return
	}
	close(p.queue)
	p.wg.Wait()
}

func (p *stagePool) depth() int {
	return len(p.queue)
}
