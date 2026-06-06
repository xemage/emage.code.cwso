package rollout

import (
	"testing"
)

func TestComputePrefixKey_deterministic(t *testing.T) {
	t.Parallel()
	in := PrefixInputs{
		BaseTreeOID:         "abc123",
		SystemPromptHash:    "deadbeef",
		SharedReadFilesHash: "cafebabe",
	}
	a := ComputePrefixKey(in)
	b := ComputePrefixKey(in)
	if a != b || len(a) != 64 {
		t.Fatalf("key = %q", a)
	}
}

func TestComputePrefixKey_changesWithBaseOID(t *testing.T) {
	t.Parallel()
	base := PrefixInputs{BaseTreeOID: "aaa", SystemPromptHash: "b", SharedReadFilesHash: "c"}
	other := PrefixInputs{BaseTreeOID: "bbb", SystemPromptHash: "b", SharedReadFilesHash: "c"}
	if ComputePrefixKey(base) == ComputePrefixKey(other) {
		t.Fatal("expected different keys for different base OIDs")
	}
}

func TestHashSharedReadFiles_orderIndependent(t *testing.T) {
	t.Parallel()
	a := []WorkspaceFile{{Path: "b.go", BlobOID: "2"}, {Path: "a.go", BlobOID: "1"}}
	b := []WorkspaceFile{{Path: "a.go", BlobOID: "1"}, {Path: "b.go", BlobOID: "2"}}
	if HashSharedReadFiles(a) != HashSharedReadFiles(b) {
		t.Fatal("manifest hash must be order-independent")
	}
}

func TestHashSharedReadFiles_empty(t *testing.T) {
	t.Parallel()
	if got := HashSharedReadFiles(nil); len(got) != 64 {
		t.Fatalf("empty manifest hash len = %d", len(got))
	}
}

func TestHashSystemPrompt(t *testing.T) {
	t.Parallel()
	if HashSystemPrompt("hi") == HashSystemPrompt("") {
		t.Fatal("expected distinct hashes")
	}
}

type stubResolver struct {
	meta WorkspaceMeta
	err  error
}

func (s *stubResolver) Resolve(_ string) (WorkspaceMeta, error) {
	return s.meta, s.err
}

func TestPrefixRouter_ResolveKey(t *testing.T) {
	t.Parallel()
	oid := "tree-deadbeef"
	router := NewPrefixRouter(PrefixRouterConfig{
		Enabled:          true,
		SystemPromptHash: HashSystemPrompt(""),
		Resolver: &stubResolver{meta: WorkspaceMeta{
			BaseTreeOID: &oid,
			Files:       []WorkspaceFile{{Path: "main.go", BlobOID: "blob1"}},
		}},
	})
	key, err := router.ResolveKey("ws-1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestPrefixRouter_disabledReturnsEmpty(t *testing.T) {
	t.Parallel()
	router := NewPrefixRouter(PrefixRouterConfig{Enabled: false})
	key, err := router.ResolveKey("ws-1")
	if err != nil || key != "" {
		t.Fatalf("key=%q err=%v", key, err)
	}
}
