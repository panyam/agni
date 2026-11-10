// A page opened with ?timing logs one "agni draw" line per burst of pan and zoom (agni issue 1331),
// so a team that sees a board lag can send back where each frame's time went. jsdom has no frames,
// so only a browser shows the lines arriving with real frame times and the GPU's name.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import type { DrawReport } from "../src/drawprofile.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

const DESIGN = "common/i2c-sensor/i2c-sensor.edn";

// drawLines collects the canvas's "agni draw" lines as the page logs them.
function drawLines(page: Page): DrawReport[] {
  const out: DrawReport[] = [];
  page.on("console", (m) => {
    const t = m.text();
    if (t.startsWith("agni draw canvas: ")) out.push(JSON.parse(t.slice("agni draw canvas: ".length)) as DrawReport);
  });
  return out;
}

async function wheelOn(page: Page, sel: string): Promise<void> {
  const r = await page.locator(sel).boundingBox();
  if (!r) throw new Error(`${sel} is not laid out`);
  await page.mouse.move(r.x + r.width / 2, r.y + r.height / 2);
  for (let i = 0; i < 6; i++) {
    await page.mouse.wheel(0, i < 3 ? -120 : 120);
    await page.waitForTimeout(30);
  }
}

describe("?timing's draw profile", () => {
  for (const mode of ["webgl", "svg"] as const) {
    it(`logs a burst of zooms with its frame times (${mode})`, async () => {
      await withPage(browser, async (page) => {
        const lines = drawLines(page);
        await page.goto(`${base()}/designs/${DESIGN}/view?mode=${mode}&timing`, { waitUntil: "networkidle" });
        await page.waitForSelector(".sheet-tile", { timeout: 60_000 });
        await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 60_000 });
        await expect.poll(() => lines.length, { timeout: 30_000 }).toBeGreaterThan(0);
        expect(lines[0]).toMatchObject({ renderer: mode, bursts: 0 });

        await wheelOn(page, mode === "webgl" ? "#view" : "#svg-view");
        await expect.poll(() => lines.filter((l) => l.bursts > 0).length, { timeout: 30_000 }).toBeGreaterThan(0);
        const burst = lines.find((l) => l.bursts > 0)!;
        expect(burst.renderer).toBe(mode);
        expect(burst.frame.n).toBeGreaterThan(0);
        expect(burst.env.gpu).not.toBe("");
        expect(burst.env.cssWidth).toBeGreaterThan(0);
        expect(burst.env.cssHeight).toBeGreaterThan(0);
        if (mode === "webgl") {
          expect(burst.scene.primitives).toBeGreaterThan(0);
          expect(burst.draw.n).toBeGreaterThan(0);
          expect(burst.drawCalls.max).toBeGreaterThan(0);
        } else {
          expect(burst.scene.elements).toBeGreaterThan(0);
          expect(burst.draw.n).toBe(0);
        }
      });
    }, 180_000);
  }
});
