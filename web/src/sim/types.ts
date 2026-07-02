// Incident-bundle schema, mirrored from internal/bundle/bundle.go and
// py/diagkit_rca/bundle.py. This is the single contract the whole pipeline
// speaks; the field names match the JSON the Go collector writes.

export const SCHEMA_VERSION = "1";

export interface LogEntry {
  ts_ms: number;
  service: string;
  level: string;
  message: string;
}

export interface Span {
  trace_id: string;
  span_id: string;
  service: string;
  operation: string;
  duration_ms: number;
  error: boolean;
  called_by: string;
}

export interface MetricPoint {
  ts_ms: number;
  value: number;
}

export interface ServiceMetrics {
  service: string;
  error_rate: MetricPoint[];
  p95_latency_ms: MetricPoint[];
}

export interface Signature {
  template: string;
  count: number;
  services: string[];
  example: string;
}

export interface Window {
  start_ms: number;
  end_ms: number;
}

export interface Bundle {
  schema_version: string;
  scenario: string;
  seed: number;
  window: Window;
  services: string[];
  logs: LogEntry[];
  traces: Span[];
  metrics: ServiceMetrics[];
  signatures: Signature[];
}

export interface Scenario {
  name: string;
  culprit: string;
  errorBoost: number;
  latencyx: number;
}

export const SCENARIOS: Record<string, Scenario> = {
  "payments-outage": { name: "payments-outage", culprit: "payments", errorBoost: 0.72, latencyx: 4.2 },
  "db-slowdown": { name: "db-slowdown", culprit: "db", errorBoost: 0.55, latencyx: 6.0 },
  healthy: { name: "healthy", culprit: "", errorBoost: 0.0, latencyx: 1.0 },
};

export const DEFAULT_SCENARIO = "payments-outage";
