import json
from pathlib import Path

from click.testing import CliRunner

from diagkit_rca.analyzer import analyze
from diagkit_rca.bundle import load_bundle
from diagkit_rca.cli import main
from diagkit_rca.report import report_json, report_markdown

FIXTURES = Path(__file__).parent


def test_json_report_is_machine_readable():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    doc = json.loads(report_json(bundle, analyze(bundle), top=4))
    assert doc["root_cause"] == "payments"
    assert doc["scenario"] == "payments-outage"
    ranked = doc["ranked"]
    assert ranked[0]["service"] == "payments"
    assert ranked[0]["reasons"]
    scores = [r["score"] for r in ranked]
    assert scores == sorted(scores, reverse=True)
    assert doc["signatures"][0]["count"] > 0


def test_markdown_report_reads_like_a_ticket():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    md = report_markdown(bundle, analyze(bundle), top=4)
    assert md.startswith("# Incident report: payments-outage")
    assert "**Likely root cause: payments**" in md
    assert "| # | Service | Score | Evidence |" in md
    assert "| Count | Services | Template |" in md


def test_analyze_format_json_via_cli():
    runner = CliRunner()
    result = runner.invoke(
        main, ["analyze", str(FIXTURES / "fixture_payments.json"), "--format", "json"]
    )
    assert result.exit_code == 0, result.output
    assert json.loads(result.output)["root_cause"] == "payments"


def test_analyze_format_markdown_via_cli():
    runner = CliRunner()
    result = runner.invoke(
        main, ["analyze", str(FIXTURES / "fixture_db.json"), "--format", "markdown"]
    )
    assert result.exit_code == 0, result.output
    assert "**Likely root cause: db**" in result.output
