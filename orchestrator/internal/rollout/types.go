// Package rollout implements the Go control-plane side of Phase 9 rollout (ADR-010):
// trajectory assembly from cwso-rollout capture records and Polar REST integration (T137).
package rollout

// CompletionRecord mirrors the cwso-rollout capture artifact (T132).
type CompletionRecord struct {
	RequestID       string    `json:"request_id"`
	Provider        string    `json:"provider"`
	PromptTokenIDs  []uint32  `json:"prompt_token_ids"`
	SampledTokenIDs []uint32  `json:"sampled_token_ids"`
	Logprobs        []float64 `json:"logprobs"`
	FinishReason    *string   `json:"finish_reason,omitempty"`
	TimestampNS     uint64    `json:"timestamp_ns"`
	PartitionKey    string    `json:"partition_key,omitempty"`    // sub-agent / compaction boundary (T149)
	MessageGroupID  string    `json:"message_group_id,omitempty"` // message-level chain grouping (T149)
}

// Step is one assistant generation step on an append-only chain.
type Step struct {
	TokenIDs       []uint32  `json:"token_ids"`
	LossMask       []uint8   `json:"loss_mask"`
	Logprobs       []float64 `json:"logprobs"`
	MessageGroupID string    `json:"message_group_id,omitempty"`
}

// Chain is an append-only token trajectory sharing a fixed context prefix.
type Chain struct {
	ChainID        string            `json:"chain_id"`
	PrefixTokenIDs []uint32          `json:"prefix_token_ids"`
	Steps          []Step            `json:"steps"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TrajectoryGroup is the trainer-facing bundle for one rollout session.
type TrajectoryGroup struct {
	SessionID string            `json:"session_id"`
	Chains    []Chain           `json:"chains"`
	Rewards   []float64         `json:"rewards,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
