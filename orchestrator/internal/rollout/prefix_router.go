package rollout

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/zeebo/blake3"
)

// PrefixInputs are the hashed components for rollout-architecture-v1 §6.
type PrefixInputs struct {
	BaseTreeOID         string
	SystemPromptHash    string
	SharedReadFilesHash string
}

// ComputePrefixKey returns blake3(base_tree_oid || system_prompt_hash || shared_read_files_hash)
// as a lowercase hex digest.
func ComputePrefixKey(in PrefixInputs) string {
	payload := in.BaseTreeOID + in.SystemPromptHash + in.SharedReadFilesHash
	sum := blake3.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// HashSystemPrompt returns the BLAKE3 hex digest of prompt bytes (empty prompt is valid).
func HashSystemPrompt(prompt string) string {
	sum := blake3.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// HashSharedReadFiles returns BLAKE3 hex over a canonical path:blob_oid manifest.
func HashSharedReadFiles(files []WorkspaceFile) string {
	if len(files) == 0 {
		sum := blake3.Sum256(nil)
		return hex.EncodeToString(sum[:])
	}
	sorted := append([]WorkspaceFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	var b strings.Builder
	for i, f := range sorted {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(f.Path)
		b.WriteByte(':')
		b.WriteString(f.BlobOID)
	}
	sum := blake3.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// WorkspaceFile is one path/blob pair from a shadow workspace manifest.
type WorkspaceFile struct {
	Path    string `json:"path"`
	BlobOID string `json:"blob_oid"`
}

// WorkspaceMeta is resolved shadow workspace state for prefix routing.
type WorkspaceMeta struct {
	BaseTreeOID *string
	Files       []WorkspaceFile
}

// WorkspaceResolver loads workspace metadata by UUID.
type WorkspaceResolver interface {
	Resolve(workspaceID string) (WorkspaceMeta, error)
}

// PrefixRouterConfig wires KV prefix routing (T135).
type PrefixRouterConfig struct {
	Enabled          bool
	SystemPromptHash string
	Resolver         WorkspaceResolver
	Client           *Client
}

// PrefixRouter computes prefix keys and optionally prewarms the sidecar cache.
type PrefixRouter struct {
	cfg PrefixRouterConfig
}

// NewPrefixRouter constructs a router; nil cfg fields disable routing behavior.
func NewPrefixRouter(cfg PrefixRouterConfig) *PrefixRouter {
	return &PrefixRouter{cfg: cfg}
}

// Enabled reports whether KV prefix routing is active.
func (r *PrefixRouter) Enabled() bool {
	return r != nil && r.cfg.Enabled
}

// ResolveKey computes the prefix key for a workspace without prewarming.
func (r *PrefixRouter) ResolveKey(workspaceID string) (string, error) {
	if r == nil || !r.cfg.Enabled {
		return "", nil
	}
	if r.cfg.Resolver == nil {
		return "", fmt.Errorf("workspace resolver not configured")
	}
	meta, err := r.cfg.Resolver.Resolve(workspaceID)
	if err != nil {
		return "", err
	}
	return r.keyFromMeta(meta), nil
}

// Prewarm resolves the prefix key and notifies cwso-rollout when configured.
func (r *PrefixRouter) Prewarm(ctx context.Context, workspaceID string) (string, error) {
	key, err := r.ResolveKey(workspaceID)
	if err != nil || key == "" {
		return key, err
	}
	if r.cfg.Client == nil {
		return key, nil
	}
	if err := r.cfg.Client.PrewarmPrefix(ctx, key); err != nil {
		return key, fmt.Errorf("prewarm prefix: %w", err)
	}
	return key, nil
}

func (r *PrefixRouter) keyFromMeta(meta WorkspaceMeta) string {
	base := ""
	if meta.BaseTreeOID != nil {
		base = *meta.BaseTreeOID
	}
	return ComputePrefixKey(PrefixInputs{
		BaseTreeOID:         base,
		SystemPromptHash:    r.cfg.SystemPromptHash,
		SharedReadFilesHash: HashSharedReadFiles(meta.Files),
	})
}
