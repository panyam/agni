// The cross-probe walk, mission #1321's exercise (agni issue 1323). An engineer at the bench opens a
// design's schematic and board as two views, clicks a part in one and finds it in the other, and
// turns the board over to match how it lies on the bench.
//
// The walk was written before the viewer can do any of it, so each step names the tickets it waits
// on. A waiting step that fails reads "not available" and does not fail the walk, and a waiting step
// that passes DOES fail it (exercise-report.ts, waitedOn), so the PR that makes a step pass also
// removes its wait here. A step that needs an earlier one reads "not reached" until that one passes.
//
// It drives the viewer as #1326 (PR 1375) built it. The canvas is the schematic view, and the Board
// opens beside it in a sheet view ([data-sheetview]) from its tile in Sheets. A view may be docked
// anywhere or floating, at any size, so the walk asks only that each is on screen. A pick in one
// lights the same part or net in the other and centres it there. Clicks land by point, as a reader's
// do, at a point clear of pads and neighbours (picks.ts). Step 3 turns the board view as #1327 (PR
// 1378) built it, through these names:
// - the board view's controls data-action="rotate" (a quarter turn) and data-action="flip-h" and
//   "flip-v" (mirror the view). An odd number of flips turns the board over, seen from underneath
//   (agni issue 1472).
// - the view states its view as data-rotation (degrees) and data-side ("front" or "back")
// A ticket that names them differently changes them here in the same PR.
//
// `make exercise-cross-probe` tells the walk which of the tickets it waits on are closed
// (CLOSED_TICKETS). A step then stops waiting on a closed one, so a step whose tickets have all
// closed must pass. The gate runs offline and leaves this out, so there only the waits as written
// apply.
//
// The gate walks a nineteen-part cut of Jetson (examples/cross-probe-project) and Clay R6, an OrCAD
// design on its Allegro board (tools/samples/cadence/clay-r6). `make exercise-cross-probe` walks the
// one WALK_DESIGN names, the whole Jetson board (examples/jetson-project) with WALK_DESIGN=jetson,
// headed unless HEADED=0, and writes the step table and a screenshot per step to EXERCISE_OUT.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { launch, viewportFor } from "./browser.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";
import { PANE, TITLE, centreOf, centredIn, clearPoint, extent, lit, openBeside, picked, zoomTo } from "./picks.js";

const base = (): string => inject("baseUrl");
// Each walk names a part to click on the schematic, a footprint to click on the board, and a net to
// click on the board's copper. The part and the net are drawn on one sheet, and the footprint's part
// on another, so the canvas has to change sheets.
//
// On Jetson, C133 is a back-side capacitor on "USB Debug, PD", TP76 a front-side test point on
// "Power", and C133's pin 1 is on the net. On Clay R6, an OrCAD design on its Allegro board (agni
// issue 1382), R28 is on the top page, TP3 a test point on the IMU block's page, and WIFI_TX runs
// across the top page with the same name on the board. Clay's test points are drawn at the end of a
// long leg, and a click on one's pad lights its pin, so the canvas frames the pin with the part it
// belongs to (agni issue 1419).
const jetsonPicks = {
  part: { ref: "C133", sheet: "USB Debug, PD" },
  footprint: { ref: "TP76", sheet: "Power" },
  net: "/USB Debug, PD/FTDI_3V3",
};
const walks = {
  cut: {
    mount: "probe",
    path: "designs/jetson-cut",
    about: "nineteen parts of Antmicro's Jetson AGX Thor baseboard, cut with their copper",
    ...jetsonPicks,
  },
  jetson: {
    mount: "jetson",
    path: "designs/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module",
    ...jetsonPicks,
  },
  clay: {
    mount: "cadence",
    path: "clay-r6",
    about: "Clay R6, its OrCAD Capture design on its Allegro board",
    part: { ref: "R28", sheet: "PAGE1" },
    footprint: { ref: "TP3", sheet: "IMU" },
    net: "WIFI_TX",
  },
};
type Walk = (typeof walks)[keyof typeof walks];
// The gate walks the Jetson cut and Clay R6. WALK_DESIGN walks the one it names.
const chosen = process.env.WALK_DESIGN ? [process.env.WALK_DESIGN] : ["cut", "clay"];
for (const w of chosen) {
  if (!(w in walks)) throw new Error(`WALK_DESIGN=${w} names no walk; use one of ${Object.keys(walks).join(", ")}`);
}


let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

const SHEET = "#svg-view";
const BOARD = `${PANE} .sheetview-canvas`;
// footprintOf is a part's pick rect in the board view, the invisible box the board draws over its
// pads on whichever side it sits. Its silkscreen and fab text carry the same key, and are the wrong
// thing to aim at.
const footprintOf = (ref: string): string => `${BOARD} .picks-front [data-ref="${ref}"], ${BOARD} .picks-back [data-ref="${ref}"]`;

// onScreen checks that a host is drawn with room to draw in, somewhere in the window.
async function onScreen(page: Page, host: string, name: string): Promise<void> {
  await page.locator(`${host} svg`).first().waitFor({ state: "visible", timeout: 30_000 });
  const b = await page.locator(host).first().boundingBox();
  const vp = page.viewportSize() ?? { width: 0, height: 0 };
  if (!b || b.width < 50 || b.height < 50) throw new Error(`the ${name} view has no room to draw in`);
  if (b.x + b.width < 0 || b.y + b.height < 0 || b.x > vp.width || b.y > vp.height) throw new Error(`the ${name} view is off screen`);
}

// centres waits for a part to come to the middle third of a host, where a reveal puts it.
async function centres(page: Page, el: string, host: string, name: string): Promise<string> {
  await page.waitForFunction(
    ({ el, host }) => {
      const e = document.querySelector(el)?.getBoundingClientRect();
      const h = document.querySelector(host)?.getBoundingClientRect();
      if (!e || !h) return false;
      const cx = e.left + e.width / 2, cy = e.top + e.height / 2;
      return cx > h.left + h.width / 3 && cx < h.right - h.width / 3 && cy > h.top + h.height / 3 && cy < h.bottom - h.height / 3;
    },
    { el, host },
    { timeout: 30_000 },
  ).catch(() => {
    throw new Error(`${name} did not come to the middle of the view`);
  });
  if (!(await centredIn(page, el, host))) throw new Error(`${name} is off the middle of the view`);
  return `${name} lit and centred`;
}

// netPoint finds a point on any of a net's copper in a host that the picker reads as that net
// alone, or null. It tries each piece's two diagonals and its midlines, so a track at any angle has
// points on it.
async function netPoint(page: Page, host: string, name: string): Promise<{ x: number; y: number } | null> {
  return page.evaluate(
    ({ host, name }) => {
      const keyed = (e: Element | null) => e?.closest("[data-kind]") ?? null;
      const mine = (e: Element | null) => !e || (e.getAttribute("data-kind") === "net" && e.getAttribute("data-net") === name);
      const h = document.querySelector(host)!.getBoundingClientRect();
      // The viewer's picker looks a few pixels around a click and takes a pad over a track, so the
      // point needs the whole ring clear of anything but this net's copper.
      const ring = [-6, -3, 0, 3, 6];
      for (const el of document.querySelectorAll(`${host} [data-kind="net"]`)) {
        if (el.getAttribute("data-net") !== name) continue;
        const r = el.getBoundingClientRect();
        for (const f of [0.5, 0.4, 0.6, 0.3, 0.7, 0.2, 0.8]) {
          const along = [
            [r.left + r.width * f, r.top + r.height * f],
            [r.left + r.width * f, r.bottom - r.height * f],
            [r.left + r.width * f, r.top + r.height / 2],
            [r.left + r.width / 2, r.top + r.height * f],
          ];
          for (const [px, py] of along) {
            const x = Math.round(px), y = Math.round(py);
            if (x < h.left + 8 || x > h.right - 8 || y < h.top + 8 || y > h.bottom - 8) continue;
            const c = keyed(document.elementFromPoint(x, y));
            if (c && mine(c) && ring.every((dx) => ring.every((dy) => mine(keyed(document.elementFromPoint(x + dx, y + dy)))))) return { x, y };
          }
        }
      }
      return null;
    },
    { host, name },
  );
}

// findIn zooms a view out from its middle until an element's centre is inside it.
async function findIn(page: Page, host: string, el: string): Promise<void> {
  for (let i = 0; i < 15; i++) {
    const h = await page.locator(host).first().boundingBox();
    const b = await page.locator(el).first().boundingBox();
    if (!h || !b) throw new Error(`${el} is not drawn`);
    const cx = b.x + b.width / 2, cy = b.y + b.height / 2;
    if (cx > h.x && cx < h.x + h.width && cy > h.y && cy < h.y + h.height) return;
    await page.mouse.move(h.x + h.width / 2, h.y + h.height / 2);
    await page.mouse.wheel(0, 400);
    await page.waitForTimeout(150);
  }
  throw new Error(`${el} stayed out of the view`);
}

// clickableNet finds a point on a net's copper to click, zooming the view in on the middle of the
// net's longest piece in it until one is clear of the pads and labels around it, as a reader would.
async function clickableNet(page: Page, host: string, name: string): Promise<{ x: number; y: number }> {
  for (let i = 0; i < 10; i++) {
    const at = await netPoint(page, host, name);
    if (at) return at;
    const t = await page.evaluate(({ host, name }) => {
      const h = document.querySelector(host)!.getBoundingClientRect();
      let best: { x: number; y: number; long: number } | undefined;
      for (const el of document.querySelectorAll(`${host} [data-kind="net"]`)) {
        if (el.getAttribute("data-net") !== name) continue;
        const r = el.getBoundingClientRect();
        const x = r.left + r.width / 2, y = r.top + r.height / 2;
        if (x < h.left || x > h.right || y < h.top || y > h.bottom) continue;
        const long = Math.max(r.width, r.height);
        if (!best || long > best.long) best = { x, y, long };
      }
      return best;
    }, { host, name });
    if (!t) throw new Error(`${name} has no copper in the view`);
    await page.mouse.move(t.x, t.y);
    await page.mouse.wheel(0, -400);
    await page.waitForTimeout(200);
  }
  throw new Error(`no point on ${name}'s copper picks only that net, even zoomed in`);
}

// inReach lists every visible control in a panel that a reader cannot reach (agni issue 1397). A
// control has to lie inside its own panel, sit clear of every other control there, and be the
// topmost element at its centre. The middle test alone misses a bar that overflows its panel,
// because the overflowing controls paint over the next panel and are still on top. The readout takes
// no pointer events, so it is held to the first two tests only.
async function inReach(page: Page, panel: string, items: string): Promise<string[]> {
  return page.evaluate(
    ({ panel, items }) => {
      const p = document.querySelector(panel)?.getBoundingClientRect();
      if (!p) return [`no ${panel}`];
      const name = (e: Element) =>
        e.id ? `#${e.id}` : ((e as HTMLElement).dataset.action ?? (e.tagName === "SELECT" ? `select.${e.className}` : e.textContent?.trim().slice(0, 20) || e.tagName));
      const shown = [...document.querySelectorAll(items)].filter((e) => {
        const r = e.getBoundingClientRect();
        return r.width > 0 && r.height > 0;
      });
      const bad: string[] = [];
      for (const e of shown) {
        const r = e.getBoundingClientRect();
        if (r.left < p.left - 1 || r.right > p.right + 1 || r.top < p.top - 1 || r.bottom > p.bottom + 1) bad.push(`${name(e)} runs outside its panel`);
        for (const o of shown) {
          if (o === e || o.contains(e) || e.contains(o)) continue;
          const q = o.getBoundingClientRect();
          if (r.left < q.right && q.left < r.right && r.top < q.bottom && q.top < r.bottom && shown.indexOf(o) > shown.indexOf(e)) bad.push(`${name(e)} and ${name(o)} overlap`);
        }
        if (getComputedStyle(e).pointerEvents === "none") continue;
        const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
        if (!top || !(top === e || e.contains(top))) bad.push(`${name(e)} is covered`);
      }
      return shown.length === 0 ? [`nothing matches ${items}`] : bad;
    },
    { panel, items },
  );
}

// tile clicks a Sheets tile by its exact name, which puts that sheet on the canvas.
async function tile(page: Page, name: string): Promise<void> {
  const tiles = page.locator(".sheet-tile");
  for (let i = 0; i < (await tiles.count()); i++) {
    if ((await tiles.nth(i).locator(".sheet-tile-name").innerText()).trim() === name) {
      await tiles.nth(i).click();
      await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === n, name, { timeout: 30_000 });
      return;
    }
  }
  throw new Error(`no Sheets tile is named ${name}`);
}

const closed = new Set((process.env.CLOSED_TICKETS ?? "").split(",").filter((t) => t !== "").map((t) => `#${t.replace(/^#/, "")}`));

for (const name of chosen) {
  const walk: Walk = walks[name as keyof typeof walks];
  const label = `${walk.path.split("/").pop()} (${walk.mount})`;
  const { part, footprint, net } = walk;
  describe(`the cross-probe walk on ${name}`, () => {
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
        const open = waitsOn.filter((t) => !closed.has(t));
        const shut = waitsOn.filter((t) => closed.has(t));
        if (shut.length > 0) detail += ` (${shut.join(", ")} closed)`;
        rows.push({ step: name, design: label, ms: Date.now() - t0, ...waitedOn(ok, open, detail) });
      };

      try {
        // Two controls, which pass today. The design opens and lists both the part's sheet and its
        // board, and the board draws the part's footprint with its data-ref key.
        await step("open", {}, async () => {
          await page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view?mode=svg`, { waitUntil: "networkidle" });
          await page.waitForFunction(() => document.querySelectorAll("#svg-view svg *").length > 0, null, { timeout: 60_000 });
          for (const title of [`/${part.sheet}`, `/${footprint.sheet}`, "board"]) {
            await page.locator(`#sheet-overview .sheet-tile[title="${title}"]`).waitFor({ timeout: 30_000 });
          }
          return `${await page.locator("#sheet-overview .sheet-tile").count()} sheets listed, ${part.sheet}, ${footprint.sheet} and the board among them`;
        });

        await step("two views", { needs: ["open"] }, async () => {
          await openBeside(page, "Board");
          await onScreen(page, SHEET, "schematic");
          await onScreen(page, BOARD, "board");
          const fp = page.locator(footprintOf(part.ref));
          await fp.first().waitFor({ state: "attached", timeout: 60_000 });
          return `the schematic on the canvas and the board in a view beside it, ${part.ref}'s footprint drawn`;
        });

        // A reader with the two views side by side reaches every control on both (agni issue 1397).
        // The board's bar once ran past its panel and over the Checks panel, and the canvas's mode
        // bar covered its readout.
        await step("every control in reach", { needs: ["two views"] }, async () => {
          const bad = [
            ...(await inReach(page, '[data-dock-panel="canvas"]', '#readout, .canvas-toolbar button, .canvas-toolbar select')),
            ...(await inReach(page, PANE, `${PANE} .sheetview-bar > *`)),
          ];
          if (bad.length > 0) throw new Error(bad.join("; "));
          return "the canvas's readout and controls and the board view's bar, each inside its panel and uncovered";
        });

        await step(`click ${part.ref}`, { needs: ["two views"] }, async () => {
          await tile(page, part.sheet);
          // A reader zooms to a small part before clicking it, and at fit zoom a capacitor has no
          // point clear of its pins.
          await zoomTo(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`);
          const at = await clearPoint(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`, part.ref);
          await page.mouse.click(at.x, at.y);
          await picked(page, part.ref);
          await lit(page, BOARD);
          return `picked on ${part.sheet}, ${await centres(page, footprintOf(part.ref), BOARD, `${part.ref}'s footprint`)}`;
        });

        await step("turn the board over", { needs: [`click ${part.ref}`] }, async () => {
          const board = page.locator(PANE).first();
          for (const a of ["rotate", "flip-h"]) {
            if ((await board.locator(`[data-action="${a}"]`).count()) === 0) throw new Error(`the board view has no data-action="${a}" control`);
          }
          await board.locator('[data-action="rotate"]').click({ timeout: 10_000 });
          await board.locator('[data-action="flip-h"]').click({ timeout: 10_000 });
          expect(await board.getAttribute("data-rotation")).toBe("90");
          expect(await board.getAttribute("data-side")).toBe("back");
          return `turned 90 and viewed from the back, ${await centres(page, footprintOf(part.ref), BOARD, `${part.ref}'s footprint`)}`;
        });

        await step(`click ${footprint.ref} on the board`, { needs: ["two views"] }, async () => {
          // TP76 is on the front, and the bottom view draws the back copper over it, as the board looks
          // from underneath. So a reader turns the board back over to reach it.
          const board = page.locator(PANE).first();
          if ((await board.getAttribute("data-side")) === "back") await board.locator('[data-action="flip-h"]').click({ timeout: 10_000 });
          // The board view is still zoomed in on the last part, so a reader zooms out to find this one
          // and back in to click it.
          await findIn(page, BOARD, footprintOf(footprint.ref));
          await zoomTo(page, footprintOf(footprint.ref));
          // A test point is one pad, which covers its whole footprint, so the click picks the pad. A
          // pad's pin is the part's, and the canvas finds the part from either.
          const at = await centreOf(page, footprintOf(footprint.ref));
          await page.mouse.click(at.x, at.y);
          await page.waitForFunction(
            ({ sel, ref }) => { const t = document.querySelector(sel)?.textContent ?? ""; return t === ref || t.startsWith(`${ref}.`); },
            { sel: TITLE, ref: footprint.ref },
            { timeout: 30_000 },
          );
          await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === n, footprint.sheet, { timeout: 30_000 });
          await lit(page, SHEET);
          return `the canvas opened ${footprint.sheet}, ${await centres(page, `${SHEET} [data-kind="component"][data-ref="${footprint.ref}"]`, SHEET, footprint.ref)}`;
        });

        await step("click a net's copper", { needs: ["two views"] }, async () => {
          await findIn(page, BOARD, `${BOARD} [data-kind="net"][data-net="${net}"]`);
          // A track is a few pixels wide at a whole-board zoom, so a reader zooms in on one first.
          const at = await clickableNet(page, BOARD, net);
          await page.mouse.click(at.x, at.y);
          await picked(page, net);
          await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === n, part.sheet, { timeout: 30_000 })
            .catch(async () => {
              throw new Error(`the canvas stayed on ${(await page.locator(".sheet-tile.active .sheet-tile-name").innerText()).trim()}, which does not draw ${net}`);
            });
          await lit(page, SHEET);
          const wires = await page.locator(`${SHEET} [data-kind="net"][data-net="${net}"]`).count();
          if (wires === 0) throw new Error(`${part.sheet} draws no wire of ${net}`);
          return `the canvas opened ${part.sheet} and lit ${net}`;
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
}
