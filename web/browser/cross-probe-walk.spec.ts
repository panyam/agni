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
// checkbox (data-action="follow-side") is off. The steps for #1588 to #1603 name the controls those
// tickets build, and waited on them until each closed:
// - data-action="selection-back" and "selection-forward" step through what was selected (#1588)
// - data-action="prev-sheet" and "next-sheet" open the previous and next sheet that draws the
//   selection, and [data-sheet-place] reads where the canvas is among them, such as "2 of 4" (#1589)
// - data-action="agreement" opens [data-agreement], which counts the parts both views place (#1590)
// - an ODB++ job packed in a .7z, the cut's board as boards/jetson-cut-odb, opens as a design (#1591)
// - a board layer row states its thickness in [data-thickness], and the stack follows the rows (#1603)
// - data-action="explode" sets data-exploded="true" on the board view (#1593)
// - data-action="stack-bodies" shows the parts' bodies in the stack view, and the canvas counts them
//   in data-stack-bodies (#1352)
// - [data-stack-look="board"] and "layers" switch the stack view's look, and the canvas names it in
//   data-stack-look (#1653)
// The mission's Windows build (#1595) is checked where releases are built, by hack/dist_smoke.sh.
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
import { PANE, TITLE, centreOf, centredIn, clearPoint, clickableNet, extent, findIn, lit, openBeside, picked, zoomTo } from "./picks.js";

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
// agreement is what the agreement panel's first line reads, and pick an entry of its lists to click
// (agni issue 1590). The cut's schematic and board agree, so it has none. Jetson's board splits ten
// of its unconnected nets a pad a net. Clay's Allegro board cuts net names to 31 characters, so a
// long one pairs with the schematic's by its pins.
//
// stack is what the board view's layer list says of F.Cu's thickness and of the whole stack (agni
// issue 1603). Jetson's file states 1.711 mm and its layers sum to it. Clay's Allegro board gives its
// stack in its cross-section and states no total, so its layers' 1.651 mm stands (agni issue 1601).
const jetsonStack = { fCu: "0.035 mm", summary: "1.711 mm, as the file states", thicknessMm: 1.711 };
const clayStack = { fCu: "0.03556 mm", summary: "the layers sum to 1.651 mm, and the file states no board thickness", thicknessMm: 1.651 };
//
// models is how many parts stand at the height their 3D model states and draw that model in the board
// look, at least (agni issue 1652). The cut's parts name Antmicro's glTF library through ${ANT3DMDL},
// which the walk's server maps, and two name a SOT-23-6 the library does not have. Clay's Allegro
// board names no models, so its parts all stay blocks.
type Models = { stated: number; meshed: number } | null;
const walks = {
  cut: {
    mount: "probe",
    path: "boards/jetson-cut",
    about: "nineteen parts of Antmicro's Jetson AGX Thor baseboard, cut with their copper",
    ...jetsonPicks,
    agreement: { parts: "19 of 19 parts on the board", pick: "" },
    stack: jetsonStack as typeof jetsonStack | null,
    models: { stated: 16, meshed: 16 } as Models,
  },
  jetson: {
    mount: "jetson",
    path: "boards/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module",
    ...jetsonPicks,
    agreement: { parts: "1,123 of 1,123 parts on the board", pick: "unconnected-(U8-GL-Pad10)_2" },
    stack: jetsonStack as typeof jetsonStack | null,
    models: { stated: 938, meshed: 938 } as Models,
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
    agreement: { parts: "146 of 149 parts on the board", pick: "PTA0/JTAG_TCLK/SWD_CLK/EZP_CLK/UART0_CTS_B/UART0_COL_B/FTM0_CH5" },
    stack: clayStack as typeof jetsonStack | null,
    models: null as Models,
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

// stepThrough steps the canvas back to the first sheet that draws the selection, then forward through
// each one to the last, checking each is lit and reads its place, then back one. It returns the
// sheets in the order it visited them (agni issue 1589).
async function stepThrough(page: Page): Promise<string[]> {
  const placeOf = async () => {
    await page.waitForFunction(() => /^\d+ of \d+$/.test(document.querySelector("#steps [data-sheet-place]")?.textContent ?? ""), null, { timeout: 10_000 });
    const [at, of] = ((await page.locator("#steps [data-sheet-place]").textContent()) ?? "").split(" of ").map(Number);
    return { at, of };
  };
  const tile = async () => ((await page.locator(".sheet-tile.active .sheet-tile-name").textContent()) ?? "").trim();
  const press = async (action: string, from: string) => {
    await page.locator(`#steps [data-action="${action}"]`).click({ timeout: 5_000 });
    await page.waitForFunction((b) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() !== b, from, { timeout: 15_000 });
    await lit(page, SHEET);
  };
  let { at, of } = await placeOf();
  if (of < 2) throw new Error(`the selection is drawn on ${of} sheet`);
  while (at > 1) {
    await press("prev-sheet", await tile());
    const now = await placeOf();
    if (now.at !== at - 1) throw new Error(`previous went from ${at} of ${of} to ${now.at} of ${now.of}`);
    at = now.at;
  }
  const visited = [await tile()];
  for (let n = 2; n <= of; n++) {
    await press("next-sheet", visited[visited.length - 1]);
    const now = await placeOf();
    if (now.at !== n) throw new Error(`next read ${now.at} of ${now.of} on ${await tile()}, not ${n} of ${of}`);
    visited.push(await tile());
  }
  if (!(await page.locator('#steps [data-action="next-sheet"]').isDisabled())) throw new Error("next is still on at the last sheet");
  await press("prev-sheet", visited[visited.length - 1]);
  if ((await tile()) !== visited[visited.length - 2]) throw new Error(`previous opened ${await tile()}, not ${visited[visited.length - 2]}`);
  return visited;
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
  const { part, footprint, net, inner, acrossSheets, agreement } = walk;
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
          await page.goto(`${base()}/mounts/${walk.mount}/${walk.path}/view?mode=svg`, { waitUntil: "networkidle" });
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

        // The rest of the mission (#1586). Each step waits on the ticket that builds it, and the canvas
        // bar's step controls (#steps) built #1588 and #1589.
        await step(`step back to ${part.ref}`, { needs: ["with Follow side off, the board stays put"] }, async () => {
          await page.locator('#steps [data-action="selection-back"]').click({ timeout: 5_000 });
          await picked(page, part.ref);
          await page.locator('#steps [data-action="selection-forward"]').click({ timeout: 5_000 });
          await picked(page, inner.net);
          return `stepped back to ${part.ref} and forward to ${inner.net}`;
        });

        await step(`step through the sheets that draw ${acrossSheets}`, { needs: ["two views"] }, async () => {
          await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="${acrossSheets}"]`);
          const at = await clickableNet(page, SHEET, acrossSheets);
          await page.mouse.click(at.x, at.y);
          await picked(page, acrossSheets);
          const visited = await stepThrough(page);
          return `${acrossSheets} is drawn on ${visited.length} sheets, and next went through ${visited.join(", ")} and previous back again`;
        });

        // The panel counts what both views hold. A click on an entry selects it, and the board view
        // finds it, a net the board names differently included (agni issue 1590).
        await step("see how well the schematic and the board agree", { needs: ["two views"] }, async () => {
          await page.locator('[data-action="agreement"]').first().click({ timeout: 5_000 });
          const panel = page.locator("[data-agreement]").first();
          await page.waitForFunction(() => /[\d,]+ of [\d,]+ parts/.test(document.querySelector("[data-agreement]")?.textContent ?? ""), null, { timeout: 60_000 })
            .catch(async () => {
              throw new Error(`the agreement reads "${(await panel.innerText()).trim()}"`);
            });
          const first = (await panel.innerText()).trim().split("\n")[0];
          if (first !== agreement.parts) throw new Error(`the agreement reads "${first}", want "${agreement.parts}"`);
          if (!agreement.pick) return first;
          const entry = panel.locator(".agreement-entry", { hasText: agreement.pick }).first();
          const view = await entry.getAttribute("data-agreement-view");
          await entry.click({ timeout: 5_000 });
          // A net the schematic lacks has no Properties of its own, so only a schematic entry is
          // checked there.
          if (view === "schematic") await picked(page, agreement.pick);
          const board = page.locator(PANE).first();
          await page.waitForFunction(({ pane, pick }) => document.querySelector(pane)?.getAttribute("data-followed") === pick, { pane: PANE, pick: agreement.pick }, { timeout: 30_000 })
            .catch(async () => {
              throw new Error(`the board view followed "${await board.getAttribute("data-followed")}", not ${agreement.pick}`);
            });
          if ((await board.getAttribute("data-absent")) !== "false") {
            throw new Error(`the board view says "${((await board.locator(".sheetview-absent").textContent()) ?? "").trim()}"`);
          }
          await page.locator('[data-action="agreement"]').first().click({ timeout: 5_000 });
          return `${first}, and ${agreement.pick} selected and found on the board`;
        });

        await step("read each board layer's thickness", { needs: ["two views"] }, async () => {
          const board = page.locator(PANE).first();
          const pop = board.locator(".sheetview-layer-pop");
          await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          try {
            await pop.locator(".board-layer-row").first().waitFor({ state: "visible", timeout: 10_000 });
            const n = await pop.locator("[data-thickness]").count();
            const stackRows = pop.locator("[data-board-stack] tr");
            if (!walk.stack) {
              if (n > 0 || (await stackRows.count()) > 0) throw new Error(`${n} layer rows state a thickness on a board whose file states no stack`);
              return "the board's file states no stack, and no layer row claims a thickness";
            }
            const fCu = await pop.locator(".board-layer-row", { has: page.locator('input[aria-label="F.Cu"]') }).locator("[data-thickness]").getAttribute("data-thickness");
            if (fCu !== walk.stack.fCu) throw new Error(`F.Cu states "${fCu}", not ${walk.stack.fCu}`);
            const dielectrics = await pop.locator('[data-board-stack] tr[data-kind="dielectric"]').count();
            if (dielectrics === 0) throw new Error("the stack lists no dielectric");
            const summary = ((await pop.locator(".board-stack-summary").textContent()) ?? "").trim();
            if (summary !== walk.stack.summary) throw new Error(`the stack says "${summary}", not "${walk.stack.summary}"`);
            return `${n} layer rows state a thickness, F.Cu ${fCu}, and the stack lists ${await stackRows.count()} rows, ${dielectrics} of them dielectric, at ${summary}`;
          } finally {
            await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          }
        });

        // The stack view (agni issue 1593). At explode 0 the stack draws as thick as the file states. The
        // step shows the first inner layer alone, as a reader does to see past Jetson's pours, and a
        // click on its copper picks its net, which the schematic lights. Leaving puts back the
        // renderer, and the layers the reader had before. The canvas names a clickable point on each
        // shown copper layer in data-stack-probes, since a WebGL drawing has no element to aim at.
        await step("explode the layer stack", { needs: ["two views"] }, async () => {
          const board = page.locator(PANE).first();
          const button = board.locator('[data-action="explode"]');
          if (!walk.stack) {
            if (!(await button.isDisabled())) throw new Error("the Stack button is on for a board whose file states no stack");
            return `the Stack button is off, "${await button.getAttribute("title")}"`;
          }
          const pop = board.locator(".sheetview-layer-pop");
          const shown = () => pop.locator(".board-layer-row input").evaluateAll((ins) => ins.map((i) => [i.getAttribute("aria-label")!, (i as HTMLInputElement).checked] as [string, boolean]));
          const rowOf = (name: string) => pop.locator(".board-layer-row", { has: page.locator(`input[aria-label="${name}"]`) });
          const before = { renderer: await board.getAttribute("data-renderer"), shown: await shown() };
          const innerLayers = before.shown.map(([name]) => name).filter((name) => /^In\d+\.Cu$/.test(name));
          if (innerLayers.length === 0) throw new Error("the board view lists no inner copper layer");
          await button.click({ timeout: 5_000 });
          await page.waitForFunction((p) => document.querySelector(p)?.getAttribute("data-exploded") === "true", PANE, { timeout: 60_000 });
          const gl = board.locator("canvas.sheetview-gl");
          const slider = board.locator("input[data-stack-explode]");
          await slider.fill("0");
          const height = Number(await gl.getAttribute("data-stack-height-mm"));
          if (!(Math.abs(height - walk.stack.thicknessMm) <= 0.001)) throw new Error(`at explode 0 the stack draws ${height} mm tall, not the stated ${walk.stack.thicknessMm} mm`);
          await slider.fill("60");
          // An inner layer that is all plane draws nothing with the Zones row hidden, as on the flat board,
          // so the step shows each inner layer alone until one has copper to click.
          type Probe = { layer: string; net: string; x: number; y: number };
          let inner: Probe | undefined;
          for (const layer of innerLayers) {
            await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
            await rowOf(layer).dblclick({ timeout: 10_000 });
            await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
            await page
              .waitForFunction(({ p, l }) => (JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-probes") ?? "[]") as Probe[]).some((q) => q.layer === l), { p: PANE, l: layer }, { timeout: 5_000 })
              .catch(() => {});
            const probes = await page.evaluate((p) => JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-probes") ?? "[]") as Probe[], PANE);
            inner = probes.find((q) => q.layer === layer);
            if (inner) break;
          }
          if (!inner) throw new Error(`none of ${innerLayers.join(", ")} shown alone offers copper to click`);
          const box = (await gl.boundingBox())!;
          await page.mouse.click(box.x + inner.x, box.y + inner.y);
          await picked(page, inner.net);
          await lit(page, SHEET);
          // The reader's layers go back as they were, one row at a time.
          await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          const now = new Map(await shown());
          for (const [name, on] of before.shown) if (now.get(name) !== on) await rowOf(name).click({ timeout: 10_000 });
          await board.locator('[data-action="layers"]').click({ timeout: 10_000 });
          await button.click({ timeout: 5_000 });
          // Leaving draws the board again in the renderer it had, so the step waits for that drawing.
          await page
            .waitForFunction(({ p, r }) => document.querySelector(p)?.getAttribute("data-exploded") === "false" && document.querySelector(p)?.getAttribute("data-renderer") === r, { p: PANE, r: before.renderer }, { timeout: 60_000 })
            .catch(async () => {
              throw new Error(`the board view came back exploded ${await board.getAttribute("data-exploded")} in ${await board.getAttribute("data-renderer")}, not flat in ${before.renderer}`);
            });
          const after = { shown: await shown() };
          if (JSON.stringify(after.shown) !== JSON.stringify(before.shown)) throw new Error("the board view came back with other layers hidden");
          return `the stack drew ${height} mm tall at explode 0, and a click on ${inner.layer} picked ${inner.net}, which the schematic lit`;
        });

        // Part bodies in the stack view (agni issue 1352). Each placed part stands on its side's face,
        // and a part whose file states no height stands at a default, which the Parts toggle's note
        // counts. A click on a body picks its part, as a click on its pads does. The canvas names a
        // clickable point on a few parts in data-stack-body-probes.
        await step("stand the parts on the stack", { needs: ["explode the layer stack"] }, async () => {
          const board = page.locator(PANE).first();
          const button = board.locator('[data-action="explode"]');
          if (!walk.stack) return "the board's file states no stack, so it has no stack view to stand parts in";
          const renderer = await board.getAttribute("data-renderer");
          await button.click({ timeout: 5_000 });
          await page.waitForFunction((p) => document.querySelector(p)?.getAttribute("data-exploded") === "true", PANE, { timeout: 60_000 });
          const gl = board.locator("canvas.sheetview-gl");
          const counts = JSON.parse((await gl.getAttribute("data-stack-bodies")) ?? "{}") as { parts?: number; defaulted?: number };
          if (!counts.parts) throw new Error("the stack view stands no part on the board");
          const note = ((await board.locator("[data-stack-bodies-note]").textContent()) ?? "").trim();
          if ((counts.defaulted ?? 0) > 0 && !note.includes("default")) throw new Error(`${counts.defaulted} parts stand at a default height, and the Parts note says "${note}"`);
          const stated = counts.parts - (counts.defaulted ?? 0);
          if (walk.models && stated < walk.models.stated) throw new Error(`${stated} parts stand at a stated height, want at least ${walk.models.stated} from their 3D models`);
          type BodyProbe = { ref: string; x: number; y: number };
          const bodyProbes = async () =>
            page.evaluate((p) => JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-body-probes") ?? "[]") as BodyProbe[], PANE);
          await page
            .waitForFunction((p) => (JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-body-probes") ?? "[]") as BodyProbe[]).length > 0, PANE, { timeout: 5_000 })
            .catch(() => {});
          const probe = (await bodyProbes())[0];
          if (!probe) throw new Error(`none of the ${counts.parts} parts offers a point to click`);
          const box = (await gl.boundingBox())!;
          await page.mouse.click(box.x + probe.x, box.y + probe.y);
          await picked(page, probe.ref);
          // With the Parts toggle off, the bodies draw nothing and pick nothing, so no part offers a point.
          await board.locator('[data-action="stack-bodies"]').uncheck({ timeout: 5_000 });
          await page.waitForFunction((p) => document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-body-probes") === "[]", PANE, { timeout: 5_000 }).catch(async () => {
            throw new Error(`with Parts off the canvas still names ${JSON.stringify(await bodyProbes())}`);
          });
          await board.locator('[data-action="stack-bodies"]').check({ timeout: 5_000 });
          await button.click({ timeout: 5_000 });
          await page
            .waitForFunction(({ p, r }) => document.querySelector(p)?.getAttribute("data-exploded") === "false" && document.querySelector(p)?.getAttribute("data-renderer") === r, { p: PANE, r: renderer }, { timeout: 60_000 })
            .catch(async () => {
              throw new Error(`the board view came back exploded ${await board.getAttribute("data-exploded")} in ${await board.getAttribute("data-renderer")}, not flat in ${renderer}`);
            });
          return `${counts.parts} parts stand on the board, ${counts.defaulted} at a default height ("${note}"), and a click on ${probe.ref}'s body picked it`;
        });

        // The board look (agni issue 1653) draws the board as it is made, with the mask, pads and
        // silkscreen, and recolours what the layer look drew. A pick still answers the same part.
        await step("see the board as it is made", { needs: ["stand the parts on the stack"] }, async () => {
          const board = page.locator(PANE).first();
          if (!walk.stack) return "the board's file states no stack, so it has no stack view";
          const button = board.locator('[data-action="explode"]');
          const renderer = await board.getAttribute("data-renderer");
          await button.click({ timeout: 5_000 });
          await page.waitForFunction((p) => document.querySelector(p)?.getAttribute("data-exploded") === "true", PANE, { timeout: 60_000 });
          const gl = board.locator("canvas.sheetview-gl");
          const look = () => gl.getAttribute("data-stack-look");
          if ((await look()) !== "layers") throw new Error(`the stack view opened in the ${await look()} look, not the layer look`);
          await board.locator('[data-stack-look="board"]').click({ timeout: 5_000 });
          if ((await look()) !== "board") throw new Error(`Board left the view in the ${await look()} look`);
          // The board look draws each part's own model where the server has one (agni issue 1652). The
          // models' answer redraws the view, and a redraw drops the clickable points until it settles.
          // So the step waits for the answer on every board, one with no models too, before it looks
          // for a point.
          type Meshes = { meshes: number; parts: number; triangles: number };
          await page
            .waitForFunction((p) => document.querySelector(`${p} canvas.sheetview-gl`)?.hasAttribute("data-stack-meshes"), PANE, { timeout: 30_000 })
            .catch(async () => {
              throw new Error(`the board look never heard from GetBoardMeshes; the stack bar says "${await board.locator("[data-stack-mesh-note]").textContent()}"`);
            });
          const meshes = JSON.parse((await gl.getAttribute("data-stack-meshes")) ?? "{}") as Meshes;
          if (walk.models && meshes.parts < walk.models.meshed) throw new Error(`the board look drew models on ${meshes.parts} parts, want at least ${walk.models.meshed}`);
          if (!walk.models && meshes.parts > 0) throw new Error(`the board names no 3D models, and the board look drew models on ${meshes.parts} parts`);
          type BodyProbe = { ref: string; x: number; y: number };
          await page
            .waitForFunction((p) => (JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-body-probes") ?? "[]") as BodyProbe[]).length > 0, PANE, { timeout: 5_000 })
            .catch(() => {});
          const probe = (await page.evaluate((p) => JSON.parse(document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-body-probes") ?? "[]") as BodyProbe[], PANE))[0];
          if (!probe) throw new Error("no part offers a point to click in the board look");
          const box = (await gl.boundingBox())!;
          await page.mouse.click(box.x + probe.x, box.y + probe.y);
          await picked(page, probe.ref);
          // The silkscreen's text sits on the top face in the silkscreen colour, and reads from
          // above (agni issue 1607).
          const silkText = () =>
            page.evaluate((p) => {
              const host = document.querySelector(p);
              let shown = 0;
              for (const g of host?.querySelectorAll<SVGGElement>("g[data-plane][data-look-fill]") ?? []) {
                if (g.style.display === "none" || !g.getAttribute("transform")?.startsWith("matrix(")) continue;
                for (const t of g.querySelectorAll<SVGTextElement>("text")) if (t.style.display !== "none" && t.getBoundingClientRect().width > 0) shown++;
              }
              return shown;
            }, PANE);
          await page.waitForFunction((p) => !!document.querySelector(p)?.querySelector("g[data-plane][data-look-fill]"), PANE, { timeout: 5_000 }).catch(() => {});
          const texts = await silkText();
          if (texts === 0) {
            const seen = await page.evaluate((p) => {
              const host = document.querySelector(p);
              return [...(host?.querySelectorAll<SVGGElement>("g[data-plane]") ?? [])].map(
                (g) => `${g.dataset.plane}${g.hasAttribute("data-look-fill") ? " silk" : ""} ${g.style.display || "shown"} ${g.querySelectorAll("text").length} texts`,
              );
            }, PANE);
            throw new Error(`the board look shows no silkscreen text on the board's face; the text planes are ${JSON.stringify(seen)}`);
          }
          // Once the view settles, the stack view hides each label a part's block covers, and names how
          // many in data-stack-occluded. With Parts off nothing hides (agni issue 1657).
          const occluded = () => gl.getAttribute("data-stack-occluded");
          await page.waitForFunction((p) => document.querySelector(`${p} canvas.sheetview-gl`)?.hasAttribute("data-stack-occluded"), PANE, { timeout: 5_000 }).catch(async () => {
            throw new Error("the stack view never reported which labels its blocks hide");
          });
          const hiddenByParts = Number(await occluded());
          await board.locator('[data-action="stack-bodies"]').uncheck({ timeout: 5_000 });
          await page.waitForFunction((p) => document.querySelector(`${p} canvas.sheetview-gl`)?.getAttribute("data-stack-occluded") === "0", PANE, { timeout: 5_000 }).catch(async () => {
            throw new Error(`with Parts off the stack view still hides ${await occluded()} labels`);
          });
          await board.locator('[data-action="stack-bodies"]').check({ timeout: 5_000 });
          await board.locator('[data-stack-look="layers"]').click({ timeout: 5_000 });
          if ((await look()) !== "layers") throw new Error(`Layers left the view in the ${await look()} look`);
          await button.click({ timeout: 5_000 });
          await page
            .waitForFunction(({ p, r }) => document.querySelector(p)?.getAttribute("data-exploded") === "false" && document.querySelector(p)?.getAttribute("data-renderer") === r, { p: PANE, r: renderer }, { timeout: 60_000 })
            .catch(async () => {
              throw new Error(`the board view came back exploded ${await board.getAttribute("data-exploded")} in ${await board.getAttribute("data-renderer")}, not flat in ${renderer}`);
            });
          const models = meshes.parts > 0 ? `drew ${meshes.parts} parts as their models (${meshes.meshes} meshes, ${meshes.triangles} triangles), ` : "drew every part as its block, ";
          return `the board look ${models}picked ${probe.ref}'s body and showed ${texts} silkscreen labels on its face, its blocks hid ${hiddenByParts} labels and none with Parts off, and Layers switched back`;
        });

        await step("no page errors", {}, async () => {
          if (pageErrors.length) throw new Error(pageErrors.join("; "));
          return "none";
        });

        // This one leaves the page, so it goes after the page error check, and a page error on the way
        // out does not fail it. The bookmark step moved to the markup walk (#1612).
        await step("open an ODB++ job packed in a .7z", {}, async () => {
          await page.goto(`${base()}/mounts/probe/boards/jetson-cut-odb/view?mode=svg`, { waitUntil: "networkidle" });
          // Earlier steps may have closed the Sheets panel, so the tile need only be in the page.
          await page.locator('#sheet-overview .sheet-tile[title="board"]').waitFor({ state: "attached", timeout: 30_000 });
          await page.locator('#svg-view svg [data-ref="C133"]').first().waitFor({ state: "attached", timeout: 30_000 });
          return "the .7z job opened, and its board draws C133";
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
