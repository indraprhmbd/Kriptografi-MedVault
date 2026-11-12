package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeCombinesStepsAndMetrics(t *testing.T) {
	a := NewTrace("A")
	a.Add(Step{Name: "first"})
	a.Add(Step{Name: "second"})
	a.Metric("psnr_db", 51.2, "dB")

	b := NewTrace("B")
	b.Add(Step{Name: "third"})
	b.Metric("used_bits", 320, "bits")
	b.Warn("key rotated")

	merged := Merge("PIPELINE", a, b)
	if merged.Module != "PIPELINE" {
		t.Fatalf("module mismatch: %s", merged.Module)
	}
	if len(merged.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(merged.Steps))
	}
	for i, step := range merged.Steps {
		if step.Step != i+1 {
			t.Fatalf("step %d must be renumbered to %d, got %d", i, i+1, step.Step)
		}
	}
	if len(merged.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(merged.Metrics))
	}
	if len(merged.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(merged.Warnings))
	}
}

func TestMergeSkipsNilTraces(t *testing.T) {
	a := NewTrace("A")
	a.Add(Step{Name: "only"})

	merged := Merge("X", nil, a, nil)
	if len(merged.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(merged.Steps))
	}
}

func TestMergeWithNoTracesReturnsEmptyTrace(t *testing.T) {
	merged := Merge("EMPTY")
	if merged == nil {
		t.Fatal("Merge must always return a trace")
	}
	if len(merged.Steps) != 0 {
		t.Fatalf("expected no steps, got %d", len(merged.Steps))
	}
}

func TestTraceJSONUsesUnderscoreKey(t *testing.T) {
	tr := NewTrace("MODULE")
	tr.Add(Step{Name: "step"})
	tr.Finish()

	raw, err := json.Marshal(struct {
		Telemetry *Trace `json:"_telemetry"`
	}{tr})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"_telemetry"`) {
		t.Fatalf("expected the _telemetry key, got %s", raw)
	}
}
