package fingerprint

import (
	"testing"

	"github.com/SAY-5/diagkit/internal/bundle"
)

func TestNormalizeCollapsesVolatileValues(t *testing.T) {
	a := Normalize("user 4821 timeout after 3003ms")
	b := Normalize("user 92 timeout after 511ms")
	if a != b {
		t.Fatalf("templates differ:\n a=%q\n b=%q", a, b)
	}
	want := "user <NUM> timeout after <DUR>"
	if a != want {
		t.Fatalf("template %q, want %q", a, want)
	}
}

func TestNormalizeHexAndQuotes(t *testing.T) {
	a := Normalize(`charge failed for user 12 conn=deadbeef99 note="retrying"`)
	b := Normalize(`charge failed for user 7 conn=cafef00d11 note="giving up"`)
	if a != b {
		t.Fatalf("hex/quote templates differ:\n a=%q\n b=%q", a, b)
	}
}

func TestClusterGroupsAndCounts(t *testing.T) {
	logs := []bundle.LogEntry{
		{Service: "payments", Level: "error", Message: "charge failed for user 1 after 100ms conn=aabbccdd"},
		{Service: "payments", Level: "error", Message: "charge failed for user 2 after 200ms conn=eeff0011"},
		{Service: "payments", Level: "error", Message: "charge failed for user 3 after 300ms conn=22334455"},
		{Service: "db", Level: "error", Message: "query timeout after 900ms on conn aabbccdd"},
		{Service: "gateway", Level: "info", Message: "handled request for user 5 in 12ms"},
	}
	sigs := Cluster(logs, "error")
	if len(sigs) != 2 {
		t.Fatalf("want 2 error signatures, got %d: %+v", len(sigs), sigs)
	}
	top := sigs[0]
	if top.Count != 3 {
		t.Fatalf("top signature count %d, want 3", top.Count)
	}
	if len(top.Services) != 1 || top.Services[0] != "payments" {
		t.Fatalf("top signature services %v, want [payments]", top.Services)
	}
}

func TestClusterLevelFilter(t *testing.T) {
	logs := []bundle.LogEntry{
		{Service: "gateway", Level: "info", Message: "handled request for user 5 in 12ms"},
		{Service: "payments", Level: "error", Message: "charge failed for user 1 after 100ms"},
	}
	if got := Cluster(logs, "error"); len(got) != 1 {
		t.Fatalf("level filter want 1, got %d", len(got))
	}
	if got := Cluster(logs); len(got) != 2 {
		t.Fatalf("no filter want 2, got %d", len(got))
	}
}
