package dispatch

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// Phase 7 (Feature C) defaults for AST write-spike detection. A "spike" is a burst of
// AST-affecting writes inside a short sliding window, which front-runs the merge engine
// with an early warning that concurrent agents are converging on the same code.
const (
	defaultASTSpikeWindowMS    = 1000
	defaultASTSpikeThreshold   = 8
	defaultASTSpikeMaxHotPaths = 5
)

// ASTWriteSpikeMonitorConfig configures the AST write-spike monitor. Window/Threshold
// shape sensitivity; PreferEBPF/EBPFChecker characterize the detection signal path the
// same way the dispatch anomaly monitor does (eBPF hook when available, userspace
// fallback otherwise). DebounceMS suppresses repeat emissions during a sustained burst.
type ASTWriteSpikeMonitorConfig struct {
	PreferEBPF  bool
	EBPFChecker func() (bool, string)
	WindowMS    int
	Threshold   int
	DebounceMS  int
	MaxHotPaths int
	Redaction   TelemetryRedactionConfig
}

// WriteEvent is one AST-affecting write observed by the monitor. Source is agnostic: an
// eBPF write probe or a userspace filesystem watcher can both feed ObserveWrite.
type WriteEvent struct {
	Path      string
	Workspace string
	Language  string
	At        time.Time
}

// ASTSpikeEvent is the telemetry envelope emitted when a write-spike crosses threshold.
type ASTSpikeEvent struct {
	SpikeID                    string   `json:"spike_id"`
	Workspace                  string   `json:"workspace,omitempty"`
	WindowMS                   int      `json:"window_ms"`
	Threshold                  int      `json:"threshold"`
	ObservedWrites             int      `json:"observed_writes"`
	DistinctPaths              int      `json:"distinct_paths"`
	HotPaths                   []string `json:"hot_paths,omitempty"`
	Languages                  []string `json:"languages,omitempty"`
	Severity                   string   `json:"severity"`
	SignalPath                 string   `json:"signal_path"`
	PrivilegeRequirement       string   `json:"privilege_requirement"`
	DetectionLatencyMS         int      `json:"detection_latency_ms"`
	DetectionLatencyMode       string   `json:"detection_latency_mode"`
	DetectionLatencyIsAdvisory bool     `json:"detection_latency_is_advisory"`
	DetectedAt                 string   `json:"detected_at"`
	Notes                      string   `json:"notes,omitempty"`
}

type writeSample struct {
	path     string
	language string
	at       time.Time
}

// ASTWriteSpikeMonitor maintains a per-workspace sliding window of recent writes and
// emits an ASTSpikeEvent when the window's write count meets the threshold. It is the
// generalization of DecisionAnomalyMonitor's event-driven core to filesystem write
// activity, reusing the shared signal-path resolver and detection-latency semantics.
type ASTWriteSpikeMonitor struct {
	publisher   memorybroker.Publisher
	now         func() time.Time
	nextID      atomic.Uint64
	resolver    signalPathResolver
	redactor    telemetryRedactor
	window      time.Duration
	threshold   int
	debounce    time.Duration
	maxHotPaths int

	mu        sync.Mutex
	windows   map[string]*spikeWindow
	notesDrop bool
}

type spikeWindow struct {
	samples     []writeSample
	lastSpikeAt time.Time
}

func NewASTWriteSpikeMonitor(publisher memorybroker.Publisher, cfg ASTWriteSpikeMonitorConfig) *ASTWriteSpikeMonitor {
	if publisher == nil {
		return nil
	}
	if cfg.WindowMS <= 0 {
		cfg.WindowMS = defaultASTSpikeWindowMS
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = defaultASTSpikeThreshold
	}
	if cfg.MaxHotPaths <= 0 {
		cfg.MaxHotPaths = defaultASTSpikeMaxHotPaths
	}
	// Default debounce to one window so a single burst yields at most one spike per window.
	if cfg.DebounceMS <= 0 {
		cfg.DebounceMS = cfg.WindowMS
	}
	redactor := newTelemetryRedactor(cfg.Redaction)
	return &ASTWriteSpikeMonitor{
		publisher:   publisher,
		now:         time.Now,
		resolver:    newSignalPathResolver(signalPathConfig{PreferEBPF: cfg.PreferEBPF, EBPFChecker: cfg.EBPFChecker}),
		redactor:    redactor,
		window:      time.Duration(cfg.WindowMS) * time.Millisecond,
		threshold:   cfg.Threshold,
		debounce:    time.Duration(cfg.DebounceMS) * time.Millisecond,
		maxHotPaths: cfg.MaxHotPaths,
		windows:     make(map[string]*spikeWindow),
		// When notes are dropped by redaction policy, hot paths (which can leak filesystem
		// structure) are dropped alongside them.
		notesDrop: redactor.enabled && redactor.anomalyNotesMode == anomalyNotesModeDrop,
	}
}

// ObserveWrite records one AST-affecting write and emits a spike event when the sliding
// window crosses the configured threshold (subject to debounce). It is safe for
// concurrent use and never blocks on the slow path beyond a single Publish.
func (m *ASTWriteSpikeMonitor) ObserveWrite(event WriteEvent) error {
	if m == nil || m.publisher == nil {
		return nil
	}
	at := event.At
	if at.IsZero() {
		at = m.now()
	}
	at = at.UTC()

	spike, ok := m.record(event, at)
	if !ok {
		return nil
	}
	return m.publisher.Publish(TopicASTSpike, spike)
}

// record appends the sample, prunes the window, and returns a spike event when the
// threshold is met and debounce has elapsed. It returns ok=false when no spike fires.
func (m *ASTWriteSpikeMonitor) record(event WriteEvent, at time.Time) (ASTSpikeEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	win := m.windows[event.Workspace]
	if win == nil {
		win = &spikeWindow{}
		m.windows[event.Workspace] = win
	}

	cutoff := at.Add(-m.window)
	pruned := win.samples[:0]
	for _, s := range win.samples {
		if s.at.After(cutoff) {
			pruned = append(pruned, s)
		}
	}
	win.samples = append(pruned, writeSample{path: event.Path, language: event.Language, at: at})

	count := len(win.samples)
	if count < m.threshold {
		return ASTSpikeEvent{}, false
	}
	if !win.lastSpikeAt.IsZero() && at.Sub(win.lastSpikeAt) < m.debounce {
		return ASTSpikeEvent{}, false
	}
	win.lastSpikeAt = at

	return m.buildSpike(event.Workspace, win.samples, at), true
}

func (m *ASTWriteSpikeMonitor) buildSpike(workspace string, samples []writeSample, detectedAt time.Time) ASTSpikeEvent {
	signalPath, privilege, notes := m.resolver.resolve()

	// Detection latency is measured from the most recent (triggering) write.
	triggerAt := samples[len(samples)-1].at
	latencyMS, mode, advisory := detectionLatency(triggerAt.Format(time.RFC3339Nano), detectedAt, signalPath)

	hotPaths, distinct := topPaths(samples, m.maxHotPaths)
	languages := distinctLanguages(samples)
	if m.notesDrop {
		hotPaths = nil
	}

	return ASTSpikeEvent{
		SpikeID:                    fmt.Sprintf("ast-spike-%d", m.nextID.Add(1)),
		Workspace:                  workspace,
		WindowMS:                   int(m.window.Milliseconds()),
		Threshold:                  m.threshold,
		ObservedWrites:             len(samples),
		DistinctPaths:              distinct,
		HotPaths:                   hotPaths,
		Languages:                  languages,
		Severity:                   spikeSeverity(len(samples), m.threshold),
		SignalPath:                 signalPath,
		PrivilegeRequirement:       privilege,
		DetectionLatencyMS:         latencyMS,
		DetectionLatencyMode:       mode,
		DetectionLatencyIsAdvisory: advisory,
		DetectedAt:                 detectedAt.Format(time.RFC3339Nano),
		Notes:                      m.redactor.redactAnomalyNotes(notes),
	}
}

func spikeSeverity(count, threshold int) string {
	if threshold > 0 && count >= 2*threshold {
		return "critical"
	}
	return "warning"
}

// topPaths returns the most-written paths (descending by frequency, then path for
// determinism), capped at limit, plus the count of distinct paths in the window.
func topPaths(samples []writeSample, limit int) ([]string, int) {
	counts := make(map[string]int, len(samples))
	order := make([]string, 0, len(samples))
	for _, s := range samples {
		if s.path == "" {
			continue
		}
		if _, seen := counts[s.path]; !seen {
			order = append(order, s.path)
		}
		counts[s.path]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return order[i] < order[j]
	})
	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}
	if len(order) == 0 {
		return nil, len(counts)
	}
	return order, len(counts)
}

func distinctLanguages(samples []writeSample) []string {
	seen := make(map[string]struct{}, len(samples))
	langs := make([]string, 0, len(samples))
	for _, s := range samples {
		if s.language == "" {
			continue
		}
		if _, ok := seen[s.language]; ok {
			continue
		}
		seen[s.language] = struct{}{}
		langs = append(langs, s.language)
	}
	if len(langs) == 0 {
		return nil
	}
	sort.Strings(langs)
	return langs
}
