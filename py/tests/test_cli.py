from pathlib import Path

from click.testing import CliRunner

from diagkit_rca.analyzer import analyze
from diagkit_rca.bundle import load_bundle
from diagkit_rca.cli import format_report, main

FIXTURES = Path(__file__).parent


def test_format_report_names_culprit():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    report = format_report(bundle, analyze(bundle), top=4)
    assert "Likely root cause: payments" in report
    assert "ranked services:" in report
    assert "latency spike" in report


def test_analyze_command_runs():
    runner = CliRunner()
    result = runner.invoke(main, ["analyze", str(FIXTURES / "fixture_payments.json")])
    assert result.exit_code == 0, result.output
    assert "Likely root cause: payments" in result.output


def test_analyze_command_stdin():
    runner = CliRunner()
    data = (FIXTURES / "fixture_db.json").read_text()
    result = runner.invoke(main, ["analyze", "-"], input=data)
    assert result.exit_code == 0, result.output
    assert "Likely root cause: db" in result.output
