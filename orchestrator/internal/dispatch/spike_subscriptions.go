package dispatch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// SpikeResourceScheme + prefix define the cwso:// URI namespace for AST spike streams
// exposed as MCP resources (T117). A subscription is addressed as
// "cwso://spikes/<subscription_id>".
const (
	SpikeResourceScheme = "cwso"
	SpikeResourcePrefix = "cwso://spikes/"
)

// astSpikeTopics are the broker topics a spike subscription may carry. Records on any
// other topic are never delivered to a spike-scoped stream.
var astSpikeTopics = []string{TopicASTSpike, TopicASTSemanticSpike, TopicASTConflictWarning}

// ASTSpikeTopics returns the broker topics relevant to spike subscriptions (copy).
func ASTSpikeTopics() []string {
	out := make([]string, len(astSpikeTopics))
	copy(out, astSpikeTopics)
	return out
}

// SpikeSubscription captures one subscribe_ast_spikes registration: the filter criteria a
// scoped SSE stream (and resources/read snapshot) applies to AST spike events.
type SpikeSubscription struct {
	ID                string    `json:"subscription_id"`
	Path              string    `json:"path,omitempty"`
	SemanticThreshold SpikeKind `json:"semantic_threshold"`
	WorkspaceScope    []string  `json:"workspace_scope,omitempty"`
	CreatedAt         time.Time `json:"created_at"`

	thresholdRank int
	scope         map[string]struct{}
}

// URI returns the cwso:// resource URI addressing this subscription's stream.
func (s *SpikeSubscription) URI() string { return SpikeResourcePrefix + s.ID }

// spikeEnvelope is the minimal projection of the AST event payloads the filter inspects.
type spikeEnvelope struct {
	Workspace  string   `json:"workspace"`
	Path       string   `json:"path"`
	SpikeKind  string   `json:"spike_kind"`
	HotPaths   []string `json:"hot_paths"`
	Workspaces []string `json:"workspaces"`
}

// Allow reports whether a broker record matches this subscription. It implements the
// transport's record-filter contract so a scoped SSE stream only carries matching events.
func (s *SpikeSubscription) Allow(topic string, payload []byte) bool {
	if s == nil {
		return false
	}
	if !isASTSpikeTopic(topic) {
		return false
	}
	var env spikeEnvelope
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &env)
	}

	if !s.matchesThreshold(topic, env.SpikeKind) {
		return false
	}
	if !s.matchesWorkspace(env) {
		return false
	}
	return s.matchesPath(topic, env)
}

func (s *SpikeSubscription) matchesThreshold(topic, kind string) bool {
	// Volume spikes (ast/spike) carry no semantic kind; treat them as the weakest signal
	// so they only pass the least-restrictive "any" threshold.
	rank := SpikeKind(kind).rank()
	if topic == TopicASTSpike {
		rank = SpikeKindCosmetic.rank()
	}
	return rank >= s.thresholdRank
}

func (s *SpikeSubscription) matchesWorkspace(env spikeEnvelope) bool {
	if len(s.scope) == 0 {
		return true
	}
	if env.Workspace != "" {
		if _, ok := s.scope[env.Workspace]; ok {
			return true
		}
	}
	for _, ws := range env.Workspaces {
		if _, ok := s.scope[ws]; ok {
			return true
		}
	}
	return false
}

func (s *SpikeSubscription) matchesPath(topic string, env spikeEnvelope) bool {
	if s.Path == "" {
		return true
	}
	candidates := []string{env.Path}
	if topic == TopicASTSpike {
		candidates = env.HotPaths
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if ok, err := path.Match(s.Path, c); err == nil && ok {
			return true
		}
	}
	return false
}

func isASTSpikeTopic(topic string) bool {
	for _, t := range astSpikeTopics {
		if t == topic {
			return true
		}
	}
	return false
}

// SpikeSubscriptionRegistry is a concurrency-safe store of active spike subscriptions.
type SpikeSubscriptionRegistry struct {
	mu    sync.RWMutex
	subs  map[string]*SpikeSubscription
	now   func() time.Time
	newID func() string
}

func NewSpikeSubscriptionRegistry() *SpikeSubscriptionRegistry {
	return &SpikeSubscriptionRegistry{
		subs:  make(map[string]*SpikeSubscription),
		now:   time.Now,
		newID: randomSubscriptionID,
	}
}

// Create validates the criteria and registers a new subscription. An empty threshold
// defaults to signature_change; an unknown threshold is rejected.
func (r *SpikeSubscriptionRegistry) Create(p string, threshold SpikeKind, workspaceScope []string) (*SpikeSubscription, error) {
	if r == nil {
		return nil, fmt.Errorf("spike subscriptions disabled")
	}
	threshold, err := NormalizeSemanticThreshold(threshold)
	if err != nil {
		return nil, err
	}
	if p != "" {
		if _, err := path.Match(p, "probe"); err != nil {
			return nil, fmt.Errorf("invalid path glob %q: %w", p, err)
		}
	}
	scope := make(map[string]struct{}, len(workspaceScope))
	cleanScope := make([]string, 0, len(workspaceScope))
	for _, ws := range workspaceScope {
		ws = strings.TrimSpace(ws)
		if ws == "" {
			continue
		}
		if _, dup := scope[ws]; dup {
			continue
		}
		scope[ws] = struct{}{}
		cleanScope = append(cleanScope, ws)
	}

	sub := &SpikeSubscription{
		ID:                r.newID(),
		Path:              p,
		SemanticThreshold: threshold,
		WorkspaceScope:    cleanScope,
		CreatedAt:         r.now().UTC(),
		thresholdRank:     thresholdRank(threshold),
		scope:             scope,
	}
	r.mu.Lock()
	r.subs[sub.ID] = sub
	r.mu.Unlock()
	return sub, nil
}

// Get returns the subscription for an id.
func (r *SpikeSubscriptionRegistry) Get(id string) (*SpikeSubscription, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	sub, ok := r.subs[id]
	return sub, ok
}

// Remove deletes a subscription, returning whether it existed.
func (r *SpikeSubscriptionRegistry) Remove(id string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.subs[id]; !ok {
		return false
	}
	delete(r.subs, id)
	return true
}

// List returns all active subscriptions ordered by creation time then id (deterministic).
func (r *SpikeSubscriptionRegistry) List() []*SpikeSubscription {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]*SpikeSubscription, 0, len(r.subs))
	for _, s := range r.subs {
		out = append(out, s)
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// NormalizeSemanticThreshold validates and canonicalizes a semantic threshold value.
func NormalizeSemanticThreshold(threshold SpikeKind) (SpikeKind, error) {
	switch threshold {
	case "":
		return SpikeKindSignatureChange, nil
	case SpikeKindSignatureChange, SpikeKindSymbolAdded, SpikeKindSymbolRemoved, SpikeThresholdAny:
		return threshold, nil
	default:
		return "", fmt.Errorf("invalid semantic_threshold %q (want signature_change|symbol_added|symbol_removed|any)", threshold)
	}
}

// ParseSpikeResourceID extracts the subscription id from a cwso://spikes/<id> URI.
func ParseSpikeResourceID(uri string) (string, bool) {
	if !strings.HasPrefix(uri, SpikeResourcePrefix) {
		return "", false
	}
	id := strings.TrimPrefix(uri, SpikeResourcePrefix)
	if id == "" || strings.ContainsAny(id, "/?#") {
		return "", false
	}
	return id, true
}

func randomSubscriptionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to a time-based id so we never panic.
		return fmt.Sprintf("spk-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
