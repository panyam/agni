// A board view turns and flips to match the board on the bench, and shows it from underneath (agni
// issue 1327). jsdom has no layout, so only a browser shows that a pad still picks once the view is
// turned, that back-side text reads mirrored from the front and the right way round from below, and
// that a view opened again after a reload comes back turned.
//
// The board is readers/kicad/testdata/board.kicad_pcb. Its "Demo Board" legend is back silkscreen
// marked (justify mirror), and R1 sits on the front. kicad-cli draws the legend mirrored from the
// front and R1 the right way round, and the reverse with --mirror.

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

async function openBoardView(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/kicad/board.kicad_pcb/view`, { waitUntil: "networkidle" });
  await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
  const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: /^Board$/ }) });
  await row.locator(".sheet-tile-beside").click();
  await page.waitForSelector(`${PANE} .picks-front`, { timeout: 30_000 });
}

// mirrored says whether a text reads mirrored on screen, from the sign of its on-screen transform.
async function mirrored(page: Page, text: string): Promise<boolean> {
  return page.evaluate(
    ({ pane, text }) => {
      const el = [...document.querySelectorAll(`${pane} .labels text`)].find((t) => t.textContent === text) as SVGGraphicsElement | undefined;
      if (!el) throw new Error(`the board view draws no ${text}`);
      const m = el.getScreenCTM()!;
      return m.a * m.d - m.b * m.c < 0;
    },
    { pane: PANE, text },
  );
}

async function click(page: Page, sel: string): Promise<void> {
  const at = await page.evaluate((sel) => {
    const r = document.querySelector(sel)!.getBoundingClientRect();
    return { x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2) };
  }, sel);
  await page.mouse.click(at.x, at.y);
}

describe("turning a board view", () => {
  it("draws back text mirrored from the front, and the right way round from below", async () => {
    await withPage(browser, async (page) => {
      await openBoardView(page);
      expect(await mirrored(page, "Demo Board")).toBe(true);
      expect(await mirrored(page, "R1")).toBe(false);

      await page.locator(`${PANE} .sheetview-bottom`).click();
      expect(await mirrored(page, "Demo Board")).toBe(false);
      expect(await mirrored(page, "R1")).toBe(true);
      const order = await page.$$eval(`${PANE} .sheetview-canvas svg > g`, (gs) => gs.map((g) => g.getAttribute("class") ?? ""));
      expect(order.indexOf("copper-back")).toBeGreaterThan(order.indexOf("copper-front"));
    });
  }, 90_000);

  it("still picks the pad under the cursor once turned and flipped", async () => {
    await withPage(browser, async (page) => {
      await openBoardView(page);
      await page.locator(`${PANE} .sheetview-rotate`).click();
      await page.locator(`${PANE} .sheetview-flip-v`).click();
      await page.locator(`${PANE} .sheetview-bottom`).click();
      await click(page, `${PANE} [data-kind="pin"][data-ref="R1"][data-pin="1"]`);
      await page.waitForFunction(() => document.querySelector('[data-component="properties"] .pr-title')?.textContent === "R1.1", null, { timeout: 30_000 });
    });
  }, 90_000);

  it("opens the board the way it was turned, after a reload", async () => {
    await withPage(browser, async (page) => {
      await openBoardView(page);
      await page.locator(`${PANE} .sheetview-rotate`).click();
      await page.locator(`${PANE} .sheetview-bottom`).click();
      const turn = () => page.$eval(`${PANE} .sheetview-canvas > div > div`, (d) => (d as HTMLElement).style.transform);
      const before = await turn();
      expect(before).toContain("rotate(90deg)");
      await page.reload({ waitUntil: "networkidle" });
      expect(await page.locator(PANE).count()).toBe(0);
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: /^Board$/ }) });
      await row.locator(".sheet-tile-beside").click();
      await page.waitForSelector(`${PANE} .picks-front`, { timeout: 30_000 });
      expect(await turn()).toBe(before);
      expect(await page.$eval(`${PANE} .sheetview-canvas svg`, (s) => s.getAttribute("data-strata"))).toBe("below");
    });
  }, 90_000);
});
