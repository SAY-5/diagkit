// Command diagkit is the collector and fingerprinter half of the support
// diagnostic tool. It runs a seeded, simulated distributed system for an
// incident window and emits a normalized incident-bundle.json that the Python
// root-cause analyzer consumes. It never touches a real network.
package main

import (
	"fmt"
	"os"

	"github.com/SAY-5/diagkit/internal/bundle"
	"github.com/SAY-5/diagkit/internal/fingerprint"
	"github.com/SAY-5/diagkit/internal/sim"
)

const usage = `diagkit - support diagnostic collector for a simulated distributed system

usage:
  diagkit collect    [--seed N] [--scenario NAME] [--out FILE]
  diagkit signatures [--seed N] [--scenario NAME] [--top N]

commands:
  collect      run the simulation and write a normalized incident bundle
  signatures   print the top recurring error log signatures

flags:
  --seed N        deterministic seed (default 42)
  --scenario NAME injected fault: payments-outage, db-slowdown, healthy (default payments-outage)
  --out FILE      output path for collect, or - for stdout (default incident-bundle.json)
  --top N         number of signatures to print (default 10)
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
	top      int
}

func parseFlags(args []string) (flags, error) {
	f := flags{seed: 42, scenario: sim.DefaultScenario, out: "incident-bundle.json", top: 10}
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

func runCollect(args []string) error {
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
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

	fmt.Printf("top error signatures (scenario=%s seed=%d)\n", b.Scenario, b.Seed)
	n := f.top
	if n > len(b.Signatures) {
		n = len(b.Signatures)
	}
	for i := 0; i < n; i++ {
		s := b.Signatures[i]
		fmt.Printf("%3d  %-9s  %s\n", s.Count, joinServices(s.Services), s.Template)
	}
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
