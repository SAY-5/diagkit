# diagkit

A support diagnostic CLI for distributed services. When an incident fires,
diagkit pulls the logs, traces, and metrics for the time window, clusters the
recurring failure signatures, correlates them with trace errors and metric
spikes, and prints the likely root cause. The goal is to cut time to resolution
by turning a wall of noise into one ranked, explainable answer.

diagkit runs against a **seeded, simulated** distributed system, so the whole
pipeline is reproducible with no real cluster required. Same seed, same
scenario, same answer, every time.

## The two-language design

diagkit is one tool built from two cooperating halves that interoperate through
a single JSON document, the incident bundle.

- **Go** is the collector and fingerprinter (`cmd/diagkit`, `internal/*`). It
  simulates a four service topology (`gateway -> orders -> payments -> db`) for
  an incident window, produces structured logs, distributed traces, and per
  service metrics from a seeded PRNG, and normalizes each log message into a
  template so recurring failures group into signature clusters.

- **Python** is the root-cause analyzer (`py/diagkit_rca`). It consumes the
  bundle, ranks each service with an explainable score built from signature
  density, metric spikes, and dependency propagation, and prints the report.

The bundle schema is defined once on each side and carries a version both halves
check. See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## Quick start

```sh
# build the Go collector
make build

# run the whole pipeline on the injected payments outage
make demo
```

`make demo` runs `diagkit collect` and pipes the bundle straight into the
Python analyzer:

```
incident scenario: payments-outage (seed 42)
window: 1700000000000..1700000600000  services=4 logs=617 traces=960 signatures=4

Likely root cause: payments - 1 signature(s), p95 latency spike 4.2x, error rate 74%, 100% of entry errors trace through it

ranked services:
  1. payments  score=1.000
       - 1 error signature(s) covering 181 log lines
       - p95 latency spike 4.2x baseline
       - error rate peaked at 74%
       - 100% of entry errors trace through it
  2. orders    score=0.655
       - 1 error signature(s) covering 181 log lines
       - 100% of entry errors trace through it
  3. gateway   score=0.655
       - 1 error signature(s) covering 181 log lines
       - 100% of entry errors trace through it
  4. db        score=0.019
       - 1 error signature(s) covering 4 log lines
```

The injected fault is a payments dependency failure. diagkit correctly names
payments as the root cause and explains why: it owns the densest error
signature, its p95 latency spiked 4.2x, its error rate peaked at 74 percent, and
every failing request at the gateway traces through it.

## Usage

```sh
# collect a bundle to a file
diagkit collect --seed 42 --scenario payments-outage --out incident-bundle.json

# inspect the top recurring error signatures (text or json)
diagkit signatures --seed 42 --scenario payments-outage --top 5
diagkit signatures --format json

# analyze a saved bundle
python -m diagkit_rca analyze incident-bundle.json

# export the report for machines or for an incident ticket
python -m diagkit_rca analyze incident-bundle.json --format json
python -m diagkit_rca analyze incident-bundle.json --format markdown

# or run the full pipeline over a pipe
diagkit collect --out - | python -m diagkit_rca analyze -
```

Scenarios: `payments-outage` (default), `db-slowdown`, `healthy`.

## Development

```sh
make test    # go test -race and pytest
make lint    # gofmt check and ruff
make demo    # collect then analyze
```

The Go side uses the standard library only. The Python side uses `click` for the
CLI and `pytest` for tests, managed with `uv`. Both are exercised in CI on every
push against Python 3.11 and 3.12.

## Docker

```sh
docker build -t diagkit .
docker run --rm diagkit
```

The image builds the Go binary in one stage and ships it alongside the Python
analyzer, running the full pipeline by default.

## Releases

- **v2.0.0** report export: `analyze --format json|markdown` for machine-readable
  and ticket-ready reports, `signatures --format json` on the Go side.
- **v1.0.0** initial release: incident collection, signature fingerprinting,
  root-cause ranking.

## License

MIT, Sai Asish Y. See [LICENSE](LICENSE).
