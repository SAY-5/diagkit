"""Root-cause ranking.

A service scores high as a likely root cause when three independent signals
line up:

1. it owns a dense cluster of recurring error signatures,
2. its metrics show an error-rate and/or p95 latency spike over the window, and
3. errors in other services propagate through it (dependency blast radius).

The score is an explainable weighted sum of those three normalized signals, so
every ranking can be justified in plain language.
"""

from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field

from .bundle import Bundle

# Signal weights. Signature density and the metric spike are the strongest
# evidence; propagation breaks ties toward the true upstream culprit.
W_SIGNATURE = 0.4
W_SPIKE = 0.35
W_PROPAGATION = 0.25


@dataclass
class RootCause:
    service: str
    score: float
    signature_count: int
    signature_errors: int
    latency_spike_x: float
    error_rate_peak: float
    propagation_pct: float
    reasons: list[str] = field(default_factory=list)


def _signature_signal(bundle: Bundle) -> tuple[dict[str, int], dict[str, int]]:
    """Return per-service signature cluster counts and total matched errors."""
    clusters: dict[str, int] = defaultdict(int)
    errors: dict[str, int] = defaultdict(int)
    for sig in bundle.signatures:
        for svc in sig.services:
            clusters[svc] += 1
            errors[svc] += sig.count
    return clusters, errors


def _metric_spikes(bundle: Bundle) -> tuple[dict[str, float], dict[str, float]]:
    """Return per-service latency spike multiplier and peak error rate."""
    latency_x: dict[str, float] = {}
    err_peak: dict[str, float] = {}
    for m in bundle.metrics:
        lat = [p.value for p in m.p95_latency_ms] or [0.0]
        baseline = min(v for v in lat if v > 0) if any(v > 0 for v in lat) else 1.0
        latency_x[m.service] = (max(lat) / baseline) if baseline > 0 else 1.0
        err_peak[m.service] = max((p.value for p in m.error_rate), default=0.0)
    return latency_x, err_peak


def _propagation(bundle: Bundle) -> dict[str, float]:
    """Fraction of the entry service's error traces that pass through each service.

    The entry service is the one that is never called by anyone. A downstream
    service with high coverage of the entry service's failing traces is a strong
    root-cause candidate.
    """
    called_by = {s.called_by for s in bundle.traces}
    entry_candidates = [s for s in bundle.services if s not in called_by]
    if entry_candidates:
        entry = entry_candidates[0]
    else:
        entry = bundle.services[0] if bundle.services else ""

    entry_error_traces: set[str] = set()
    svc_error_traces: dict[str, set[str]] = defaultdict(set)
    for span in bundle.traces:
        if span.error:
            svc_error_traces[span.service].add(span.trace_id)
            if span.service == entry:
                entry_error_traces.add(span.trace_id)

    total = len(entry_error_traces)
    pct: dict[str, float] = {}
    for svc in bundle.services:
        if total == 0:
            pct[svc] = 0.0
        else:
            overlap = len(svc_error_traces[svc] & entry_error_traces)
            pct[svc] = overlap / total
    return pct


def _normalize(values: dict[str, float]) -> dict[str, float]:
    hi = max(values.values(), default=0.0)
    if hi <= 0:
        return {k: 0.0 for k in values}
    return {k: v / hi for k, v in values.items()}


def analyze(bundle: Bundle) -> list[RootCause]:
    """Rank services by likelihood of being the incident root cause."""
    clusters, sig_errors = _signature_signal(bundle)
    latency_x, err_peak = _metric_spikes(bundle)
    prop = _propagation(bundle)

    # Combine latency and error-rate into one spike signal per service.
    spike_raw = {
        svc: (latency_x.get(svc, 1.0) - 1.0) + err_peak.get(svc, 0.0) * 5.0
        for svc in bundle.services
    }

    n_sig = _normalize({svc: float(sig_errors.get(svc, 0)) for svc in bundle.services})
    n_spike = _normalize({svc: max(0.0, v) for svc, v in spike_raw.items()})
    n_prop = _normalize({svc: prop.get(svc, 0.0) for svc in bundle.services})

    results: list[RootCause] = []
    for svc in bundle.services:
        score = (
            W_SIGNATURE * n_sig[svc] + W_SPIKE * n_spike[svc] + W_PROPAGATION * n_prop[svc]
        )
        rc = RootCause(
            service=svc,
            score=round(score, 4),
            signature_count=clusters.get(svc, 0),
            signature_errors=sig_errors.get(svc, 0),
            latency_spike_x=round(latency_x.get(svc, 1.0), 2),
            error_rate_peak=round(err_peak.get(svc, 0.0), 3),
            propagation_pct=round(prop.get(svc, 0.0) * 100, 1),
            reasons=_reasons(svc, clusters, sig_errors, latency_x, err_peak, prop),
        )
        results.append(rc)

    results.sort(key=lambda r: (r.score, r.signature_errors, r.service), reverse=True)
    return results


def _reasons(
    svc: str,
    clusters: dict[str, int],
    sig_errors: dict[str, int],
    latency_x: dict[str, float],
    err_peak: dict[str, float],
    prop: dict[str, float],
) -> list[str]:
    out: list[str] = []
    if clusters.get(svc, 0) > 0:
        out.append(
            f"{clusters[svc]} error signature(s) covering {sig_errors.get(svc, 0)} log lines"
        )
    lx = latency_x.get(svc, 1.0)
    if lx >= 1.5:
        out.append(f"p95 latency spike {lx:.1f}x baseline")
    ep = err_peak.get(svc, 0.0)
    if ep >= 0.1:
        out.append(f"error rate peaked at {ep * 100:.0f}%")
    p = prop.get(svc, 0.0)
    if p >= 0.1:
        out.append(f"{p * 100:.0f}% of entry errors trace through it")
    if not out:
        out.append("no significant failure signal")
    return out
