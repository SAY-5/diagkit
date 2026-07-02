// Port of internal/fingerprint/fingerprint.go: reduce free-form log messages to
// stable templates by replacing volatile tokens (numbers, durations, hex, ids,
// quoted strings) with placeholders, then group identical templates into
// signature clusters. Ordering of the replacers matches the Go original so a
// duration like "3003ms" becomes <DUR> rather than <NUM>ms.

import type { LogEntry, Signature } from "./types";

const replacers: { re: RegExp; with: string }[] = [
  { re: /"[^"]*"/g, with: "<STR>" }, // quoted strings
  { re: /'[^']*'/g, with: "<STR>" }, // single-quoted
  { re: /\b0x[0-9a-fA-F]+\b/g, with: "<HEX>" }, // 0x hex
  { re: /\b[0-9a-fA-F]{8,}\b/g, with: "<HEX>" }, // bare long hex/ids
  { re: /\b\d+(\.\d+)?ms\b/g, with: "<DUR>" }, // durations
  { re: /\b\d+(\.\d+)?s\b/g, with: "<DUR>" }, // second durations
  { re: /\b\d+(\.\d+)?\b/g, with: "<NUM>" }, // remaining numbers
  { re: /\b[0-9a-f]{8}-[0-9a-f-]{20,}\b/g, with: "<UUID>" }, // uuids
];

// normalize reduces a single log message to its template form.
export function normalize(msg: string): string {
  let out = msg;
  // uuids first (they contain hyphens the hex rule would miss)
  const uuid = replacers[replacers.length - 1];
  out = out.replace(uuid.re, uuid.with);
  for (const r of replacers.slice(0, replacers.length - 1)) {
    out = out.replace(r.re, r.with);
  }
  return out.split(/\s+/).filter(Boolean).join(" ");
}

// cluster groups log entries into signature clusters keyed by their template.
// Only the given levels are considered; passing an empty list clusters every
// entry.
export function cluster(logs: LogEntry[], levels: string[]): Signature[] {
  const want = new Set(levels);

  interface Agg {
    count: number;
    services: Set<string>;
    example: string;
  }
  const groups = new Map<string, Agg>();

  for (const e of logs) {
    if (want.size > 0 && !want.has(e.level)) continue;
    const tmpl = normalize(e.message);
    let g = groups.get(tmpl);
    if (!g) {
      g = { count: 0, services: new Set(), example: e.message };
      groups.set(tmpl, g);
    }
    g.count++;
    g.services.add(e.service);
  }

  const sigs: Signature[] = [];
  for (const [tmpl, g] of groups) {
    const svcs = [...g.services].sort();
    sigs.push({ template: tmpl, count: g.count, services: svcs, example: g.example });
  }

  sigs.sort((a, b) => {
    if (a.count !== b.count) return b.count - a.count;
    return a.template < b.template ? -1 : a.template > b.template ? 1 : 0;
  });
  return sigs;
}
