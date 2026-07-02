// Port of py/diagkit_rca/analyzer.py: rank services by likelihood of being the
// incident root cause. A service scores high when three independent signals
// line up: (1) a dense cluster of recurring error signatures, (2) an error-rate
// and/or p95 latency spike over the window, and (3) other services' errors
// propagating through it. The score is an explainable weighted sum of those
// three normalized signals.

import type { Bundle } from "./types";

// Signal weights, identical to the analyzer. Signature density and the metric
// spike are the strongest evidence; propagation breaks ties toward the true
// upstream culprit.
export const W_SIGNATURE = 0.4;
export const W_SPIKE = 0.35;
export const W_PROPAGATION = 0.25;

export interface RootCause {
  service: string;
  score: number;
  signatureCount: number;
  signatureErrors: number;
  latencySpikeX: number;
  errorRatePeak: number;
  propagationPct: number;
  reasons: string[];
}

function round(v: number, digits: number): number {
  const f = 10 ** digits;
  return Math.round((v + Number.EPSILON) * f) / f;
}

function signatureSignal(b: Bundle): [Record<string, number>, Record<string, number>] {
  const clusters: Record<string, number> = {};
  const errors: Record<string, number> = {};
  for (const sig of b.signatures) {
    for (const svc of sig.services) {
      clusters[svc] = (clusters[svc] ?? 0) + 1;
      errors[svc] = (errors[svc] ?? 0) + sig.count;
    }
  }
  return [clusters, errors];
}

function metricSpikes(b: Bundle): [Record<string, number>, Record<string, number>] {
  const latencyX: Record<string, number> = {};
  const errPeak: Record<string, number> = {};
  for (const m of b.metrics) {
    const lat = m.p95_latency_ms.map((p) => p.value);
    const values = lat.length ? lat : [0.0];
    const positive = values.filter((v) => v > 0);
    const baseline = positive.length ? Math.min(...positive) : 1.0;
    latencyX[m.service] = baseline > 0 ? Math.max(...values) / baseline : 1.0;
    errPeak[m.service] = m.error_rate.length ? Math.max(...m.error_rate.map((p) => p.value)) : 0.0;
  }
  return [latencyX, errPeak];
}

function propagation(b: Bundle): Record<string, number> {
  const entryCandidates = [...new Set(b.traces.filter((s) => !s.called_by).map((s) => s.service))].sort();
  const entry = entryCandidates.length ? entryCandidates[0] : b.services.length ? b.services[0] : "";

  const entryErrorTraces = new Set<string>();
  const svcErrorTraces: Record<string, Set<string>> = {};
  for (const span of b.traces) {
    if (span.error) {
      (svcErrorTraces[span.service] ??= new Set()).add(span.trace_id);
      if (span.service === entry) entryErrorTraces.add(span.trace_id);
    }
  }

  const total = entryErrorTraces.size;
  const pct: Record<string, number> = {};
  for (const svc of b.services) {
    if (total === 0) {
      pct[svc] = 0.0;
    } else {
      const set = svcErrorTraces[svc] ?? new Set();
      let overlap = 0;
      for (const t of set) if (entryErrorTraces.has(t)) overlap++;
      pct[svc] = overlap / total;
    }
  }
  return pct;
}

function normalizeSignal(values: Record<string, number>): Record<string, number> {
  const vals = Object.values(values);
  const hi = vals.length ? Math.max(...vals) : 0.0;
  const out: Record<string, number> = {};
  if (hi <= 0) {
    for (const k of Object.keys(values)) out[k] = 0.0;
    return out;
  }
  for (const [k, v] of Object.entries(values)) out[k] = v / hi;
  return out;
}

function reasons(
  svc: string,
  clusters: Record<string, number>,
  sigErrors: Record<string, number>,
  latencyX: Record<string, number>,
  errPeak: Record<string, number>,
  prop: Record<string, number>
): string[] {
  const out: string[] = [];
  if ((clusters[svc] ?? 0) > 0) {
    out.push(`${clusters[svc]} error signature(s) covering ${sigErrors[svc] ?? 0} log lines`);
  }
  const lx = latencyX[svc] ?? 1.0;
  if (lx >= 1.5) out.push(`p95 latency spike ${lx.toFixed(1)}x baseline`);
  const ep = errPeak[svc] ?? 0.0;
  if (ep >= 0.1) out.push(`error rate peaked at ${Math.round(ep * 100)}%`);
  const p = prop[svc] ?? 0.0;
  if (p >= 0.1) out.push(`${Math.round(p * 100)}% of entry errors trace through it`);
  if (out.length === 0) out.push("no significant failure signal");
  return out;
}

// analyze ranks services by likelihood of being the incident root cause.
export function analyze(b: Bundle): RootCause[] {
  const [clusters, sigErrors] = signatureSignal(b);
  const [latencyX, errPeak] = metricSpikes(b);
  const prop = propagation(b);

  const spikeRaw: Record<string, number> = {};
  for (const svc of b.services) {
    spikeRaw[svc] = (latencyX[svc] ?? 1.0) - 1.0 + (errPeak[svc] ?? 0.0) * 5.0;
  }

  const nSigInput: Record<string, number> = {};
  const nSpikeInput: Record<string, number> = {};
  const nPropInput: Record<string, number> = {};
  for (const svc of b.services) {
    nSigInput[svc] = sigErrors[svc] ?? 0;
    nSpikeInput[svc] = Math.max(0.0, spikeRaw[svc]);
    nPropInput[svc] = prop[svc] ?? 0.0;
  }
  const nSig = normalizeSignal(nSigInput);
  const nSpike = normalizeSignal(nSpikeInput);
  const nProp = normalizeSignal(nPropInput);

  const results: RootCause[] = b.services.map((svc) => {
    const score = W_SIGNATURE * nSig[svc] + W_SPIKE * nSpike[svc] + W_PROPAGATION * nProp[svc];
    return {
      service: svc,
      score: round(score, 4),
      signatureCount: clusters[svc] ?? 0,
      signatureErrors: sigErrors[svc] ?? 0,
      latencySpikeX: round(latencyX[svc] ?? 1.0, 2),
      errorRatePeak: round(errPeak[svc] ?? 0.0, 3),
      propagationPct: round((prop[svc] ?? 0.0) * 100, 1),
      reasons: reasons(svc, clusters, sigErrors, latencyX, errPeak, prop),
    };
  });

  results.sort((a, b2) => {
    if (a.score !== b2.score) return b2.score - a.score;
    if (a.signatureErrors !== b2.signatureErrors) return b2.signatureErrors - a.signatureErrors;
    return a.service < b2.service ? 1 : a.service > b2.service ? -1 : 0;
  });
  return results;
}
