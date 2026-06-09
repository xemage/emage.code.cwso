package rollout

import "context"

const defaultSWEBenchInstance = "stub-instance-0"

// SWEBenchConfig controls the SWE-bench harness evaluator PoC (T148).
type SWEBenchConfig struct {
	Enabled    bool
	InstanceID string
	Client     *Client
}

// SWEBenchPlugin scores agent patches in an isolated runtime (Polar §3.5).
// PoC stub: records instance metadata and neutral reward until harness launch is wired.
type SWEBenchPlugin struct {
	enabled    bool
	instanceID string
	client     *Client
}

// NewSWEBenchPlugin constructs the SWE-bench evaluator hook.
func NewSWEBenchPlugin(cfg SWEBenchConfig) *SWEBenchPlugin {
	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = defaultSWEBenchInstance
	}
	return &SWEBenchPlugin{
		enabled:    cfg.Enabled,
		instanceID: instanceID,
		client:     cfg.Client,
	}
}

func (p *SWEBenchPlugin) ID() EvaluatorID { return EvaluatorSWEBench }

func (p *SWEBenchPlugin) Enabled() bool {
	return p != nil && p.enabled
}

func (p *SWEBenchPlugin) Evaluate(ctx context.Context, req EvalRequest) (EvalResult, error) {
	result := EvalResult{
		EvaluatorID: EvaluatorSWEBench,
		Reward:      0,
		Metadata: map[string]string{
			"instance_id": p.instanceID,
			"status":      "stub",
		},
	}
	if !p.Enabled() {
		return result, nil
	}
	if req.TimedOut {
		result.Metadata["skipped"] = "session_timeout"
		return result, nil
	}
	if p.client != nil {
		if _, err := p.client.Stat(ctx); err != nil {
			result.Metadata["sidecar_probe"] = "unavailable"
		} else {
			result.Metadata["sidecar_probe"] = "ok"
		}
	}
	// POC-DEBT: Launch SWE-bench/SWE-Gym harness in fresh Docker runtime;
	// apply patch from trajectory metadata and run instance test suite.
	return result, nil
}

// Prewarm probes evaluator sidecar readiness during agent RUNNING (T146 hook surface).
func (p *SWEBenchPlugin) Prewarm(ctx context.Context, sessionID string) error {
	if !p.Enabled() || sessionID == "" || p.client == nil {
		return nil
	}
	_, err := p.client.Stat(ctx)
	return err
}
