package main

import "testing"

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
	// The top signature under a payments outage should implicate payments.
	found := false
	for _, s := range b.Signatures[0].Services {
		if s == "payments" {
			found = true
		}
	}
	if !found {
		t.Fatalf("top signature services %v, expected payments", b.Signatures[0].Services)
	}
}
