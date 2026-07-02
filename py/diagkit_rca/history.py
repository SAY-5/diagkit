"""Reader for the incident history store the Go side writes.

The store is a directory of archived bundle files plus an index.json. The Go
`diagkit archive` command appends to it; this module lists past incidents and
finds signatures that recur across them.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from .bundle import Bundle, load_bundle

INDEX_VERSION = "1"
INDEX_FILE = "index.json"
DEFAULT_DIR = "diagkit-history"


@dataclass
class IncidentRecord:
    id: str
    file: str
    scenario: str
    seed: int
    logs: int
    traces: int
    signatures: int


@dataclass
class Recurrence:
    template: str
    incidents: list[str] = field(default_factory=list)
    total_count: int = 0
    services: list[str] = field(default_factory=list)


def load_index(directory: str) -> list[IncidentRecord]:
    """Load the history index. A missing store is an empty history."""
    path = Path(directory) / INDEX_FILE
    if not path.exists():
        return []
    data = json.loads(path.read_text(encoding="utf-8"))
    version = data.get("version", "")
    if version != INDEX_VERSION:
        raise ValueError(
            f"unsupported history index version {version!r}, expected {INDEX_VERSION!r}"
        )
    return [
        IncidentRecord(
            id=e["id"],
            file=e["file"],
            scenario=e.get("scenario", ""),
            seed=e.get("seed", 0),
            logs=e.get("logs", 0),
            traces=e.get("traces", 0),
            signatures=e.get("signatures", 0),
        )
        for e in data.get("incidents", [])
    ]


def load_incident(directory: str, record: IncidentRecord) -> Bundle:
    return load_bundle(str(Path(directory) / record.file))


def find_recurrences(directory: str, min_incidents: int = 2) -> list[Recurrence]:
    """Signatures seen in at least min_incidents archived incidents, ranked by
    how many incidents they recur in, then by total matched log lines."""
    by_template: dict[str, Recurrence] = {}
    for record in load_index(directory):
        bundle = load_incident(directory, record)
        for sig in bundle.signatures:
            rec = by_template.setdefault(sig.template, Recurrence(template=sig.template))
            rec.incidents.append(record.id)
            rec.total_count += sig.count
            rec.services = sorted(set(rec.services) | set(sig.services))
    out = [r for r in by_template.values() if len(r.incidents) >= min_incidents]
    out.sort(key=lambda r: (-len(r.incidents), -r.total_count, r.template))
    return out
