package logging

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseLevelAndNew(t *testing.T) {
	cases := map[string]Level{
		"debug":   LevelDebug,
		"warn":    LevelWarn,
		"error":   LevelError,
		"fatal":   LevelFatal,
		"INFO":    LevelInfo,
		"unknown": LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q)=%v want %v", in, got, want)
		}
	}

	l := New("warn")
	if l.level != LevelWarn {
		t.Fatalf("expected New to set warn level, got %v", l.level)
	}
}

func TestLoggerFilteringAndStructuredOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{w: buf, level: LevelWarn, base: map[string]any{"component": "test"}}

	l.Info().Str("ignored", "yes").Msg("should not be written")
	if buf.Len() != 0 {
		t.Fatalf("expected info log below threshold to be filtered, got %q", buf.String())
	}

	l.Warn().Str("k", "v").Int("n", 2).Err(errors.New("boom")).Any("flag", true).Msg("written")
	out := buf.String()
	if !strings.Contains(out, `"level":"warn"`) || !strings.Contains(out, `"msg":"written"`) {
		t.Fatalf("missing required fields in output: %q", out)
	}
	for _, needle := range []string{`"component":"test"`, `"k":"v"`, `"n":2`, `"error":"boom"`, `"flag":true`, `"time":"`} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %s in output: %q", needle, out)
		}
	}
}

func TestWithCreatesChildWithoutMutatingParent(t *testing.T) {
	bufParent := &bytes.Buffer{}
	parent := &Logger{w: bufParent, level: LevelDebug, base: map[string]any{"app": "cwso"}}
	child := parent.With("request_id", "rid-1")

	if _, ok := parent.base["request_id"]; ok {
		t.Fatal("parent logger base must not be mutated by With")
	}

	childBuf := &bytes.Buffer{}
	child.w = childBuf
	child.Debug().Msg("child event")
	if got := childBuf.String(); !strings.Contains(got, `"request_id":"rid-1"`) || !strings.Contains(got, `"app":"cwso"`) {
		t.Fatalf("expected inherited and child fields, got %q", got)
	}

	parent.Debug().Msg("parent event")
	if got := bufParent.String(); strings.Contains(got, `"request_id":"rid-1"`) {
		t.Fatalf("parent log unexpectedly contains child field: %q", got)
	}
}

func TestErrNilDoesNotAddErrorField(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{w: buf, level: LevelDebug, base: map[string]any{}}

	l.Error().Err(nil).Msg("no error object")
	out := buf.String()
	if strings.Contains(out, `"error":`) {
		t.Fatalf("unexpected error field for nil error: %q", out)
	}
}

func TestAnySupportsNonStringTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{w: buf, level: LevelDebug, base: map[string]any{}}
	l.Info().Any("payload", map[string]any{"x": 1}).Msg("structured")
	if got := buf.String(); !strings.Contains(got, `"payload":{"x":1}`) {
		t.Fatalf("expected nested payload in output: %q", got)
	}
}

func TestEventFluentChainingReturnsSamePointer(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{w: buf, level: LevelDebug, base: map[string]any{}}
	e := l.Info()
	if e.Str("k", "v") != e {
		t.Fatal("Str should return receiver for chaining")
	}
	if e.Int("n", 1) != e {
		t.Fatal("Int should return receiver for chaining")
	}
	if e.Any("x", true) != e {
		t.Fatal("Any should return receiver for chaining")
	}
	if e.Err(fmt.Errorf("x")) != e {
		t.Fatal("Err should return receiver for chaining")
	}
}
