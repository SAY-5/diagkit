// Package bundle defines the incident-bundle JSON contract shared between the
// Go collector and the Python root-cause analyzer. The schema is the single
// interoperation point between the two languages: the Go side writes it, the
// Python side reads it.
package bundle

import (
	"encoding/json"
	"io"
	"sort"
)

// SchemaVersion identifies the bundle contract. Both languages check it.
const SchemaVersion = "1"

// LogEntry is one structured log line emitted by a simulated service.
type LogEntry struct {
	Timestamp int64  `json:"ts_ms"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// Span is one unit of work in a distributed trace. A span may record an error
// and names the upstream service it called (empty for the entry span).
type Span struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	Service    string `json:"service"`
	Operation  string `json:"operation"`
	DurationMs int    `json:"duration_ms"`
	Error      bool   `json:"error"`
	CalledBy   string `json:"called_by"`
}

// MetricPoint is a single sample of a per-service time series.
type MetricPoint struct {
	Timestamp int64   `json:"ts_ms"`
	Value     float64 `json:"value"`
}

// ServiceMetrics holds the metric series collected for one service over the
// incident window.
type ServiceMetrics struct {
	Service      string        `json:"service"`
	ErrorRate    []MetricPoint `json:"error_rate"`
	P95LatencyMs []MetricPoint `json:"p95_latency_ms"`
}

// Signature is a recurring log template with the number of matching log lines.
type Signature struct {
	Template string   `json:"template"`
	Count    int      `json:"count"`
	Services []string `json:"services"`
	Example  string   `json:"example"`
}

// Window is the incident time range in epoch milliseconds.
type Window struct {
	StartMs int64 `json:"start_ms"`
	EndMs   int64 `json:"end_ms"`
}

// Bundle is the full normalized incident record.
type Bundle struct {
	SchemaVersion string           `json:"schema_version"`
	Scenario      string           `json:"scenario"`
	Seed          int64            `json:"seed"`
	Window        Window           `json:"window"`
	Services      []string         `json:"services"`
	Logs          []LogEntry       `json:"logs"`
	Traces        []Span           `json:"traces"`
	Metrics       []ServiceMetrics `json:"metrics"`
	Signatures    []Signature      `json:"signatures"`
}

// Normalize sorts the collections into a deterministic order so that equal
// inputs always serialize to identical bytes.
func (b *Bundle) Normalize() {
	sort.Strings(b.Services)
	sort.SliceStable(b.Logs, func(i, j int) bool {
		if b.Logs[i].Timestamp != b.Logs[j].Timestamp {
			return b.Logs[i].Timestamp < b.Logs[j].Timestamp
		}
		if b.Logs[i].Service != b.Logs[j].Service {
			return b.Logs[i].Service < b.Logs[j].Service
		}
		return b.Logs[i].Message < b.Logs[j].Message
	})
	sort.SliceStable(b.Traces, func(i, j int) bool {
		if b.Traces[i].TraceID != b.Traces[j].TraceID {
			return b.Traces[i].TraceID < b.Traces[j].TraceID
		}
		return b.Traces[i].SpanID < b.Traces[j].SpanID
	})
	sort.SliceStable(b.Metrics, func(i, j int) bool {
		return b.Metrics[i].Service < b.Metrics[j].Service
	})
	sort.SliceStable(b.Signatures, func(i, j int) bool {
		if b.Signatures[i].Count != b.Signatures[j].Count {
			return b.Signatures[i].Count > b.Signatures[j].Count
		}
		return b.Signatures[i].Template < b.Signatures[j].Template
	})
}

// Write serializes the bundle as indented JSON.
func (b *Bundle) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

// Read decodes a bundle from JSON.
func Read(r io.Reader) (*Bundle, error) {
	var b Bundle
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}
