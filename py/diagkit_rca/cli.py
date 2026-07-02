"""Command line entry point for the root-cause analyzer.

    python -m diagkit_rca analyze incident-bundle.json
    diagkit collect --out - | python -m diagkit_rca analyze -
"""

from __future__ import annotations

import click

from .analyzer import RootCause, analyze
from .bundle import Bundle, load_bundle
from .report import report_json, report_markdown


@click.group()
def main() -> None:
    """diagkit root-cause analyzer."""


@main.command()
@click.argument("bundle_path")
@click.option("--top", default=4, show_default=True, help="number of ranked services to show")
@click.option(
    "--format",
    "fmt",
    type=click.Choice(["text", "json", "markdown"]),
    default="text",
    show_default=True,
    help="report format",
)
def analyze_cmd(bundle_path: str, top: int, fmt: str) -> None:
    """Analyze an incident bundle and print the ranked root-cause report.

    BUNDLE_PATH is a file path, or - to read from stdin.
    """
    bundle = load_bundle(bundle_path)
    ranked = analyze(bundle)
    if fmt == "json":
        click.echo(report_json(bundle, ranked, top))
    elif fmt == "markdown":
        click.echo(report_markdown(bundle, ranked, top))
    else:
        click.echo(format_report(bundle, ranked, top))


# Register under the name "analyze" (the function name avoids shadowing the import).
main.add_command(analyze_cmd, name="analyze")


def format_report(bundle: Bundle, ranked: list[RootCause], top: int) -> str:
    lines: list[str] = []
    lines.append(f"incident scenario: {bundle.scenario} (seed {bundle.seed})")
    lines.append(
        f"window: {bundle.window.start_ms}..{bundle.window.end_ms}  "
        f"services={len(bundle.services)} logs={len(bundle.logs)} "
        f"traces={len(bundle.traces)} signatures={len(bundle.signatures)}"
    )
    lines.append("")

    if ranked:
        top_rc = ranked[0]
        summary = _summarize(top_rc)
        lines.append(f"Likely root cause: {top_rc.service} - {summary}")
        lines.append("")

    lines.append("ranked services:")
    for i, rc in enumerate(ranked[:top], start=1):
        lines.append(f"  {i}. {rc.service:<9} score={rc.score:.3f}")
        for reason in rc.reasons:
            lines.append(f"       - {reason}")
    return "\n".join(lines)


def _summarize(rc: RootCause) -> str:
    parts = [f"{rc.signature_count} signature(s)"]
    if rc.latency_spike_x >= 1.5:
        parts.append(f"p95 latency spike {rc.latency_spike_x:.1f}x")
    if rc.error_rate_peak >= 0.1:
        parts.append(f"error rate {rc.error_rate_peak * 100:.0f}%")
    if rc.propagation_pct >= 10:
        parts.append(f"{rc.propagation_pct:.0f}% of entry errors trace through it")
    return ", ".join(parts)


if __name__ == "__main__":
    main()
