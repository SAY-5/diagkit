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

func TestNoiseSignaturePresentInEveryScenario(t *testing.T) {
	counts := map[string]int{}
	for name := range Scenarios {
		b := Generate(42, name)
		n := 0
		for _, l := range b.Logs {
			if l.Level == "error" && l.Service == "orders" &&
				len(l.Message) > 14 && l.Message[:14] == "config refresh" {
				n++
			}
		}
		if n == 0 {
			t.Fatalf("scenario %s has no config-refresh noise logs", name)
		}
		counts[name] = n
	}
	if counts["healthy"] != counts["payments-outage"] {
		t.Fatalf("noise volume differs across scenarios: %v", counts)
	}
}

// errorCounts tallies error spans per service for a generated scenario.
func errorCounts(seed int64, scenario string) map[string]int {
	b := Generate(seed, scenario)
	out := map[string]int{}
	for _, s := range b.Traces {
		if s.Error {
			out[s.Service]++
		}
	}
	return out
}

func TestCascadingTimeoutCulpritIsOrders(t *testing.T) {
	ec := errorCounts(42, "cascading-timeout")
	// orders fails on its own; payments and db below it stay near baseline.
	if ec["orders"] <= 5*ec["payments"] || ec["orders"] <= 5*ec["db"] {
		t.Fatalf("orders (%d) should dominate payments (%d) and db (%d)", ec["orders"], ec["payments"], ec["db"])
	}
	// the failure cascades to the gateway above it
	if ec["gateway"] < ec["orders"] {
		t.Fatalf("gateway (%d) should inherit orders errors (%d)", ec["gateway"], ec["orders"])
	}
}

func TestConfigRolloutCulpritIsGateway(t *testing.T) {
	ec := errorCounts(42, "config-rollout")
	// the edge fails alone: nothing below it is degraded
	for _, svc := range []string{"orders", "payments", "db"} {
		if ec["gateway"] <= 5*ec[svc] {
			t.Fatalf("gateway (%d) should dominate %s (%d)", ec["gateway"], svc, ec[svc])
		}
	}
}

func TestNewScenariosAreDistinct(t *testing.T) {
	a := serialize(t, Generate(42, "cascading-timeout"))
	b := serialize(t, Generate(42, "config-rollout"))
	if bytes.Equal(a, b) {
		t.Fatal("cascading-timeout and config-rollout produced identical bundles")
	}
}
