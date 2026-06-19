package sandbox

import (
	"context"
	"errors"
	"testing"
)

// stubRunner is a minimal RunnerInterface used in routing tests.
type stubRunner struct {
	name string
	err  error
}

func (s *stubRunner) Name() string { return s.name }
func (s *stubRunner) Execute(_ context.Context, req RunRequest) (RunResult, error) {
	if s.err != nil {
		return RunResult{}, s.err
	}
	return RunResult{ExitCode: 0}, nil
}
func (s *stubRunner) Health(_ context.Context) error { return s.err }

func newStub(name string) *stubRunner { return &stubRunner{name: name} }

func newFullRouter(t *testing.T, opts ...func(*TierRouterConfig)) *TierRouter {
	t.Helper()
	cfg := TierRouterConfig{
		DockerRunner:      newStub("docker-trusted"),
		GVisorRunner:      newStub("gvisor-fast-ephemeral"),
		FirecrackerRunner: newStub("firecracker-secure-isolation"),
	}
	for _, o := range opts {
		o(&cfg)
	}
	r, err := NewTierRouter(cfg)
	if err != nil {
		t.Fatalf("NewTierRouter: %v", err)
	}
	return r
}

// --- Construction tests ---

func TestNewTierRouterRequiresGVisor(t *testing.T) {
	_, err := NewTierRouter(TierRouterConfig{GVisorRunner: nil})
	if !errors.Is(err, ErrGVisorRequired) {
		t.Fatalf("expected ErrGVisorRequired, got %v", err)
	}
}

func TestNewTierRouterSucceedsWithOnlyGVisor(t *testing.T) {
	_, err := NewTierRouter(TierRouterConfig{GVisorRunner: newStub("gvisor-fast-ephemeral")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTierRouterName(t *testing.T) {
	r := newFullRouter(t)
	if r.Name() != "tier-router" {
		t.Fatalf("unexpected name: %q", r.Name())
	}
}

// --- Default profile routing ---

func TestEmptyProfileRoutesToGVisor(t *testing.T) {
	r := newFullRouter(t)
	res, err := r.Execute(context.Background(), RunRequest{Name: "x", SandboxProfile: ""})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor tier, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonPolicyDefault {
		t.Fatalf("expected POLICY_DEFAULT reason, got %q", res.RoutingReason)
	}
}

// --- Explicit profile routing ---

func TestGVisorProfileRoutesDeterministically(t *testing.T) {
	r := newFullRouter(t)
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileGVisorFastEphemeral,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor tier, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonPolicyRequested {
		t.Fatalf("expected POLICY_REQUESTED, got %q", res.RoutingReason)
	}
}

func TestFirecrackerProfileRoutesToFirecracker(t *testing.T) {
	r := newFullRouter(t)
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileFirecrackerSecure,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileFirecrackerSecure) {
		t.Fatalf("expected firecracker tier, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonPolicyRequested {
		t.Fatalf("expected POLICY_REQUESTED, got %q", res.RoutingReason)
	}
}

// --- Non-escalation enforcement ---

func TestDockerTrustedWithoutAuthorizationOverridesToGVisor(t *testing.T) {
	// AllowDockerTrusted defaults to false — caller cannot self-select docker-trusted.
	r := newFullRouter(t)
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileDockerTrusted,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor override, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonOverrideNonEscalation {
		t.Fatalf("expected POLICY_OVERRIDE_NON_ESCALATION, got %q", res.RoutingReason)
	}
}

func TestDockerTrustedWithAuthorizationAndRunnerUsesDockerTier(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.AllowDockerTrusted = true
	})
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileDockerTrusted,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileDockerTrusted) {
		t.Fatalf("expected docker-trusted tier, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonPolicyRequested {
		t.Fatalf("expected POLICY_REQUESTED, got %q", res.RoutingReason)
	}
}

func TestDockerTrustedAuthorizedButRunnerNilFallsBackToGVisor(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.AllowDockerTrusted = true
		c.DockerRunner = nil
	})
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileDockerTrusted,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor fallback, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonOverrideFallback {
		t.Fatalf("expected POLICY_OVERRIDE_FALLBACK, got %q", res.RoutingReason)
	}
}

// --- Degraded mode ---

func TestFirecrackerDegradedModeFallsBackToGVisor(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.DegradedMode = true
	})
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileFirecrackerSecure,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor fallback, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonDegradedFallback {
		t.Fatalf("expected DEGRADED_FALLBACK_GVISOR, got %q", res.RoutingReason)
	}
}

func TestFirecrackerRunnerNilFallsBackToGVisor(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.FirecrackerRunner = nil
	})
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: ProfileFirecrackerSecure,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor fallback, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonDegradedFallback {
		t.Fatalf("expected DEGRADED_FALLBACK_GVISOR, got %q", res.RoutingReason)
	}
}

func TestEmptyProfileDegradedModeStillRoutesToGVisor(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.DegradedMode = true
		c.FirecrackerRunner = nil
	})
	res, err := r.Execute(context.Background(), RunRequest{Name: "x"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier != string(ProfileGVisorFastEphemeral) {
		t.Fatalf("expected gvisor, got %q", res.SandboxTier)
	}
	if res.RoutingReason != ReasonPolicyDefault {
		t.Fatalf("expected POLICY_DEFAULT, got %q", res.RoutingReason)
	}
}

// --- Default-deny for invalid profiles ---

func TestUnknownProfileIsRejected(t *testing.T) {
	r := newFullRouter(t)
	_, err := r.Execute(context.Background(), RunRequest{
		Name:           "x",
		SandboxProfile: SandboxProfile("unrecognised-tier"),
	})
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile, got %v", err)
	}
}

// --- Health check ---

func TestHealthPassesWhenAllRunnersHealthy(t *testing.T) {
	r := newFullRouter(t)
	if err := r.Health(context.Background()); err != nil {
		t.Fatalf("expected healthy, got %v", err)
	}
}

func TestHealthReportsUnhealthyRunner(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.GVisorRunner = &stubRunner{name: "gvisor-fast-ephemeral", err: errors.New("unhealthy")}
	})
	if err := r.Health(context.Background()); err == nil {
		t.Fatal("expected health check failure")
	}
}

func TestHealthSkipsNilRunners(t *testing.T) {
	r := newFullRouter(t, func(c *TierRouterConfig) {
		c.DockerRunner = nil
		c.FirecrackerRunner = nil
	})
	if err := r.Health(context.Background()); err != nil {
		t.Fatalf("expected healthy with nil runners skipped, got %v", err)
	}
}

// --- Result annotation ---

func TestResultAnnotatedWithTierAndReason(t *testing.T) {
	r := newFullRouter(t)
	res, err := r.Execute(context.Background(), RunRequest{
		Name:           "annotate-test",
		SandboxProfile: ProfileGVisorFastEphemeral,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.SandboxTier == "" {
		t.Error("SandboxTier must not be empty")
	}
	if res.RoutingReason == "" {
		t.Error("RoutingReason must not be empty")
	}
}
