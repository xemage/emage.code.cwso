package dispatch

import (
	"reflect"
	"testing"
)

func TestNormalizeLatencyRequirement(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", LatencyBatch, false},
		{"batch", LatencyBatch, false},
		{"REALTIME", LatencyRealtime, false},
		{" realtime ", LatencyRealtime, false},
		{"soon", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeLatencyRequirement(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("NormalizeLatencyRequirement(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeLatencyRequirement(%q): unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("NormalizeLatencyRequirement(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestProfileTaskRoutingMatrix(t *testing.T) {
	cases := []struct {
		name         string
		desc         string
		ctx          int
		latency      string
		wantTags     []string
		wantClass    string
		wantSeqLabel bool
	}{
		{
			name:      "realtime small context routes to lpu",
			desc:      "fix a missing semicolon",
			ctx:       1000,
			latency:   LatencyRealtime,
			wantTags:  []string{WorkloadRealtime},
			wantClass: HardwareClassLPU,
		},
		{
			name:         "huge context routes to ssm with sequence label",
			desc:         "analyze the entire codebase for dead code",
			ctx:          120000,
			latency:      LatencyBatch,
			wantTags:     []string{WorkloadLongContext},
			wantClass:    HardwareClassSSM,
			wantSeqLabel: true,
		},
		{
			name:         "huge context wins even when realtime requested",
			desc:         "summarize repository",
			ctx:          40000,
			latency:      LatencyRealtime,
			wantTags:     []string{WorkloadLongContext},
			wantClass:    HardwareClassSSM,
			wantSeqLabel: true,
		},
		{
			name:      "batch deterministic edit routes to wasm-local",
			desc:      "Rename the function fooBar to fooBaz",
			ctx:       2000,
			latency:   LatencyBatch,
			wantTags:  []string{WorkloadDeterministicEdit, WorkloadInferenceHeavy},
			wantClass: HardwareClassWasmLocal,
		},
		{
			name:      "general task routes to dense gpu",
			desc:      "Implement a brand new authentication subsystem",
			ctx:       16000,
			latency:   LatencyBatch,
			wantTags:  []string{WorkloadInferenceHeavy},
			wantClass: HardwareClassGPU,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ProfileTask(c.desc, c.ctx, c.latency)
			if !reflect.DeepEqual(got.Tags, c.wantTags) {
				t.Fatalf("tags = %v, want %v", got.Tags, c.wantTags)
			}
			if got.RecommendedClass != c.wantClass {
				t.Fatalf("class = %q, want %q", got.RecommendedClass, c.wantClass)
			}
			hasSeqLabel := len(got.RequestLabels) > 0
			if hasSeqLabel != c.wantSeqLabel {
				t.Fatalf("seq label present = %v, want %v (labels=%v)", hasSeqLabel, c.wantSeqLabel, got.RequestLabels)
			}
		})
	}
}

func TestProfileTaskNegativeContextIsClamped(t *testing.T) {
	got := ProfileTask("rename x", -5, LatencyBatch)
	if got.ContextSizeEstimate != 0 {
		t.Fatalf("expected clamped context 0, got %d", got.ContextSizeEstimate)
	}
}

func TestProfileTaskIsDeterministic(t *testing.T) {
	a := ProfileTask("analyze the entire codebase", 50000, LatencyBatch)
	b := ProfileTask("analyze the entire codebase", 50000, LatencyBatch)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("profile not deterministic: %+v vs %+v", a, b)
	}
}
