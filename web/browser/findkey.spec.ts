// Command-F opens Find by name (agni issue 1472). jsdom has no dock and dispatches no browser
// shortcut, so only a browser shows that the key brings the Query panel forward from behind Trace,
// focuses the box, and leaves another text field's Command-F alone.

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

// openBehindTrace opens a design and puts the Trace tab in front of the Query panel.
async function openBehindTrace(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/kicad/hier_bus_root.kicad_sch/view?mode=svg`, { waitUntil: "networkidle" });
  await page.waitForSelector(".query-mode", { timeout: 30_000 });
  await page.locator(".dv-default-tab-content", { hasText: /^Trace$/ }).first().click();
  await page.waitForSelector(".query-modes", { state: "hidden", timeout: 10_000 });
}

describe("Command-F", () => {
  for (const key of ["Meta+f", "Control+f"]) {
    it(`brings Find by name forward and focuses its box on ${key}`, async () => {
      await withPage(browser, async (page) => {
        await openBehindTrace(page);
        await page.locator("#svg-view").click({ position: { x: 5, y: 5 } });
        await page.keyboard.press(key);
        await page.waitForSelector("input.query-term", { state: "visible", timeout: 10_000 });
        expect(await page.locator(".query-mode.on").innerText()).toBe("Find by name");
        expect(await page.evaluate(() => document.activeElement?.classList.contains("query-term"))).toBe(true);

        // Enter runs the search, and a hit lights its part on the drawing.
        await page.keyboard.type("R1");
        await page.keyboard.press("Enter");
        await page.locator(".query-row .query-locate", { hasText: /^R1$/ }).first().click({ timeout: 30_000 });
        await page.waitForFunction(() => (document.querySelector("[data-highlight]") as HTMLElement | null)?.dataset.highlight?.includes('"R1"'), null, { timeout: 30_000 });
      });
    }, 90_000);
  }

  it("selects the box's text when pressed again in the box", async () => {
    await withPage(browser, async (page) => {
      await openBehindTrace(page);
      await page.keyboard.press("Control+f");
      await page.keyboard.type("CAN");
      await page.keyboard.press("End");
      await page.keyboard.press("Control+f");
      expect(await page.$eval("input.query-term", (i) => [(i as HTMLInputElement).selectionStart, (i as HTMLInputElement).selectionEnd])).toEqual([0, 3]);
    });
  }, 90_000);

  it("leaves the key to the query box", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/kicad/hier_bus_root.kicad_sch/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector(".query-mode", { timeout: 30_000 });
      await page.locator("textarea.query-text").click();
      await page.keyboard.press("Control+f");
      expect(await page.locator(".query-mode.on").innerText()).toBe("Query");
      expect(await page.locator("input.query-term").count()).toBe(0);
    });
  }, 90_000);
});
