// A located subject glides into view instead of jumping (agni issue 1546). The suite runs with
// reduced motion, so every other walk sees the jump. This spec turns motion back on and samples the
// canvas's zoom while Find by name moves it, on the cut of Jetson.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser } from "playwright-core";
import { launch } from "./browser.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("a located subject", () => {
  for (const motion of ["no-preference", "reduce"] as const) {
    it(`${motion === "reduce" ? "jumps" : "glides"} into view with reduced motion ${motion === "reduce" ? "asked for" : "not asked for"}`, async () => {
      const context = await browser.newContext({ viewport: { width: 1400, height: 1000 }, reducedMotion: motion });
      const page = await context.newPage();
      try {
        await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg`, { waitUntil: "networkidle" });
        await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
        await page.keyboard.press("Control+f");
        await page.keyboard.type("TP76");
        await page.keyboard.press("Enter");
        const row = page.locator(".query-row .query-locate", { hasText: /^TP76$/ }).first();
        await row.waitFor({ timeout: 30_000 });
        // Sample the drawing's width over the canvas's each frame from the click until it settles.
        await page.evaluate(() => {
          const w = window as unknown as { zooms: number[] };
          w.zooms = [];
          const host = document.querySelector("#svg-view")!;
          const sample = () => {
            const doc = host.querySelector("svg");
            if (doc) w.zooms.push(doc.getBoundingClientRect().width / host.getBoundingClientRect().width);
            if (w.zooms.length < 120) requestAnimationFrame(sample);
          };
          requestAnimationFrame(sample);
        });
        await row.click();
        await page.waitForFunction(() => (window as unknown as { zooms: number[] }).zooms.length >= 120, null, { timeout: 30_000 });
        const zooms = await page.evaluate(() => (window as unknown as { zooms: number[] }).zooms);
        const final = zooms[zooms.length - 1];
        expect(final).toBeGreaterThan(3);
        // Views between the fitted sheet and the part, distinct from both, show a glide.
        const between = new Set(zooms.filter((z) => z > 1.2 && z < final * 0.9).map((z) => z.toFixed(2)));
        if (motion === "reduce") expect(between.size).toBe(0);
        else expect(between.size).toBeGreaterThanOrEqual(4);
      } finally {
        await context.close();
      }
    }, 120_000);
  }
});
