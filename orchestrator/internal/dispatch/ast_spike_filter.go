package dispatch

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// SpikeKind classifies the semantic significance of an AST-affecting write. The ordering
// (none < cosmetic < symbol_added/symbol_removed < signature_change) drives threshold
// gating: the filter only emits a semantic spike when an edit reaches the configured
// SemanticThreshold.
type SpikeKind string

const (
	SpikeKindNone            SpikeKind = "none"
	SpikeKindCosmetic        SpikeKind = "cosmetic"
	SpikeKindSymbolAdded     SpikeKind = "symbol_added"
	SpikeKindSymbolRemoved   SpikeKind = "symbol_removed"
	SpikeKindSignatureChange SpikeKind = "signature_change"

	// SpikeThresholdAny is the least-restrictive threshold: any classified spike fires.
	SpikeThresholdAny SpikeKind = "any"
)

// Phase 7 (Feature C) spike-filter defaults.
const (
	defaultConflictWindowMS = 2000
	defaultSignatureTTLMS   = 30000
	defaultMaxConflictPeers = 8
)

// rank orders spike kinds by semantic significance. symbol_added and symbol_removed share
// rank 2 because neither dominates the other; signature_change is the strongest signal.
func (k SpikeKind) rank() int {
	switch k {
	case SpikeKindCosmetic:
		return 1
	case SpikeKindSymbolAdded, SpikeKindSymbolRemoved:
		return 2
	case SpikeKindSignatureChange:
		return 3
	default:
		return 0
	}
}

// thresholdRank is the minimum kind rank that satisfies a configured threshold.
// SpikeThresholdAny accepts anything cosmetic-or-stronger (rank ≥ 1).
func thresholdRank(threshold SpikeKind) int {
	if threshold == SpikeThresholdAny {
		return 1
	}
	if r := threshold.rank(); r > 0 {
		return r
	}
	// Unknown/empty threshold defaults to the strongest signal (signature_change).
	return SpikeKindSignatureChange.rank()
}

// SemanticScorer maps a write — plus the previously-seen signature for its symbol — to a
// spike kind and a confidence in [0,1]. The default heuristic is deterministic and
// dependency-free; a sparse Wasm micro-agent (Feature B) can replace it later via the
// Scorer config field without touching the filter's correlation logic.
type SemanticScorer func(ev WriteEvent, priorSignature string, seenSymbol bool) (SpikeKind, float64)

// HeuristicSemanticScorer is the default, deterministic scorer. It trusts a feeder-supplied
// ChangeKind when present; otherwise it derives the kind from the symbol's signature delta.
func HeuristicSemanticScorer(ev WriteEvent, priorSignature string, seenSymbol bool) (SpikeKind, float64) {
	if ev.ChangeKind != "" && ev.ChangeKind.rank() >= 0 {
		switch ev.ChangeKind {
		case SpikeKindSignatureChange:
			return SpikeKindSignatureChange, 0.95
		case SpikeKindSymbolAdded, SpikeKindSymbolRemoved:
			return ev.ChangeKind, 0.9
		case SpikeKindCosmetic:
			return SpikeKindCosmetic, 0.6
		case SpikeKindNone:
			return SpikeKindNone, 0
		}
	}
	if ev.Symbol == "" || ev.SignatureHash == "" {
		// No semantic signal available; volume monitoring (T115) covers this write.
		return SpikeKindNone, 0
	}
	if !seenSymbol {
		return SpikeKindSymbolAdded, 0.75
	}
	if priorSignature != ev.SignatureHash {
		return SpikeKindSignatureChange, 0.9
	}
	return SpikeKindCosmetic, 0.3
}

// SemanticSpikeEvent is published when a write crosses the configured semantic threshold.
type SemanticSpikeEvent struct {
	SpikeID                    string  `json:"spike_id"`
	Workspace                  string  `json:"workspace,omitempty"`
	Path                       string  `json:"path,omitempty"`
	Symbol                     string  `json:"symbol,omitempty"`
	NodePath                   string  `json:"node_path,omitempty"`
	SpikeKind                  string  `json:"spike_kind"`
	Confidence                 float64 `json:"confidence"`
	SemanticThreshold          string  `json:"semantic_threshold"`
	SignalPath                 string  `json:"signal_path"`
	PrivilegeRequirement       string  `json:"privilege_requirement"`
	DetectionLatencyMS         int     `json:"detection_latency_ms"`
	DetectionLatencyMode       string  `json:"detection_latency_mode"`
	DetectionLatencyIsAdvisory bool    `json:"detection_latency_is_advisory"`
	DetectedAt                 string  `json:"detected_at"`
	Notes                      string  `json:"notes,omitempty"`
}

// SemanticConflictWarning is the pre-warning emitted when two or more workspaces produce
// semantic spikes on the same symbol surface inside the correlation window — letting the
// orchestrator warn agents of an imminent conflict before merge_concurrent_results runs.
type SemanticConflictWarning struct {
	WarningID             string   `json:"warning_id"`
	Symbol                string   `json:"symbol,omitempty"`
	Path                  string   `json:"path,omitempty"`
	SpikeKind             string   `json:"spike_kind"`
	Confidence            float64  `json:"confidence"`
	Workspaces            []string `json:"workspaces"`
	PotentialConflictWith []string `json:"potential_conflict_with"`
	Severity              string   `json:"severity"`
	SignalPath            string   `json:"signal_path"`
	PrivilegeRequirement  string   `json:"privilege_requirement"`
	DetectedAt            string   `json:"detected_at"`
	Notes                 string   `json:"notes,omitempty"`
}

// ASTSpikeFilterConfig configures the semantic spike filter.
type ASTSpikeFilterConfig struct {
	PreferEBPF        bool
	EBPFChecker       func() (bool, string)
	SemanticThreshold SpikeKind
	ConflictWindowMS  int
	SignatureTTLMS    int
	MaxConflictPeers  int
	Scorer            SemanticScorer
	Redaction         TelemetryRedactionConfig
}

type symbolSignature struct {
	signature string
	updatedAt time.Time
}

type symbolWriter struct {
	workspace  string
	kind       SpikeKind
	confidence float64
	at         time.Time
}

// ASTSpikeFilter classifies write-spikes by semantic significance (step 2 of Feature C)
// and emits cross-workspace conflict pre-warnings (step 5). It sits downstream of the raw
// ASTWriteSpikeMonitor: the monitor detects write *volume*, the filter decides whether an
// edit *matters* and whether it overlaps a sibling agent's in-flight change.
type ASTSpikeFilter struct {
	publisher        memorybroker.Publisher
	now              func() time.Time
	nextSpikeID      atomic.Uint64
	nextWarnID       atomic.Uint64
	resolver         signalPathResolver
	redactor         telemetryRedactor
	scorer           SemanticScorer
	threshold        SpikeKind
	thresholdRank    int
	conflictWindow   time.Duration
	signatureTTL     time.Duration
	maxConflictPeers int
	notesDrop        bool

	mu         sync.Mutex
	signatures map[string]symbolSignature // symbol -> last seen signature
	writers    map[string][]symbolWriter  // symbol -> recent semantic writers
}

func NewASTSpikeFilter(publisher memorybroker.Publisher, cfg ASTSpikeFilterConfig) *ASTSpikeFilter {
	if publisher == nil {
		return nil
	}
	if cfg.SemanticThreshold == "" {
		cfg.SemanticThreshold = SpikeKindSignatureChange
	}
	if cfg.ConflictWindowMS <= 0 {
		cfg.ConflictWindowMS = defaultConflictWindowMS
	}
	if cfg.SignatureTTLMS <= 0 {
		cfg.SignatureTTLMS = defaultSignatureTTLMS
	}
	if cfg.MaxConflictPeers <= 0 {
		cfg.MaxConflictPeers = defaultMaxConflictPeers
	}
	scorer := cfg.Scorer
	if scorer == nil {
		scorer = HeuristicSemanticScorer
	}
	redactor := newTelemetryRedactor(cfg.Redaction)
	return &ASTSpikeFilter{
		publisher:        publisher,
		now:              time.Now,
		resolver:         newSignalPathResolver(signalPathConfig{PreferEBPF: cfg.PreferEBPF, EBPFChecker: cfg.EBPFChecker}),
		redactor:         redactor,
		scorer:           scorer,
		threshold:        cfg.SemanticThreshold,
		thresholdRank:    thresholdRank(cfg.SemanticThreshold),
		conflictWindow:   time.Duration(cfg.ConflictWindowMS) * time.Millisecond,
		signatureTTL:     time.Duration(cfg.SignatureTTLMS) * time.Millisecond,
		maxConflictPeers: cfg.MaxConflictPeers,
		notesDrop:        redactor.enabled && redactor.anomalyNotesMode == anomalyNotesModeDrop,
		signatures:       make(map[string]symbolSignature),
		writers:          make(map[string][]symbolWriter),
	}
}

// ObserveWrite classifies one write and publishes a semantic spike when it crosses the
// threshold, plus a conflict pre-warning when a different workspace recently produced a
// semantic spike on the same symbol. Safe for concurrent use.
func (f *ASTSpikeFilter) ObserveWrite(event WriteEvent) error {
	if f == nil || f.publisher == nil {
		return nil
	}
	at := event.At
	if at.IsZero() {
		at = f.now()
	}
	at = at.UTC()

	spike, warning, emitSpike, emitWarn := f.classify(event, at)

	if emitSpike {
		if err := f.publisher.Publish(TopicASTSemanticSpike, spike); err != nil {
			return err
		}
	}
	if emitWarn {
		if err := f.publisher.Publish(TopicASTConflictWarning, warning); err != nil {
			return err
		}
	}
	return nil
}

func (f *ASTSpikeFilter) classify(event WriteEvent, at time.Time) (SemanticSpikeEvent, SemanticConflictWarning, bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	prior, seen := f.signatures[event.Symbol]
	kind, confidence := f.scorer(event, prior.signature, seen && event.Symbol != "")

	// Record the latest signature for this symbol so the next write can diff against it.
	if event.Symbol != "" && event.SignatureHash != "" {
		f.signatures[event.Symbol] = symbolSignature{signature: event.SignatureHash, updatedAt: at}
	}
	f.pruneSignatures(at)

	if kind.rank() < f.thresholdRank {
		return SemanticSpikeEvent{}, SemanticConflictWarning{}, false, false
	}

	signalPath, privilege, notes := f.resolver.resolve()
	latencyMS, mode, advisory := detectionLatency(at.Format(time.RFC3339Nano), at, signalPath)

	spike := SemanticSpikeEvent{
		SpikeID:                    fmt.Sprintf("semantic-spike-%d", f.nextSpikeID.Add(1)),
		Workspace:                  event.Workspace,
		Path:                       event.Path,
		Symbol:                     event.Symbol,
		NodePath:                   event.NodePath,
		SpikeKind:                  string(kind),
		Confidence:                 confidence,
		SemanticThreshold:          string(f.threshold),
		SignalPath:                 signalPath,
		PrivilegeRequirement:       privilege,
		DetectionLatencyMS:         latencyMS,
		DetectionLatencyMode:       mode,
		DetectionLatencyIsAdvisory: advisory,
		DetectedAt:                 at.Format(time.RFC3339Nano),
		Notes:                      f.redactor.redactAnomalyNotes(notes),
	}

	warning, emitWarn := f.detectConflict(event, kind, confidence, at, signalPath, privilege, notes)

	// Symbol/path/node-path expose source structure; drop them (like T115 hot paths) when
	// the notes-drop redaction policy is active. Correlation already happened above, so
	// blanking the emitted copies does not affect conflict detection.
	if f.notesDrop {
		spike.Path, spike.Symbol, spike.NodePath = "", "", ""
		warning.Path, warning.Symbol = "", ""
	}
	return spike, warning, true, emitWarn
}

// detectConflict records the current semantic writer for the symbol and, if a different
// workspace also wrote that symbol inside the correlation window, returns a pre-warning.
// Symbol-less writes cannot be correlated, so they never raise a conflict.
func (f *ASTSpikeFilter) detectConflict(
	event WriteEvent,
	kind SpikeKind,
	confidence float64,
	at time.Time,
	signalPath, privilege, notes string,
) (SemanticConflictWarning, bool) {
	if event.Symbol == "" {
		return SemanticConflictWarning{}, false
	}

	cutoff := at.Add(-f.conflictWindow)
	recent := f.writers[event.Symbol][:0]
	for _, w := range f.writers[event.Symbol] {
		if w.at.After(cutoff) {
			recent = append(recent, w)
		}
	}

	peers := make(map[string]struct{})
	for _, w := range recent {
		if w.workspace != "" && w.workspace != event.Workspace {
			peers[w.workspace] = struct{}{}
		}
	}

	f.writers[event.Symbol] = append(recent, symbolWriter{
		workspace:  event.Workspace,
		kind:       kind,
		confidence: confidence,
		at:         at,
	})

	if len(peers) == 0 {
		return SemanticConflictWarning{}, false
	}

	peerList := make([]string, 0, len(peers))
	for ws := range peers {
		peerList = append(peerList, ws)
	}
	sort.Strings(peerList)
	if len(peerList) > f.maxConflictPeers {
		peerList = peerList[:f.maxConflictPeers]
	}

	workspaces := append([]string{}, peerList...)
	if event.Workspace != "" {
		workspaces = append(workspaces, event.Workspace)
		sort.Strings(workspaces)
	}

	return SemanticConflictWarning{
		WarningID:             fmt.Sprintf("ast-conflict-%d", f.nextWarnID.Add(1)),
		Symbol:                event.Symbol,
		Path:                  event.Path,
		SpikeKind:             string(kind),
		Confidence:            confidence,
		Workspaces:            workspaces,
		PotentialConflictWith: peerList,
		Severity:              conflictSeverity(kind),
		SignalPath:            signalPath,
		PrivilegeRequirement:  privilege,
		DetectedAt:            at.Format(time.RFC3339Nano),
		Notes:                 f.redactor.redactAnomalyNotes(notes),
	}, true
}

func conflictSeverity(kind SpikeKind) string {
	if kind == SpikeKindSignatureChange {
		return "critical"
	}
	return "warning"
}

func (f *ASTSpikeFilter) pruneSignatures(at time.Time) {
	cutoff := at.Add(-f.signatureTTL)
	for sym, sig := range f.signatures {
		if !sig.updatedAt.After(cutoff) {
			delete(f.signatures, sym)
		}
	}
}
