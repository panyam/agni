// The cross-probe walk, mission #1321's exercise (agni issue 1323). An engineer at the bench opens a
// design's schematic and board as two views, clicks a part in one and finds it in the other, and
// turns the board over to match how it lies on the bench.
//
// The walk was written before the viewer can do any of it, so each step names the tickets it waits
// on. A waiting step that fails reads "not available" and does not fail the walk, and a waiting step
// that passes DOES fail it (exercise-report.ts, waitedOn), so the PR that makes a step pass also
// removes its wait here. A step that needs an earlier one reads "not reached" until that one passes.
//
// The steps define what the later tickets build, in terms a test can hold (#1326 has the shape):
// - each view is an element with data-artifact-view="schematic" or "board", holding an <svg>. A view
//   is its own dock panel or floating window at any size, so the walk asks only that each is visible
//   and on screen, never where, how big, or whether two overlap
// - parts and copper carry the drawing's own data-ref and data-net keys, as the single view does
// - a part located in a board view gets an element with class locate-ring
// - the board view states its view as data-rotation (degrees) and data-side ("front" or "back")
// - the board view has controls data-action="rotate" (a quarter turn) and data-action="flip"
// A ticket that names them differently changes them here in the same PR.
//
// The gate walks a nineteen-part cut of Jetson (examples/cross-probe-project). `make
// exercise-cross-probe` walks the whole board (examples/jetson-project), headed unless HEADED=0, and
// writes the step table and a screenshot per step to EXERCISE_OUT.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Locator, Page } from "playwright-core";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { launch, viewportFor } from "./browser.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";

const base = (): string => inject("baseUrl");
const walks = {
  cut: {
    mount: "probe",
    path: "designs/jetson-cut",
    about: "nineteen parts of Antmicro's Jetson AGX Thor baseboard, cut with their copper",
  },
  jetson: {
    mount: "jetson",
    path: "designs/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module",
  },
};
const which = (process.env.WALK_DESIGN || "cut") as keyof typeof walks;
const walk = walks[which];
if (!walk) throw new Error(`WALK_DESIGN=${which} names no walk; use one of ${Object.keys(walks).join(", ")}`);
const label = `${walk.path.split("/").pop()} (${walk.mount})`;

// The walk's parts are in both designs. C133 is a back-side capacitor on "USB Debug, PD", TP76 a
// front-side test point on "Power", and C133's pin 1 is on the net below.
const part = { ref: "C133", sheet: "USB Debug, PD" };
const footprint = { ref: "TP76", sheet: "Power" };
const net = "/USB Debug, PD/FTDI_3V3";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

// view is the first open view of a kind. More may be open, and the walk drives the first.
const view = (page: Page, which: "schematic" | "board"): Locator => page.locator(`[data-artifact-view="${which}"]`).first();

// box is the union of the client rects of every element a locator matches, or undefined for none.
async function box(l: Locator): Promise<DOMRect | undefined> {
  const r = await l.evaluateAll((els) => {
    const rs = els.map((e) => e.getBoundingClientRect()).filter((b) => b.width > 0 || b.height > 0);
    if (rs.length === 0) return undefined;
    const x0 = Math.min(...rs.map((b) => b.left));
    const y0 = Math.min(...rs.map((b) => b.top));
    return { x: x0, y: y0, w: Math.max(...rs.map((b) => b.right)) - x0, h: Math.max(...rs.map((b) => b.bottom)) - y0 };
  });
  return r ? new DOMRect(r.x, r.y, r.w, r.h) : undefined;
}

const centre = (b: DOMRect): [number, number] => [b.x + b.width / 2, b.y + b.height / 2];
const inside = (p: [number, number], b: DOMRect): boolean => p[0] >= b.left && p[0] <= b.right && p[1] >= b.top && p[1] <= b.bottom;

// centred checks that a part sits near the middle of its view, within the central half each way.
async function centred(where: Locator, what: Locator, name: string): Promise<string> {
  const p = await box(where);
  const b = await box(what);
  if (!p || !b) throw new Error(`${name} is not drawn in the view`);
  const [cx, cy] = centre(b);
  const dx = Math.abs(cx - (p.x + p.width / 2)) / p.width;
  const dy = Math.abs(cy - (p.y + p.height / 2)) / p.height;
  if (dx > 0.25 || dy > 0.25) throw new Error(`${name} sits ${Math.round(dx * 100)}% and ${Math.round(dy * 100)}% off the view's centre`);
  return `${name} centred within ${Math.round(Math.max(dx, dy) * 100)}%`;
}

// ringed checks that the board view rings a part: a locate ring whose centre falls on the part's
// footprint.
async function ringed(page: Page, ref: string): Promise<string> {
  const board = view(page, "board");
  const ring = board.locator(".locate-ring");
  await ring.first().waitFor({ timeout: 15_000 });
  const r = await box(ring);
  const fp = await box(board.locator(`[data-ref="${ref}"]`));
  if (!r || !fp) throw new Error(`no ring or no ${ref} footprint drawn`);
  if (!inside(centre(r), fp)) throw new Error(`the ring's centre is off ${ref}'s footprint`);
  return `ringed, ${await centred(board, board.locator(`[data-ref="${ref}"]`), ref)}`;
}

describe("the cross-probe walk", () => {
  it("finds a schematic part on the board and back", async () => {
    const rows: Row[] = [];
    const passed = new Set<string>();
    const out = process.env.EXERCISE_OUT;
    if (out) mkdirSync(out, { recursive: true });
    const page = await browser.newPage({ viewport: viewportFor({ width: 1400, height: 1000 }) });
    const pageErrors: string[] = [];
    page.on("pageerror", (e) => pageErrors.push(String(e)));

    let n = 0;
    const step = async (
      name: string,
      opts: { waitsOn?: string[]; needs?: string[] },
      body: () => Promise<string>,
    ): Promise<void> => {
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
      if (out) await page.screenshot({ path: join(out, `cross-probe-${n}-${name.replace(/\W+/g, "-")}.png`) }).catch(() => {});
      rows.push({ step: name, design: label, ms: Date.now() - t0, ...waitedOn(ok, waitsOn, detail) });
    };

    try {
      // Two controls, which pass today. The design opens and lists both the part's sheet and its
      // board, and the board draws the part's footprint with its data-ref key.
      await step("open", {}, async () => {
        await page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view`, { waitUntil: "networkidle" });
        await page.waitForFunction(() => document.querySelectorAll("#svg-view svg *").length > 0, null, { timeout: 60_000 });
        for (const title of [`/${part.sheet}`, `/${footprint.sheet}`, "board"]) {
          await page.locator(`#sheet-overview .sheet-tile[title="${title}"]`).waitFor({ timeout: 30_000 });
        }
        return `${await page.locator("#sheet-overview .sheet-tile").count()} sheets listed, ${part.sheet}, ${footprint.sheet} and the board among them`;
      });

      await step("board draws the part", { needs: ["open"] }, async () => {
        await page.locator('#sheet-overview .sheet-tile[title="board"]').click();
        const fp = page.locator(`#svg-view [data-ref="${part.ref}"]`);
        await fp.first().waitFor({ state: "attached", timeout: 120_000 });
        return `${await fp.count()} elements of ${part.ref}'s footprint drawn`;
      });

      await step("two views", { waitsOn: ["#1326"], needs: ["board draws the part"] }, async () => {
        const vp = page.viewportSize() ?? { width: 0, height: 0 };
        for (const v of ["schematic", "board"] as const) {
          if ((await page.locator(`[data-artifact-view="${v}"]`).count()) === 0) throw new Error(`the page has no data-artifact-view="${v}" element`);
          await view(page, v).locator("svg").first().waitFor({ state: "visible", timeout: 10_000 });
          const b = await box(view(page, v));
          if (!b || b.width < 50 || b.height < 50) throw new Error(`the ${v} view has no room to draw in`);
          if (b.right < 0 || b.bottom < 0 || b.left > vp.width || b.top > vp.height) throw new Error(`the ${v} view is off screen`);
        }
        return "a schematic view and a board view, both on screen";
      });

      await step(`click ${part.ref}`, { waitsOn: ["#1325", "#1326"], needs: ["two views"] }, async () => {
        const sch = view(page, "schematic");
        const tab = sch.locator(`.sheet-tab-label[title="${part.sheet}"]`);
        if (await tab.count()) await tab.first().click();
        await sch.locator(`[data-ref="${part.ref}"]`).first().click({ timeout: 15_000 });
        return ringed(page, part.ref);
      });

      await step("turn the board over", { waitsOn: ["#1327"], needs: [`click ${part.ref}`] }, async () => {
        const board = view(page, "board");
        await board.locator('[data-action="rotate"]').click({ timeout: 10_000 });
        await board.locator('[data-action="flip"]').click({ timeout: 10_000 });
        expect(await board.getAttribute("data-rotation")).toBe("90");
        expect(await board.getAttribute("data-side")).toBe("back");
        return `turned 90 and viewed from the back, ${await ringed(page, part.ref)}`;
      });

      await step(`click ${footprint.ref} on the board`, { waitsOn: ["#1326"], needs: ["two views"] }, async () => {
        await view(page, "board").locator(`[data-ref="${footprint.ref}"]`).first().click({ timeout: 15_000 });
        const sch = view(page, "schematic");
        await sch.locator(`.sheet-tab.active .sheet-tab-label[title="${footprint.sheet}"]`).waitFor({ timeout: 15_000 });
        return `opened ${footprint.sheet}, ${await centred(sch, sch.locator(`[data-ref="${footprint.ref}"]`), footprint.ref)}`;
      });

      await step("click a net's copper", { waitsOn: ["#1326"], needs: ["two views"] }, async () => {
        await view(page, "board").locator(`[data-kind="net"][data-net="${net}"]`).first().click({ timeout: 15_000 });
        const lit = view(page, "schematic").locator(".highlight-overlay svg *");
        await lit.first().waitFor({ timeout: 15_000 });
        return `${net} lit ${await lit.count()} elements on the schematic`;
      });

      await step("no page errors", {}, async () => {
        if (pageErrors.length) throw new Error(pageErrors.join("; "));
        return "none";
      });
    } finally {
      const report = `## Cross-probe walk, ${walk.about}\n\n${table(rows)}\n`;
      process.stdout.write(`\n${report}\n`);
      if (out) writeFileSync(join(out, "cross-probe.md"), report);
      await page.close();
    }
    const broken = rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`);
    expect(exitCode(rows), broken.join("\n")).toBe(0);
  }, 300_000);
});
