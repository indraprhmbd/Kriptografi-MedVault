// Package telemetry defines the glass box trace structure returned by every
// MedVault operation.
//
// The same Trace value is embedded in the JSON response envelope under the
// "_telemetry" key so the browser can render the intermediate cryptographic
// states without the backend having to know anything about the UI.
package telemetry

import (
	"sync/atomic"
	"time"
)

// Step is one observable stage of an operation. Fields are omitted when empty
// so the payload stays small and the UI can render a compact card.
type Step struct {
	Step          int    `json:"step"`
	Name          string `json:"name"`
	Algorithm     string `json:"algorithm,omitempty"`
	InputPreview  string `json:"input_preview,omitempty"`
	OutputPreview string `json:"output_preview,omitempty"`
	Detail        string `json:"detail,omitempty"`
	InfoTag       string `json:"info_tag,omitempty"`
	Fingerprint   string `json:"derived_key_fingerprint,omitempty"`
	NonceHex      string `json:"nonce_hex,omitempty"`
	TagHex        string `json:"poly1305_tag_hex,omitempty"`
	StorageB64    string `json:"final_storage_b64,omitempty"`
	Verified      *bool  `json:"verified,omitempty"`
}

// Metric is a labelled numeric measurement such as PSNR or bit capacity.
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// Trace is the container embedded in every API response.
type Trace struct {
	Module     string   `json:"module"`
	DurationUS int64    `json:"duration_us"`
	Steps      []Step   `json:"steps"`
	Metrics    []Metric `json:"metrics,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`

	start atomic.Int64
}

// NewTrace starts a trace and records the start time.
func NewTrace(module string) *Trace {
	t := &Trace{Module: module, Steps: make([]Step, 0, 6)}
	t.start.Store(time.Now().UnixNano())
	return t
}

// Add appends a step and auto numbers it.
func (t *Trace) Add(s Step) {
	s.Step = len(t.Steps) + 1
	t.Steps = append(t.Steps, s)
}

// Metric records a numeric measurement.
func (t *Trace) Metric(name string, value float64, unit string) {
	t.Metrics = append(t.Metrics, Metric{Name: name, Value: value, Unit: unit})
}

// Warn records a non fatal condition worth surfacing in the inspector.
func (t *Trace) Warn(msg string) {
	t.Warnings = append(t.Warnings, msg)
}

// Finish stamps the elapsed duration in microseconds.
func (t *Trace) Finish() *Trace {
	if t == nil {
		return nil
	}
	t.DurationUS = (time.Now().UnixNano() - t.start.Load()) / 1000
	return t
}

// Preview truncates a value for display, keeping the payload bounded.
func Preview(s string, max int) string {
	if max <= 0 {
		max = 120
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Merge folds several traces into one envelope payload.
//
// Each stage of a multi step operation builds its own trace with its own
// duration; merging keeps the inspector showing one linear story instead of a
// nest of unrelated payloads. Step numbers are rewritten so the sequence is
// always contiguous.
func Merge(module string, traces ...*Trace) *Trace {
	merged := &Trace{Module: module, Steps: make([]Step, 0, 8)}
	started := time.Now()

	for _, tr := range traces {
		if tr == nil {
			continue
		}
		for _, step := range tr.Steps {
			step.Step = len(merged.Steps) + 1
			merged.Steps = append(merged.Steps, step)
		}
		merged.Metrics = append(merged.Metrics, tr.Metrics...)
		merged.Warnings = append(merged.Warnings, tr.Warnings...)
		merged.DurationUS += tr.DurationUS
	}

	if merged.DurationUS == 0 {
		merged.DurationUS = time.Since(started).Microseconds()
	}
	return merged
}
