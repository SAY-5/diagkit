// Package sim generates a deterministic, seeded incident for a small simulated
// distributed system. There is no real cluster and no wall clock in the logic:
// given the same seed and scenario, it always produces identical logs, traces,
// and metrics. The default scenario injects a payments dependency failure that
// cascades upstream to the gateway.
package sim

import (
	"fmt"
	"math/rand/v2"

	"github.com/SAY-5/diagkit/internal/bundle"
)

// The simulated topology. The gateway fronts requests and calls orders, which
// calls payments, which depends on db. Failures in a downstream service surface
// as errors in its callers.
var (
	services = []string{"gateway", "orders", "payments", "db"}
	callers  = map[string]string{
		"orders":   "gateway",
		"payments": "orders",
		"db":       "payments",
	}
)

// Scenario describes an injected fault: the culprit service and how strongly it
// degrades during the incident.
type Scenario struct {
	Name       string
	Culprit    string
	ErrorBoost float64 // added error probability for the culprit during the window
	Latencyx   float64 // p95 latency multiplier for the culprit during the window
}

// Scenarios is the catalog of injectable faults keyed by name.
var Scenarios = map[string]Scenario{
	"payments-outage": {
		Name:       "payments-outage",
		Culprit:    "payments",
		ErrorBoost: 0.72,
		Latencyx:   4.2,
	},
	"db-slowdown": {
		Name:       "db-slowdown",
		Culprit:    "db",
		ErrorBoost: 0.55,
		Latencyx:   6.0,
	},
	// orders starts timing out on its own work; the slowness and errors
	// cascade up to the gateway while payments and db stay healthy below it.
	"cascading-timeout": {
		Name:       "cascading-timeout",
		Culprit:    "orders",
		ErrorBoost: 0.6,
		Latencyx:   5.0,
	},
	// a bad config lands on the gateway itself: requests fail at the edge
	// with nothing wrong anywhere downstream.
	"config-rollout": {
		Name:       "config-rollout",
		Culprit:    "gateway",
		ErrorBoost: 0.5,
		Latencyx:   2.5,
	},
	"healthy": {
		Name:       "healthy",
		Culprit:    "",
		ErrorBoost: 0.0,
		Latencyx:   1.0,
	},
}

// DefaultScenario is used when no scenario is named.
const DefaultScenario = "payments-outage"

// baseWindow is a fixed synthetic window so bundles never depend on wall clock.
const (
	windowStartMs = int64(1_700_000_000_000)
	windowSpanMs  = int64(600_000) // ten minutes
	buckets       = 10             // metric samples per service
	requests      = 240            // simulated requests over the window
	noiseEvery    = 40             // cadence of the scenario-independent noise log
)

// baseLatency is the healthy p95 per service in milliseconds.
var baseLatency = map[string]float64{
	"gateway":  40,
	"orders":   65,
	"payments": 90,
	"db":       25,
}

// Generate builds a full incident bundle for the given seed and scenario name.
// An unknown scenario name falls back to the default.
func Generate(seed int64, scenarioName string) *bundle.Bundle {
	sc, ok := Scenarios[scenarioName]
	if !ok {
		sc = Scenarios[DefaultScenario]
	}

	// A seeded PRNG keeps generation reproducible without touching globals.
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))

	b := &bundle.Bundle{
		SchemaVersion: bundle.SchemaVersion,
		Scenario:      sc.Name,
		Seed:          seed,
		Window:        bundle.Window{StartMs: windowStartMs, EndMs: windowStartMs + windowSpanMs},
		Services:      append([]string(nil), services...),
	}

	genTracesAndLogs(rng, sc, b)
	genMetrics(sc, b)
	return b
}

// errorProb returns the per-request error probability for a service under a
// scenario. The culprit is heavily degraded; everyone else keeps a low baseline.
func errorProb(sc Scenario, service string) float64 {
	base := 0.02
	if service == sc.Culprit {
		return base + sc.ErrorBoost
	}
	return base
}

func genTracesAndLogs(rng *rand.Rand, sc Scenario, b *bundle.Bundle) {
	// order of the call chain from entry to leaf
	chain := []string{"gateway", "orders", "payments", "db"}

	for i := 0; i < requests; i++ {
		traceID := fmt.Sprintf("t-%05d", i)
		ts := windowStartMs + int64(rng.IntN(int(windowSpanMs)))

		// Walk down the chain. A downstream error propagates up as a caller error.
		downstreamFailed := false
		errored := make(map[string]bool)
		for depth := len(chain) - 1; depth >= 0; depth-- {
			svc := chain[depth]
			selfError := rng.Float64() < errorProb(sc, svc)
			// A caller errors if it fails itself or its callee failed.
			isError := selfError || (depth < len(chain)-1 && errored[chain[depth+1]])
			errored[svc] = isError
			if isError {
				downstreamFailed = true
			}
		}

		for depth, svc := range chain {
			span := bundle.Span{
				TraceID:    traceID,
				SpanID:     fmt.Sprintf("%s-%s", traceID, svc),
				Service:    svc,
				Operation:  operationFor(svc),
				DurationMs: spanLatency(rng, sc, svc),
				Error:      errored[svc],
				CalledBy:   callers[svc],
			}
			b.Traces = append(b.Traces, span)

			if errored[svc] {
				b.Logs = append(b.Logs, errorLog(rng, ts, svc, depth == len(chain)-1))
			} else if rng.Float64() < 0.15 {
				b.Logs = append(b.Logs, infoLog(rng, ts, svc))
			}
		}

		// A steady, scenario-independent noise signature: orders retries a
		// flaky config fetch on a fixed cadence in every scenario, healthy or
		// not. Baseline diffing exists to suppress exactly this kind of line.
		if i%noiseEvery == 0 {
			b.Logs = append(b.Logs, bundle.LogEntry{
				Timestamp: ts,
				Service:   "orders",
				Level:     "error",
				Message:   fmt.Sprintf("config refresh retry %d failed for key %08x", i/noiseEvery+1, rng.Uint32()),
			})
		}
		_ = downstreamFailed
	}
}

func operationFor(svc string) string {
	switch svc {
	case "gateway":
		return "handleRequest"
	case "orders":
		return "createOrder"
	case "payments":
		return "charge"
	default:
		return "query"
	}
}

func spanLatency(rng *rand.Rand, sc Scenario, svc string) int {
	base := baseLatency[svc]
	if svc == sc.Culprit {
		base *= sc.Latencyx
	}
	jitter := (rng.Float64()*0.4 - 0.2) * base
	v := base + jitter
	if v < 1 {
		v = 1
	}
	return int(v)
}

// errorLog produces a message with embedded volatile values (ids, durations,
// hex) so the fingerprinter has something real to normalize.
func errorLog(rng *rand.Rand, ts int64, svc string, leaf bool) bundle.LogEntry {
	user := rng.IntN(9000) + 1
	dur := rng.IntN(4000) + 100
	hexID := fmt.Sprintf("%08x", rng.Uint32())
	var msg string
	switch svc {
	case "payments":
		msg = fmt.Sprintf("charge failed for user %d after %dms conn=%s", user, dur, hexID)
	case "db":
		msg = fmt.Sprintf("query timeout after %dms on conn %s", dur, hexID)
	case "orders":
		msg = fmt.Sprintf("createOrder failed for user %d downstream error %s", user, hexID)
	default:
		msg = fmt.Sprintf("request %s failed for user %d after %dms", hexID, user, dur)
	}
	return bundle.LogEntry{Timestamp: ts, Service: svc, Level: "error", Message: msg}
}

func infoLog(rng *rand.Rand, ts int64, svc string) bundle.LogEntry {
	user := rng.IntN(9000) + 1
	dur := rng.IntN(200) + 10
	return bundle.LogEntry{
		Timestamp: ts,
		Service:   svc,
		Level:     "info",
		Message:   fmt.Sprintf("handled request for user %d in %dms", user, dur),
	}
}

func genMetrics(sc Scenario, b *bundle.Bundle) {
	for _, svc := range services {
		sm := bundle.ServiceMetrics{Service: svc}
		for k := 0; k < buckets; k++ {
			ts := windowStartMs + int64(k)*(windowSpanMs/buckets)
			er := 0.02
			lat := baseLatency[svc]
			if svc == sc.Culprit {
				// spike ramps in over the middle of the window
				ramp := rampFactor(k, buckets)
				er = 0.02 + sc.ErrorBoost*ramp
				lat = baseLatency[svc] * (1 + (sc.Latencyx-1)*ramp)
			}
			sm.ErrorRate = append(sm.ErrorRate, bundle.MetricPoint{Timestamp: ts, Value: round3(er)})
			sm.P95LatencyMs = append(sm.P95LatencyMs, bundle.MetricPoint{Timestamp: ts, Value: round3(lat)})
		}
		b.Metrics = append(b.Metrics, sm)
	}
}

// rampFactor rises from 0 to 1 across the window, giving a visible spike shape.
func rampFactor(k, n int) float64 {
	if n <= 1 {
		return 1
	}
	f := float64(k) / float64(n-1)
	if f < 0.3 {
		return f / 0.3 * 0.5
	}
	return f
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}
