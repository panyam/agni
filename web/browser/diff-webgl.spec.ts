// The visual diff draws in WebGL when the canvas does (agni issue 1335). Only a browser has WebGL2,
// so only a browser shows each side drawing its revision and lighting its changes in WebGL, and the
// union lighting the new revision's changes on one canvas. The pair is the tutorial gateway against
// its rev B, whose pages line up, so the union is offered.

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

// litOn is what one side's WebGL drawing lit (canvas.ts, data-lit), or null when it has not drawn.
const litOn = (page: Page, side: "a" | "b") =>
  page.evaluate((s) => {
    const raw = document.getElementById(`diff-gl-${s}`)?.dataset.lit;
    return raw ? (JSON.parse(raw) as string[]) : null;
  }, side);

describe("the visual diff in WebGL", () => {
  it("draws both revisions in WebGL, lights each side's changes, and lights the union", async () => {
    await withPage(browser, async (page) => {
      const against = encodeURIComponent("tut/designs/gateway/gateway-rev-b.edn");
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view?mode=webgl&against=${against}`, { waitUntil: "networkidle" });
      await expect.poll(() => litOn(page, "a"), { timeout: 60_000 }).not.toBeNull();
      await expect.poll(() => litOn(page, "b"), { timeout: 60_000 }).not.toBeNull();
      const a = (await litOn(page, "a"))!;
      const b = (await litOn(page, "b"))!;
      // Rev B adds R4 and R5, so only the new side lights them, and both light the nets it changed.
      expect(b).toEqual(expect.arrayContaining(["component:R4", "component:R5"]));
      expect(a).not.toContain("component:R4");
      expect(a).toEqual(expect.arrayContaining(["net:I2C_SCL", "net:I2C_SDA"]));
      // The SVG hosts stand aside while WebGL draws.
      expect(await page.$eval("#diff-svg-a", (e) => (e as HTMLElement).style.display)).toBe("none");

      await page.locator(".diff-modes .mode-btn").nth(1).click();
      await expect.poll(() => litOn(page, "a"), { timeout: 30_000 }).toEqual(expect.arrayContaining(b));
    });
  }, 180_000);
});
