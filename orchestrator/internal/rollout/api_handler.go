package rollout

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// NewHTTPHandler mounts Polar REST routes (T137).
func NewHTTPHandler(svc *Service) http.Handler {
	mux := http.NewServeMux()
	h := &apiHandler{svc: svc}
	mux.HandleFunc("POST /rollout/task/submit", h.submitTask)
	mux.HandleFunc("POST /rollout/task/offline_generate", h.offlineGenerate)
	mux.HandleFunc("GET /rollout/task/{task_id}", h.getTask)
	mux.HandleFunc("GET /rollout/status", h.fleetStatus)
	mux.HandleFunc("POST /callbacks/session_result", h.sessionResult)
	mux.HandleFunc("POST /nodes/register", h.registerNode)
	mux.HandleFunc("POST /nodes/{id}/heartbeat", h.heartbeatNode)
	mux.HandleFunc("GET /nodes/{id}/tasks", h.getNodeTasks)
	mux.HandleFunc("POST /v1/chat/completions", h.proxyNotConfigured)
	return mux
}

type apiHandler struct {
	svc *Service
}

func (h *apiHandler) submitTask(w http.ResponseWriter, r *http.Request) {
	var req SubmitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.svc.SubmitTask(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *apiHandler) offlineGenerate(w http.ResponseWriter, r *http.Request) {
	var req OfflineGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.svc.GenerateOfflineTask(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *apiHandler) getTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	resp, err := h.svc.GetTask(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *apiHandler) fleetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.FleetStatus(r.Context()))
}

func (h *apiHandler) sessionResult(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TaskID       string            `json:"task_id"`
		SessionID    string            `json:"session_id,omitempty"`
		Trajectories []TrajectoryGroup `json:"trajectories"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var group *TrajectoryGroup
	if len(body.Trajectories) > 0 {
		group = &body.Trajectories[0]
	}
	if err := h.svc.CompleteSession(body.TaskID, body.SessionID, group); err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *apiHandler) registerNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.RegisterNode(body.NodeID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"node_id": body.NodeID})
}

func (h *apiHandler) heartbeatNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if err := h.svc.HeartbeatNode(nodeID); err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *apiHandler) getNodeTasks(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id required")
		return
	}

	// Fetch assigned tasks for this node
	if h.svc == nil || h.svc.nodeRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"assigned_tasks": []interface{}{},
		})
		return
	}

	assignments := h.svc.nodeRegistry.GetAssignedTasks(nodeID)

	// Build response with task specs
	type taskInfo struct {
		TaskID     string   `json:"task_id"`
		SessionID  string   `json:"session_id"`
		TaskSpec   TaskSpec `json:"task_spec"`
		AssignedAt string   `json:"assigned_at"`
	}

	var tasks []taskInfo
	for _, assignment := range assignments {
		// Look up the task in the service to get the task spec
		if task, err := h.svc.getTaskLocked(assignment.TaskID); err == nil {
			tasks = append(tasks, taskInfo{
				TaskID:     assignment.TaskID,
				SessionID:  assignment.SessionID,
				TaskSpec:   task.Spec,
				AssignedAt: assignment.AssignedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"assigned_tasks": tasks,
	})
}

func (h *apiHandler) proxyNotConfigured(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented,
		"transparent proxy is served by cwso-rollout sidecar; enable CWSO_ROLLOUT_PROXY_ENABLED")
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(body, dst)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
