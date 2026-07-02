"""The incident-bundle schema, mirrored from the Go ``internal/bundle`` package.

This is the single interoperation contract between the collector and the
analyzer. The dataclasses here match the JSON the Go side writes.
"""

from __future__ import annotations

import json
import sys
from dataclasses import dataclass, field
from typing import IO

SCHEMA_VERSION = "1"


@dataclass
class LogEntry:
    ts_ms: int
    service: str
    level: str
    message: str


@dataclass
class Span:
    trace_id: str
    span_id: str
    service: str
    operation: str
    duration_ms: int
    error: bool
    called_by: str


@dataclass
class MetricPoint:
    ts_ms: int
    value: float


@dataclass
class ServiceMetrics:
    service: str
    error_rate: list[MetricPoint] = field(default_factory=list)
    p95_latency_ms: list[MetricPoint] = field(default_factory=list)


@dataclass
class Signature:
    template: str
    count: int
    services: list[str] = field(default_factory=list)
    example: str = ""


@dataclass
class Window:
    start_ms: int
    end_ms: int


@dataclass
class Bundle:
    schema_version: str
    scenario: str
    seed: int
    window: Window
    services: list[str]
    logs: list[LogEntry]
    traces: list[Span]
    metrics: list[ServiceMetrics]
    signatures: list[Signature]


def _from_dict(data: dict) -> Bundle:
    return Bundle(
        schema_version=data.get("schema_version", ""),
        scenario=data.get("scenario", ""),
        seed=data.get("seed", 0),
        window=Window(**data.get("window", {"start_ms": 0, "end_ms": 0})),
        services=list(data.get("services", [])),
        logs=[LogEntry(**e) for e in data.get("logs", [])],
        traces=[Span(**s) for s in data.get("traces", [])],
        metrics=[
            ServiceMetrics(
                service=m["service"],
                error_rate=[MetricPoint(**p) for p in m.get("error_rate", [])],
                p95_latency_ms=[MetricPoint(**p) for p in m.get("p95_latency_ms", [])],
            )
            for m in data.get("metrics", [])
        ],
        signatures=[Signature(**s) for s in data.get("signatures", [])],
    )


def load_bundle(path: str) -> Bundle:
    """Load a bundle from a file path, or from stdin when ``path`` is ``-``."""
    if path == "-":
        return load_bundle_stream(sys.stdin)
    with open(path, encoding="utf-8") as fh:
        return load_bundle_stream(fh)


def load_bundle_stream(stream: IO[str]) -> Bundle:
    data = json.load(stream)
    bundle = _from_dict(data)
    if bundle.schema_version != SCHEMA_VERSION:
        raise ValueError(
            f"unsupported bundle schema {bundle.schema_version!r}, expected {SCHEMA_VERSION!r}"
        )
    return bundle
