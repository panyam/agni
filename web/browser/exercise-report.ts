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
