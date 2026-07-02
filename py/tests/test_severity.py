from pathlib import Path

from click.testing import CliRunner

from diagkit_rca import analyze, load_bundle, severity
from diagkit_rca.cli import main

FIXTURES = Path(__file__).parent


def test_healthy_window_scores_zero():
    sev = severity(load_bundle(str(FIXTURES / "fixture_healthy.json")))
    assert sev.score == 0.0
    assert sev.affected_services == 0


def test_payments_outage_severity_reflects_blast_radius():
    sev = severity(load_bundle(str(FIXTURES / "fixture_payments.json")))
    assert sev.score > 0
    # payments plus the two callers it drags down, but not db
    assert sev.affected_services == 3
    assert sev.total_services == 4
    assert sev.peak_error_rate >= 0.5
    assert 0 < sev.degraded_share <= 1


def test_edge_incident_has_smaller_blast_radius():
    edge = severity(load_bundle(str(FIXTURES / "fixture_rollout.json")))
    cascade = severity(load_bundle(str(FIXTURES / "fixture_db.json")))
    # a gateway-only failure touches one service; a db failure drags down all four
    assert edge.affected_services == 1
    assert cascade.affected_services == 4
    assert edge.score < cascade.score


def test_severity_in_analyze_output():
    runner = CliRunner()
    result = runner.invoke(main, ["analyze", str(FIXTURES / "fixture_payments.json")])
    assert result.exit_code == 0, result.output
    assert "severity:" in result.output
    assert "/100" in result.output


def test_severity_in_history_output(tmp_path):
    from tests.test_history import make_store

    store = make_store(tmp_path, ["fixture_payments.json"])
    runner = CliRunner()
    result = runner.invoke(main, ["history", "--dir", str(store)])
    assert result.exit_code == 0, result.output
    assert "severity=" in result.output


def test_cascading_timeout_names_orders():
    ranked = analyze(load_bundle(str(FIXTURES / "fixture_cascade.json")))
    assert ranked[0].service == "orders"


def test_config_rollout_names_gateway():
    ranked = analyze(load_bundle(str(FIXTURES / "fixture_rollout.json")))
    assert ranked[0].service == "gateway"
