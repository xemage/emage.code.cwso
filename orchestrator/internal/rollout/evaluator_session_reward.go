package rollout

import (
	"context"
	"encoding/json"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// SessionRewardPlugin attaches merge SM rewards from the rollout/reward topic (T136/T148).
type SessionRewardPlugin struct {
	enabled bool
	reader  RewardReader
}

// NewSessionRewardPlugin constructs the built-in session reward evaluator.
func NewSessionRewardPlugin(enabled bool, reader RewardReader) *SessionRewardPlugin {
	if reader == nil {
		enabled = false
	}
	return &SessionRewardPlugin{enabled: enabled, reader: reader}
}

func (p *SessionRewardPlugin) ID() EvaluatorID { return EvaluatorSessionReward }

func (p *SessionRewardPlugin) Enabled() bool {
	return p != nil && p.enabled
}

func (p *SessionRewardPlugin) Evaluate(ctx context.Context, req EvalRequest) (EvalResult, error) {
	_ = ctx
	result := EvalResult{
		EvaluatorID: EvaluatorSessionReward,
		Reward:      0,
		Metadata:    map[string]string{"source": "rollout/reward"},
	}
	if !p.Enabled() || req.SessionID == "" || p.reader == nil {
		return result, nil
	}
	records := p.reader.Query(memorybroker.QueryOptions{Topics: []string{TopicReward}, Limit: 512})
	var total float64
	var outcome string
	for _, rec := range records {
		var ev RewardEvent
		if err := json.Unmarshal(rec.Payload, &ev); err != nil {
			continue
		}
		if ev.SessionID != req.SessionID {
			continue
		}
		total += ev.Reward
		outcome = mergeOutcomeFromReward(ev)
	}
	result.Reward = total
	if outcome != "" {
		result.Metadata["merge_outcome"] = outcome
	}
	return result, nil
}
