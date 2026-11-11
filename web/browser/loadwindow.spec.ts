// A design draws its sheet before its rules, report, parts and reviews arrive. On a slow machine the
// rest can take seconds, and until agni issue 1454 the busy overlay covered the drawn sheet the whole
// time and took every click and wheel. This spec holds the rules rpc back, so that window stays open
// as long as the spec needs, and acts on the sheet inside it, as a reader would.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Route } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { TITLE, pickPoint } from "./picks.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("while a design's rules load", () => {
  it("the drawn sheet takes a click, and a pill that takes no input says what is loading", async () => {
    await withPage(browser, async (page) => {
      const held: Route[] = [];
      await page.route(/\/ListRules$/, (route) => void held.push(route));
      await page.goto(`${base()}/designs/kicad/geom.kicad_sch/view?mode=svg`, { waitUntil: "domcontentloaded" });
      await page.waitForSelector('#svg-view svg [data-kind="component"]', { timeout: 30_000 });
      await expect.poll(() => held.length, { timeout: 30_000 }).toBeGreaterThan(0);
      expect(await page.evaluate(() => document.getElementById("render-busy")?.dataset.busy)).toBe("on");

      const ref = await page.getAttribute('#svg-view svg [data-kind="component"]', "data-ref");
      const at = await pickPoint(page, `#svg-view svg [data-kind="component"][data-ref="${ref}"]`, "component");
      await page.mouse.click(at.x, at.y);
      await expect.poll(() => page.locator(TITLE).textContent(), { timeout: 10_000 }).toBe(ref);

      const pill = await page.evaluate(() => {
        const el = document.getElementById("render-busy")!;
        const s = getComputedStyle(el);
        return { soft: el.classList.contains("soft"), display: s.display, events: s.pointerEvents, label: el.textContent?.trim() };
      });
      expect(pill).toEqual({ soft: true, display: "flex", events: "none", label: "loading rules…" });

      for (const r of held) await r.continue();
      await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 30_000 });
      expect(await page.evaluate(() => getComputedStyle(document.getElementById("render-busy")!).display)).toBe("none");
    });
  }, 120_000);
});
