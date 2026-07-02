// Faithful TypeScript port of internal/sim/sim.go: a deterministic, seeded
// incident generator for a four-service topology
// (gateway -> orders -> payments -> db). Given the same seed and scenario it
// produces byte-identical logs, traces, and metrics to the Go collector,
// because the PRNG (pcg.ts) and the exact order of draws are reproduced.

import { Rand } from "./pcg";
import { SCENARIOS, DEFAULT_SCENARIO, type Bundle, type Scenario, type LogEntry, type Span } from "./types";

const services = ["gateway", "orders", "payments", "db"];
const callers: Record<string, string> = {
  orders: "gateway",
  payments: "orders",
  db: "payments",
};

const windowStartMs = 1_700_000_000_000;
const windowSpanMs = 600_000; // ten minutes
const buckets = 10;
const requests = 240;

const baseLatency: Record<string, number> = {
  gateway: 40,
  orders: 65,
  payments: 90,
  db: 25,
};

function errorProb(sc: Scenario, service: string): number {
  const base = 0.02;
  if (service === sc.culprit) return base + sc.errorBoost;
  return base;
}

function operationFor(svc: string): string {
  switch (svc) {
    case "gateway":
      return "handleRequest";
    case "orders":
      return "createOrder";
    case "payments":
      return "charge";
    default:
      return "query";
  }
}

function spanLatency(rng: Rand, sc: Scenario, svc: string): number {
  let base = baseLatency[svc];
  if (svc === sc.culprit) base *= sc.latencyx;
  const jitter = (rng.float64() * 0.4 - 0.2) * base;
  let v = base + jitter;
  if (v < 1) v = 1;
  return Math.trunc(v);
}

function errorLog(rng: Rand, ts: number, svc: string, _leaf: boolean): LogEntry {
  const user = rng.intN(9000) + 1;
  const dur = rng.intN(4000) + 100;
  const hexID = rng.uint32().toString(16).padStart(8, "0");
  let msg: string;
  switch (svc) {
    case "payments":
      msg = `charge failed for user ${user} after ${dur}ms conn=${hexID}`;
      break;
    case "db":
      msg = `query timeout after ${dur}ms on conn ${hexID}`;
      break;
    case "orders":
      msg = `createOrder failed for user ${user} downstream error ${hexID}`;
      break;
    default:
      msg = `request ${hexID} failed for user ${user} after ${dur}ms`;
  }
  return { ts_ms: ts, service: svc, level: "error", message: msg };
}

function infoLog(rng: Rand, ts: number, svc: string): LogEntry {
  const user = rng.intN(9000) + 1;
  const dur = rng.intN(200) + 10;
  return { ts_ms: ts, service: svc, level: "info", message: `handled request for user ${user} in ${dur}ms` };
}

function round3(v: number): number {
  return Math.trunc(v * 1000 + 0.5) / 1000;
}

function rampFactor(k: number, n: number): number {
  if (n <= 1) return 1;
  const f = k / (n - 1);
  if (f < 0.3) return (f / 0.3) * 0.5;
  return f;
}

function genTracesAndLogs(rng: Rand, sc: Scenario, b: Bundle): void {
  const chain = ["gateway", "orders", "payments", "db"];
  for (let i = 0; i < requests; i++) {
    const traceID = `t-${String(i).padStart(5, "0")}`;
    const ts = windowStartMs + rng.intN(windowSpanMs);

    const errored: Record<string, boolean> = {};
    for (let depth = chain.length - 1; depth >= 0; depth--) {
      const svc = chain[depth];
      const selfError = rng.float64() < errorProb(sc, svc);
      const isError = selfError || (depth < chain.length - 1 && errored[chain[depth + 1]]);
      errored[svc] = isError;
    }

    for (let depth = 0; depth < chain.length; depth++) {
      const svc = chain[depth];
      const span: Span = {
        trace_id: traceID,
        span_id: `${traceID}-${svc}`,
        service: svc,
        operation: operationFor(svc),
        duration_ms: spanLatency(rng, sc, svc),
        error: errored[svc],
        called_by: callers[svc] ?? "",
      };
      b.traces.push(span);

      if (errored[svc]) {
        b.logs.push(errorLog(rng, ts, svc, depth === chain.length - 1));
      } else if (rng.float64() < 0.15) {
        b.logs.push(infoLog(rng, ts, svc));
      }
    }
  }
}

function genMetrics(sc: Scenario, b: Bundle): void {
  for (const svc of services) {
    const errorRate = [];
    const p95 = [];
    for (let k = 0; k < buckets; k++) {
      const ts = windowStartMs + k * (windowSpanMs / buckets);
      let er = 0.02;
      let lat = baseLatency[svc];
      if (svc === sc.culprit) {
        const ramp = rampFactor(k, buckets);
        er = 0.02 + sc.errorBoost * ramp;
        lat = baseLatency[svc] * (1 + (sc.latencyx - 1) * ramp);
      }
      errorRate.push({ ts_ms: ts, value: round3(er) });
      p95.push({ ts_ms: ts, value: round3(lat) });
    }
    b.metrics.push({ service: svc, error_rate: errorRate, p95_latency_ms: p95 });
  }
}

// generate builds a full incident bundle for the given seed and scenario name.
// An unknown scenario name falls back to the default.
export function generate(seed: number, scenarioName: string): Bundle {
  const sc = SCENARIOS[scenarioName] ?? SCENARIOS[DEFAULT_SCENARIO];
  const rng = new Rand(BigInt(seed));

  const b: Bundle = {
    schema_version: "1",
    scenario: sc.name,
    seed,
    window: { start_ms: windowStartMs, end_ms: windowStartMs + windowSpanMs },
    services: [...services],
    logs: [],
    traces: [],
    metrics: [],
    signatures: [],
  };

  genTracesAndLogs(rng, sc, b);
  genMetrics(sc, b);
  return b;
}

export { windowStartMs, windowSpanMs, buckets };
