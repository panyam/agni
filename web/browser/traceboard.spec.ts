// A trace lights its route on the board view beside the canvas too, and the view frames it (agni
// issue 1551). Before, the route lit only the schematic. jsdom draws no board, so only a browser
// against a server shows the board view asking for the route's overlay. The design is the gate's cut
// of Jetson, where R119.1 reaches D21.2 through R120, and the board view opens in SVG on headless
// Chrome's software GPU.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { PANE, openBeside } from "./picks.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("a trace beside the board", () => {
  it("lights the route on the board view and frames it", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      await openBeside(page, "Board");
      const overlay = `${PANE} .highlight-overlay svg`;
      expect(await page.locator(`${overlay} *`).count()).toBe(0);
      const before = await page.locator(`${PANE} .sheetview-canvas svg`).first().evaluate((s) => s.getBoundingClientRect().width);

      await page.locator(".dv-default-tab-content", { hasText: /^Trace$/ }).first().click();
      await page.locator("input.trace-pin").nth(0).fill("R119.1");
      await page.locator("input.trace-pin").nth(1).fill("D21.2");
      await page.click("button.trace-run");
      await page.waitForFunction((sel) => document.querySelectorAll(`${sel} *`).length > 0, overlay, { timeout: 30_000 });

      // The route's tracks light, and its pads fill (agni issue 1543), R120's included, since the
      // route crosses it.
      expect(await page.locator(`${overlay} line`).count()).toBeGreaterThan(0);
      expect(await page.locator(`${overlay} path[fill]`).count()).toBeGreaterThan(2);
      // The view followed the route, so the board is drawn larger than it was fitted.
      await page.mouse.move(5, 5);
      const host = page.locator(`${PANE} .sheetview-canvas`).first();
      const hb = await host.boundingBox();
      await page.mouse.move(hb!.x + hb!.width / 2, hb!.y + hb!.height / 2);
      const after = await page.locator(`${PANE} .sheetview-canvas svg`).first().evaluate((s) => s.getBoundingClientRect().width);
      expect(after).toBeGreaterThan(before * 2);
    });
  }, 120_000);
});
