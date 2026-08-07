package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// requiredTopLevelKeys mirrors schemas/dashboard_status.json "required" list.
var requiredTopLevelKeys = []string{
	"version", "timestamp", "overall", "system", "sidecars",
	"config", "jobs", "clients", "rollout",
}

// allowedTopLevelKeys is the exhaustive set permitted by the schema (additionalProperties: false).
var allowedTopLevelKeys = map[string]struct{}{
	"version": {}, "timestamp": {}, "overall": {}, "system": {},
	"sidecars": {}, "config": {}, "jobs": {}, "clients": {}, "rollout": {},
}

func TestStatusResponseConformsToSchema(t *testing.T) {
	h := newTestHandler("tok")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)

	var generic map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	// No extra top-level keys (additionalProperties: false).
	for k := range generic {
		if _, ok := allowedTopLevelKeys[k]; !ok {
			t.Errorf("unexpected top-level key %q not in schema", k)
		}
	}
	// All required keys present.
	for _, k := range requiredTopLevelKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("required key %q missing from /dashboard/status response", k)
		}
	}

	// overall must be one of the three permitted values.
	overall, _ := generic["overall"].(string)
	switch overall {
	case "healthy", "degraded", "unhealthy":
	default:
		t.Errorf("overall: unexpected value %q", overall)
	}

	// jobs sub-object must contain all required fields.
	jobs, _ := generic["jobs"].(map[string]any)
	for _, k := range []string{"workers", "queue_capacity", "queue_depth", "active", "total_completed", "total_failed"} {
		if _, ok := jobs[k]; !ok {
			t.Errorf("jobs.%s missing", k)
		}
	}

	// clients sub-object must contain all required fields.
	clients, _ := generic["clients"].(map[string]any)
	for _, k := range []string{"total_requests", "auth_failures", "rate_limit_hits", "tool_calls"} {
		if _, ok := clients[k]; !ok {
			t.Errorf("clients.%s missing", k)
		}
	}

	// rollout.enabled must be present.
	rollout, _ := generic["rollout"].(map[string]any)
	if _, ok := rollout["enabled"]; !ok {
		t.Error("rollout.enabled missing")
	}
}

func TestClientMetrics_Concurrent(t *testing.T) {
	var m ClientMetrics
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); m.RecordRequest() }()
		go func() { defer wg.Done(); m.RecordAuthFailure() }()
		go func() { defer wg.Done(); m.RecordToolCall("dispatch_concurrent_jobs") }()
	}
	wg.Wait()
	if got := m.TotalRequests.Load(); got != 100 {
		t.Fatalf("TotalRequests: want 100, got %d", got)
	}
	if got := m.AuthFailures.Load(); got != 100 {
		t.Fatalf("AuthFailures: want 100, got %d", got)
	}
	calls := m.ToolCallSnapshot()
	if n, ok := calls["dispatch_concurrent_jobs"]; !ok || n != 100 {
		t.Fatalf("tool_calls[dispatch_concurrent_jobs]: want 100, got %v", calls)
	}
}

func TestClientMetrics_ZeroCallToolsOmitted(t *testing.T) {
	var m ClientMetrics
	m.RecordToolCall("query_ast")
	snap := m.ToolCallSnapshot()
	if _, ok := snap["merge_concurrent_results"]; ok {
		t.Fatal("tool with zero calls should be omitted from snapshot")
	}
	if snap["query_ast"] != 1 {
		t.Fatalf("expected query_ast=1, got %v", snap)
	}
}

func TestSidecarChecker_EmptySocketNotConnected(t *testing.T) {
	sc := NewSidecarChecker(map[string]string{"hal": ""})
	snap := sc.Snapshot()
	if snap["hal"].Connected {
		t.Fatal("empty socket should not be connected")
	}
}

func TestSidecarChecker_UnreachableSocket(t *testing.T) {
	sc := NewSidecarChecker(map[string]string{"git_shadow": "/tmp/cwso-test-nonexistent.sock"})
	snap := sc.Snapshot()
	if snap["git_shadow"].Connected {
		t.Fatal("non-existent socket should not be connected")
	}
}

func TestSidecarChecker_CacheTTL(t *testing.T) {
	sc := NewSidecarChecker(map[string]string{"x": "/tmp/cwso-test-nonexistent.sock"})
	sc.cacheTTL = 10 * time.Second

	snap1 := sc.Snapshot()
	snap2 := sc.Snapshot()
	// second call should reuse cached result, not re-dial
	if snap1["x"].Connected != snap2["x"].Connected {
		t.Fatal("cache should return consistent result within TTL")
	}
}

func newTestHandler(token string) *Handler {
	var m ClientMetrics
	return New(Config{
		Token:    token,
		Sidecars: map[string]string{},
		Metrics:  &m,
		ConfigSnap: ConfigSnapshot{
			Transport:     "http",
			SandboxRunner: "none",
			FeatureFlags:  map[string]bool{},
			Warnings:      nil,
		},
		JobStats: func() JobsSnapshotRaw { return JobsSnapshotRaw{Workers: 4, QueueCapacity: 64} },
		Rollout:  func() RolloutSnapshot { return RolloutSnapshot{Enabled: false} },
	})
}

func TestHandler_501WhenTokenUnset(t *testing.T) {
	h := newTestHandler("")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("want 501, got %d", rec.Code)
	}
}

func TestHandler_401MissingToken(t *testing.T) {
	h := newTestHandler("secret123")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestHandler_401WrongToken(t *testing.T) {
	h := newTestHandler("secret123")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestHandler_200ValidToken(t *testing.T) {
	h := newTestHandler("secret123")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("want application/json content-type, got %q", ct)
	}
}

func TestHandler_StatusJSON_NoSecretLeakage(t *testing.T) {
	const token = "super-secret-dashboard-token"
	h := newTestHandler(token)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, token) {
		t.Fatal("dashboard token must not appear in JSON response")
	}
	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if resp.Overall == "" {
		t.Fatal("overall field must be set")
	}
	if resp.Jobs.Workers != 4 {
		t.Fatalf("workers: want 4, got %d", resp.Jobs.Workers)
	}
	if resp.Rollout.Enabled {
		t.Fatal("rollout should be disabled")
	}
}

func TestHandler_HTMLEndpoint(t *testing.T) {
	h := newTestHandler("tok")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "CWSO") {
		t.Fatal("HTML page should contain CWSO branding")
	}
}

func TestHandler_OverallDegradedWhenSidecarDown(t *testing.T) {
	var m ClientMetrics
	h := New(Config{
		Token:    "tok",
		Sidecars: map[string]string{"git_shadow": "/tmp/cwso-no-socket.sock"},
		Metrics:  &m,
		ConfigSnap: ConfigSnapshot{
			Transport:     "http",
			SandboxRunner: "none",
			FeatureFlags:  map[string]bool{},
		},
		JobStats: func() JobsSnapshotRaw { return JobsSnapshotRaw{} },
		Rollout:  func() RolloutSnapshot { return RolloutSnapshot{} },
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/status", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(rec, req)

	var resp StatusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Overall == "healthy" {
		t.Fatal("overall should be degraded when a sidecar is unreachable")
	}
}
