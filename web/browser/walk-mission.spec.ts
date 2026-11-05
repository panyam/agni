// The walk mission's exercise, mission #1337 (agni issue 1338). An engineer clicks one part on a
// large schematic, sees what it connects to, widens the walk hop by hop, and asks a goal question
// from where the walk has reached.
//
// The engine answers the walk (WalkDesign, agni issue 1339) and the viewer draws none of it yet. So
// each step runs twice. Its engine row asks WalkDesign over the same Connect API the viewer calls,
// and passes today. Its viewer row drives the page through the names below, and waits on the ticket
// that builds them. As in the cross-probe walk, a waiting row that fails reads "not available", and
// a waiting row that passes FAILS the walk (exercise-report.ts, waitedOn), so the PR that makes a
// row pass also removes its wait here.
//
// The viewer rows drive the walk as #1341 is to build it, through these names:
// - a part's Properties offers each walk kind as [data-action="walk"][data-walk-kind="<kind>"]
// - the Trace tab lists the walk's nets as [data-walk-net="<net>"][data-hop="<n>"]
// - [data-action="walk-wider"] widens the walk by one hop
// - #1340's goal questions are [data-action="walk-goal"][data-goal="pull-up" | "pin"]
// A ticket that names them differently changes them here in the same PR.
//
// `make exercise-walk` tells the walk which of the tickets it waits on are closed (CLOSED_TICKETS,
// hack/closed_tickets.sh), so a row whose tickets have all closed must pass. The gate walks the
// cut of Jetson (examples/cross-probe-project) from R119, an LED's resistor whose walk widens over
// three hops. `make exercise-walk WALK_DESIGN=jetson` walks the whole board from R361, the fitted
// pull-up on the 3.3 V side of the connector I2C bus, whose second hop crosses the level translator
// U80 by its B4/A4 pin pair alone. The run writes
// the step table and a screenshot per step to EXERCISE_OUT.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, viewportFor } from "./browser.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";
import { picked, zoomTo } from "./picks.js";

const base = (): string => inject("baseUrl");
const repoRoot = fileURLToPath(new URL("../..", import.meta.url));
const walks = {
  cut: {
    mount: "probe",
    path: "designs/jetson-cut",
    file: "examples/cross-probe-project/designs/jetson-cut",
    about: "nineteen parts of Antmicro's Jetson AGX Thor baseboard",
    start: "R119",
    sheet: "USB Debug, PD",
    // A part the second hop crosses, which names the hop the reader sees grow.
    paired: "",
  },
  jetson: {
    mount: "jetson",
    path: "designs/jetson",
    file: "examples/jetson-project/designs/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module",
    start: "R361",
    sheet: "SoM_IO",
    paired: "U80",
  },
};
const which = (process.env.WALK_DESIGN || "cut") as keyof typeof walks;
const walk = walks[which];
if (!walk) throw new Error(`WALK_DESIGN=${which} names no walk; use one of ${Object.keys(walks).join(", ")}`);
const label = `${walk.path.split("/").pop()} (${walk.mount})`;
const kind = "small-parts";

interface WalkNet {
  hop: number;
  net: { name: string };
  entered: { refDes: string; enterPin?: string; exitPin?: string; paired?: boolean };
}
interface Walk {
  nets?: WalkNet[];
  boundary?: { refDes: string }[];
  reason?: string;
}

// rpc asks the server a Connect question in JSON, as the viewer's client does.
async function rpc<T>(method: string, body: unknown): Promise<T> {
  const r = await fetch(`${base()}/agni.v1.webapi.${method}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
    body: JSON.stringify(body),
  });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method}: ${r.status} ${text}`);
  return JSON.parse(text) as T;
}

let snapshot = "";
async function walkOf(hops: number): Promise<Walk> {
  if (!snapshot) {
    const s = await rpc<{ name: string }>("SnapshotService/Ingest", { snapshot: { uri: `mount://${walk.mount}/${walk.path}` } });
    snapshot = s.name;
  }
  const r = await rpc<{ walk: Walk }>("DesignService/WalkDesign", { snapshot, from: { refDes: walk.start }, kind, hops });
  if (r.walk.reason) throw new Error(`the walk did not start: ${r.walk.reason}`);
  return r.walk;
}

// shape is what two answers to one walk must share: each net with its hop and the part it was
// entered through, then the parts left at the boundary.
function shape(w: Walk): string {
  const nets = (w.nets ?? []).map((n) => `${n.hop}:${n.net.name}<${n.entered.refDes}`);
  const edge = (w.boundary ?? []).map((b) => b.refDes);
  return `${nets.join(" ")} | ${edge.join(" ")}`;
}

const names = (w: Walk, hop?: number): string[] => (w.nets ?? []).filter((n) => hop === undefined || n.hop === hop).map((n) => n.net.name);

// control finds a viewer control, and fails at once when the page has none, so a row waiting on the
// ticket that builds it costs the gate nothing.
async function control(page: Page, sel: string) {
  const c = page.locator(sel);
  if ((await c.count()) === 0) throw new Error(`the page has no ${sel}`);
  return c.first();
}

// outlinePoint is a point on a part's drawn outline, clear of its pins, that the page answers with the
// part itself. clearPoint (picks.ts) searches inside a body, and a hollow resistor has none.
async function outlinePoint(page: Page, sel: string, ref: string): Promise<{ x: number; y: number }> {
  return page.evaluate(
    ({ sel, ref }) => {
      for (const el of document.querySelectorAll(sel)) {
        const r = el.getBoundingClientRect();
        for (let fy = 0; fy <= 1; fy += 0.02) {
          for (let fx = 0; fx <= 1; fx += 0.02) {
            const x = Math.round(r.left + r.width * fx), y = Math.round(r.top + r.height * fy);
            const keyed = (px: number, py: number) => document.elementFromPoint(px, py)?.closest("[data-kind]") ?? null;
            const own = (h: Element | null) => h?.getAttribute("data-kind") === "component" && h.getAttribute("data-ref") === ref;
            // The picker probes a ring around a click and prefers a pin, so every probe must land on
            // the part or on nothing.
            const ring = [-5, 0, 5].flatMap((dx) => [-5, 0, 5].map((dy) => keyed(x + dx, y + dy)));
            if (own(keyed(x, y)) && ring.every((h) => !h || own(h))) return { x, y };
          }
        }
      }
      throw new Error(`no point on ${sel} answers with ${ref}`);
    },
    { sel, ref },
  );
}

// viewerWalk reads the walk the page lists, as #1341 is to draw it.
async function viewerWalk(page: Page): Promise<string[]> {
  const rows = page.locator("[data-walk-net]");
  await rows.first().waitFor({ state: "visible", timeout: 15_000 }).catch(() => {
    throw new Error("the page lists no walk ([data-walk-net])");
  });
  return rows.evaluateAll((els) => els.map((e) => `${e.getAttribute("data-hop")}:${e.getAttribute("data-walk-net")}`));
}

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

const closed = new Set((process.env.CLOSED_TICKETS ?? "").split(",").filter((t) => t !== "").map((t) => `#${t.replace(/^#/, "")}`));

describe("the walk mission", () => {
  it("walks outwards from one part, hop by hop", async () => {
    const rows: Row[] = [];
    const passed = new Set<string>();
    const out = process.env.EXERCISE_OUT;
    if (out) mkdirSync(out, { recursive: true });
    const page = await browser.newPage({ viewport: viewportFor({ width: 1400, height: 1000 }) });
    const pageErrors: string[] = [];
    page.on("pageerror", (e) => pageErrors.push(String(e)));

    let n = 0;
    const step = async (name: string, opts: { waitsOn?: string[]; needs?: string[] }, body: () => Promise<string>): Promise<void> => {
      n++;
      const waitsOn = opts.waitsOn ?? [];
      const missing = (opts.needs ?? []).filter((s) => !passed.has(s));
      if (missing.length > 0) {
        rows.push({ step: name, design: label, status: "not available", detail: `not reached, after ${missing.join(", ")}` });
        return;
      }
      const t0 = Date.now();
      let ok = false;
      let detail: string;
      try {
        detail = await body();
        ok = true;
        passed.add(name);
      } catch (e) {
        detail = (e instanceof Error ? e.message : String(e)).split("\n")[0];
      }
      if (out) await page.screenshot({ path: join(out, `walk-${n}-${name.replace(/\W+/g, "-")}.png`) }).catch(() => {});
      const open = waitsOn.filter((t) => !closed.has(t));
      const shut = waitsOn.filter((t) => closed.has(t));
      if (shut.length > 0) detail += ` (${shut.join(", ")} closed)`;
      rows.push({ step: name, design: label, ms: Date.now() - t0, ...waitedOn(ok, open, detail) });
    };

    try {
      await step("open", {}, async () => {
        await page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view`, { waitUntil: "networkidle" });
        await page.waitForFunction(() => document.querySelectorAll("#svg-view svg *").length > 0, null, { timeout: 60_000 });
        await page.locator(`#sheet-overview .sheet-tile[title="/${walk.sheet}"]`).click({ timeout: 30_000 });
        await page.locator(`#svg-view [data-kind="component"][data-ref="${walk.start}"]`).first().waitFor({ state: "attached", timeout: 30_000 });
        return `${walk.sheet} drawn, with ${walk.start} on it`;
      });

      // Step 1. A click on the part shows hop 1, its own nets.
      let one: Walk = {};
      await step("1. hop 1 (engine)", {}, async () => {
        one = await walkOf(1);
        const own = one.nets ?? [];
        if (own.length < 2) throw new Error(`${walk.start} reached ${own.length} nets, want both of its own`);
        const stray = own.filter((x) => x.hop !== 1 || x.entered.refDes !== walk.start);
        if (stray.length > 0) throw new Error(`hop 1 holds ${stray.map((x) => x.net.name).join(", ")}, which ${walk.start} did not enter`);
        return `${walk.start}'s nets: ${names(one).join(", ")}`;
      });
      await step("1. hop 1 (viewer)", { waitsOn: ["#1341"], needs: ["open", "1. hop 1 (engine)"] }, async () => {
        // A reader zooms to the part and clicks it. A resistor's body is a hollow outline, and only
        // the outline takes a click, so the point is found on it (outlinePoint).
        const part = `#svg-view [data-kind="component"][data-ref="${walk.start}"]`;
        await zoomTo(page, part);
        const at = await outlinePoint(page, part, walk.start);
        await page.mouse.click(at.x, at.y);
        await picked(page, walk.start);
        await (await control(page, `[data-action="walk"][data-walk-kind="${kind}"]`)).click({ timeout: 10_000 });
        const got = await viewerWalk(page);
        expect(got.sort()).toEqual(names(one, 1).map((x) => `1:${x}`).sort());
        return `the Trace tab lists ${got.length} nets at hop 1`;
      });

      // Step 2. Widening adds hop 2, then hop 3, and each adds only nets not yet reached.
      let three: Walk = {};
      await step("2. widen (engine)", { needs: ["1. hop 1 (engine)"] }, async () => {
        const two = await walkOf(2);
        three = await walkOf(3);
        const steps: string[] = [];
        let before = names(one);
        for (const [w, hop] of [[two, 2], [three, 3]] as const) {
          const now = names(w);
          const lost = before.filter((x) => !now.includes(x));
          if (lost.length > 0) throw new Error(`hop ${hop} dropped ${lost.join(", ")}`);
          const added = now.filter((x) => !before.includes(x));
          const wrong = (w.nets ?? []).filter((x) => added.includes(x.net.name) && x.hop !== hop);
          if (wrong.length > 0) throw new Error(`hop ${hop} added ${wrong.map((x) => `${x.net.name} at hop ${x.hop}`).join(", ")}`);
          steps.push(`hop ${hop} adds ${added.length}`);
          before = now;
        }
        if (names(three).length <= names(one).length) throw new Error(`the walk never widened past hop 1: ${names(three).join(", ")}`);
        if (walk.paired) {
          const via = (three.nets ?? []).find((x) => x.entered.refDes === walk.paired);
          if (!via?.entered.paired) throw new Error(`no net was entered through ${walk.paired} by its pin pair`);
          steps.push(`${walk.paired} crossed pin ${via.entered.enterPin} --> ${via.entered.exitPin} only, into ${via.net.name}`);
        }
        return steps.join(", ");
      });
      await step("2. widen (viewer)", { waitsOn: ["#1341"], needs: ["1. hop 1 (viewer)", "2. widen (engine)"] }, async () => {
        for (let i = 0; i < 2; i++) await (await control(page, '[data-action="walk-wider"]')).click({ timeout: 10_000 });
        const got = await viewerWalk(page);
        expect(got.sort()).toEqual((three.nets ?? []).map((x) => `${x.hop}:${x.net.name}`).sort());
        return `the Trace tab lists ${got.length} nets over 3 hops`;
      });

      // Steps 3 and 4. A goal question from where the walk has reached.
      await step("3. to the pull-up", { waitsOn: ["#1340", "#1341"], needs: ["2. widen (viewer)"] }, async () => {
        await (await control(page, '[data-action="walk-goal"][data-goal="pull-up"]')).click({ timeout: 10_000 });
        return "the walk shows the route to the pull-up and its rail";
      });
      await step("4. to a pin", { waitsOn: ["#1340", "#1341"], needs: ["2. widen (viewer)"] }, async () => {
        await (await control(page, '[data-action="walk-goal"][data-goal="pin"]')).click({ timeout: 10_000 });
        return "the walk shows the route to the pin, or says none lies within the radius";
      });

      // Step 5. The CLI gives the same walk as the API the viewer calls.
      await step("5. agni walk agrees (engine)", { needs: ["2. widen (engine)"] }, async () => {
        const json = execFileSync(inject("agniBin"), ["walk", walk.file, "--from", walk.start, "--kind", kind, "--hops", "3", "--format", "json"], {
          cwd: repoRoot,
          encoding: "utf8",
          stdio: ["ignore", "pipe", "ignore"],
        });
        const cli = JSON.parse(json) as Walk;
        expect(shape(cli)).toBe(shape(three));
        return `agni walk and WalkDesign agree on ${names(cli).length} nets and ${(cli.boundary ?? []).length} boundary parts`;
      });
      await step("5. agni walk agrees (viewer)", { waitsOn: ["#1341"], needs: ["2. widen (viewer)", "5. agni walk agrees (engine)"] }, async () => {
        const got = await viewerWalk(page);
        expect(got.sort()).toEqual((three.nets ?? []).map((x) => `${x.hop}:${x.net.name}`).sort());
        return "the viewer lists the nets agni walk prints";
      });

      await step("no page errors", {}, async () => {
        if (pageErrors.length) throw new Error(pageErrors.join("; "));
        return "none";
      });
    } finally {
      const report = `## Walk mission, ${walk.about}, from ${walk.start}\n\n${table(rows)}\n`;
      process.stdout.write(`\n${report}\n`);
      if (out) writeFileSync(join(out, "walk.md"), report);
      await page.close();
    }
    const broken = rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`);
    expect(exitCode(rows), broken.join("\n")).toBe(0);
  }, 300_000);
});
