// A finding's net lights on an EDIF companion schematic drawn in WebGL (agni issue 1443). The design
// is examples/common/designs/i2c-sensor, an EDIF netlist whose design.yaml names its .eds export as
// the drawing. The .eds names its wires and carries no net ids, and a finding names its net by id.
// The server keys each wire with its net's id when it packs the sheet, and the finding's spec also
// carries the net's name for a wire keyed by name alone. Either half lights SCL, a name one net has.
// Before agni issue 1443 neither existed and SCL stayed dark. The canvas matches the finding against
// the packed keys in the page, so only a browser shows the whole join.

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

const DESIGN = "common/i2c-sensor/i2c-sensor.edn";

// lit reads the subjects the WebGL canvas lit (data-lit, canvas.ts).
const lit = (page: Page): Promise<string[]> => page.evaluate(() => JSON.parse((document.getElementById("view") as HTMLElement).dataset.lit ?? "[]") as string[]);

describe("a finding's net on an EDIF companion schematic in WebGL", () => {
  for (const engine of ["server", "wasm"]) {
    it(`lights SCL when every finding is shown (${engine} engine)`, async () => {
      await withPage(browser, async (page) => {
        const q = `mode=webgl${engine === "wasm" ? "&engine=wasm" : ""}`;
        await page.goto(`${base()}/designs/${DESIGN}/view?${q}`, { waitUntil: "networkidle" });
        await page.waitForSelector(".sheet-tile", { state: "attached", timeout: 60_000 });
        await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 60_000 });

        await page.click(".highlight-btn");
        const all = page.locator(".highlight-show-all");
        if (!(await all.isChecked())) await all.check();
        await page.click(".highlight-btn");
        await showPanel(page, "checks");
        await page.click(".checks-run");
        await page.waitForFunction(() => document.querySelector(".checks-run")?.textContent?.trim() === "Run checks", null, { timeout: 60_000 });
        expect(await page.locator(".check-locate", { hasText: /^SCL$/ }).count()).toBeGreaterThan(0);

        await expect.poll(() => lit(page), { timeout: 30_000 }).toContain("net:SCL");
        // The part's finding lit before this change too, so it shows the run reached the canvas.
        expect(await lit(page)).toContain("component:C1");
      });
    }, 180_000);
  }
});
