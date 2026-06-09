package rollout

import (
	"github.com/emage/cwso/orchestrator/internal/config"
)

// RegistryConfigFrom loads evaluator registry settings from orchestrator config (T148).
func RegistryConfigFrom(cfg *config.Config, rewards RewardReader, client *Client) RegistryConfig {
	if cfg == nil {
		return RegistryConfig{}
	}
	return RegistryConfig{
		Enabled:              cfg.RolloutEvaluatorRegistryEnabled,
		SessionRewardEnabled: cfg.RolloutEvaluatorSessionRewardEnabled,
		SWEBenchEnabled:      cfg.RolloutEvaluatorSWEBenchEnabled,
		SWEBenchInstance:     cfg.RolloutSWEBenchInstance,
		Rewards:              rewards,
		Client:               client,
	}
}
