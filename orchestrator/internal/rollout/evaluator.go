package rollout

import "context"

// Evaluator prewarms rollout evaluation during the agent RUNNING stage (T146).
type Evaluator interface {
	Enabled() bool
	Prewarm(ctx context.Context, sessionID string) error
}

// StubEvaluator is a no-op prewarm when no evaluator sidecar is configured.
type StubEvaluator struct {
	client  *Client
	enabled bool
}

// NewStubEvaluator constructs an evaluator prewarm hook (stub OK without sidecar).
func NewStubEvaluator(client *Client, enabled bool) *StubEvaluator {
	return &StubEvaluator{client: client, enabled: enabled}
}

func (e *StubEvaluator) Enabled() bool {
	return e != nil && e.enabled
}

func (e *StubEvaluator) Prewarm(ctx context.Context, sessionID string) error {
	if !e.Enabled() || sessionID == "" {
		return nil
	}
	if e.client == nil {
		return nil
	}
	_, err := e.client.Stat(ctx)
	return err
}
