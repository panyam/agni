// A net lit on the board repaints its own copper (agni issue 1543). jsdom draws no board, so only a
// browser against a server shows the board view beside the canvas asking HighlightSheet for a found
// net and drawing its pads filled rather than ringed. The design is the gate's cut of Jetson, and the
// board view opens in SVG on headless Chrome's software GPU. canvasboardnet.test.ts covers WebGL.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { PANE, openBeside } from "./picks.js";

const base = (): string => inject("baseUrl");
const NET = "/USB Debug, PD/FTDI_3V3";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("a net lit on the board", () => {
  it("fills its pads and vias on the board view rather than ringing them", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      await openBeside(page, "Board");
      await page.keyboard.press("Control+f");
      await page.keyboard.type("FTDI_3V3");
      await page.keyboard.press("Enter");
      await page.locator(".query-row .query-locate", { hasText: NET }).first().click({ timeout: 30_000 });
      const overlay = `${PANE} .highlight-overlay svg`;
      await page.waitForFunction((sel) => document.querySelectorAll(`${sel} path`).length > 0, overlay, { timeout: 30_000 });
      const drawn = await page.$$eval(`${overlay} *`, (els) => els.map((e) => ({ tag: e.tagName, fill: e.getAttribute("fill") ?? "", stroke: e.getAttribute("stroke-opacity") ?? "" })));
      const filledLands = drawn.filter((e) => e.tag === "path" && e.fill !== "" && e.fill !== "none");
      const rings = drawn.filter((e) => e.tag === "circle" && e.fill === "none");
      expect(filledLands.length).toBeGreaterThan(2);
      expect(rings).toEqual([]);
      // The viewer's default path opacity of 0.4 repaints the track opaque.
      expect(drawn.filter((e) => e.tag === "line").every((e) => e.stroke === "1.0")).toBe(true);
    });
  }, 120_000);
});
