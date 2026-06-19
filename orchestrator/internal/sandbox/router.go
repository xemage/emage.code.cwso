package sandbox

import (
	"context"
	"errors"
	"fmt"
)

// Routing reason codes — emitted in RunResult.RoutingReason for auditability.
const (
	// ReasonPolicyDefault is used when the caller supplied no profile; the
	// server applies its safe default (gVisor).
	ReasonPolicyDefault = "POLICY_DEFAULT"

	// ReasonPolicyRequested is used when the caller's profile matches policy
	// and is applied as-is.
	ReasonPolicyRequested = "POLICY_REQUESTED"

	// ReasonOverrideNonEscalation is used when the caller requested
	// docker-trusted (lowest isolation) but that tier is not authorized for
	// dispatch callers; the request is overridden to gVisor.
	ReasonOverrideNonEscalation = "POLICY_OVERRIDE_NON_ESCALATION"

	// ReasonOverrideFallback is used when docker-trusted was authorized but
	// the docker runner is not configured; falls back to gVisor.
	ReasonOverrideFallback = "POLICY_OVERRIDE_FALLBACK"

	// ReasonDegradedFallback is used when firecracker was requested or
	// required but is unavailable; routes to gVisor instead.
	ReasonDegradedFallback = "DEGRADED_FALLBACK_GVISOR"
)

var (
	// ErrInvalidProfile is returned when the caller supplies an unrecognised
	// sandbox profile. This enforces default-deny for unknown values.
	ErrInvalidProfile = errors.New("unknown sandbox profile")

	// ErrGVisorRequired is returned when the tier router is constructed
	// without a gVisor runner, which is the mandatory fallback tier.
	ErrGVisorRequired = errors.New("tier router requires a gvisor runner")
)

// TierRouterConfig holds the three optional runner backends and routing policy
// knobs. GVisorRunner is the only mandatory field.
type TierRouterConfig struct {
	// DockerRunner handles docker-trusted workloads (internal tooling).
	// May be nil; if nil, docker-trusted requests fall back to gVisor.
	DockerRunner RunnerInterface

	// GVisorRunner handles gvisor-fast-ephemeral workloads (benign sub-agents).
	// Required — it is the default tier and the degraded-mode fallback.
	GVisorRunner RunnerInterface

	// FirecrackerRunner handles firecracker-secure-isolation workloads
	// (untrusted/LLM-generated code). May be nil when DegradedMode is true.
	FirecrackerRunner RunnerInterface

	// DegradedMode is true when Firecracker prerequisites (KVM, vhost-net)
	// are absent on the host. Firecracker requests are silently demoted to
	// gVisor and a DEGRADED_FALLBACK_GVISOR reason is recorded.
	DegradedMode bool

	// AllowDockerTrusted permits dispatch callers to explicitly request the
	// docker-trusted tier. Must only be true for fully-internal orchestrator
	// workloads. Defaults to false (non-escalation enforcement).
	AllowDockerTrusted bool
}

// TierRouter implements RunnerInterface and enforces the 3-tier sandbox
// routing policy defined in ADR-003. It selects the appropriate backend
// based on the SandboxProfile in the RunRequest, enforcing:
//   - Non-escalation: callers cannot request docker-trusted unless explicitly
//     authorized by AllowDockerTrusted.
//   - Degraded-mode fallback: Firecracker requests route to gVisor when KVM
//     is unavailable.
//   - Default-deny: unknown profiles are rejected.
type TierRouter struct {
	cfg TierRouterConfig
}

// NewTierRouter constructs a validated TierRouter. GVisorRunner is mandatory.
func NewTierRouter(cfg TierRouterConfig) (*TierRouter, error) {
	if cfg.GVisorRunner == nil {
		return nil, ErrGVisorRequired
	}
	return &TierRouter{cfg: cfg}, nil
}

// Name satisfies RunnerInterface.
func (r *TierRouter) Name() string { return "tier-router" }

// Execute routes the request to the appropriate runner and annotates the
// result with SandboxTier and RoutingReason for auditing.
func (r *TierRouter) Execute(ctx context.Context, req RunRequest) (RunResult, error) {
	runner, tier, reason, err := r.resolve(req.SandboxProfile)
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox tier router: %w", err)
	}

	// Normalise profile so the downstream runner sees the effective value.
	req.SandboxProfile = SandboxProfile(tier)

	result, execErr := runner.Execute(ctx, req)
	result.SandboxTier = tier
	result.RoutingReason = reason
	return result, execErr
}

// Health checks all configured runners and returns the first error encountered.
func (r *TierRouter) Health(ctx context.Context) error {
	runners := []RunnerInterface{r.cfg.DockerRunner, r.cfg.GVisorRunner, r.cfg.FirecrackerRunner}
	for _, rr := range runners {
		if rr == nil {
			continue
		}
		if err := rr.Health(ctx); err != nil {
			return fmt.Errorf("tier-router health [%s]: %w", rr.Name(), err)
		}
	}
	return nil
}

// resolve determines the effective (tier, reason) for a given requested profile
// and returns the runner to use. It never allows privilege escalation by callers.
func (r *TierRouter) resolve(requested SandboxProfile) (runner RunnerInterface, tier string, reason string, err error) {
	// Empty profile → server-side default (gVisor is the safe default).
	if requested == "" {
		return r.cfg.GVisorRunner, string(ProfileGVisorFastEphemeral), ReasonPolicyDefault, nil
	}

	// Reject unknown profiles (default-deny).
	if !ValidSandboxProfiles[requested] {
		return nil, "", "", fmt.Errorf("%w: %q", ErrInvalidProfile, requested)
	}

	switch requested {
	case ProfileDockerTrusted:
		return r.resolveDockerTrusted()

	case ProfileGVisorFastEphemeral:
		return r.cfg.GVisorRunner, string(ProfileGVisorFastEphemeral), ReasonPolicyRequested, nil

	case ProfileFirecrackerSecure:
		return r.resolveFirecracker()
	}

	// Unreachable given the ValidSandboxProfiles check above, but keeps the
	// compiler satisfied and guards against future profile additions.
	return nil, "", "", fmt.Errorf("%w: %q", ErrInvalidProfile, requested)
}

// resolveDockerTrusted enforces the non-escalation constraint: docker-trusted
// is only served when AllowDockerTrusted is explicitly set and the docker
// runner is configured. Otherwise the request is overridden to gVisor.
func (r *TierRouter) resolveDockerTrusted() (RunnerInterface, string, string, error) {
	if !r.cfg.AllowDockerTrusted {
		// Non-escalation enforcement: dispatch callers may never self-select
		// docker-trusted. Override to gVisor.
		return r.cfg.GVisorRunner, string(ProfileGVisorFastEphemeral), ReasonOverrideNonEscalation, nil
	}
	if r.cfg.DockerRunner == nil {
		// Docker-trusted authorized but runner not wired; safe fallback.
		return r.cfg.GVisorRunner, string(ProfileGVisorFastEphemeral), ReasonOverrideFallback, nil
	}
	return r.cfg.DockerRunner, string(ProfileDockerTrusted), ReasonPolicyRequested, nil
}

// resolveFirecracker routes to Firecracker when available; in degraded mode
// (KVM absent) it falls back to gVisor with an explicit reason code.
func (r *TierRouter) resolveFirecracker() (RunnerInterface, string, string, error) {
	if r.cfg.DegradedMode || r.cfg.FirecrackerRunner == nil {
		return r.cfg.GVisorRunner, string(ProfileGVisorFastEphemeral), ReasonDegradedFallback, nil
	}
	return r.cfg.FirecrackerRunner, string(ProfileFirecrackerSecure), ReasonPolicyRequested, nil
}
