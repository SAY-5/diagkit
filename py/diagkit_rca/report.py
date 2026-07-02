"""Report rendering for the analyzer: plain text, JSON, and markdown.

The JSON report is the machine-readable form of the ranked root-cause result,
suitable for piping into other tools. The markdown report is formatted for
pasting straight into an incident ticket.
"""

from __future__ import annotations

import json
from dataclasses import asdict

from .analyzer import RootCause
from .bundle import Bundle


def report_json(bundle: Bundle, ranked: list[RootCause], top: int) -> str:
    doc = {
        "schema_version": bundle.schema_version,
        "scenario": bundle.scenario,
        "seed": bundle.seed,
        "window": {"start_ms": bundle.window.start_ms, "end_ms": bundle.window.end_ms},
        "root_cause": ranked[0].service if ranked else None,
        "ranked": [asdict(rc) for rc in ranked[:top]],
        "signatures": [
            {"template": s.template, "count": s.count, "services": s.services}
            for s in bundle.signatures
        ],
    }
    return json.dumps(doc, indent=2)


def report_markdown(bundle: Bundle, ranked: list[RootCause], top: int) -> str:
    lines: list[str] = []
    lines.append(f"# Incident report: {bundle.scenario} (seed {bundle.seed})")
    lines.append("")
    lines.append(
        f"Window `{bundle.window.start_ms}..{bundle.window.end_ms}`, "
        f"{len(bundle.services)} services, {len(bundle.logs)} log lines, "
        f"{len(bundle.traces)} spans, {len(bundle.signatures)} signatures."
    )
    lines.append("")
    if ranked:
        lines.append(f"**Likely root cause: {ranked[0].service}**")
        lines.append("")

    lines.append("## Ranked services")
    lines.append("")
    lines.append("| # | Service | Score | Evidence |")
    lines.append("|---|---------|-------|----------|")
    for i, rc in enumerate(ranked[:top], start=1):
        lines.append(f"| {i} | {rc.service} | {rc.score:.3f} | {'; '.join(rc.reasons)} |")
    lines.append("")

    lines.append("## Error signatures")
    lines.append("")
    lines.append("| Count | Services | Template |")
    lines.append("|-------|----------|----------|")
    for s in bundle.signatures:
        lines.append(f"| {s.count} | {', '.join(s.services)} | `{s.template}` |")
    return "\n".join(lines)
