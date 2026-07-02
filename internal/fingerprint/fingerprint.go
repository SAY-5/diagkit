// Package fingerprint turns free-form log messages into stable templates by
// replacing volatile tokens (numbers, ids, hex, quoted values, durations) with
// placeholders, then groups identical templates into signature clusters. Two
// messages that differ only in their variable parts collapse to one signature.
package fingerprint

import (
	"regexp"
	"sort"
	"strings"

	"github.com/SAY-5/diagkit/internal/bundle"
)

// Ordering matters: more specific patterns run before the bare-number pattern
// so, for example, a duration like "3003ms" becomes <DUR> rather than <NUM>ms.
var replacers = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`"[^"]*"`), "<STR>"},                         // quoted strings
	{regexp.MustCompile(`'[^']*'`), "<STR>"},                         // single-quoted
	{regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`), "<HEX>"},              // 0x hex
	{regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`), "<HEX>"},             // bare long hex/ids
	{regexp.MustCompile(`\b\d+(\.\d+)?ms\b`), "<DUR>"},               // durations
	{regexp.MustCompile(`\b\d+(\.\d+)?s\b`), "<DUR>"},                // second durations
	{regexp.MustCompile(`\b\d+(\.\d+)?\b`), "<NUM>"},                 // remaining numbers
	{regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f-]{20,}\b`), "<UUID>"}, // uuids
}

// Normalize reduces a single log message to its template form.
func Normalize(msg string) string {
	out := msg
	// uuids first (they contain hyphens the hex rule would miss)
	out = replacers[len(replacers)-1].re.ReplaceAllString(out, replacers[len(replacers)-1].with)
	for _, r := range replacers[:len(replacers)-1] {
		out = r.re.ReplaceAllString(out, r.with)
	}
	return strings.Join(strings.Fields(out), " ")
}

// Cluster groups log entries into signature clusters keyed by their template.
// Only the given levels are considered; passing nil clusters every entry.
func Cluster(logs []bundle.LogEntry, levels ...string) []bundle.Signature {
	want := map[string]bool{}
	for _, l := range levels {
		want[l] = true
	}

	type agg struct {
		count    int
		services map[string]bool
		example  string
	}
	groups := map[string]*agg{}

	for _, e := range logs {
		if len(want) > 0 && !want[e.Level] {
			continue
		}
		tmpl := Normalize(e.Message)
		g := groups[tmpl]
		if g == nil {
			g = &agg{services: map[string]bool{}, example: e.Message}
			groups[tmpl] = g
		}
		g.count++
		g.services[e.Service] = true
	}

	sigs := make([]bundle.Signature, 0, len(groups))
	for tmpl, g := range groups {
		svcs := make([]string, 0, len(g.services))
		for s := range g.services {
			svcs = append(svcs, s)
		}
		sort.Strings(svcs)
		sigs = append(sigs, bundle.Signature{
			Template: tmpl,
			Count:    g.count,
			Services: svcs,
			Example:  g.example,
		})
	}

	sort.SliceStable(sigs, func(i, j int) bool {
		if sigs[i].Count != sigs[j].Count {
			return sigs[i].Count > sigs[j].Count
		}
		return sigs[i].Template < sigs[j].Template
	})
	return sigs
}
