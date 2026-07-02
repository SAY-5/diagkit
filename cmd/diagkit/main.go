// Command diagkit is the collector and fingerprinter half of the support
// diagnostic tool. It runs a seeded, simulated distributed system for an
// incident window and emits a normalized incident-bundle.json that the Python
// root-cause analyzer consumes. It never touches a real network.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SAY-5/diagkit/internal/bundle"
	"github.com/SAY-5/diagkit/internal/fingerprint"
	"github.com/SAY-5/diagkit/internal/history"
	"github.com/SAY-5/diagkit/internal/sim"
)

const usage = `diagkit - support diagnostic collector for a simulated distributed system

usage:
  diagkit collect    [--seed N] [--scenario NAME] [--out FILE] [--baseline]
  diagkit signatures [--seed N] [--scenario NAME] [--top N] [--format text|json]
  diagkit archive <bundle> [--dir DIR]

commands:
  collect      run the simulation and write a normalized incident bundle
  signatures   print the top recurring error log signatures
  archive      copy a bundle into the incident history store and index it

flags:
  --seed N        deterministic seed (default 42)
  --scenario NAME injected fault: payments-outage, db-slowdown, healthy (default payments-outage)
  --out FILE      output path for collect, or - for stdout (default incident-bundle.json)
  --top N         number of signatures to print (default 10)
  --format FMT    signatures output format: text or json (default text)
  --baseline      collect a healthy-window bundle for baseline diffing (out defaults to baseline.json)
  --dir DIR       history directory for archive (default diagkit-history)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "diagkit:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "collect":
		return runCollect(rest)
	case "signatures":
		return runSignatures(rest)
	case "archive":
		return runArchive(rest)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q (try --help)", cmd)
	}
}

// flags holds the parsed common options.
type flags struct {
	seed     int64
	scenario string
	out      string
	outSet   bool
	top      int
	format   string
	baseline bool
}

func parseFlags(args []string) (flags, error) {
	f := flags{seed: 42, scenario: sim.DefaultScenario, out: "incident-bundle.json", top: 10, format: "text"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s needs a value", a)
			}
			i++
			return args[i], nil
		}
		var err error
		switch a {
		case "--seed":
			var v string
			if v, err = next(); err != nil {
				return f, err
			}
			if _, err = fmt.Sscan(v, &f.seed); err != nil {
				return f, fmt.Errorf("invalid --seed %q", v)
			}
		case "--scenario":
			if f.scenario, err = next(); err != nil {
				return f, err
			}
		case "--out":
			if f.out, err = next(); err != nil {
				return f, err
			}
			f.outSet = true
		case "--baseline":
			f.baseline = true
		case "--format":
			if f.format, err = next(); err != nil {
				return f, err
			}
			if f.format != "text" && f.format != "json" {
				return f, fmt.Errorf("invalid --format %q (want text or json)", f.format)
			}
		case "--top":
			var v string
			if v, err = next(); err != nil {
				return f, err
			}
			if _, err = fmt.Sscan(v, &f.top); err != nil {
				return f, fmt.Errorf("invalid --top %q", v)
			}
		default:
			return f, fmt.Errorf("unknown flag %q", a)
		}
	}
	return f, nil
}

func buildBundle(f flags) *bundle.Bundle {
	b := sim.Generate(f.seed, f.scenario)
	b.Signatures = fingerprint.Cluster(b.Logs, "error")
	b.Normalize()
	return b
}

// applyBaseline rewrites collect flags for baseline capture: the healthy
// scenario over the same seed and window, written to baseline.json unless the
// caller chose a path.
func applyBaseline(f flags) flags {
	if !f.baseline {
		return f
	}
	f.scenario = "healthy"
	if !f.outSet {
		f.out = "baseline.json"
	}
	return f
}

func runCollect(args []string) error {
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	f = applyBaseline(f)
	b := buildBundle(f)

	if f.out == "-" {
		return b.Write(os.Stdout)
	}
	file, err := os.Create(f.out)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := b.Write(file); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (scenario=%s seed=%d logs=%d traces=%d signatures=%d)\n",
		f.out, b.Scenario, b.Seed, len(b.Logs), len(b.Traces), len(b.Signatures))
	return nil
}

func runSignatures(args []string) error {
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	b := buildBundle(f)

	n := f.top
	if n > len(b.Signatures) {
		n = len(b.Signatures)
	}
	if f.format == "json" {
		return writeSignaturesJSON(os.Stdout, b.Signatures[:n])
	}
	fmt.Printf("top error signatures (scenario=%s seed=%d)\n", b.Scenario, b.Seed)
	for i := 0; i < n; i++ {
		s := b.Signatures[i]
		fmt.Printf("%3d  %-9s  %s\n", s.Count, joinServices(s.Services), s.Template)
	}
	return nil
}

// writeSignaturesJSON emits the top signature clusters as an indented JSON
// array so other tools can consume them without parsing the text layout.
func writeSignaturesJSON(w io.Writer, sigs []bundle.Signature) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(sigs)
}

// parseArchiveArgs splits the positional bundle path from the --dir option.
func parseArchiveArgs(args []string) (path, dir string, err error) {
	dir = history.DefaultDir
	if len(args) == 0 {
		return "", "", fmt.Errorf("archive needs a bundle path (or - for stdin)")
	}
	path = args[0]
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--dir":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("flag --dir needs a value")
			}
			i++
			dir = rest[i]
		default:
			return "", "", fmt.Errorf("unknown flag %q", rest[i])
		}
	}
	return path, dir, nil
}

func runArchive(args []string) error {
	path, dir, err := parseArchiveArgs(args)
	if err != nil {
		return err
	}

	var r io.Reader = os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	b, err := bundle.Read(r)
	if err != nil {
		return err
	}
	if b.SchemaVersion != bundle.SchemaVersion {
		return fmt.Errorf("unsupported bundle schema %q, expected %q", b.SchemaVersion, bundle.SchemaVersion)
	}

	e, err := history.Archive(dir, b)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "archived %s as %s (scenario=%s seed=%d signatures=%d)\n",
		path, e.ID, e.Scenario, e.Seed, e.Signatures)
	return nil
}

func joinServices(svcs []string) string {
	if len(svcs) == 0 {
		return "-"
	}
	out := svcs[0]
	for _, s := range svcs[1:] {
		out += "," + s
	}
	return out
}
