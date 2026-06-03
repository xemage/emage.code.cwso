package dispatch

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// Signal-path vocabulary shared by every event-driven monitor (dispatch anomalies and
// AST write-spikes). A monitor either rides a privileged eBPF kernel hook — near-zero
// detection latency, but the timestamp is advisory because it is produced in-kernel — or
// falls back to an unprivileged userspace path whose latency is measured against the
// triggering event.
const (
	signalPathEBPF      = "ebpf-hook"
	signalPathUserspace = "fallback-userspace"

	detectionModeAdvisory  = "advisory"
	detectionModeMeasured  = "measured"
	detectionModeEstimated = "estimated"
)

// signalPathConfig configures eBPF preference and the (injectable) availability checker.
type signalPathConfig struct {
	PreferEBPF  bool
	EBPFChecker func() (bool, string)
}

// signalPathResolver resolves the active detection signal path. It is shared by the
// dispatch anomaly monitor and the AST write-spike monitor (Phase 7) so both characterize
// privilege requirement, eBPF-vs-userspace, and latency semantics identically.
type signalPathResolver struct {
	preferEBPF bool
	checkEBPF  func() (bool, string)
}

func newSignalPathResolver(cfg signalPathConfig) signalPathResolver {
	check := cfg.EBPFChecker
	if check == nil {
		check = defaultEBPFChecker
	}
	return signalPathResolver{preferEBPF: cfg.PreferEBPF, checkEBPF: check}
}

// resolve reports the active path, its privilege requirement, and any fallback notes.
// When eBPF is preferred but unavailable, it degrades to the userspace path and records
// why in notes rather than failing — detection must never depend on privilege.
func (r signalPathResolver) resolve() (path, privilege, notes string) {
	if r.preferEBPF {
		ok, reason := r.checkEBPF()
		if ok {
			return signalPathEBPF, "CAP_BPF/CAP_PERFMON or root", ""
		}
		return signalPathUserspace, "none", fmt.Sprintf("ebpf unavailable: %s", strings.TrimSpace(reason))
	}
	return signalPathUserspace, "none", ""
}

// detectionLatency derives the detection latency and its trustworthiness for a signal
// path. eBPF timestamps are advisory (the hook fires in-kernel, so a userspace delta is
// meaningless); userspace latency is measured against the triggering event's emit time,
// or estimated when that time is missing/unparseable.
func detectionLatency(emittedAt string, detectedAt time.Time, path string) (int, string, bool) {
	if path == signalPathEBPF {
		return 0, detectionModeAdvisory, true
	}
	emittedAt = strings.TrimSpace(emittedAt)
	if emittedAt == "" {
		return 0, detectionModeEstimated, false
	}
	ts, err := time.Parse(time.RFC3339Nano, emittedAt)
	if err != nil {
		return 0, detectionModeEstimated, false
	}
	delta := detectedAt.Sub(ts.UTC())
	if delta < 0 {
		delta = 0
	}
	return int(delta.Milliseconds()), detectionModeMeasured, false
}

// defaultEBPFChecker reports whether an eBPF hook is plausibly attachable on this host.
// It is intentionally conservative: any doubt resolves to the userspace fallback.
func defaultEBPFChecker() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "non-linux host"
	}
	if os.Geteuid() != 0 {
		return false, "missing root/CAP_BPF privileges"
	}
	if _, err := os.Stat("/sys/fs/bpf"); err != nil {
		return false, "bpffs is unavailable"
	}
	return true, ""
}
