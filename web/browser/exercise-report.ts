// What the public demo exercise (exercise.mjs, mission #851) decides, kept apart from the browser
// driving so it can be unit tested: a step's status, the run's exit code, how a request is counted,
// and whether a request carries a dropped design's content. Erasable TypeScript with no imports, so
// exercise.mjs loads it through Node's own type stripping.

// A step that ran is "ok" or "fail". One that has nothing to run against yet is "not available",
// naming the ticket that adds it, and is never a failure, so the exercise runs before the mission is
// done and says which steps are missing rather than skipping them.
export type Status = "ok" | "fail" | "not available";

export interface Row {
  step: string;
  design: string;
  status: Status;
  // ms is how long the step took, when timing it means something.
  ms?: number;
  detail: string;
}

// exitCode is non-zero only when a step that exists failed.
export function exitCode(rows: readonly Row[]): number {
  return rows.some((r) => r.status === "fail") ? 1 : 0;
}

// RequestKind splits a page's requests the way the mission reads them: analysis calls, which a
// static site and a dropped design must never make, files fetched for a seeded design, and the
// page's own assets.
export type RequestKind = "api" | "file" | "asset";

export function classify(path: string, prefix: string): RequestKind {
  if (path.includes("/agni.v1.")) return "api";
  if (path.startsWith(`${prefix}raw/`) || path.startsWith(`${prefix}files/`) || path.startsWith("/raw/")) return "file";
  return "asset";
}

export interface SeenRequest {
  url: string;
  method: string;
  body: string;
  // startMs is when the page sent it, on the exercise's clock, and endMs when its response finished
  // or it failed; a request still unanswered has none.
  startMs?: number;
  endMs?: number;
}

// slowestFetch reads the page's `agni fetch <file>: {bytes, ms, attempts}` lines and names the file
// that took longest, with its size, time and any retries, or "" when the page logged none.
export function slowestFetch(lines: readonly string[]): string {
  let best: { file: string; bytes: number; ms: number; attempts: number } | undefined;
  for (const l of lines) {
    const m = /^agni fetch (.+?): (\{.*\})$/.exec(l);
    if (!m) continue;
    try {
      const v = JSON.parse(m[2]);
      if (!best || v.ms > best.ms) best = { file: m[1], ...v };
    } catch {
      // A line that is not ours is skipped.
    }
  }
  if (!best) return "";
  const mb = (best.bytes / (1 << 20)).toFixed(1);
  return `slowest file ${best.file} ${mb} MB in ${(best.ms / 1000).toFixed(1)} s${best.attempts > 1 ? ` after ${best.attempts} attempts` : ""}`;
}

// pendingFiles names each design file the page asked for and never received, with how long it had
// waited at nowMs, so a timed-out open says which file it was waiting on (agni issue 1002).
export function pendingFiles(requests: readonly SeenRequest[], prefix: string, nowMs: number): string {
  const waiting = requests.filter((r) => r.startMs !== undefined && r.endMs === undefined && classify(new URL(r.url).pathname, prefix) === "file");
  if (waiting.length === 0) return "no design file pending";
  const name = (u: string) => {
    const p = new URL(u).pathname;
    return decodeURIComponent(p.slice(p.indexOf("/raw/") >= 0 ? p.indexOf("/raw/") + 5 : 0));
  };
  return `waiting on ${waiting.map((r) => `${name(r.url)} (${Math.round((nowMs - (r.startMs ?? nowMs)) / 1000)} s)`).join(", ")}`;
}

// leaks returns each request whose address or body carries one of a dropped design's names. A
// dropped design is read in the page and never sent anywhere, so any such request fails step 5.
export function leaks(requests: readonly SeenRequest[], markers: readonly string[]): string[] {
  const out: string[] = [];
  for (const r of requests) {
    let url = r.url;
    try {
      url = decodeURIComponent(r.url);
    } catch {
      // A malformed escape is checked as written.
    }
    const hit = markers.find((m) => url.includes(m) || r.body.includes(m));
    if (hit) out.push(`${r.method} ${r.url} carries ${hit}`);
  }
  return out;
}

// markersFrom picks names out of a design file that only that design would send: EDIF net names and
// KiCad labels, four characters or longer so GND or EN cannot match by accident.
export function markersFrom(text: string, max = 8): string[] {
  const names = new Set<string>();
  const pattern = /\(net\s+(?:\(rename\s+)?"?([A-Za-z0-9_+\-./]+)"?|\((?:global_|hierarchical_)?label\s+"([^"]+)"/g;
  for (const m of text.matchAll(pattern)) {
    const name = m[1] ?? m[2];
    if (name && name.length >= 4) names.add(name);
    if (names.size >= max) break;
  }
  return [...names];
}

// table renders the rows as a markdown table, for the terminal and for pasting into the mission log.
export function table(rows: readonly Row[]): string {
  const lines = ["| Step | Design | Status | Time | Detail |", "|---|---|---|---|---|"];
  for (const r of rows) {
    const time = r.ms === undefined ? "" : r.ms >= 1000 ? `${(r.ms / 1000).toFixed(1)} s` : `${Math.round(r.ms)} ms`;
    lines.push(`| ${r.step} | ${r.design} | ${r.status} | ${time} | ${r.detail.replace(/\|/g, "\\|")} |`);
  }
  return lines.join("\n");
}

// A TimedRequest is one request a page made while a step ran, read off the page's "agni timing" log
// (agni issue 968). offsetMs is when it started after the step did, wallMs how long the page waited
// for it, and engineMs how much of that the engine reported spending, absent for a request that failed.
export interface TimedRequest {
  rpc: string;
  offsetMs: number;
  wallMs: number;
  engineMs?: number;
  widest?: string;
  // slowest is the request's slowest timed rule or interface profile, which the engine lists slowest
  // first.
  slowest?: string;
  error?: string;
}

// A Breakdown splits a step's time between the requests it made and the page's own work.
// inRequestsMs is the time at least one request was in flight, so overlapping requests count once,
// and pageMs is the rest: laying out, drawing, and anything else the page did between requests.
export interface Breakdown {
  totalMs: number;
  requests: TimedRequest[];
  inRequestsMs: number;
  pageMs: number;
}

const TIMING = /^agni timing (\w+): (.*)$/s;

// checksBreakdown reads the timing lines logged between t0 and t1 on the page's clock. A line that
// carries no page clock (a page built before 968) is left out rather than guessed at.
export function checksBreakdown(lines: readonly string[], t0: number, t1: number): Breakdown {
  const requests: TimedRequest[] = [];
  const spans: [number, number][] = [];
  for (const line of lines) {
    const m = TIMING.exec(line);
    if (!m) continue;
    let t;
    try {
      t = JSON.parse(m[2]);
    } catch {
      continue;
    }
    const start = t?.page?.startMs;
    const wall = t?.page?.wallMs;
    if (typeof start !== "number" || typeof wall !== "number" || start < t0 || start > t1) continue;
    const r: TimedRequest = { rpc: m[1], offsetMs: start - t0, wallMs: wall };
    if (t.error !== undefined) r.error = String(t.error);
    if (t.totalMicros !== undefined) r.engineMs = Number(t.totalMicros) / 1000;
    const widest = [...(t.stages ?? [])].sort((a, b) => Number(b.micros ?? 0) - Number(a.micros ?? 0))[0];
    if (widest) r.widest = `${widest.name} ${Math.round(Number(widest.micros ?? 0) / 1000)} ms`;
    const slowest = t.rules?.[0];
    if (slowest) r.slowest = `slowest ${slowest.rule} ${Math.round(Number(slowest.micros ?? 0) / 1000)} ms`;
    requests.push(r);
    spans.push([start, Math.min(start + wall, t1)]);
  }
  requests.sort((a, b) => a.offsetMs - b.offsetMs);
  spans.sort((a, b) => a[0] - b[0]);
  let inRequestsMs = 0;
  let end = -Infinity;
  for (const [s, e] of spans) {
    if (e <= end) continue;
    inRequestsMs += e - Math.max(s, end);
    end = e;
  }
  const totalMs = t1 - t0;
  return { totalMs, requests, inRequestsMs, pageMs: totalMs - inRequestsMs };
}

const sec = (ms: number) => (ms >= 1000 ? `${(ms / 1000).toFixed(1)} s` : `${Math.round(ms)} ms`);

// breakdownDetail is a Breakdown as one row's detail: each request in the order it started, then the
// time the page spent outside every request.
export function breakdownDetail(b: Breakdown): string {
  const reqs = b.requests.map((r) => {
    const inner = r.error !== undefined ? `failed: ${r.error}` : [r.engineMs === undefined ? "" : `engine ${sec(r.engineMs)}`, r.widest ?? "", r.slowest ?? ""].filter(Boolean).join(", ");
    return `${r.rpc} at +${sec(r.offsetMs)} took ${sec(r.wallMs)}${inner ? ` (${inner})` : ""}`;
  });
  return `${b.requests.length} requests: ${reqs.join("; ") || "none"}; in requests ${sec(b.inRequestsMs)}, page outside them ${sec(b.pageMs)}`;
}
