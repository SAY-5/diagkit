package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAY-5/diagkit/internal/bundle"
	"github.com/SAY-5/diagkit/internal/fingerprint"
	"github.com/SAY-5/diagkit/internal/sim"
)

func testBundle() *bundle.Bundle {
	b := sim.Generate(42, "payments-outage")
	b.Signatures = fingerprint.Cluster(b.Logs, "error")
	b.Normalize()
	return b
}

func TestLoadMissingDirIsEmpty(t *testing.T) {
	idx, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if idx.Version != IndexVersion || len(idx.Incidents) != 0 {
		t.Fatalf("unexpected empty index: %+v", idx)
	}
}

func TestArchiveTwiceYieldsTwoSequentialEntries(t *testing.T) {
	dir := t.TempDir()
	b := testBundle()

	e1, err := Archive(dir, b)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := Archive(dir, b)
	if err != nil {
		t.Fatal(err)
	}
	if e1.ID != "inc-0001" || e2.ID != "inc-0002" {
		t.Fatalf("ids = %q, %q", e1.ID, e2.ID)
	}

	idx, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Incidents) != 2 {
		t.Fatalf("index has %d incidents, want 2", len(idx.Incidents))
	}
	for _, e := range idx.Incidents {
		if e.Scenario != "payments-outage" || e.Seed != 42 || e.Signatures == 0 {
			t.Fatalf("bad entry: %+v", e)
		}
		f, err := os.Open(filepath.Join(dir, e.File))
		if err != nil {
			t.Fatalf("archived file missing: %v", err)
		}
		got, err := bundle.Read(f)
		f.Close()
		if err != nil {
			t.Fatalf("archived bundle unreadable: %v", err)
		}
		if got.Scenario != b.Scenario || len(got.Logs) != len(b.Logs) {
			t.Fatalf("archived bundle differs: %s %d", got.Scenario, len(got.Logs))
		}
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, IndexFile), []byte(`{"version":"99"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for unknown index version")
	}
}
