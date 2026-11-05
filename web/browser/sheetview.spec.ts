// A sheet view beside the canvas follows what the reader selects (agni issue 1326). jsdom has no
// layout and no elementFromPoint, so only a browser shows that a pick in one view lights and centres
// the same part in the other, in either direction, and that a view keeps following once it floats.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

const PANE = "[data-sheetview]";
const TITLE = '[data-component="properties"] .pr-title';

async function openViewer(page: Page, design: string): Promise<void> {
  await page.goto(`${base()}/designs/${design}/view`, { waitUntil: "networkidle" });
  await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
}

// openBeside opens the sheet named exactly `name` in a new view from its Sheets tile, and waits for
// the view to draw.
async function openBeside(page: Page, name: string): Promise<void> {
  const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: new RegExp(`^${name}$`) }) });
  await row.locator(".sheet-tile-beside").click();
  await page.waitForSelector(`${PANE} .sheetview-canvas svg`, { timeout: 30_000 });
}

// centreOf is an element's centre in client pixels.
async function centreOf(page: Page, sel: string): Promise<{ x: number; y: number }> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) throw new Error(`no element matches ${sel}`);
    const r = el.getBoundingClientRect();
    return { x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2) };
  }, sel);
}

// clearPoint finds a whole-pixel point inside `sel` where every probe of the picker lands on `ref` or
// on nothing keyed, the search board-pick.spec.ts uses.
async function clearPoint(page: Page, sel: string, ref: string): Promise<{ x: number; y: number }> {
  return page.evaluate(
    ({ sel, ref }) => {
      const keyed = (e: Element | null) => e?.closest("[data-kind]") ?? null;
      const el = document.querySelector(sel);
      if (!el) throw new Error(`no element matches ${sel}`);
      const r = el.getBoundingClientRect();
      for (let fy = 0.1; fy < 0.95; fy += 0.05) {
        for (let fx = 0.1; fx < 0.95; fx += 0.05) {
          const x = Math.round(r.left + r.width * fx);
          const y = Math.round(r.top + r.height * fy);
          const hits = [-5, 0, 5].flatMap((dx) => [-5, 0, 5].map((dy) => keyed(document.elementFromPoint(x + dx, y + dy))));
          const centre = keyed(document.elementFromPoint(x, y));
          if (centre?.getAttribute("data-ref") === ref && centre.getAttribute("data-kind") === "component" &&
            hits.every((h) => !h || (h.getAttribute("data-ref") === ref && h.getAttribute("data-kind") === "component"))) {
            return { x, y };
          }
        }
      }
      throw new Error(`no point inside ${sel} picks only ${ref}`);
    },
    { sel, ref },
  );
}

// firstClear is the first part inside `within` with a clear point, skipping `skip`, since at fit
// zoom most bodies have pads or a neighbour within the probe ring, and a floating view can cover part
// of the canvas.
async function firstClear(page: Page, within: string, skip: string[] = []): Promise<{ ref: string; x: number; y: number }> {
  const refs = await page.$$eval(`${within} [data-kind="component"][data-ref]`, (rs) => [...new Set(rs.map((r) => r.getAttribute("data-ref")!))]);
  for (const ref of refs.filter((r) => !skip.includes(r))) {
    const at = await clearPoint(page, `${within} [data-kind="component"][data-ref="${ref}"]`, ref).catch(() => null);
    if (at) return { ref, ...at };
  }
  throw new Error(`no part inside ${within} has a clear point`);
}

async function picked(page: Page, want: string): Promise<void> {
  await page.waitForFunction(({ sel, want }) => document.querySelector(sel)?.textContent === want, { sel: TITLE, want }, { timeout: 30_000 });
}

// lit waits for a host's highlight overlay to draw something.
async function lit(page: Page, host: string): Promise<void> {
  await page.waitForFunction((h) => document.querySelectorAll(`${h} .highlight-overlay svg *`).length > 0, host, { timeout: 30_000 });
}

// centredIn says whether an element's centre lies within the middle third of a host, which is where
// a reveal puts it.
async function centredIn(page: Page, el: string, host: string): Promise<boolean> {
  return page.evaluate(
    ({ el, host }) => {
      const e = document.querySelector(el)!.getBoundingClientRect();
      const h = document.querySelector(host)!.getBoundingClientRect();
      const cx = e.left + e.width / 2;
      const cy = e.top + e.height / 2;
      return cx > h.left + h.width / 3 && cx < h.right - h.width / 3 && cy > h.top + h.height / 3 && cy < h.bottom - h.height / 3;
    },
    { el, host },
  );
}

describe("a sheet view beside the canvas", () => {
  it("finds a schematic part on the board, and a board footprint on the schematic", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      expect(await page.locator(PANE).count()).toBe(0);
      await openBeside(page, "Board");
      await page.waitForSelector(`${PANE} .picks-front`, { timeout: 30_000 });

      // The board in the view to the schematic on the canvas. Both are fitted, and the part found on
      // the board is the first whose body has a point clear of its pads.
      const onBoard = await firstClear(page, `${PANE} .picks-front`);
      await page.mouse.click(onBoard.x, onBoard.y);
      await picked(page, onBoard.ref);
      await lit(page, "#svg-view");
      await page.waitForFunction(
        (ref) => {
          const e = document.querySelector(`#svg-view [data-kind="component"][data-ref="${ref}"]`)!.getBoundingClientRect();
          const h = document.querySelector("#svg-view")!.getBoundingClientRect();
          const cx = e.left + e.width / 2;
          return cx > h.left + h.width / 3 && cx < h.right - h.width / 3;
        },
        onBoard.ref,
        { timeout: 30_000 },
      );
      expect(await centredIn(page, `#svg-view [data-kind="component"][data-ref="${onBoard.ref}"]`, "#svg-view")).toBe(true);

      // The schematic on the canvas to the board in the view. The canvas has zoomed to the part, so it
      // is the part clicked again, and the board view, still fitted from before, has to move to it.
      const onSheet = { ref: onBoard.ref, ...(await clearPoint(page, `#svg-view [data-kind="component"][data-ref="${onBoard.ref}"]`, onBoard.ref)) };
      expect(await centredIn(page, `${PANE} .picks-front [data-ref="${onSheet.ref}"]`, `${PANE} .sheetview-canvas`)).toBe(false);
      await page.mouse.click(onSheet.x, onSheet.y);
      await picked(page, onSheet.ref);
      await lit(page, `${PANE} .sheetview-canvas`);
      await page.waitForFunction(
        ({ pane, ref }) => {
          const e = document.querySelector(`${pane} .picks-front [data-ref="${ref}"]`)!.getBoundingClientRect();
          const h = document.querySelector(`${pane} .sheetview-canvas`)!.getBoundingClientRect();
          const cx = e.left + e.width / 2;
          return cx > h.left + h.width / 3 && cx < h.right - h.width / 3;
        },
        { pane: PANE, ref: onSheet.ref },
        { timeout: 30_000 },
      );
      expect(await centredIn(page, `${PANE} .picks-front [data-ref="${onSheet.ref}"]`, `${PANE} .sheetview-canvas`)).toBe(true);

      // A pad picked on the board centres its pin on the schematic. A pin's highlight is a point, and
      // a point used to reveal nothing.
      const pad = `${PANE} [data-kind="pin"][data-ref="${onSheet.ref}"]`;
      const pin = await page.$eval(pad, (e) => e.getAttribute("data-pin")!);
      await page.mouse.click(...(Object.values(await centreOf(page, `${pad}[data-pin="${pin}"]`)) as [number, number]));
      await picked(page, `${onSheet.ref}.${pin}`);
      const pinOnSheet = `#svg-view [data-kind="pin"][data-ref="${onSheet.ref}"][data-pin="${pin}"]`;
      await page.waitForFunction((sel) => {
        const e = document.querySelector(sel)!.getBoundingClientRect();
        const h = document.querySelector("#svg-view")!.getBoundingClientRect();
        const cx = e.left + e.width / 2, cy = e.top + e.height / 2;
        return Math.abs(cx - (h.left + h.width / 2)) < h.width / 10 && Math.abs(cy - (h.top + h.height / 2)) < h.height / 10;
      }, pinOnSheet, { timeout: 30_000 });
    });
  }, 120_000);

  it("switches its sheet when its own does not draw the selection", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad/hier_numeric_root.kicad_sch");
      await openBeside(page, "Hier numeric root");
      const root = await page.$eval(`${PANE} .sheetview-sheet`, (s) => (s as HTMLSelectElement).value);
      await page.locator(".sheet-tile", { hasText: /^IO$/ }).click();
      await page.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO", null, { timeout: 30_000 });
      await page.waitForTimeout(500);
      const ref = await page.$eval('#svg-view [data-kind="component"][data-ref]', (e) => e.getAttribute("data-ref")!);
      await page.mouse.click(...Object.values(await centreOf(page, `#svg-view [data-kind="component"][data-ref="${ref}"]`)) as [number, number]);
      await page.waitForFunction(
        ({ pane, root }) => (document.querySelector(`${pane} .sheetview-sheet`) as HTMLSelectElement).value !== root,
        { pane: PANE, root },
        { timeout: 30_000 },
      );
      await lit(page, `${PANE} .sheetview-canvas`);
    });
  }, 120_000);

  it("keeps following once it floats, and stays put when its link is off", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      await openBeside(page, "Board");
      await page.locator(`${PANE} .sheetview-float`).click();
      await page.waitForFunction((pane) => document.querySelector(pane)?.closest(".dv-render-overlay-float") !== null, PANE, { timeout: 30_000 });
      const first = await firstClear(page, "#svg-view");
      await page.mouse.click(first.x, first.y);
      await picked(page, first.ref);
      await lit(page, `${PANE} .sheetview-canvas`);

      await page.selectOption(`${PANE} .sheetview-link`, "off");
      const before = await page.$eval(`${PANE} .sheetview-canvas > div`, (d) => (d as HTMLElement).style.transform);
      const second = await firstClear(page, "#svg-view", [first.ref]);
      await page.mouse.click(second.x, second.y);
      await picked(page, second.ref);
      await page.waitForTimeout(1500);
      expect(await page.$eval(`${PANE} .sheetview-canvas > div`, (d) => (d as HTMLElement).style.transform)).toBe(before);
    });
  }, 120_000);
});
