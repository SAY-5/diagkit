from pathlib import Path

from click.testing import CliRunner

from diagkit_rca import analyze, diff_baseline, load_bundle
from diagkit_rca.cli import main

FIXTURES = Path(__file__).parent
NOISE = "config refresh retry <NUM> failed for key <HEX>"


def _load():
    incident = load_bundle(str(FIXTURES / "fixture_payments.json"))
    baseline = load_bundle(str(FIXTURES / "fixture_healthy.json"))
    return incident, baseline


def test_baseline_signature_is_suppressed_not_flagged():
    incident, baseline = _load()
    diff = diff_baseline(incident, baseline)
    assert NOISE in [d.template for d in diff.suppressed]
    flagged = [d.template for d in diff.new + diff.escalated]
    assert NOISE not in flagged


def test_injected_incident_still_tops_with_baseline():
    incident, baseline = _load()
    ranked = analyze(incident, baseline)
    assert ranked[0].service == "payments"
    assert ranked[0].score > ranked[1].score


def test_culprit_signature_is_escalated():
    incident, baseline = _load()
    diff = diff_baseline(incident, baseline)
    escalated = {d.template for d in diff.escalated}
    assert "charge failed for user <NUM> after <DUR> conn=<HEX>" in escalated


def test_metric_deltas_point_at_culprit():
    incident, baseline = _load()
    diff = diff_baseline(incident, baseline)
    assert diff.error_rate_delta["payments"] >= 0.5
    assert diff.latency_delta_x["payments"] >= 3.0
    assert diff.latency_delta_x["orders"] < 1.5


def test_analyze_cli_with_baseline():
    runner = CliRunner()
    result = runner.invoke(
        main,
        [
            "analyze",
            str(FIXTURES / "fixture_payments.json"),
            "--baseline",
            str(FIXTURES / "fixture_healthy.json"),
        ],
    )
    assert result.exit_code == 0, result.output
    assert "Likely root cause: payments" in result.output
    assert "deviation from baseline (healthy):" in result.output
    assert "suppressed" in result.output
    assert NOISE not in result.output
