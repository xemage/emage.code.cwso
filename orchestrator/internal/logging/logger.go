// Package logging is a thin stdlib-only structured JSON logger.
//
// Phase 1 uses stdlib only to keep dependencies minimal. A richer logger
// (zerolog) is planned for Phase 3 hardening (T029).
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level is a log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func parseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Logger writes structured JSON lines to an io.Writer.
type Logger struct {
	mu    sync.Mutex
	w     io.Writer
	level Level
	base  map[string]any
}

// New returns a Logger writing to stderr at the given text level.
// stdio transport REQUIRES non-stdout logging to avoid corrupting JSON-RPC framing.
func New(levelStr string) *Logger {
	return &Logger{
		w:     os.Stderr,
		level: parseLevel(levelStr),
		base:  map[string]any{},
	}
}

// Event is a fluent log builder.
type Event struct {
	logger *Logger
	level  Level
	fields map[string]any
}

func (l *Logger) event(lvl Level) *Event {
	e := &Event{logger: l, level: lvl, fields: make(map[string]any, 8)}
	for k, v := range l.base {
		e.fields[k] = v
	}
	return e
}

// Debug starts a debug-level event.
func (l *Logger) Debug() *Event { return l.event(LevelDebug) }

// Info starts an info-level event.
func (l *Logger) Info() *Event { return l.event(LevelInfo) }

// Warn starts a warn-level event.
func (l *Logger) Warn() *Event { return l.event(LevelWarn) }

// Error starts an error-level event.
func (l *Logger) Error() *Event { return l.event(LevelError) }

// Fatal starts a fatal-level event; Msg() will exit(1).
func (l *Logger) Fatal() *Event { return l.event(LevelFatal) }

// Str adds a string field.
func (e *Event) Str(k, v string) *Event { e.fields[k] = v; return e }

// Int adds an int field.
func (e *Event) Int(k string, v int) *Event { e.fields[k] = v; return e }

// Err adds an error field.
func (e *Event) Err(err error) *Event {
	if err != nil {
		e.fields["error"] = err.Error()
	}
	return e
}

// Any adds an arbitrary field.
func (e *Event) Any(k string, v any) *Event { e.fields[k] = v; return e }

var levelNames = map[Level]string{
	LevelDebug: "debug", LevelInfo: "info", LevelWarn: "warn",
	LevelError: "error", LevelFatal: "fatal",
}

// Msg finalizes and emits the event.
func (e *Event) Msg(msg string) {
	if e.level < e.logger.level {
		return
	}
	e.fields["level"] = levelNames[e.level]
	e.fields["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	e.fields["msg"] = msg
	b, err := json.Marshal(e.fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger marshal error: %v\n", err)
		return
	}
	e.logger.mu.Lock()
	_, _ = e.logger.w.Write(append(b, '\n'))
	e.logger.mu.Unlock()
	if e.level == LevelFatal {
		os.Exit(1)
	}
}

// With returns a child logger with persistent fields.
func (l *Logger) With(k string, v any) *Logger {
	child := &Logger{w: l.w, level: l.level, base: make(map[string]any, len(l.base)+1)}
	for kk, vv := range l.base {
		child.base[kk] = vv
	}
	child.base[k] = v
	return child
}
