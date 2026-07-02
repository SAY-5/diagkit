// The full diagkit pipeline in the browser, mirroring cmd/diagkit buildBundle
// plus the Python analyzer: generate a seeded incident, cluster error-log
// signatures, normalize ordering, then rank the root cause.

import { generate } from "./generate";
import { cluster } from "./fingerprint";
import { analyze, type RootCause } from "./analyzer";
import type { Bundle } from "./types";

// normalizeBundle sorts collections into the deterministic order defined by
// Bundle.Normalize in internal/bundle/bundle.go.
function normalizeBundle(b: Bundle): void {
  b.services.sort();
  b.logs.sort((x, y) => {
    if (x.ts_ms !== y.ts_ms) return x.ts_ms - y.ts_ms;
    if (x.service !== y.service) return x.service < y.service ? -1 : 1;
    return x.message < y.message ? -1 : x.message > y.message ? 1 : 0;
  });
  b.traces.sort((x, y) => {
    if (x.trace_id !== y.trace_id) return x.trace_id < y.trace_id ? -1 : 1;
    return x.span_id < y.span_id ? -1 : x.span_id > y.span_id ? 1 : 0;
  });
  b.metrics.sort((x, y) => (x.service < y.service ? -1 : x.service > y.service ? 1 : 0));
  b.signatures.sort((x, y) => {
    if (x.count !== y.count) return y.count - x.count;
    return x.template < y.template ? -1 : x.template > y.template ? 1 : 0;
  });
}

export interface Diagnosis {
  bundle: Bundle;
  ranking: RootCause[];
}

// buildBundle runs the collector half: seeded simulation, error-signature
// clustering, and deterministic normalization.
export function buildBundle(seed: number, scenario: string): Bundle {
  const b = generate(seed, scenario);
  b.signatures = cluster(b.logs, ["error"]);
  normalizeBundle(b);
  return b;
}

// diagnose runs the whole pipeline end to end and returns the bundle plus the
// ranked root causes.
export function diagnose(seed: number, scenario: string): Diagnosis {
  const bundle = buildBundle(seed, scenario);
  const ranking = analyze(bundle);
  return { bundle, ranking };
}

// selfCheck logs the default-seed diagnosis so it is easy to confirm in the
// browser console that payments is the top root cause with the expected
// evidence.
export function selfCheck(): void {
  const { bundle, ranking } = diagnose(42, "payments-outage");
  const top = ranking[0];
  // eslint-disable-next-line no-console
  console.info(
    `[diagkit] scenario=payments-outage seed=42 logs=${bundle.logs.length} traces=${bundle.traces.length} signatures=${bundle.signatures.length}`
  );
  // eslint-disable-next-line no-console
  console.info(
    `[diagkit] top root cause: ${top.service} score=${top.score.toFixed(3)} latency=${top.latencySpikeX}x error=${Math.round(
      top.errorRatePeak * 100
    )}% propagation=${top.propagationPct}%`
  );
  if (top.service !== "payments") {
    // eslint-disable-next-line no-console
    console.warn("[diagkit] expected payments as top root cause for the default seed");
  }
}
