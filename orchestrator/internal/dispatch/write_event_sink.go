package dispatch

// WriteEventSink consumes AST-affecting write events from a runtime source (e.g. the
// write_shadow_file tool, or a future eBPF/fs-watch feeder). It is implemented by
// *ASTWriteSpikeMonitor (volume detection) and *ASTSpikeFilter (semantic classification),
// and composed via NewWriteEventFanout so one write feeds both stages.
type WriteEventSink interface {
	ObserveWrite(WriteEvent) error
}

type writeEventFanout struct {
	sinks []WriteEventSink
}

// NewWriteEventFanout composes sinks so a single write is observed by each. Nil sinks are
// dropped; if no non-nil sink remains it returns nil so callers can treat "no feeder"
// uniformly (the feeder is simply not attached).
func NewWriteEventFanout(sinks ...WriteEventSink) WriteEventSink {
	live := make([]WriteEventSink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			live = append(live, s)
		}
	}
	if len(live) == 0 {
		return nil
	}
	return writeEventFanout{sinks: live}
}

// ObserveWrite fans the event to every sink. Sinks are best-effort and independent: a
// failing sink does not prevent the others from observing the event; the first error (if any)
// is returned for visibility.
func (f writeEventFanout) ObserveWrite(ev WriteEvent) error {
	var firstErr error
	for _, s := range f.sinks {
		if err := s.ObserveWrite(ev); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
