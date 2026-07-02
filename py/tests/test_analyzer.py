from pathlib import Path

from diagkit_rca import analyze, load_bundle
from diagkit_rca.bundle import SCHEMA_VERSION

FIXTURES = Path(__file__).parent


def test_payments_outage_top_root_cause():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    ranked = analyze(bundle)
    assert ranked[0].service == "payments"
    top = ranked[0]
    assert top.score > 0
    assert top.signature_count >= 1
    assert top.latency_spike_x >= 1.5
    assert top.propagation_pct >= 50.0
    assert any("latency" in r for r in top.reasons)


def test_db_slowdown_top_root_cause():
    bundle = load_bundle(str(FIXTURES / "fixture_db.json"))
    ranked = analyze(bundle)
    assert ranked[0].service == "db"


def test_ranking_is_ordered_by_score():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    ranked = analyze(bundle)
    scores = [r.score for r in ranked]
    assert scores == sorted(scores, reverse=True)


def test_all_services_ranked():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    ranked = analyze(bundle)
    assert {r.service for r in ranked} == set(bundle.services)


def test_schema_version_matches_fixture():
    bundle = load_bundle(str(FIXTURES / "fixture_payments.json"))
    assert bundle.schema_version == SCHEMA_VERSION


def test_bad_schema_rejected(tmp_path):
    import io

    from diagkit_rca.bundle import load_bundle_stream

    stream = io.StringIO('{"schema_version": "999", "window": {"start_ms":0,"end_ms":0}}')
    try:
        load_bundle_stream(stream)
    except ValueError as exc:
        assert "schema" in str(exc)
    else:
        raise AssertionError("expected ValueError for bad schema")
