package rollout

import (
	"context"
	"fmt"
	"time"
)

func (g *Gateway) runInit(job sessionJob) {
	state := job.state
	if g.cfg.Hooks.Init != nil {
		_ = g.cfg.Hooks.Init(g.rootCtx, state)
	}
	select {
	case g.readyQueue <- job:
	case <-g.rootCtx.Done():
	}
}

func (g *Gateway) runRunning(job sessionJob) {
	state := job.state
	if g.cfg.Evaluator != nil && g.cfg.Evaluator.Enabled() {
		go func(sid string) {
			_ = g.cfg.Evaluator.Prewarm(context.Background(), sid)
		}(state.SessionID)
	}
	runCtx, cancel := context.WithTimeout(g.rootCtx, g.cfg.SessionTimeout)
	defer cancel()
	timedOut := g.executeRun(runCtx, state)
	g.postPool.submit(g.rootCtx, func() {
		g.runPostRun(state, timedOut)
	})
}

func (g *Gateway) executeRun(ctx context.Context, state SessionState) bool {
	if g.cfg.Hooks.Run == nil {
		<-ctx.Done()
		return ctx.Err() == context.DeadlineExceeded
	}
	err := g.cfg.Hooks.Run(ctx, state)
	if err == nil {
		return false
	}
	return ctx.Err() == context.DeadlineExceeded
}

func (g *Gateway) runPostRun(state SessionState, timedOut bool) {
	state.TimedOut = timedOut
	group := g.capturePostRun(state)
	out := SessionOutcome{Group: group, TimedOut: timedOut}
	if timedOut {
		out.Status = TaskFailed
		out.Error = "session timeout"
	} else {
		out.Status = TaskCompleted
	}
	_ = g.svc.ApplySessionOutcome(state.TaskID, state.SessionID, out)
}

func (g *Gateway) capturePostRun(state SessionState) *TrajectoryGroup {
	if g.cfg.Hooks.PostRun != nil {
		group, err := g.cfg.Hooks.PostRun(g.rootCtx, state, state.TimedOut)
		if err == nil && group != nil {
			return group
		}
	}
	return g.partialTraceFromClient(state)
}

func (g *Gateway) partialTraceFromClient(state SessionState) *TrajectoryGroup {
	meta := map[string]string{"gateway_stage": string(StagePostRun)}
	if state.TimedOut {
		meta["timeout"] = "true"
	}
	if g.cfg.Client == nil {
		return &TrajectoryGroup{
			SessionID: state.SessionID,
			Metadata:  meta,
		}
	}
	group, err := g.cfg.Client.BuildFromDrain(g.rootCtx, state.SessionID, 64)
	if err != nil || len(group.Chains) == 0 {
		return &TrajectoryGroup{
			SessionID: state.SessionID,
			Metadata:  meta,
		}
	}
	if group.Metadata == nil {
		group.Metadata = meta
	} else {
		group.Metadata["gateway_stage"] = string(StagePostRun)
		if state.TimedOut {
			group.Metadata["timeout"] = "true"
		}
	}
	group.SessionID = state.SessionID
	return &group
}

// ApplySessionOutcome stores gateway POSTRUN results on the task record.
func (s *Service) ApplySessionOutcome(taskID, sessionID string, out SessionOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return errNotFound
	}
	if out.Error != "" {
		task.LastError = out.Error
	}
	if out.Group != nil {
		s.storeSessionGroup(task, sessionID, out.Group)
	}
	s.applyTerminalStatus(task, sessionID, out)
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Service) storeSessionGroup(task *Task, sessionID string, group *TrajectoryGroup) {
	if task.NumSamples > 1 {
		if task.SessionGroups == nil {
			task.SessionGroups = make(map[string]*TrajectoryGroup, task.NumSamples)
		}
		task.SessionGroups[sessionID] = group
		return
	}
	task.TrajectoryGroup = group
}

func (s *Service) applyTerminalStatus(task *Task, sessionID string, out SessionOutcome) {
	if task.NumSamples > 1 {
		if out.TimedOut {
			task.Status = TaskFailed
			if task.LastError == "" {
				task.LastError = fmt.Sprintf("session %s timeout", sessionID)
			}
			return
		}
		if len(task.SessionGroups) >= task.NumSamples {
			task.Status = TaskCompleted
		}
		return
	}
	task.Status = out.Status
}
