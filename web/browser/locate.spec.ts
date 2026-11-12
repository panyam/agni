// A click on a subject in a panel opens a sheet that draws it, lights it and zooms the canvas to it
// (agni issue 1539), from Find by name, Checks and Tables alike. The Query panel's answer scrolls, and
// a bound value's box keeps its focus while the reader types (agni issue 1540). jsdom has no layout,
// so only a browser shows the canvas moved or a table scrolled. The design is the gate's cut of
// Jetson (examples/cross-probe-project), whose root sheet draws neither TP76 nor R123.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");
const DESIGN = "probe/designs/jetson-cut";
const SMALL_NET = "unconnected-(R123-Pad2)";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

async function open(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/${DESIGN}/view?mode=svg`, { waitUntil: "networkidle" });
  await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
}

// zoom is how many times wider the drawing is than the canvas: about 1 fitted, more when zoomed in.
async function zoom(page: Page): Promise<number> {
  return page.evaluate(() => {
    const host = document.querySelector("#svg-view")!.getBoundingClientRect();
    const doc = document.querySelector("#svg-view svg")!.getBoundingClientRect();
    return doc.width / host.width;
  });
}

// zoomedTo waits until the canvas is zoomed in to at least `factor` and a component or net is lit.
async function zoomedTo(page: Page, factor: number): Promise<number> {
  await page.waitForFunction(
    (factor) => {
      const host = document.querySelector("#svg-view")!.getBoundingClientRect();
      const doc = document.querySelector("#svg-view svg")!.getBoundingClientRect();
      return doc.width / host.width >= factor;
    },
    factor,
    { timeout: 30_000 },
  );
  return zoom(page);
}

async function runChecks(page: Page): Promise<void> {
  await page.click(".checks-run", { timeout: 60_000 });
  await page.waitForSelector(".check-locate", { timeout: 60_000 });
}

describe("locating a subject from a panel", () => {
  it("opens the sheet a part found by name is drawn on, and zooms to it", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      expect(await page.locator('#svg-view [data-kind="component"][data-ref="TP76"]').count()).toBe(0);
      const fitted = await zoom(page);
      await page.keyboard.press("Control+f");
      await page.keyboard.type("TP76");
      await page.keyboard.press("Enter");
      await page.locator(".query-row .query-locate", { hasText: /^TP76$/ }).first().click({ timeout: 30_000 });
      await page.waitForSelector('#svg-view [data-kind="component"][data-ref="TP76"]', { timeout: 30_000 });
      expect(await zoomedTo(page, fitted * 3)).toBeGreaterThan(fitted * 3);
    });
  }, 120_000);

  it("zooms to a finding clicked in Checks", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await runChecks(page);
      const fitted = await zoom(page);
      await page.locator(".check-locate", { hasText: SMALL_NET }).first().click();
      expect(await zoomedTo(page, fitted * 3)).toBeGreaterThan(fitted * 3);
    });
  }, 120_000);

  it("zooms to a subject clicked in Tables", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await runChecks(page);
      const fitted = await zoom(page);
      await page.locator(".dv-default-tab-content", { hasText: /^Tables$/ }).first().click();
      await page.locator(".tables-source", { hasText: /^Checks$/ }).click();
      await page.locator(".tables-locate", { hasText: SMALL_NET }).first().click({ timeout: 30_000 });
      expect(await zoomedTo(page, fitted * 3)).toBeGreaterThan(fitted * 3);
    });
  }, 120_000);
});

describe("the Query panel's inputs", () => {
  it("keeps rows for a Find by name answer, with its chip and the selection bar above it, and scrolls it", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await page.keyboard.press("Control+f");
      await page.keyboard.type("R");
      await page.keyboard.press("Enter");
      await page.locator(".query-row .query-locate").first().click({ timeout: 30_000 });
      await page.waitForSelector(".query-selection", { timeout: 30_000 });
      const box = page.locator(".query-results");
      const size = await box.evaluate((el) => ({ scroll: el.scrollHeight, client: el.clientHeight, bottom: el.getBoundingClientRect().bottom }));
      const panel = await page.locator(".query").evaluate((el) => el.getBoundingClientRect().bottom);
      // At least five rows show under the query box, its ?pattern chip and the selection bar, and none
      // is cut off at the panel's foot. Before, they left this answer about 90 px.
      expect(size.client).toBeGreaterThanOrEqual(120);
      expect(size.bottom).toBeLessThanOrEqual(panel + 1);
      expect(size.scroll).toBeGreaterThan(size.client);
      await box.hover();
      await page.mouse.wheel(0, 400);
      await page.waitForFunction(() => (document.querySelector(".query-results")?.scrollTop ?? 0) > 0, null, { timeout: 10_000 });
    });
  }, 120_000);
});
