import json
import shutil
from pathlib import Path

from click.testing import CliRunner

from diagkit_rca.cli import main
from diagkit_rca.history import find_recurrences, load_index

FIXTURES = Path(__file__).parent


def make_store(tmp_path: Path, fixtures: list[str]) -> Path:
    """Build a history store the way the Go archive command lays it out."""
    store = tmp_path / "diagkit-history"
    store.mkdir()
    incidents = []
    for i, name in enumerate(fixtures, start=1):
        inc_id = f"inc-{i:04d}"
        shutil.copy(FIXTURES / name, store / f"{inc_id}.json")
        data = json.loads((FIXTURES / name).read_text())
        incidents.append(
            {
                "id": inc_id,
                "file": f"{inc_id}.json",
                "scenario": data["scenario"],
                "seed": data["seed"],
                "window": data["window"],
                "logs": len(data["logs"]),
                "traces": len(data["traces"]),
                "signatures": len(data["signatures"]),
            }
        )
    (store / "index.json").write_text(json.dumps({"version": "1", "incidents": incidents}))
    return store


def test_empty_store_is_empty_history(tmp_path):
    assert load_index(str(tmp_path / "missing")) == []


def test_archiving_twice_yields_a_recurrence(tmp_path):
    store = make_store(tmp_path, ["fixture_payments.json", "fixture_payments.json"])
    recs = find_recurrences(str(store))
    assert recs, "expected at least one recurring signature"
    templates = [r.template for r in recs]
    assert "charge failed for user <NUM> after <DUR> conn=<HEX>" in templates
    top = recs[0]
    assert len(top.incidents) == 2
    assert top.incidents == ["inc-0001", "inc-0002"]


def test_single_incident_has_no_recurrences(tmp_path):
    store = make_store(tmp_path, ["fixture_payments.json"])
    assert find_recurrences(str(store)) == []


def test_recurrences_span_different_scenarios(tmp_path):
    # The steady config-refresh noise shows up in both incidents; the
    # payments-only charge signature does too since db-slowdown cascades.
    store = make_store(tmp_path, ["fixture_payments.json", "fixture_db.json"])
    recs = find_recurrences(str(store))
    templates = [r.template for r in recs]
    assert "config refresh retry <NUM> failed for key <HEX>" in templates


def test_history_command_lists_incidents(tmp_path):
    store = make_store(tmp_path, ["fixture_payments.json", "fixture_db.json"])
    runner = CliRunner()
    result = runner.invoke(main, ["history", "--dir", str(store)])
    assert result.exit_code == 0, result.output
    assert "incident history (2 archived):" in result.output
    assert "inc-0001" in result.output
    assert "root cause: payments" in result.output
    assert "root cause: db" in result.output


def test_recurrences_command_output(tmp_path):
    store = make_store(tmp_path, ["fixture_payments.json", "fixture_payments.json"])
    runner = CliRunner()
    result = runner.invoke(main, ["recurrences", "--dir", str(store)])
    assert result.exit_code == 0, result.output
    assert "recurring signatures (seen in 2+ incidents):" in result.output
    assert "2 incidents" in result.output


def test_recurrences_command_empty_store(tmp_path):
    runner = CliRunner()
    result = runner.invoke(main, ["recurrences", "--dir", str(tmp_path / "none")])
    assert result.exit_code == 0, result.output
    assert "no signatures recur" in result.output
