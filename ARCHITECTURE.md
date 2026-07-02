# Architecture

diagkit is one tool with two cooperating halves written in different languages.
They never talk over a network. They interoperate through a single JSON
document, the incident bundle, so each half can be developed, tested, and run
independently.

```
  +-------------------+        incident-bundle.json        +----------------------+
  |  Go collector     |  ------------------------------->  |  Python analyzer     |
  |  cmd/diagkit       |        (shared schema)             |  py/diagkit_rca      |
  |                    |                                    |                      |
  |  - simulate system |                                    |  - load bundle       |
  |  - logs/traces/    |                                    |  - cluster signatures|
  |    metrics         |                                    |  - correlate traces  |
  |  - fingerprint     |                                    |    and metrics       |
  |    signatures      |                                    |  - rank root causes  |
  +-------------------+                                     +----------------------+
```

## The shared contract

The bundle schema is defined once on each side and kept in lockstep:

- Go: `internal/bundle/bundle.go`
- Python: `py/diagkit_rca/bundle.py`

Both carry a `schema_version` and the analyzer rejects a bundle whose version it
does not understand. The Go writer normalizes ordering so identical inputs
serialize to identical bytes, which is what makes the whole pipeline
reproducible.

## Go side: collector and fingerprinter

`internal/sim` simulates a four service topology: `gateway -> orders ->
payments -> db`. For a fixed synthetic window it produces:

- structured logs (level, service, message, timestamp),
- traces made of spans (service, operation, duration, error flag, caller),
- per service metric series (error rate and p95 latency).

Generation is driven entirely by a seeded PRNG (`math/rand/v2` with an explicit
`PCG` source). There is no global randomness and no wall clock in the logic, so
the same seed and scenario always yield the same bundle. A scenario injects a
fault: `payments-outage` degrades payments, whose errors cascade up through
orders and gateway; `db-slowdown` does the same at the leaf; `cascading-timeout`
degrades orders in the middle of the chain; `config-rollout` fails the gateway
alone at the edge; `healthy` injects nothing.

`internal/fingerprint` normalizes each log message into a template by replacing
volatile tokens (numbers, durations, hex, ids, quoted strings) with
placeholders, then groups identical templates into signature clusters with
counts and the set of services that emitted them. So
`user 4821 timeout after 3003ms` and `user 92 timeout after 511ms` collapse to
one signature.

`cmd/diagkit` exposes `collect` (write the bundle) and `signatures` (print the
top clusters), both accepting `--seed` and `--scenario`.

## Python side: root-cause analyzer

`py/diagkit_rca` loads a bundle and ranks each service with an explainable score
built from three independent signals:

1. **Signature density** - how many recurring error signatures and log lines the
   service owns.
2. **Metric spike** - how far its p95 latency and error rate rise above baseline
   over the window.
3. **Dependency propagation** - the fraction of the entry service's failing
   traces that pass through it, capturing blast radius.

Each signal is normalized across services and combined as a weighted sum. The
top scorer is reported as the likely root cause with a plain-language breakdown,
for example: `payments - 1 signature, p95 latency spike 4.2x, error rate 74%,
100% of entry errors trace through it`.

## Reproducibility

Every artifact is a pure function of `(seed, scenario)`. There is no real
cluster, no network, and no clock in the decision logic, so `make demo` and the
CI pipeline produce the same ranked root cause on every run.
