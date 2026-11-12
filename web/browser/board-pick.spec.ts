// A click on the board view picks what it lands on (agni issue 1324). jsdom has no elementFromPoint,
// so only a browser can show that a real click on a painted pad, footprint body, track or via
// reaches the Selection, and that the side the layer control hides stops picking.
//
// The board is readers/kicad/testdata/board.kicad_pcb, the one served board with both a via and a
// part on the back.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { hideDrawers, showPanel } from "./panels.js";
import { PANE, openBeside, pickPoint as clearPoint } from "./picks.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// A board opened on its own opens on the board, its only sheet (agni issue 1466).
async function openBoard(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/kicad/board.kicad_pcb/view?mode=svg`, { waitUntil: "networkidle" });
  await page.waitForSelector("#svg-view .picks-front", { timeout: 30_000 });
}

const tileNames = (page: Page) => page.$$eval(".sheet-tile-name", (ns) => ns.map((n) => n.textContent?.trim() ?? ""));

async function clickAndRead(page: Page, at: { x: number; y: number }, want: string): Promise<void> {
  await page.mouse.click(at.x, at.y);
  const title = () => page.evaluate(() => document.querySelector('[data-component="properties"] .pr-title')?.textContent ?? "");
  try {
    await page.waitForFunction((w) => document.querySelector('[data-component="properties"] .pr-title')?.textContent === w, want, {
      timeout: 15_000,
    });
  } catch {
    expect(await title(), `the pick at ${Math.round(at.x)},${Math.round(at.y)}`).toBe(want);
  }
}

describe("picking the board", () => {
  // A board with no drawn schematic lists the board alone, with no auto-layout of its netlist, and
  // draws it with no click (agni issue 1466).
  it("opens a board on its own as the board alone", async () => {
    await withPage(browser, async (page) => {
      await openBoard(page);
      await page.waitForSelector(".sheet-tile", { state: "attached", timeout: 30_000 });
      expect(await tileNames(page)).toEqual(["Board"]);
    });
  }, 60_000);

  it("picks a pad as its pin, a body as its part, and copper as its net", async () => {
    await withPage(browser, async (page) => {
      await openBoard(page);
      const pad = await page.evaluate(() => {
        const el = document.querySelector('#svg-view .copper-front [data-kind="pin"][data-net]')!;
        return { ref: el.getAttribute("data-ref")!, pin: el.getAttribute("data-pin")!, net: el.getAttribute("data-net")! };
      });

      await clickAndRead(page, await clearPoint(page, `#svg-view [data-kind="pin"][data-ref="${pad.ref}"][data-pin="${pad.pin}"]`, "pin"), `${pad.ref}.${pad.pin}`);
      await clickAndRead(page, await clearPoint(page, `#svg-view .picks-front [data-ref="${pad.ref}"]`, "component"), pad.ref);

      // A net's tracks share one keyed group per layer, so the line itself carries no key.
      // The longest front track, since a short one between two pads has no point clear of them.
      const track = await page.$$eval('#svg-view .copper-front g[data-kind="net"]', (gs) => {
        const reach = (g: Element) => {
          const r = g.getBoundingClientRect();
          return Math.hypot(r.width, r.height);
        };
        return gs.reduce((a, b) => (reach(b) > reach(a) ? b : a)).getAttribute("data-net")!;
      });
      await clickAndRead(page, await clearPoint(page, `#svg-view .copper-front g[data-kind="net"][data-net="${track}"] > line`, "net"), track);

      // The fixture's via is on the track's net, so pick something else first; picking the net
      // already shown would leave the title as it was and prove nothing.
      await clickAndRead(page, await clearPoint(page, `#svg-view .picks-front [data-ref="${pad.ref}"]`, "component"), pad.ref);
      const via = await page.$eval('#svg-view .through circle[data-kind="net"]', (el) => el.getAttribute("data-net")!);
      await clickAndRead(page, await clearPoint(page, `#svg-view .through circle[data-kind="net"][data-net="${via}"]`, "net"), via);
    });
  }, 90_000);

  // On a dense board a designator lies over its neighbour's pads, and a keyed label there picked
  // the neighbour (agni issue 1347). The test lays J1's designator over R1's pad 1, large enough to
  // cover every probe, and the pad must still pick its own pin.
  it("picks a pad through a designator drawn over it", async () => {
    await withPage(browser, async (page) => {
      await openBoard(page);
      const at = await page.evaluate(() => {
        const pad = document.querySelector('#svg-view [data-kind="pin"][data-ref="R1"][data-pin="1"]') as SVGGraphicsElement;
        const label = [...document.querySelectorAll("#svg-view .labels text")].find((t) => t.textContent === "J1") as SVGTextElement;
        if (!pad || !label) throw new Error("the board draws no R1.1 pad or no J1 designator");
        const b = pad.getBBox();
        // A KiCad label carries its run's length and starts at its ink, so the enlarged copy
        // drops both and centres on the pad.
        label.removeAttribute("transform");
        label.removeAttribute("textLength");
        label.removeAttribute("lengthAdjust");
        label.setAttribute("text-anchor", "middle");
        label.setAttribute("dominant-baseline", "central");
        label.setAttribute("x", String(b.x + b.width / 2));
        label.setAttribute("y", String(b.y + b.height / 2));
        label.setAttribute("font-size", "120");
        const r = pad.getBoundingClientRect();
        const x = Math.round(r.left + r.width / 2);
        const y = Math.round(r.top + r.height / 2);
        // The label's painted box, since a label that passes clicks through is invisible to
        // elementFromPoint, which is the fix.
        const l = label.getBoundingClientRect();
        if (!(l.left < x - 5 && l.right > x + 5 && l.top < y - 5 && l.bottom > y + 5)) throw new Error("the moved designator does not cover the pad");
        return { x, y };
      });
      await clickAndRead(page, at, "R1.1");
    });
  }, 90_000);

  // A part picked on the board lists every sheet that draws it, the schematic's and the board
  // (agni issue 1325), so a reader can open the schematic from the copper.
  it("lists the schematic sheet and the board for a part picked on the board", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/view?mode=svg`, { waitUntil: "networkidle" });
      // Exactly "Board", since gateway's schematic sheet is called "Sample Board".
      await showPanel(page, "overview");
      await page.locator(".sheet-tile", { hasText: /^\s*Board\s*$/ }).first().click();
      await page.waitForSelector("#svg-view .picks-front", { timeout: 30_000 });
      // The first front part whose body has a point clear of its pads at fit zoom.
      const refs = await page.$$eval("#svg-view .picks-front [data-ref]", (rs) => rs.map((r) => r.getAttribute("data-ref")!));
      let picked = "";
      for (const ref of refs) {
        const at = await clearPoint(page, `#svg-view .picks-front [data-ref="${ref}"]`, "component").catch(() => null);
        if (!at) continue;
        await clickAndRead(page, at, ref);
        picked = ref;
        break;
      }
      expect(picked, `none of ${refs.length} front parts has a body clear of its pads`).not.toBe("");
      const sheets = '[data-component="properties"] .pr-group[data-group="sheets"] .pr-ref-sheet';
      await page.waitForFunction((sel) => document.querySelectorAll(sel).length >= 2, sheets, { timeout: 30_000 });
      const names = await page.$$eval(sheets, (bs) => bs.map((b) => (b.textContent ?? "").trim()));
      expect(names).toEqual(["Sample Board", "Board"]);
    });
  }, 90_000);

  it("stops picking a back part once the back is hidden", async () => {
    await withPage(browser, async (page) => {
      await openBoard(page);
      const back = '#svg-view .picks-back [data-kind="component"]';
      const ref = await page.$eval(back, (el) => el.getAttribute("data-ref")!);
      const at = await clearPoint(page, back, "component");
      await clickAndRead(page, at, ref);

      await showPanel(page, "overview");
      await page.locator(".board-layer-preset", { hasText: /^Front$/ }).click();
      await page.waitForFunction((sel) => document.querySelector(sel)!.getClientRects().length === 0, back, { timeout: 30_000 });
      await hideDrawers(page);
      const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.closest(".picks-back") !== null, at);
      expect(hit).toBe(false);
    });
  }, 90_000);
});

// A hidden layer draws nothing and takes no clicks (agni 1351). board_inner.kicad_pcb has a track
// on In1.Cu alone, with nothing else under it.
describe("hiding a board layer", () => {
  it("hides one inner layer, which then picks nothing, and keeps the choice for the design", async () => {
    await withPage(browser, async (page) => {
      const track = '#svg-view .copper-inner[data-layer="In1.Cu"] g[data-kind="net"] > line';
      const open = async () => {
        await showPanel(page, "overview");
        await page.locator(".sheet-tile-name", { hasText: /^Board$/ }).click();
        await page.waitForSelector('#svg-view [data-layer="In1.Cu"]', { state: "attached", timeout: 30_000 });
      };
      await page.goto(`${base()}/designs/kicad/board_inner.kicad_pcb/view?mode=svg`, { waitUntil: "networkidle" });
      await open();
      const net = await page.$eval(track, (el) => el.parentElement!.getAttribute("data-net")!);
      const at = await clearPoint(page, track, "net");
      // The positive control. Shown, the track picks its net.
      await clickAndRead(page, at, net);

      await showPanel(page, "overview");
      await page.locator(".board-layer-row", { hasText: "In1.Cu" }).locator("input").click();
      await page.waitForFunction((sel) => document.querySelector(sel)!.getClientRects().length === 0, track, { timeout: 30_000 });
      await hideDrawers(page);
      const reached = (p: { x: number; y: number }) =>
        page.evaluate(({ x, y }) => {
          const nets = [-5, 0, 5].flatMap((dx) => [-5, 0, 5].map((dy) => document.elementFromPoint(x + dx, y + dy)?.closest("[data-net]")?.getAttribute("data-net") ?? ""));
          return [...new Set(nets.filter((n) => n !== ""))];
        }, p);
      expect(await reached(at)).toEqual([]);
      // The other inner layer still draws.
      expect(await page.$eval('#svg-view [data-layer="In2.Cu"]', (el) => el.getClientRects().length > 0)).toBe(true);

      // The design keeps the choice through a reload.
      await page.reload({ waitUntil: "networkidle" });
      await open();
      expect(await page.locator(".board-layer-row", { hasText: "In1.Cu" }).locator("input").isChecked()).toBe(false);
      expect(await page.$eval(track, (el) => el.getClientRects().length)).toBe(0);

      // A double-click shows one layer alone. The panel draws its rows again after the first click's
      // toggle, so this is a real double-click, not two calls on one element.
      await showPanel(page, "overview");
      await page.locator(".board-layer-row", { hasText: "B.Cu" }).dblclick();
      await page.waitForFunction(() => document.querySelector('#svg-view [data-layer="F.Cu"]')!.getClientRects().length === 0, null, { timeout: 30_000 });
      const shown = await page.$$eval(".board-layer-row", (rs) => rs.filter((r) => (r.querySelector("input") as HTMLInputElement).checked).map((r) => r.textContent!.trim()));
      expect(shown).toEqual(["B.Cu"]);

      // A double-click on a hidden row shows it alone too. Its first click shows the row and draws
      // the panel again, and on Jetson the old handler then hid every layer.
      await page.locator(".board-layer-row", { hasText: "In1.Cu" }).dblclick();
      await page.waitForFunction((sel) => document.querySelector(sel)!.getClientRects().length > 0, track, { timeout: 30_000 });
      const alone = await page.$$eval(".board-layer-row", (rs) => rs.filter((r) => (r.querySelector("input") as HTMLInputElement).checked).map((r) => r.textContent!.trim()));
      expect(alone).toEqual(["In1.Cu"]);
    });
  }, 90_000);

  // agni issue 1401. A view beside the canvas has its own layer list, so a reader can hide In1.Cu
  // there and keep it on the canvas. Each inner layer draws in a colour of its own.
  it("hides a layer in a view beside the canvas and leaves the canvas drawing it", async () => {
    await withPage(browser, async (page) => {
      const inner = (scope: string) => `${scope} .copper-inner[data-layer="In1.Cu"] g[data-kind="net"] > line`;
      const drawn = (sel: string) => page.$eval(sel, (el) => el.getClientRects().length > 0);
      await page.goto(`${base()}/designs/kicad/board_inner.kicad_pcb/view?mode=svg`, { waitUntil: "networkidle" });
      await showPanel(page, "overview");
      await page.locator(".sheet-tile-name", { hasText: /^Board$/ }).click();
      await page.waitForSelector(inner("#svg-view"), { state: "attached", timeout: 30_000 });
      await openBeside(page, "Board");
      await page.waitForSelector(inner(`${PANE} .sheetview-canvas`), { state: "attached", timeout: 30_000 });
      // The positive control. Both drawings show the track before anything is hidden.
      expect(await drawn(inner(`${PANE} .sheetview-canvas`))).toBe(true);
      expect(await drawn(inner("#svg-view"))).toBe(true);

      await page.locator(`${PANE} [data-action="layers"]`).click();
      const rows = page.locator(`${PANE} .sheetview-layer-pop .board-layer-row`);
      await rows.first().waitFor({ state: "visible", timeout: 10_000 });
      const swatch = (name: string) =>
        rows.filter({ hasText: name }).locator(".board-layer-swatch").evaluate((el) => getComputedStyle(el).backgroundColor);
      expect(await swatch("In1.Cu"), "In1.Cu and In2.Cu share a swatch colour").not.toBe(await swatch("In2.Cu"));

      await rows.filter({ hasText: "In1.Cu" }).locator("input").click();
      await page.waitForFunction((sel) => document.querySelector(sel)!.getClientRects().length === 0, inner(`${PANE} .sheetview-canvas`), { timeout: 10_000 });
      expect(await drawn(inner("#svg-view")), "hiding In1.Cu beside the canvas hid it on the canvas too").toBe(true);
      expect(await drawn(`${PANE} .sheetview-canvas [data-layer="In2.Cu"]`)).toBe(true);
      expect(await page.locator('.sheet-overview .board-layer-row, #sheet-overview .board-layer-row', { hasText: "In1.Cu" }).locator("input").isChecked()).toBe(true);
    });
  }, 90_000);
});
