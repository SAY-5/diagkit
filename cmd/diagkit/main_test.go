package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/SAY-5/diagkit/internal/bundle"
)

func TestParseFlagsDefaults(t *testing.T) {
	f, err := parseFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.seed != 42 || f.scenario != "payments-outage" || f.out != "incident-bundle.json" || f.top != 10 {
		t.Fatalf("unexpected defaults: %+v", f)
	}
}

func TestParseFlagsOverrides(t *testing.T) {
	f, err := parseFlags([]string{"--seed", "7", "--scenario", "db-slowdown", "--out", "-", "--top", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if f.seed != 7 || f.scenario != "db-slowdown" || f.out != "-" || f.top != 3 {
		t.Fatalf("unexpected parse: %+v", f)
	}
}

func TestParseFlagsErrors(t *testing.T) {
	if _, err := parseFlags([]string{"--seed"}); err == nil {
		t.Fatal("expected error for missing value")
	}
	if _, err := parseFlags([]string{"--nope"}); err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if _, err := parseFlags([]string{"--seed", "xx"}); err == nil {
		t.Fatal("expected error for bad seed")
	}
}

func TestBuildBundleHasSignatures(t *testing.T) {
	f, _ := parseFlags(nil)
	b := buildBundle(f)
	if len(b.Signatures) == 0 {
		t.Fatal("expected signatures after build")
	}
	// A dense signature under a payments outage should implicate payments.
	// Cascade signatures from its callers can carry a few more lines, so look
	// across the top three clusters rather than only the first.
	found := false
	for _, sig := range b.Signatures[:3] {
		for _, s := range sig.Services {
			if s == "payments" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("top signatures %v, expected payments among them", b.Signatures[:3])
	}
}

func TestParseFlagsFormat(t *testing.T) {
	f, err := parseFlags([]string{"--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if f.format != "json" {
		t.Fatalf("format = %q, want json", f.format)
	}
	if _, err := parseFlags([]string{"--format", "yaml"}); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestWriteSignaturesJSON(t *testing.T) {
	f, _ := parseFlags(nil)
	b := buildBundle(f)
	var buf bytes.Buffer
	if err := writeSignaturesJSON(&buf, b.Signatures); err != nil {
		t.Fatal(err)
	}
	var sigs []bundle.Signature
	if err := json.Unmarshal(buf.Bytes(), &sigs); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(sigs) != len(b.Signatures) {
		t.Fatalf("round-tripped %d signatures, want %d", len(sigs), len(b.Signatures))
	}
	if sigs[0].Template != b.Signatures[0].Template || sigs[0].Count != b.Signatures[0].Count {
		t.Fatalf("top signature mismatch: %+v vs %+v", sigs[0], b.Signatures[0])
	}
}

func TestApplyBaseline(t *testing.T) {
	f, err := parseFlags([]string{"--baseline"})
	if err != nil {
		t.Fatal(err)
	}
	f = applyBaseline(f)
	if f.scenario != "healthy" {
		t.Fatalf("baseline scenario = %q, want healthy", f.scenario)
	}
	if f.out != "baseline.json" {
		t.Fatalf("baseline out = %q, want baseline.json", f.out)
	}

	f2, _ := parseFlags([]string{"--baseline", "--out", "custom.json"})
	f2 = applyBaseline(f2)
	if f2.out != "custom.json" {
		t.Fatalf("explicit out overridden: %q", f2.out)
	}

	f3, _ := parseFlags(nil)
	f3 = applyBaseline(f3)
	if f3.scenario != "payments-outage" || f3.out != "incident-bundle.json" {
		t.Fatalf("non-baseline flags changed: %+v", f3)
	}
}
