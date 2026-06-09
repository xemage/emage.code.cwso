package rollout

import (
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
)

// GatewayConfigFrom loads gateway pool settings from orchestrator config (T146).
func GatewayConfigFrom(cfg *config.Config, client *Client) GatewayConfig {
	if cfg == nil {
		return GatewayConfig{}
	}
	return GatewayConfig{
		InitWorkers:    cfg.RolloutGatewayInitWorkers,
		ReadyBuffer:    cfg.RolloutGatewayReadyBuffer,
		RunningWorkers: cfg.RolloutGatewayRunningWorkers,
		PostRunWorkers: cfg.RolloutGatewayPostRunWorkers,
		SessionTimeout: time.Duration(cfg.RolloutGatewaySessionTimeout) * time.Second,
		Evaluator:      NewStubEvaluator(client, cfg.RolloutEvaluatorPrewarmEnabled),
		Client:         client,
	}
}
