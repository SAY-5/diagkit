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

# A signature already present in the baseline is only escalated when the
# incident count grows past this multiple; below it, it counts as steady noise.
ESCALATION_X = 3.0


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


@dataclass
class SignatureDeviation:
    template: str
    count: int
    baseline_count: int
    status: str  # "new", "escalated", or "baseline"
    services: list[str] = field(default_factory=list)


@dataclass
class BaselineDiff:
    """How the incident deviates from a healthy-window baseline bundle."""

    baseline_scenario: str
    new: list[SignatureDeviation] = field(default_factory=list)
    escalated: list[SignatureDeviation] = field(default_factory=list)
    suppressed: list[SignatureDeviation] = field(default_factory=list)
    error_rate_delta: dict[str, float] = field(default_factory=dict)
    latency_delta_x: dict[str, float] = field(default_factory=dict)


@dataclass
class Severity:
    """Incident blast radius on a 0..100 scale.

    The score is the product of three shares: how many services are visibly
    failing, how high the error rate peaks, and how much of the window is
    degraded. A quiet window scores zero.
    """

    score: float
    affected_services: int
    total_services: int
    affected_share: float
    peak_error_rate: float
    degraded_share: float


# A service is affected when at least this share of its spans errored, and a
# metric bucket is degraded when some service's error rate reaches it.
SEVERITY_ERROR_FLOOR = 0.1


def severity(bundle: Bundle) -> Severity:
    """Score the incident's blast radius from traces and metrics."""
    spans: dict[str, int] = defaultdict(int)
    errors: dict[str, int] = defaultdict(int)
    for span in bundle.traces:
        spans[span.service] += 1
        if span.error:
            errors[span.service] += 1
    affected = sum(
        1
        for svc in bundle.services
        if spans.get(svc, 0) > 0 and errors.get(svc, 0) / spans[svc] >= SEVERITY_ERROR_FLOOR
    )
    total = len(bundle.services)
    affected_share = affected / total if total else 0.0

    peak = 0.0
    degraded_buckets: set[int] = set()
    buckets = 0
    for m in bundle.metrics:
        buckets = max(buckets, len(m.error_rate))
        for k, p in enumerate(m.error_rate):
            peak = max(peak, p.value)
            if p.value >= SEVERITY_ERROR_FLOOR:
                degraded_buckets.add(k)
    degraded_share = len(degraded_buckets) / buckets if buckets else 0.0

    return Severity(
        score=round(100 * affected_share * peak * degraded_share, 1),
        affected_services=affected,
        total_services=total,
        affected_share=round(affected_share, 3),
        peak_error_rate=round(peak, 3),
        degraded_share=round(degraded_share, 3),
    )


def _baseline_counts(baseline: Bundle) -> dict[str, int]:
    return {sig.template: sig.count for sig in baseline.signatures}


def diff_baseline(bundle: Bundle, baseline: Bundle) -> BaselineDiff:
    """Classify each incident signature against the baseline and compute
    per-service metric deltas.

    A signature absent from the baseline is new. One present but grown past
    ESCALATION_X its baseline count is escalated. Anything else is steady
    noise the baseline suppresses.
    """
    base_counts = _baseline_counts(baseline)
    diff = BaselineDiff(baseline_scenario=baseline.scenario)
    for sig in bundle.signatures:
        base = base_counts.get(sig.template)
        dev = SignatureDeviation(
            template=sig.template,
            count=sig.count,
            baseline_count=base or 0,
            status="new",
            services=sig.services,
        )
        if base is None:
            diff.new.append(dev)
        elif sig.count >= ESCALATION_X * base:
            dev.status = "escalated"
            diff.escalated.append(dev)
        else:
            dev.status = "baseline"
            diff.suppressed.append(dev)

    base_lat, base_err = _metric_peaks(baseline)
    inc_lat, inc_err = _metric_peaks(bundle)
    for svc in bundle.services:
        b_lat = base_lat.get(svc, 0.0)
        diff.latency_delta_x[svc] = round(inc_lat.get(svc, 0.0) / b_lat, 2) if b_lat > 0 else 1.0
        diff.error_rate_delta[svc] = round(inc_err.get(svc, 0.0) - base_err.get(svc, 0.0), 3)
    return diff


def _signature_signal(
    bundle: Bundle, baseline_counts: dict[str, int] | None = None
) -> tuple[dict[str, int], dict[str, int]]:
    """Return per-service signature cluster counts and total matched errors.

    With baseline counts, steady-noise signatures are skipped entirely and
    escalated ones only contribute their count in excess of the baseline.
    """
    clusters: dict[str, int] = defaultdict(int)
    errors: dict[str, int] = defaultdict(int)
    for sig in bundle.signatures:
        excess = sig.count
        if baseline_counts is not None:
            base = baseline_counts.get(sig.template)
            if base is not None and sig.count < ESCALATION_X * base:
                continue
            excess = sig.count - (base or 0)
        for svc in sig.services:
            clusters[svc] += 1
            errors[svc] += excess
    return clusters, errors


def _metric_peaks(bundle: Bundle) -> tuple[dict[str, float], dict[str, float]]:
    """Return per-service peak p95 latency and peak error rate."""
    lat_peak: dict[str, float] = {}
    err_peak: dict[str, float] = {}
    for m in bundle.metrics:
        lat_peak[m.service] = max((p.value for p in m.p95_latency_ms), default=0.0)
        err_peak[m.service] = max((p.value for p in m.error_rate), default=0.0)
    return lat_peak, err_peak


def _metric_spikes(
    bundle: Bundle, baseline: Bundle | None = None
) -> tuple[dict[str, float], dict[str, float]]:
    """Return per-service latency spike multiplier and peak error rate.

    Without a baseline the latency reference is the quietest in-window sample;
    with one, spikes are measured against the baseline peaks so recurring load
    patterns do not register as deviations.
    """
    if baseline is not None:
        base_lat, base_err = _metric_peaks(baseline)
        inc_lat, inc_err = _metric_peaks(bundle)
        latency_x = {
            svc: (inc_lat.get(svc, 0.0) / base_lat[svc]) if base_lat.get(svc, 0.0) > 0 else 1.0
            for svc in bundle.services
        }
        err_delta = {
            svc: max(0.0, inc_err.get(svc, 0.0) - base_err.get(svc, 0.0))
            for svc in bundle.services
        }
        return latency_x, err_delta

    latency_x: dict[str, float] = {}
    err_peak: dict[str, float] = {}
    for m in bundle.metrics:
        lat = [p.value for p in m.p95_latency_ms] or [0.0]
        floor = min(v for v in lat if v > 0) if any(v > 0 for v in lat) else 1.0
        latency_x[m.service] = (max(lat) / floor) if floor > 0 else 1.0
        err_peak[m.service] = max((p.value for p in m.error_rate), default=0.0)
    return latency_x, err_peak


def _propagation(bundle: Bundle) -> dict[str, float]:
    """Fraction of the entry service's error traces that pass through each service.

    The entry service is the one whose own spans have no caller (it sits at the
    top of the call chain). A downstream service with high coverage of the entry
    service's failing traces is a strong root-cause candidate.
    """
    entry_candidates = sorted(
        {s.service for s in bundle.traces if not s.called_by}
    )
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


def analyze(bundle: Bundle, baseline: Bundle | None = None) -> list[RootCause]:
    """Rank services by likelihood of being the incident root cause.

    With a baseline bundle, signatures the baseline already carries are
    suppressed and metric spikes are measured against the baseline peaks, so
    recurring noise does not pollute the ranking.
    """
    baseline_counts = _baseline_counts(baseline) if baseline is not None else None
    clusters, sig_errors = _signature_signal(bundle, baseline_counts)
    latency_x, err_peak = _metric_spikes(bundle, baseline)
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
