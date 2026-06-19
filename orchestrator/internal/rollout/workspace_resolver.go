package rollout

import (
	"fmt"

	"github.com/emage/cwso/orchestrator/internal/shadow"
)

// ShadowWorkspaceResolver resolves workspaces via cwso-git-shadow IPC.
type ShadowWorkspaceResolver struct {
	client *shadow.Client
}

// NewShadowWorkspaceResolver constructs a resolver for the given shadow socket client.
func NewShadowWorkspaceResolver(client *shadow.Client) *ShadowWorkspaceResolver {
	return &ShadowWorkspaceResolver{client: client}
}

// Resolve returns base tree OID and file manifest for prefix hashing.
func (r *ShadowWorkspaceResolver) Resolve(workspaceID string) (WorkspaceMeta, error) {
	if r == nil || r.client == nil {
		return WorkspaceMeta{}, fmt.Errorf("shadow client not configured")
	}
	if workspaceID == "" {
		return WorkspaceMeta{}, fmt.Errorf("workspace_id required")
	}
	var out struct {
		BaseTreeOID *string         `json:"base_tree_oid"`
		Files       []WorkspaceFile `json:"files"`
	}
	if err := r.client.Call("get_workspace", map[string]string{
		"workspace_uuid": workspaceID,
	}, &out); err != nil {
		return WorkspaceMeta{}, fmt.Errorf("resolve workspace: %w", err)
	}
	return WorkspaceMeta{BaseTreeOID: out.BaseTreeOID, Files: out.Files}, nil
}
