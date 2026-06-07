package rollout

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// TaskStatus is the rollout task lifecycle state (schemas/rollout_task_status.json).
type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"

	defaultNumSamples = 1
	maxNumSamples     = 32
)

// TaskSpec is the inner task_spec from POST /rollout/task/submit.
type TaskSpec struct {
	Description string `json:"description"`
	WorkspaceID string `json:"workspace_id"`
	MaxSteps    int    `json:"max_steps,omitempty"`
}

// SubmitRequest is the POST /rollout/task/submit body.
type SubmitRequest struct {
	TaskSpec           TaskSpec `json:"task_spec"`
	TrainerCallbackURL string   `json:"trainer_callback_url,omitempty"`
	PrewarmKVPrefix    *bool    `json:"prewarm_kv_prefix,omitempty"`
	NumSamples         int      `json:"num_samples,omitempty"`
}

// SubmitResponse is returned when a rollout task is accepted.
type SubmitResponse struct {
	TaskID     string     `json:"task_id"`
	Status     TaskStatus `json:"status"`
	PrefixKey  string     `json:"prefix_key,omitempty"`
	NumSamples int        `json:"num_samples,omitempty"`
	SessionIDs []string   `json:"session_ids,omitempty"`
}

// PartialResult mirrors rollout_task_status partial_results items.
type PartialResult struct {
	Step         int     `json:"step"`
	Reward       float64 `json:"reward"`
	MergeOutcome string  `json:"merge_outcome"`
}

// TrajectorySummary is a compact trajectory entry in task status.
type TrajectorySummary struct {
	ChainID            string  `json:"chain_id"`
	LossMaskTokenCount int     `json:"loss_mask_token_count"`
	TotalReward        float64 `json:"total_reward,omitempty"`
}

// TaskStatusResponse is GET /rollout/task/{task_id}.
type TaskStatusResponse struct {
	TaskID            string              `json:"task_id"`
	Status            TaskStatus          `json:"status"`
	PartialResults    []PartialResult     `json:"partial_results,omitempty"`
	Trajectories      []TrajectorySummary `json:"trajectories,omitempty"`
	Error             string              `json:"error,omitempty"`
	NumSamples        int                 `json:"num_samples,omitempty"`
	SessionIDs        []string            `json:"session_ids,omitempty"`
	SessionsCompleted int                 `json:"sessions_completed,omitempty"`
}

// FleetStatus is GET /rollout/status.
type FleetStatus struct {
	PendingSessions int     `json:"pending_sessions"`
	RunningSessions int     `json:"running_sessions"`
	RegisteredNodes int     `json:"registered_nodes"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

// Task is an in-memory rollout task record.
type Task struct {
	ID              string
	Status          TaskStatus
	Spec            TaskSpec
	NumSamples      int
	SessionID       string
	SessionIDs      []string
	SessionGroups   map[string]*TrajectoryGroup
	CallbackURL     string
	PrefixKey       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastError       string
	PartialResults  []PartialResult
	TrajectoryGroup *TrajectoryGroup
}

// Node is a registered rollout worker node.
type Node struct {
	ID            string
	LastHeartbeat time.Time
}

// RewardReader queries published merge rewards.
type RewardReader interface {
	Query(opts memorybroker.QueryOptions) []memorybroker.Record
}

// Service implements Polar REST semantics (rollout-architecture-v1 §8).
type Service struct {
	mu           sync.RWMutex
	tasks        map[string]*Task
	nodes        map[string]*Node
	rewards      RewardReader
	client       *Client
	prefixRouter *PrefixRouter
}

// NewService constructs an in-memory rollout API service.
func NewService(rewards RewardReader, client *Client, prefixRouter *PrefixRouter) *Service {
	return &Service{
		tasks:        make(map[string]*Task),
		nodes:        make(map[string]*Node),
		rewards:      rewards,
		client:       client,
		prefixRouter: prefixRouter,
	}
}

// SubmitTask enqueues a rollout task (optionally fanning out num_samples sessions).
func (s *Service) SubmitTask(ctx context.Context, req SubmitRequest) (SubmitResponse, error) {
	if req.TaskSpec.Description == "" || req.TaskSpec.WorkspaceID == "" {
		return SubmitResponse{}, fmt.Errorf("task_spec.description and workspace_id are required")
	}
	num := req.NumSamples
	if num <= 0 {
		num = defaultNumSamples
	}
	if num > maxNumSamples {
		return SubmitResponse{}, fmt.Errorf("num_samples must be <= %d", maxNumSamples)
	}
	id, err := newUUID()
	if err != nil {
		return SubmitResponse{}, err
	}
	now := time.Now().UTC()
	prefixKey := ""
	if req.PrewarmKVPrefix == nil || *req.PrewarmKVPrefix {
		prefixKey, err = s.resolvePrefixKey(ctx, req.TaskSpec.WorkspaceID)
		if err != nil {
			return SubmitResponse{}, err
		}
	}
	task := &Task{
		ID:          id,
		Status:      TaskRunning,
		Spec:        req.TaskSpec,
		NumSamples:  num,
		CallbackURL: req.TrainerCallbackURL,
		PrefixKey:   prefixKey,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if num == 1 {
		task.SessionID = id
	} else {
		task.SessionIDs = make([]string, num)
		task.SessionGroups = make(map[string]*TrajectoryGroup, num)
		for i := range task.SessionIDs {
			task.SessionIDs[i], err = newUUID()
			if err != nil {
				return SubmitResponse{}, err
			}
		}
	}
	s.mu.Lock()
	s.tasks[id] = task
	s.mu.Unlock()
	resp := SubmitResponse{TaskID: id, Status: TaskRunning, PrefixKey: prefixKey}
	if num > 1 {
		resp.NumSamples = num
		resp.SessionIDs = append([]string(nil), task.SessionIDs...)
	}
	return resp, nil
}

// GetTask returns task status with rewards and optional trajectories.
func (s *Service) GetTask(ctx context.Context, taskID string) (TaskStatusResponse, error) {
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		return TaskStatusResponse{}, errNotFound
	}
	resp := TaskStatusResponse{
		TaskID: taskID,
		Status: task.Status,
		Error:  task.LastError,
	}
	if task.NumSamples > 1 {
		resp.NumSamples = task.NumSamples
		resp.SessionIDs = append([]string(nil), task.SessionIDs...)
		resp.SessionsCompleted = len(task.SessionGroups)
	}
	rewards := s.collectRewardsForTask(task)
	resp.PartialResults = s.mergePartialResults(task, rewards)
	resp.Trajectories = s.trajectorySummaries(ctx, task)
	return resp, nil
}

// FleetStatus returns aggregate rollout fleet metrics.
func (s *Service) FleetStatus(ctx context.Context) FleetStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pending, running int
	for _, t := range s.tasks {
		sessions := taskSessionCount(t)
		switch t.Status {
		case TaskQueued:
			pending += sessions
		case TaskRunning:
			if t.NumSamples > 1 && len(t.SessionGroups) > 0 {
				running += t.NumSamples - len(t.SessionGroups)
			} else {
				running += sessions
			}
		}
	}
	status := FleetStatus{
		PendingSessions: pending,
		RunningSessions: running,
		RegisteredNodes: len(s.nodes),
	}
	if s.client != nil {
		if stats, err := s.client.PrefixStats(ctx); err == nil {
			status.CacheHitRate = stats.HitRate
		}
	}
	return status
}

func (s *Service) resolvePrefixKey(ctx context.Context, workspaceID string) (string, error) {
	if s.prefixRouter == nil || !s.prefixRouter.Enabled() {
		return "", nil
	}
	return s.prefixRouter.Prewarm(ctx, workspaceID)
}

// RegisterNode records a rollout worker node.
func (s *Service) RegisterNode(nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node id required")
	}
	s.mu.Lock()
	s.nodes[nodeID] = &Node{ID: nodeID, LastHeartbeat: time.Now().UTC()}
	s.mu.Unlock()
	return nil
}

// HeartbeatNode updates node liveness.
func (s *Service) HeartbeatNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[nodeID]
	if !ok {
		return errNotFound
	}
	n.LastHeartbeat = time.Now().UTC()
	return nil
}

// CompleteSession marks a task or individual session completed from trainer callback.
func (s *Service) CompleteSession(taskID, sessionID string, group *TrajectoryGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return errNotFound
	}
	if task.NumSamples <= 1 {
		task.Status = TaskCompleted
		task.UpdatedAt = time.Now().UTC()
		if group != nil {
			task.TrajectoryGroup = group
		}
		return nil
	}
	if sessionID == "" {
		return fmt.Errorf("session_id required when num_samples > 1")
	}
	if !containsString(task.SessionIDs, sessionID) {
		return fmt.Errorf("unknown session_id for task")
	}
	if task.SessionGroups == nil {
		task.SessionGroups = make(map[string]*TrajectoryGroup, task.NumSamples)
	}
	task.SessionGroups[sessionID] = group
	if len(task.SessionGroups) >= task.NumSamples {
		task.Status = TaskCompleted
	}
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Service) collectRewardsForTask(task *Task) []RewardEvent {
	ids := sessionIDsForTask(task)
	if len(ids) == 0 {
		return nil
	}
	var out []RewardEvent
	for _, sid := range ids {
		out = append(out, s.collectRewards(sid)...)
	}
	return out
}

func sessionIDsForTask(task *Task) []string {
	if len(task.SessionIDs) > 0 {
		return task.SessionIDs
	}
	if task.SessionID != "" {
		return []string{task.SessionID}
	}
	return nil
}

func taskSessionCount(task *Task) int {
	if task.NumSamples > 0 {
		return task.NumSamples
	}
	return 1
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (s *Service) collectRewards(sessionID string) []RewardEvent {
	if s.rewards == nil || sessionID == "" {
		return nil
	}
	records := s.rewards.Query(memorybroker.QueryOptions{Topics: []string{TopicReward}, Limit: 512})
	out := make([]RewardEvent, 0, len(records))
	for _, rec := range records {
		var ev RewardEvent
		if err := json.Unmarshal(rec.Payload, &ev); err != nil {
			continue
		}
		if ev.SessionID != sessionID {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func (s *Service) mergePartialResults(task *Task, rewards []RewardEvent) []PartialResult {
	if len(rewards) == 0 {
		return task.PartialResults
	}
	out := make([]PartialResult, 0, len(rewards))
	for i, ev := range rewards {
		out = append(out, PartialResult{
			Step:         i,
			Reward:       ev.Reward,
			MergeOutcome: mergeOutcomeFromReward(ev),
		})
	}
	return out
}

func mergeOutcomeFromReward(ev RewardEvent) string {
	switch ev.Kind {
	case RewardMergeSuccess:
		return "success"
	case RewardSyntaxFail:
		return "syntax_fail"
	default:
		return "conflict"
	}
}

func (s *Service) trajectorySummaries(ctx context.Context, task *Task) []TrajectorySummary {
	if task.NumSamples > 1 {
		var out []TrajectorySummary
		for _, sid := range task.SessionIDs {
			if g, ok := task.SessionGroups[sid]; ok && g != nil {
				out = append(out, summariesFromGroup(*g)...)
			}
		}
		return out
	}
	if task.TrajectoryGroup != nil {
		return summariesFromGroup(*task.TrajectoryGroup)
	}
	if s.client == nil {
		return nil
	}
	group, err := s.client.BuildFromDrain(ctx, task.SessionID, 64)
	if err != nil || len(group.Chains) == 0 {
		return nil
	}
	s.mu.Lock()
	task.TrajectoryGroup = &group
	s.mu.Unlock()
	return summariesFromGroup(group)
}

func summariesFromGroup(group TrajectoryGroup) []TrajectorySummary {
	out := make([]TrajectorySummary, 0, len(group.Chains))
	var totalReward float64
	if len(group.Rewards) > 0 {
		for _, r := range group.Rewards {
			totalReward += r
		}
	}
	for _, ch := range group.Chains {
		count := 0
		for _, step := range ch.Steps {
			for _, m := range step.LossMask {
				if m == 1 {
					count++
				}
			}
		}
		out = append(out, TrajectorySummary{
			ChainID:            ch.ChainID,
			LossMaskTokenCount: count,
			TotalReward:        totalReward,
		})
	}
	return out
}

var errNotFound = errors.New("not found")

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
