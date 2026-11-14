// Stepping back and forward through what was selected (agni issue 1588), and through the sheets that
// draw the selection (agni issue 1589), on the cross-probe walk's cut of Jetson. jsdom has no layout
// and no keyboard focus, so only a browser shows that a step replays the selection in the board view
// too, that a key typed in the query box stays in the box, and that a sheet view steps its own sheet.
// The cross-probe walk steps through every sheet that draws a net (cross-probe-walk.spec.ts).

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { showPanel } from "./panels.js";
import { PANE, TITLE, clearPoint, clickableNet, findIn, openBeside, pickPoint, picked, zoomTo } from "./picks.js";

const base = (): string => inject("baseUrl");
const SHEET = "#svg-view";
const BOARD = `${PANE} .sheetview-canvas`;
const STEPS = "#steps";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

async function openCut(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg`, { waitUntil: "networkidle" });
  await page.waitForSelector(`${SHEET} svg`, { timeout: 60_000 });
}

// openTile opens a sheet on the canvas from its tile in Sheets. The cut opens on its root sheet,
// which draws the two sheet symbols and no parts.
async function openTile(page: Page, name: string): Promise<void> {
  await showPanel(page, "overview");
  await page.locator(".sheet-tile", { has: page.locator(".sheet-tile-name", { hasText: new RegExp(`^${name}$`) }) }).first().click();
  await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() === n, name, { timeout: 30_000 });
  await page.waitForTimeout(500);
}

// pickPart clicks the first part on the canvas's sheet that has a point clear of its pins, other than
// those in skip, and returns its designator. At fit zoom most parts are too small to click, and a part
// zoomed to puts the next off screen, so it brings each part into view and zooms to it, as a reader
// would.
async function pickPart(page: Page, skip: string[]): Promise<string> {
  const refs = await page.$$eval(`${SHEET} [data-kind="component"][data-ref]`, (rs) => [...new Set(rs.map((r) => r.getAttribute("data-ref")!))]);
  for (const ref of refs.filter((r) => r && !skip.includes(r) && !r.startsWith("#"))) {
    const sel = `${SHEET} [data-kind="component"][data-ref="${ref}"]`;
    await findIn(page, SHEET, sel);
    await zoomTo(page, sel);
    const at = await clearPoint(page, sel, ref).catch(() => pickPoint(page, sel, "component")).catch(() => null);
    if (!at) continue;
    await page.mouse.click(at.x, at.y);
    await picked(page, ref);
    return ref;
  }
  throw new Error(`no part on the sheet has a clear point, apart from ${skip.join(", ")}`);
}

const tile = async (page: Page) => ((await page.locator(".sheet-tile.active .sheet-tile-name").textContent()) ?? "").trim();
const control = (page: Page, action: string) => page.locator(`${STEPS} [data-action="${action}"]`);
const title = async (page: Page) => ((await page.locator(TITLE).textContent()) ?? "").trim();

// boardLights waits for the board view to light a part, by its footprint's pick rect lying under the
// view's highlight.
async function boardLights(page: Page, ref: string): Promise<void> {
  await page.waitForFunction(
    ({ board, ref }) => {
      const fp = document.querySelector(`${board} .picks-front [data-ref="${ref}"], ${board} .picks-back [data-ref="${ref}"]`);
      const lit = document.querySelectorAll(`${board} .highlight-overlay svg *`).length > 0;
      if (!fp || !lit) return false;
      const r = fp.getBoundingClientRect();
      const h = document.querySelector(board)!.getBoundingClientRect();
      const cx = r.left + r.width / 2, cy = r.top + r.height / 2;
      return cx > h.left && cx < h.right && cy > h.top && cy < h.bottom;
    },
    { board: BOARD, ref },
    { timeout: 30_000 },
  );
}

describe("stepping through what was selected", () => {
  it("steps back to A in both views, forward to B, and a new pick drops C", async () => {
    await withPage(browser, async (page) => {
      await openCut(page);
      await openBeside(page, "Board");
      await page.waitForSelector(`${BOARD} .picks-front`, { timeout: 30_000 });
      await openTile(page, "USB Debug, PD");
      expect(await control(page, "selection-back").isDisabled()).toBe(true);
      expect(await control(page, "selection-forward").isDisabled()).toBe(true);

      const picks: string[] = [];
      for (let i = 0; i < 3; i++) picks.push(await pickPart(page, picks));
      const [a, b] = picks;
      expect(await control(page, "selection-forward").isDisabled()).toBe(true);

      // One step by its button and one by its key.
      await control(page, "selection-back").click();
      await picked(page, b);
      await page.keyboard.press("[");
      await picked(page, a);
      await boardLights(page, a);
      expect(await control(page, "selection-back").isDisabled()).toBe(true);

      await page.keyboard.press("]");
      await picked(page, b);
      await boardLights(page, b);
      expect(await control(page, "selection-forward").isDisabled()).toBe(false);

      // A new pick made from B drops C, so forward has nowhere to go. The pick is a net, since the
      // sheet's other parts are resistors and diodes whose outlines have no point clear of their pins.
      await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="GND"]`);
      const net = await clickableNet(page, SHEET, "GND");
      await page.mouse.click(net.x, net.y);
      const d = "GND";
      await picked(page, d);
      expect(await control(page, "selection-forward").isDisabled()).toBe(true);
      await control(page, "selection-back").click();
      await picked(page, b);
      // Forward from B goes to the new pick, where C used to be.
      await control(page, "selection-forward").click();
      await picked(page, d);
    });
  }, 150_000);

  it("leaves a key typed in the query box to the box", async () => {
    await withPage(browser, async (page) => {
      await openCut(page);
      await openTile(page, "USB Debug, PD");
      const picks: string[] = [];
      for (let i = 0; i < 2; i++) picks.push(await pickPart(page, picks));
      const sheet = await tile(page);
      await page.keyboard.press("ControlOrMeta+Shift+F");
      const box = page.locator(".query-text");
      await box.waitFor({ state: "visible", timeout: 10_000 });
      await box.click();
      await box.pressSequentially("[.,]");
      await page.waitForTimeout(500);
      expect(await title(page)).toBe(picks[1]);
      expect(await tile(page)).toBe(sheet);
      expect(await box.inputValue()).toContain("[.,]");
    });
  }, 120_000);
});

describe("stepping through the sheets", () => {
  it("opens the next sheet with nothing selected, and stops at the last", async () => {
    await withPage(browser, async (page) => {
      await openCut(page);
      const order = await page.$$eval(".sheet-tile-name", (els) => els.map((e) => (e.textContent ?? "").trim()).filter((n) => n !== "Board"));
      expect(order.length).toBeGreaterThanOrEqual(3);
      let at = order.indexOf(await tile(page));
      expect(at).toBeGreaterThanOrEqual(0);
      await page.waitForFunction((s) => document.querySelector(`${s} [data-sheet-place]`)?.textContent?.includes(" of "), STEPS, { timeout: 10_000 });
      expect(await page.locator(`${STEPS} [data-sheet-place]`).textContent()).toBe(`${at + 1} of ${order.length}`);
      while (at < order.length - 1) {
        await page.locator(SHEET).click({ position: { x: 5, y: 5 } }).catch(() => {});
        await page.keyboard.press(".");
        at++;
        await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() === n, order[at], { timeout: 15_000 });
      }
      expect(await control(page, "next-sheet").isDisabled()).toBe(true);
      await page.keyboard.press(".");
      await page.waitForTimeout(500);
      expect(await tile(page)).toBe(order[order.length - 1]);
      await page.keyboard.press(",");
      await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() === n, order[order.length - 2], { timeout: 15_000 });
    });
  }, 150_000);

  it("steps a schematic view beside the canvas once the reader presses in it, and leaves the canvas", async () => {
    await withPage(browser, async (page) => {
      await openCut(page);
      await openTile(page, "Power");
      await findIn(page, SHEET, `${SHEET} [data-kind="net"][data-net="GND"]`);
      const at = await clickableNet(page, SHEET, "GND");
      await page.mouse.click(at.x, at.y);
      await picked(page, "GND");
      const canvasSheet = await tile(page);
      const names = await page.$$eval(".sheet-tile-name", (els) => els.map((e) => (e.textContent ?? "").trim()));
      const other = names.find((n) => n !== "Board" && n !== canvasSheet)!;
      await openBeside(page, other);
      const pane = page.locator(PANE).first();
      const shown = async () => pane.locator(".sheetview-sheet").evaluate((s) => (s as HTMLSelectElement).selectedOptions[0]?.textContent ?? "");
      await page.waitForFunction((p) => / of \d+$/.test(document.querySelector(`${p} [data-sheet-place]`)?.textContent ?? ""), PANE, { timeout: 15_000 });
      const before = await shown();
      const place = (await pane.locator("[data-sheet-place]").textContent()) ?? "";
      const dir = place.startsWith("1 of") ? "." : ",";
      await pane.locator(".sheetview-bar").click({ position: { x: 2, y: 2 } });
      await page.keyboard.press(dir);
      await page.waitForFunction(
        ({ p, b }) => (document.querySelector(`${p} .sheetview-sheet`) as HTMLSelectElement).selectedOptions[0]?.textContent !== b,
        { p: PANE, b: before },
        { timeout: 15_000 },
      );
      expect(await tile(page)).toBe(canvasSheet);
    });
  }, 150_000);
});
