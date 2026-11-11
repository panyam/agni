// The board-open mission's exercise (agni issue 1500, mission #1499). It times how long Jetson's board
// takes to draw in the viewer, five ways:
//
//   server, cold         a fresh `agni serve` (its design cache lives in memory) and a fresh browser
//   server, reload       the same page reloaded
//   server, new browser  a fresh browser on that warm server, as a teammate's first visit is
//   wasm, cold           a fresh browser with ?engine=wasm, so OPFS is empty, as a demo visitor's is
//   wasm, reload         the same page reloaded
//
// Each time runs from the navigation to the canvas's first WebGL `agni:sheet-drawn` mark on the
// board, read on the wall clock, so it spans a redirect through /load. The page runs with ?timing,
// and each row names the requests that held it longest and the files a browser engine fetched
// (openDetail in exercise-report.ts).
//
// `make exercise-board-open` walks Jetson, headed, three times, and judges each median against the
// mission's targets: 1.5 s cold and 0.5 s on a reload. The gate walks the cut once and judges
// nothing, since headless Chrome draws WebGL in SwiftShader. The table goes to
// EXERCISE_OUT/board-open.md for the mission log.

import { afterAll, beforeAll, expect, inject, it } from "vitest";
import type { Browser, BrowserContext, Page } from "playwright-core";
import { mkdirSync, writeFileSync } from "node:fs";
import { loadavg } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, viewportFor } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";
import { exitCode, median, openDetail, table, type Row } from "./exercise-report.js";

const repoRoot = fileURLToPath(new URL("../..", import.meta.url));

const designs = {
  // The cross-probe walk's nineteen-part cut of Jetson, with its copper.
  cut: { mount: "probe", path: "designs/jetson-cut", about: "the nineteen-part cut of Jetson" },
  // Jetson itself. `make -C examples/jetson-project board` copies it in.
  jetson: { mount: "jetson", path: "designs/jetson", about: "Antmicro's Jetson AGX Thor baseboard" },
};
const chosen = (process.env.WALK_DESIGN ?? "cut") as keyof typeof designs;
if (!(chosen in designs)) throw new Error(`WALK_DESIGN=${chosen} names no design; use one of ${Object.keys(designs).join(", ")}`);
const design = designs[chosen];
// Only Jetson is judged. The cut is too small for its times to say anything about the mission.
const judged = chosen === "jetson";
const runs = Math.max(1, Number(process.env.RUNS ?? "1"));

const COLD_MS = 1500;
const RELOAD_MS = 500;
type Kind = "server, cold" | "server, reload" | "server, new browser" | "wasm, cold" | "wasm, reload";
const KINDS: Kind[] = ["server, cold", "server, reload", "server, new browser", "wasm, cold", "wasm, reload"];
const target = (k: Kind): number => (k.endsWith("reload") ? RELOAD_MS : COLD_MS);

const DRAWN = "agni:sheet-drawn";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

// serve starts an agni server of its own, so the first open on it is cold.
async function serve(): Promise<Server> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  return start({
    what: "agni serve (board-open)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `:${port}`, "--mount", "jetson=examples/jetson-project", "--mount", "probe=examples/cross-probe-project", "--web-dir", "web"],
    cwd: repoRoot,
    base,
    health: base,
  });
}

interface Open {
  ms: number;
  lines: string[];
}

// timed runs a navigation and waits for the canvas's first WebGL draw in the document it lands on.
// It reads the mark's time on the wall clock, so a redirect through /load counts.
// Only the lines of work that started before the draw are kept, since the panels' requests that follow
// it never held the board up. A line is logged when its request answers, so its start on the wall
// clock is its arrival less its page.wallMs. The page's own clock restarts at the /load redirect, so
// it can't place a line.
async function timed(page: Page, navigate: () => Promise<unknown>): Promise<Open> {
  const got: { at: number; text: string }[] = [];
  const onConsole = (m: { text(): string }) => {
    const text = m.text();
    if (text.startsWith("agni ")) got.push({ at: Date.now(), text });
  };
  const before = (drawn: number): string[] =>
    got
      .filter((l) => {
        const m = /^agni timing \w+: (.*)$/s.exec(l.text);
        if (!m) return l.at <= drawn;
        try {
          return l.at - Number(JSON.parse(m[1])?.page?.wallMs ?? 0) <= drawn;
        } catch {
          return false;
        }
      })
      .map((l) => l.text);
  page.on("console", onConsole);
  const t0 = Date.now();
  await navigate();
  const deadline = t0 + 300_000;
  try {
    for (;;) {
      if (Date.now() > deadline) throw new Error(`the board did not draw within ${(deadline - t0) / 1000} s at ${page.url()}`);
      // A redirect destroys the document under an evaluate, which then throws. The loop asks again.
      const at = await page
        .evaluate((name) => {
          const m = performance.getEntriesByName(name).find((e) => {
            const d = (e as PerformanceMark).detail as { mode?: string; view?: string } | null;
            return d?.mode === "webgl" && (d.view ?? "canvas") === "canvas";
          });
          return m ? performance.timeOrigin + m.startTime : null;
        }, DRAWN)
        .catch(() => null);
      if (at !== null) {
        // A request that started before the draw may answer after it. Give those a moment to log.
        await new Promise((r) => setTimeout(r, 1000));
        return { ms: at - t0, lines: before(at) };
      }
      await new Promise((r) => setTimeout(r, 25));
    }
  } finally {
    page.off("console", onConsole);
  }
}

async function freshPage(): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ viewport: viewportFor({ width: 1400, height: 1000 }) });
  return { context, page: await context.newPage() };
}

it(
  `opens ${design.about}'s board cold and reloaded, against a server and in the browser engine`,
  async () => {
    const seen = new Map<Kind, Open[]>(KINDS.map((k) => [k, []]));
    for (let i = 0; i < runs; i++) {
      const server = await serve();
      const contexts: BrowserContext[] = [];
      try {
        const url = `${server.base}/designs/${design.mount}/${design.path}/view?mode=webgl&sheet=board&timing=1`;
        const a = await freshPage();
        contexts.push(a.context);
        seen.get("server, cold")!.push(await timed(a.page, () => a.page.goto(url, { waitUntil: "commit" })));
        seen.get("server, reload")!.push(await timed(a.page, () => a.page.reload({ waitUntil: "commit" })));
        const b = await freshPage();
        contexts.push(b.context);
        seen.get("server, new browser")!.push(await timed(b.page, () => b.page.goto(url, { waitUntil: "commit" })));
        const c = await freshPage();
        contexts.push(c.context);
        seen.get("wasm, cold")!.push(await timed(c.page, () => c.page.goto(`${url}&engine=wasm`, { waitUntil: "commit" })));
        seen.get("wasm, reload")!.push(await timed(c.page, () => c.page.reload({ waitUntil: "commit" })));
        server.assertAlive();
      } finally {
        for (const ctx of contexts) await ctx.close();
        await server.stop();
      }
    }

    const load = loadavg()[0].toFixed(1);
    const rows: Row[] = KINDS.map((k) => {
      const opens = seen.get(k)!;
      const ms = median(opens.map((o) => o.ms));
      // The detail comes from the run nearest the median, so its stages add up to the time shown.
      const nearest = [...opens].sort((x, y) => Math.abs(x.ms - ms) - Math.abs(y.ms - ms))[0];
      const all = opens.length > 1 ? `median of ${opens.map((o) => `${(o.ms / 1000).toFixed(2)} s`).join(", ")}; ` : "";
      const judge = judged ? `target ${(target(k) / 1000).toFixed(1)} s; ` : "not judged on the cut; ";
      return {
        step: k,
        design: chosen,
        status: judged && ms > target(k) ? "fail" : "ok",
        ms,
        detail: `${judge}${all}${openDetail(nearest.lines)}; load average ${load}`,
      };
    });
    const report = `## Board open, ${design.about}\n\n${table(rows)}\n`;
    console.log(`\n${report}`);
    const out = process.env.EXERCISE_OUT;
    if (out) {
      mkdirSync(out, { recursive: true });
      writeFileSync(join(out, "board-open.md"), report);
      // Every open's timing lines, for reading a row's stages in full.
      writeFileSync(join(out, "board-open-lines.json"), JSON.stringify(Object.fromEntries([...seen].map(([k, opens]) => [k, opens])), null, 1));
    }
    // Every open drew, or timed() threw. Each row also names at least one timed request, so a page
    // that stopped logging under ?timing fails here rather than reporting a bare time.
    expect(rows.filter((r) => r.detail.includes("no timed requests")).map((r) => r.step)).toEqual([]);
    expect(rows).toHaveLength(KINDS.length);
    expect(exitCode(rows)).toBe(0);
  },
  1_800_000,
);
