package sim

import (
	"bytes"
	"testing"

	"github.com/SAY-5/diagkit/internal/bundle"
)

func serialize(t *testing.T, b *bundle.Bundle) []byte {
	t.Helper()
	b.Normalize()
	var buf bytes.Buffer
	if err := b.Write(&buf); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	return buf.Bytes()
}

func TestGenerateDeterministic(t *testing.T) {
	a := serialize(t, Generate(42, "payments-outage"))
	b := serialize(t, Generate(42, "payments-outage"))
	if !bytes.Equal(a, b) {
		t.Fatal("same seed and scenario produced different bundles")
	}
}

func TestGenerateSeedChangesOutput(t *testing.T) {
	a := serialize(t, Generate(1, "payments-outage"))
	b := serialize(t, Generate(2, "payments-outage"))
	if bytes.Equal(a, b) {
		t.Fatal("different seeds produced identical bundles")
	}
}

func TestUnknownScenarioFallsBack(t *testing.T) {
	b := Generate(7, "does-not-exist")
	if b.Scenario != DefaultScenario {
		t.Fatalf("unknown scenario got %q, want default %q", b.Scenario, DefaultScenario)
	}
}

func TestCulpritIsMostDegraded(t *testing.T) {
	b := Generate(42, "payments-outage")
	errCount := map[string]int{}
	for _, s := range b.Traces {
		if s.Error {
			errCount[s.Service]++
		}
	}
	if errCount["payments"] == 0 {
		t.Fatal("expected payments to have error spans")
	}
	// payments should carry more error spans than db, its healthy dependency.
	if errCount["payments"] <= errCount["db"] {
		t.Fatalf("expected payments (%d) to error more than db (%d)", errCount["payments"], errCount["db"])
	}
}

func TestBundleShape(t *testing.T) {
	b := Generate(42, "payments-outage")
	if b.SchemaVersion != bundle.SchemaVersion {
		t.Fatalf("schema version %q", b.SchemaVersion)
	}
	if len(b.Services) != 4 {
		t.Fatalf("want 4 services, got %d", len(b.Services))
	}
	if len(b.Metrics) != 4 {
		t.Fatalf("want 4 metric series, got %d", len(b.Metrics))
	}
	if len(b.Traces) == 0 || len(b.Logs) == 0 {
		t.Fatal("expected traces and logs")
	}
}
