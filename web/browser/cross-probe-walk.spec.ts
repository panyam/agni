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
// Mission #1586 grows the walk by a step per ticket (agni issue 1587 and its siblings). A board view
// turns to the side a selection is on and shows the copper layers it uses, unless its "Follow side"
// checkbox (data-action="follow-side") is off. The steps for #1588 to #1595 name the controls those
// tickets build, and wait on them:
// - data-action="selection-back" and "selection-forward" step through what was selected (#1588)
// - data-action="next-sheet" opens the next sheet that draws the selection (#1589)
// - data-action="agreement" opens [data-agreement], which counts the parts both views place (#1590)
// - an ODB++ job packed in a .7z, the cut's board as designs/jetson-cut-odb, opens as a design (#1591)
// - a board layer row states its thickness in [data-thickness] (#1592)
// - data-action="explode" sets data-exploded="true" on the board view (#1593)
// - data-action="bookmark" takes a note in [data-bookmark-note], and [data-bookmark] lists it after
//   a reload (#1594)
// - the latest release carries a Windows build (#1595)
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
import { showPanel } from "./panels.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";
import { PANE, TITLE, centreOf, centredIn, clearPoint, extent, lit, openBeside, picked, zoomTo } from "./picks.js";

const base = (): string => inject("baseUrl");
// Each walk names a part to click on the schematic, a footprint to click on the board, and a net to
// click on the board's copper. The part and the net are drawn on one sheet, and the footprint's part
// on another, so the canvas has to change sheets.
//
// On Jetson, C133 is a back-side capacitor on "USB Debug, PD", TP76 a front-side test point on
// "Power", and C133's pin 1 is on the net. Q2's gate net on "Power" also runs on In5.Cu, and GND is
// drawn on both sheets. On Clay R6, an OrCAD design on its Allegro board (agni
// issue 1382), R28 is on the top page, TP3 a test point on the IMU block's page, and WIFI_TX runs
// across the top page with the same name on the board. Clay places every part on its top, so R28's
// step turns the board back to the top. VBAT is drawn on the top page and three others, and runs on
// In2.Cu. Clay's test points are drawn at the end of a
// long leg, and a click on one's pad lights its pin, so the canvas frames the pin with the part it
// belongs to (agni issue 1419).
const jetsonPicks = {
  part: { ref: "C133", sheet: "USB Debug, PD", side: "back" },
  footprint: { ref: "TP76", sheet: "Power" },
  net: "/USB Debug, PD/FTDI_3V3",
  inner: { net: "Net-(Q2-G)", sheet: "Power", layer: "In5.Cu" },
  acrossSheets: "GND",
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
    part: { ref: "R28", sheet: "PAGE1", side: "front" },
    footprint: { ref: "TP3", sheet: "IMU" },
    net: "WIFI_TX",
    inner: { net: "VBAT", sheet: "PAGE1", layer: "In2.Cu" },
    acrossSheets: "VBAT",
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

// The other side of the board from side.
const opposite = (side: string): string => (side === "back" ? "front" : "back");

// layerShown reads a board view's layer list for whether a row is on.
async function layerShown(page: Page, name: string): Promise<boolean> {
  const box = page.locator(`${PANE} .sheetview-layer-pop input[aria-label="${name}"]`).first();
  if ((await box.count()) === 0) throw new Error(`the board view lists no ${name} row`);
  return box.isChecked();
}

// setLayer turns one row of a board view's layer list on or off, as a reader does from its Layers
// button, and closes the list again.
async function setLayer(page: Page, name: string, on: boolean): Promise<void> {
  if ((await layerShown(page, name)) === on) return;
  const board = page.locator(PANE).first();
  await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
  await board.locator(`.sheetview-layer-pop input[aria-label="${name}"]`).click({ timeout: 10_000 });
  await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
  if ((await layerShown(page, name)) !== on) throw new Error(`${name} did not turn ${on ? "on" : "off"}`);
}

// turnTo turns a board view to a side by hand, with its left-right flip.
async function turnTo(page: Page, side: string): Promise<void> {
  const board = page.locator(PANE).first();
  if ((await board.getAttribute("data-side")) !== side) await board.locator('[data-action="flip-h"]').click({ timeout: 10_000 });
  if ((await board.getAttribute("data-side")) !== side) throw new Error(`the board view did not turn to its ${side}`);
}

// waitForSide waits for a board view to show a side.
async function waitForSide(page: Page, side: string): Promise<void> {
  await page.waitForFunction(({ pane, side }) => document.querySelector(pane)?.getAttribute("data-side") === side, { pane: PANE, side }, { timeout: 10_000 })
    .catch(async () => {
      throw new Error(`the board view shows its ${await page.locator(PANE).first().getAttribute("data-side")}, not its ${side}`);
    });
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

// findIn zooms a view out from its middle until an element's centre is inside it. A view that is
// still drawing its sheet has no box for the element yet, so findIn gives it a few seconds.
async function findIn(page: Page, host: string, el: string): Promise<void> {
  for (let i = 0, waited = 0; i < 15; i++) {
    const h = await page.locator(host).first().boundingBox();
    const b = await page.locator(el).first().boundingBox();
    if ((!h || !b) && waited < 20) {
      waited++;
      i--;
      await page.waitForTimeout(250);
      continue;
    }
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
  await showPanel(page, "overview");
  const tiles = page.locator(".sheet-tile");
  for (let i = 0; i < (await tiles.count()); i++) {
    if (((await tiles.nth(i).locator(".sheet-tile-name").textContent()) ?? "").trim() === name) {
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
  const { part, footprint, net, inner, acrossSheets } = walk;
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
            await page.locator(`#sheet-overview .sheet-tile[title="${title}"]`).waitFor({ state: "attached", timeout: 30_000 });
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

        // A reader has the board turned away from the part, as it was left after the last one.
        await step(`turn the board away from ${part.ref}`, { needs: ["two views"] }, async () => {
          await turnTo(page, opposite(part.side));
          return `the board view shows its ${opposite(part.side)}, and ${part.ref} is on its ${part.side}`;
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

        // The board view turns to the part's side on its own and shows its copper (agni issue 1587).
        await step(`the board turns to ${part.ref}'s side`, { needs: [`click ${part.ref}`, `turn the board away from ${part.ref}`] }, async () => {
          await waitForSide(page, part.side);
          const copper = part.side === "back" ? "B.Cu" : "F.Cu";
          if (!(await layerShown(page, copper))) throw new Error(`${copper} is hidden`);
          const shown = await page.locator(footprintOf(part.ref)).first().evaluate((e) => getComputedStyle(e).display !== "none");
          if (!shown) throw new Error(`${part.ref}'s footprint is hidden`);
          return `turned to its ${part.side} with ${copper} on, ${await centres(page, footprintOf(part.ref), BOARD, `${part.ref}'s footprint`)}`;
        });

        await step("turn the board over", { needs: [`click ${part.ref}`] }, async () => {
          const board = page.locator(PANE).first();
          for (const a of ["rotate", "flip-h"]) {
            if ((await board.locator(`[data-action="${a}"]`).count()) === 0) throw new Error(`the board view has no data-action="${a}" control`);
          }
          await board.locator('[data-action="rotate"]').click({ timeout: 10_000 });
          await board.locator('[data-action="flip-h"]').click({ timeout: 10_000 });
          expect(await board.getAttribute("data-rotation")).toBe("90");
          expect(await board.getAttribute("data-side")).toBe(opposite(part.side));
          return `turned 90 and viewed from the ${opposite(part.side)}, ${await centres(page, footprintOf(part.ref), BOARD, `${part.ref}'s footprint`)}`;
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
              throw new Error(`the canvas stayed on ${((await page.locator(".sheet-tile.active .sheet-tile-name").textContent()) ?? "").trim()}, which does not draw ${net}`);
            });
          await lit(page, SHEET);
          const wires = await page.locator(`${SHEET} [data-kind="net"][data-net="${net}"]`).count();
          if (wires === 0) throw new Error(`${part.sheet} draws no wire of ${net}`);
          return `the canvas opened ${part.sheet} and lit ${net}`;
        });

        // A reader who hid an inner layer clicks a net routed on it on the schematic, and the board
        // view shows that layer again (agni issue 1587).
        await step(`click ${inner.net} on the schematic`, { needs: ["two views"] }, async () => {
          await setLayer(page, inner.layer, false);
          await tile(page, inner.sheet);
          await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="${inner.net}"]`);
          const at = await clickableNet(page, SHEET, inner.net);
          await page.mouse.click(at.x, at.y);
          await picked(page, inner.net);
          await page.waitForFunction(
            ({ pane, layer }) => (document.querySelector(`${pane} .sheetview-layer-pop input[aria-label="${layer}"]`) as HTMLInputElement | null)?.checked === true,
            { pane: PANE, layer: inner.layer },
            { timeout: 10_000 },
          ).catch(() => {
            throw new Error(`${inner.layer} stayed hidden`);
          });
          return `the board view turned ${inner.layer} back on for ${inner.net}`;
        });

        // With its Follow side box off, the board view keeps the side and layers its reader set. The
        // two steps above, with the box on, are this step's positive control.
        await step("with Follow side off, the board stays put", { needs: [`the board turns to ${part.ref}'s side`, `click ${inner.net} on the schematic`] }, async () => {
          const board = page.locator(PANE).first();
          await board.locator('[data-action="follow-side"]').uncheck({ timeout: 10_000 });
          await turnTo(page, opposite(part.side));
          await setLayer(page, inner.layer, false);
          await tile(page, part.sheet);
          await zoomTo(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`);
          const at = await clearPoint(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`, part.ref);
          await page.mouse.click(at.x, at.y);
          await picked(page, part.ref);
          await tile(page, inner.sheet);
          await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="${inner.net}"]`);
          const n = await clickableNet(page, SHEET, inner.net);
          await page.mouse.click(n.x, n.y);
          await picked(page, inner.net);
          // The view has nothing to wait for when it does nothing, so the step gives a follow time to
          // land before it reads the view.
          await page.waitForTimeout(1500);
          const side = await board.getAttribute("data-side");
          if (side !== opposite(part.side)) throw new Error(`the board view turned to its ${side}`);
          if (await layerShown(page, inner.layer)) throw new Error(`${inner.layer} came back on`);
          await board.locator('[data-action="follow-side"]').check({ timeout: 10_000 });
          return `clicked ${part.ref} and ${inner.net}, and the view kept its ${side} and left ${inner.layer} hidden`;
        });

        // The rest of the mission (#1586). Each step waits on the ticket that builds it.
        await step(`step back to ${part.ref}`, { needs: ["with Follow side off, the board stays put"], waitsOn: ["#1588"] }, async () => {
          await page.locator('[data-action="selection-back"]').first().click({ timeout: 5_000 });
          await picked(page, part.ref);
          await page.locator('[data-action="selection-forward"]').first().click({ timeout: 5_000 });
          await picked(page, inner.net);
          return `stepped back to ${part.ref} and forward to ${inner.net}`;
        });

        await step(`step through the sheets that draw ${acrossSheets}`, { needs: ["two views"], waitsOn: ["#1589"] }, async () => {
          const before = (await page.locator(".sheet-tile.active .sheet-tile-name").innerText()).trim();
          await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="${acrossSheets}"]`);
          const at = await clickableNet(page, SHEET, acrossSheets);
          await page.mouse.click(at.x, at.y);
          await picked(page, acrossSheets);
          await page.locator('[data-action="next-sheet"]').first().click({ timeout: 5_000 });
          await page.waitForFunction((b) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent !== b, before, { timeout: 10_000 });
          await lit(page, SHEET);
          return `went from ${before} to ${(await page.locator(".sheet-tile.active .sheet-tile-name").innerText()).trim()}`;
        });

        await step("see how well the schematic and the board agree", { needs: ["two views"], waitsOn: ["#1590"] }, async () => {
          await page.locator('[data-action="agreement"]').first().click({ timeout: 5_000 });
          const text = (await page.locator("[data-agreement]").first().innerText({ timeout: 10_000 })).trim();
          if (!/\d+ of \d+ parts/.test(text)) throw new Error(`the agreement reads "${text}"`);
          return text.split("\n")[0];
        });

        await step("read each board layer's thickness", { needs: ["two views"], waitsOn: ["#1592"] }, async () => {
          const board = page.locator(PANE).first();
          await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          const n = await board.locator(".sheetview-layer-pop [data-thickness]").count();
          await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          if (n === 0) throw new Error("no layer row states a thickness");
          return `${n} layer rows state a thickness`;
        });

        await step("explode the layer stack", { needs: ["two views"], waitsOn: ["#1592", "#1593"] }, async () => {
          const board = page.locator(PANE).first();
          await board.locator('[data-action="explode"]').click({ timeout: 5_000 });
          expect(await board.getAttribute("data-exploded")).toBe("true");
          await board.locator('[data-action="explode"]').click({ timeout: 5_000 });
          return "the board view showed its layers apart and put them back";
        });

        await step("a Windows build is published", { waitsOn: ["#1595"] }, async () => {
          const r = await fetch("https://api.github.com/repos/panyam/agni/releases/latest", { signal: AbortSignal.timeout(10_000) });
          const assets: { name: string }[] = ((await r.json()) as { assets?: { name: string }[] }).assets ?? [];
          const win = assets.find((a) => /windows/i.test(a.name));
          if (!win) throw new Error(`the latest release has no Windows build among ${assets.length} assets`);
          return win.name;
        });

        await step("no page errors", {}, async () => {
          if (pageErrors.length) throw new Error(pageErrors.join("; "));
          return "none";
        });

        // These two leave the page, so they go after the page error check, and a page error on the way out
        // fails neither.
        await step(`bookmark ${part.ref} and find it after a reload`, { needs: ["two views"], waitsOn: ["#1594"] }, async () => {
          await tile(page, part.sheet);
          await zoomTo(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`);
          const at = await clearPoint(page, `${SHEET} [data-kind="component"][data-ref="${part.ref}"]`, part.ref);
          await page.mouse.click(at.x, at.y);
          await picked(page, part.ref);
          await page.locator('[data-action="bookmark"]').first().click({ timeout: 5_000 });
          await page.locator("[data-bookmark-note]").first().fill("suspect, check its value");
          await page.keyboard.press("Enter");
          await page.reload({ waitUntil: "networkidle" });
          const mark = page.locator("[data-bookmark]").filter({ hasText: part.ref });
          await mark.first().waitFor({ timeout: 30_000 });
          return (await mark.first().innerText()).trim();
        });

        await step("open an ODB++ job packed in a .7z", { waitsOn: ["#1591"] }, async () => {
          await page.goto(`${base()}/designs/probe/designs/jetson-cut-odb/view?mode=svg`, { waitUntil: "networkidle" });
          await page.locator('#sheet-overview .sheet-tile[title="board"]').waitFor({ timeout: 30_000 });
          return "the .7z job opened with its board";
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
