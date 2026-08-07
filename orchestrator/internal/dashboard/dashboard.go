// Package dashboard provides the operator dashboard: health aggregation, metrics
// snapshot, and the HTTP handler for /dashboard and /dashboard/status.
package dashboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ClientMetrics tracks per-request counters incremented by HTTP middleware.
// All fields are safe for concurrent access via sync/atomic or sync.Map.
type ClientMetrics struct {
	TotalRequests atomic.Uint64
	AuthFailures  atomic.Uint64
	RateLimitHits atomic.Uint64
	toolCalls     sync.Map // map[string]*atomic.Uint64
}

// RecordRequest increments the total request counter.
func (m *ClientMetrics) RecordRequest() { m.TotalRequests.Add(1) }

// RecordAuthFailure increments the auth failure counter.
func (m *ClientMetrics) RecordAuthFailure() { m.AuthFailures.Add(1) }

// RecordRateLimitHit increments the rate-limit hit counter.
func (m *ClientMetrics) RecordRateLimitHit() { m.RateLimitHits.Add(1) }

// RecordToolCall increments the call count for the named MCP tool.
func (m *ClientMetrics) RecordToolCall(tool string) {
	v, _ := m.toolCalls.LoadOrStore(tool, new(atomic.Uint64))
	v.(*atomic.Uint64).Add(1)
}

// ToolCallSnapshot returns a map of tool name → call count, omitting zero-count tools.
func (m *ClientMetrics) ToolCallSnapshot() map[string]uint64 {
	out := make(map[string]uint64)
	m.toolCalls.Range(func(k, v any) bool {
		if n := v.(*atomic.Uint64).Load(); n > 0 {
			out[k.(string)] = n
		}
		return true
	})
	return out
}

// SidecarInfo describes a sidecar socket and its cached reachability.
type SidecarInfo struct {
	Socket    string
	connected bool
	checkedAt time.Time
}

// SidecarChecker probes configured sidecar sockets with a 5 s cache.
type SidecarChecker struct {
	mu       sync.Mutex
	entries  map[string]*SidecarInfo // keyed by sidecar name
	cacheTTL time.Duration
}

// NewSidecarChecker constructs a checker for the named socket paths.
// An empty socket path marks the sidecar as disabled (not connected).
func NewSidecarChecker(sockets map[string]string) *SidecarChecker {
	entries := make(map[string]*SidecarInfo, len(sockets))
	for name, path := range sockets {
		entries[name] = &SidecarInfo{Socket: path}
	}
	return &SidecarChecker{entries: entries, cacheTTL: 5 * time.Second}
}

// Snapshot returns the current reachability state of all sidecars.
// Results older than the cache TTL are refreshed via a timeout-guarded dial.
func (c *SidecarChecker) Snapshot() map[string]SidecarStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	out := make(map[string]SidecarStatus, len(c.entries))
	for name, info := range c.entries {
		if info.Socket == "" {
			out[name] = SidecarStatus{Connected: false, Socket: ""}
			continue
		}
		if now.Sub(info.checkedAt) >= c.cacheTTL {
			info.connected = probeDial(info.Socket)
			info.checkedAt = now
		}
		// Emit only the basename to avoid leaking host-specific path prefixes (F4).
		out[name] = SidecarStatus{Connected: info.connected, Socket: filepath.Base(info.Socket)}
	}
	return out
}

// probeDial attempts a timeout-guarded Unix socket connection and immediately closes it.
func probeDial(path string) bool {
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// SidecarStatus is the wire-format sidecar health entry.
type SidecarStatus struct {
	Connected bool   `json:"connected"`
	Socket    string `json:"socket"`
}

// JobsSnapshot mirrors jobs.JobsSnapshot without creating an import cycle.
type JobsSnapshot struct {
	Workers        int    `json:"workers"`
	QueueCapacity  int    `json:"queue_capacity"`
	QueueDepth     int    `json:"queue_depth"`
	Active         int    `json:"active"`
	TotalCompleted uint64 `json:"total_completed"`
	TotalFailed    uint64 `json:"total_failed"`
}

// ConfigSnapshot is the safe, secret-free view of runtime configuration.
type ConfigSnapshot struct {
	Transport     string          `json:"transport"`
	SandboxRunner string          `json:"sandbox_runner"`
	FeatureFlags  map[string]bool `json:"feature_flags"`
	Warnings      []string        `json:"warnings"`
}

// RolloutSnapshot summarises rollout pipeline state.
type RolloutSnapshot struct {
	Enabled               bool    `json:"enabled"`
	ActiveTasks           int     `json:"active_tasks,omitempty"`
	TrajectoryCaptureRate float64 `json:"trajectory_capture_rate,omitempty"`
	CaptureDrops          uint64  `json:"capture_drops,omitempty"`
	LastRewardSignal      *string `json:"last_reward_signal"`
}

// StatusResponse is the canonical /dashboard/status JSON body.
type StatusResponse struct {
	Version   string                   `json:"version"`
	Timestamp string                   `json:"timestamp"`
	Overall   string                   `json:"overall"` // "healthy" | "degraded" | "unhealthy"
	System    SystemInfo               `json:"system"`
	Sidecars  map[string]SidecarStatus `json:"sidecars"`
	Config    ConfigSnapshot           `json:"config"`
	Jobs      JobsSnapshot             `json:"jobs"`
	Clients   ClientsSnapshot          `json:"clients"`
	Rollout   RolloutSnapshot          `json:"rollout"`
}

// SystemInfo holds basic process metadata.
type SystemInfo struct {
	UptimeSeconds int64 `json:"uptime_seconds"`
}

// ClientsSnapshot is the wire-format client activity section.
type ClientsSnapshot struct {
	TotalRequests uint64            `json:"total_requests"`
	AuthFailures  uint64            `json:"auth_failures"`
	RateLimitHits uint64            `json:"rate_limit_hits"`
	ToolCalls     map[string]uint64 `json:"tool_calls"`
}

// StatsProvider is implemented by jobs.Manager.
type StatsProvider interface {
	Stats() JobsSnapshotRaw
}

// JobsSnapshotRaw is the type returned by jobs.Manager.Stats().
// It mirrors jobs.JobsSnapshot to avoid importing internal/jobs from here.
type JobsSnapshotRaw struct {
	Workers        int
	QueueCapacity  int
	QueueDepth     int
	Active         int
	TotalCompleted uint64
	TotalFailed    uint64
}

// Handler is the operator dashboard HTTP handler.
type Handler struct {
	tokenHash   []byte // SHA-256 of CWSO_DASHBOARD_TOKEN; nil ⇒ 501
	sidecar     *SidecarChecker
	metrics     *ClientMetrics
	clientMet   RequestMetrics // for recording dashboard-route auth failures
	configSnap  ConfigSnapshot
	jobStats    func() JobsSnapshotRaw
	rollout     func() RolloutSnapshot
	startedAt   time.Time
	htmlPayload []byte
}

// RequestMetrics mirrors transport.RequestMetrics to avoid import cycles.
type RequestMetrics interface {
	RecordAuthFailure()
}

// Config wires the Handler.
type Config struct {
	Token      string // raw token from env; empty ⇒ dashboard disabled
	Sidecars   map[string]string
	Metrics    *ClientMetrics
	ClientMet  RequestMetrics // optional; records dashboard-route 401s
	ConfigSnap ConfigSnapshot
	JobStats   func() JobsSnapshotRaw
	Rollout    func() RolloutSnapshot
	HTML       []byte
}

// New constructs a Handler. If cfg.Token is empty, all routes return 501.
func New(cfg Config) *Handler {
	html := cfg.HTML
	if len(html) == 0 {
		html = defaultHTML
	}
	h := &Handler{
		sidecar:     NewSidecarChecker(cfg.Sidecars),
		metrics:     cfg.Metrics,
		clientMet:   cfg.ClientMet,
		configSnap:  cfg.ConfigSnap,
		jobStats:    cfg.JobStats,
		rollout:     cfg.Rollout,
		startedAt:   time.Now(),
		htmlPayload: html,
	}
	if cfg.Token != "" {
		sum := sha256.Sum256([]byte(cfg.Token))
		h.tokenHash = sum[:]
	}
	return h
}

// ServeHTTP dispatches /dashboard and /dashboard/status.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.tokenHash == nil {
		http.Error(w, "dashboard not configured", http.StatusNotImplemented)
		return
	}
	if !h.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/dashboard/status":
		h.serveStatus(w, r)
	case "/dashboard":
		h.serveHTML(w)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) auth(r *http.Request) bool {
	hdr := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(hdr, "Bearer ")
	if !ok || token == "" {
		if h.clientMet != nil {
			h.clientMet.RecordAuthFailure()
		}
		return false
	}
	sum := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(sum[:], h.tokenHash) != 1 {
		if h.clientMet != nil {
			h.clientMet.RecordAuthFailure()
		}
		return false
	}
	return true
}

func (h *Handler) serveStatus(w http.ResponseWriter, _ *http.Request) {
	sidecars := h.sidecar.Snapshot()
	overall := "healthy"
	for _, s := range sidecars {
		if !s.Connected && s.Socket != "" {
			overall = "degraded"
			break
		}
	}
	if len(h.configSnap.Warnings) > 0 && overall == "healthy" {
		overall = "degraded"
	}

	raw := h.jobStats()
	jobs := JobsSnapshot{
		Workers:        raw.Workers,
		QueueCapacity:  raw.QueueCapacity,
		QueueDepth:     raw.QueueDepth,
		Active:         raw.Active,
		TotalCompleted: raw.TotalCompleted,
		TotalFailed:    raw.TotalFailed,
	}

	resp := StatusResponse{
		Version:   "0.1.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Overall:   overall,
		System: SystemInfo{
			UptimeSeconds: int64(time.Since(h.startedAt).Seconds()),
		},
		Sidecars: sidecars,
		Config:   h.configSnap,
		Jobs:     jobs,
		Clients: ClientsSnapshot{
			TotalRequests: h.metrics.TotalRequests.Load(),
			AuthFailures:  h.metrics.AuthFailures.Load(),
			RateLimitHits: h.metrics.RateLimitHits.Load(),
			ToolCalls:     h.metrics.ToolCallSnapshot(),
		},
		Rollout: h.rollout(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) serveHTML(w http.ResponseWriter) {
	// Dashboard uses inline scripts/styles; override the middleware-set CSP.
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(h.htmlPayload)
}
