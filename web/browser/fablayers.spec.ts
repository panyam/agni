// A board opens with its fab layers hidden (agni issue 1486). Their text is each part's value and
// reference, and on a dense board it covers the copper and runs past the edges, which read as a
// rendering bug. jsdom has no layout, so only a browser shows the text itself gone, in both
// renderers, and back once the reader shows every layer.
//
// The board is the cross-probe cut of Jetson (examples/cross-probe-project), the one board in the
// tree with fab items.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { showPanel } from "./panels.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// shownText counts the texts the reader can see in a drawing's text layer.
function shownText(page: Page, root: string): Promise<number> {
  return page.evaluate((root) => {
    const svg = document.querySelector(root);
    if (!svg) throw new Error(`no ${root}`);
    return [...svg.querySelectorAll("text")].filter((t) => (t as SVGGraphicsElement).getBoundingClientRect().width > 0).length;
  }, root);
}

describe("a board's fab layers", () => {
  for (const mode of ["svg", "webgl"] as const) {
    it(`start hidden and show with the all preset (${mode})`, async () => {
      await withPage(browser, async (page) => {
        await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=${mode}&sheet=board`, { waitUntil: "networkidle" });
        await page.waitForFunction(() => performance.getEntriesByName("agni:sheet-drawn").length > 0, null, { timeout: 60_000 });
        await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 60_000 });
        const fab = page.locator('.board-layer-row input[aria-label="F.Fab"]').first();
        expect(await fab.isChecked()).toBe(false);
        const root = mode === "svg" ? "#svg-view svg" : "#text-overlay svg";
        const before = await shownText(page, root);

        await showPanel(page, "overview");
        await page.locator(".board-layer-preset", { hasText: /^all$/i }).first().click();
        expect(await fab.isChecked()).toBe(true);
        await expect.poll(() => shownText(page, root), { timeout: 30_000 }).toBeGreaterThan(before);
      });
    }, 180_000);
  }
});
