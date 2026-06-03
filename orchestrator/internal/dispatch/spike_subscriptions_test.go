package dispatch

import (
	"encoding/json"
	"testing"
)

func mustPayload(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func TestSpikeRegistryCreateDefaultsAndValidation(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()

	sub, err := r.Create("", "", nil)
	if err != nil {
		t.Fatalf("create with defaults: %v", err)
	}
	if sub.SemanticThreshold != SpikeKindSignatureChange {
		t.Fatalf("expected default threshold signature_change, got %q", sub.SemanticThreshold)
	}
	if sub.URI() != SpikeResourcePrefix+sub.ID {
		t.Fatalf("unexpected URI %q", sub.URI())
	}

	if _, err := r.Create("", "bogus", nil); err == nil {
		t.Fatal("expected error for invalid threshold")
	}
	if _, err := r.Create("[", "any", nil); err == nil {
		t.Fatal("expected error for invalid path glob")
	}
}

func TestSpikeRegistryGetListRemove(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()
	a, _ := r.Create("a/*.go", SpikeKindSignatureChange, nil)
	b, _ := r.Create("b/*.go", SpikeThresholdAny, []string{"ws1", "ws1", " ", "ws2"})

	if len(b.WorkspaceScope) != 2 {
		t.Fatalf("expected deduped/cleaned scope of 2, got %v", b.WorkspaceScope)
	}
	if got, ok := r.Get(a.ID); !ok || got.ID != a.ID {
		t.Fatalf("Get(%s) failed", a.ID)
	}
	if list := r.List(); len(list) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(list))
	}
	if !r.Remove(a.ID) {
		t.Fatal("expected Remove to succeed")
	}
	if r.Remove(a.ID) {
		t.Fatal("expected second Remove to fail")
	}
	if _, ok := r.Get(a.ID); ok {
		t.Fatal("expected subscription to be gone")
	}
}

func TestSpikeSubscriptionAllowThresholdGating(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()
	sub, _ := r.Create("", SpikeKindSignatureChange, nil)

	sig := mustPayload(t, SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/a.go", SpikeKind: string(SpikeKindSignatureChange)})
	if !sub.Allow(TopicASTSemanticSpike, sig) {
		t.Fatal("signature_change should pass signature_change threshold")
	}
	added := mustPayload(t, SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/a.go", SpikeKind: string(SpikeKindSymbolAdded)})
	if sub.Allow(TopicASTSemanticSpike, added) {
		t.Fatal("symbol_added should NOT pass signature_change threshold")
	}
	// Non-AST topics are never delivered to a spike-scoped stream.
	if sub.Allow("dispatch/decision", sig) {
		t.Fatal("non-AST topic must not match")
	}
}

func TestSpikeSubscriptionAllowAnyThresholdIncludesVolume(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()
	sub, _ := r.Create("", SpikeThresholdAny, nil)

	added := mustPayload(t, SemanticSpikeEvent{Workspace: "ws1", SpikeKind: string(SpikeKindSymbolAdded)})
	if !sub.Allow(TopicASTSemanticSpike, added) {
		t.Fatal("symbol_added should pass 'any' threshold")
	}
	vol := mustPayload(t, ASTSpikeEvent{Workspace: "ws1", HotPaths: []string{"pkg/a.go"}})
	if !sub.Allow(TopicASTSpike, vol) {
		t.Fatal("volume spike should pass 'any' threshold")
	}
}

func TestSpikeSubscriptionAllowPathGlob(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()
	sub, _ := r.Create("pkg/*.go", SpikeThresholdAny, nil)

	match := mustPayload(t, SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/a.go", SpikeKind: string(SpikeKindSignatureChange)})
	if !sub.Allow(TopicASTSemanticSpike, match) {
		t.Fatal("matching path should pass glob")
	}
	noMatch := mustPayload(t, SemanticSpikeEvent{Workspace: "ws1", Path: "cmd/main.go", SpikeKind: string(SpikeKindSignatureChange)})
	if sub.Allow(TopicASTSemanticSpike, noMatch) {
		t.Fatal("non-matching path should be filtered")
	}
	// Volume spike matches on any hot path.
	vol := mustPayload(t, ASTSpikeEvent{Workspace: "ws1", HotPaths: []string{"cmd/main.go", "pkg/b.go"}})
	if !sub.Allow(TopicASTSpike, vol) {
		t.Fatal("volume spike with one matching hot path should pass")
	}
}

func TestSpikeSubscriptionAllowWorkspaceScope(t *testing.T) {
	r := NewSpikeSubscriptionRegistry()
	sub, _ := r.Create("", SpikeThresholdAny, []string{"ws-keep"})

	in := mustPayload(t, SemanticSpikeEvent{Workspace: "ws-keep", SpikeKind: string(SpikeKindSignatureChange)})
	if !sub.Allow(TopicASTSemanticSpike, in) {
		t.Fatal("in-scope workspace should pass")
	}
	out := mustPayload(t, SemanticSpikeEvent{Workspace: "ws-other", SpikeKind: string(SpikeKindSignatureChange)})
	if sub.Allow(TopicASTSemanticSpike, out) {
		t.Fatal("out-of-scope workspace should be filtered")
	}
	// Conflict warning matches when any listed workspace is in scope.
	warn := mustPayload(t, SemanticConflictWarning{Workspaces: []string{"ws-other", "ws-keep"}, SpikeKind: string(SpikeKindSignatureChange)})
	if !sub.Allow(TopicASTConflictWarning, warn) {
		t.Fatal("conflict warning listing an in-scope workspace should pass")
	}
}

func TestParseSpikeResourceID(t *testing.T) {
	id, ok := ParseSpikeResourceID(SpikeResourcePrefix + "abc123")
	if !ok || id != "abc123" {
		t.Fatalf("expected abc123, got %q ok=%v", id, ok)
	}
	if _, ok := ParseSpikeResourceID("cwso://other/abc"); ok {
		t.Fatal("non-spike URI should not parse")
	}
	if _, ok := ParseSpikeResourceID(SpikeResourcePrefix + "ab/cd"); ok {
		t.Fatal("URI with path separator should not parse")
	}
	if _, ok := ParseSpikeResourceID(SpikeResourcePrefix); ok {
		t.Fatal("empty id should not parse")
	}
}
