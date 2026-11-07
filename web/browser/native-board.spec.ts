// Mission #1353's third step: open an Altium or Allegro board straight from its native file, see the
// board, and click a pad to pick that pad (agni issue 1383). The boards come from the samples corpus
// (`make samples-oracle`), so a board that is missing fails the spec rather than skipping it.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { pickPoint, TITLE, zoomTo } from "./picks.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

const boards = [
  { name: "the BeagleBone AI's Allegro .brd", design: "allegro/beaglebone-ai/BeagleBone-AI.brd" },
  { name: "foxBMS's Altium .PcbDoc", design: "altium/foxbms/hw-master/master.PcbDoc" },
];

async function openBoard(page: Page, design: string): Promise<void> {
  await page.goto(`${base()}/designs/${design}/view`, { waitUntil: "networkidle" });
  // A board-only design lists its board as a sheet. Open it unless the viewer opened it already.
  const tile = page.locator(".sheet-tile", { hasText: /board/i }).first();
  if (await tile.count()) await tile.click();
  await page.waitForSelector("#svg-view .copper-front [data-kind=\"pin\"][data-net]", { timeout: 60_000 });
}

describe("opening a native board", () => {
  for (const b of boards) {
    it(`draws ${b.name} and picks a pad as its pin`, async () => {
      await withPage(browser, async (page) => {
        await openBoard(page, b.design);
        // The largest front pad on a net, so the zoom has the least distance to go.
        const pad = await page.$$eval('#svg-view .copper-front [data-kind="pin"][data-net]', (els) => {
          const area = (e: Element) => {
            const r = e.getBoundingClientRect();
            return r.width * r.height;
          };
          const el = els.reduce((a, c) => (area(c) > area(a) ? c : a));
          return { ref: el.getAttribute("data-ref")!, pin: el.getAttribute("data-pin")! };
        });
        const sel = `#svg-view [data-kind="pin"][data-ref="${pad.ref}"][data-pin="${pad.pin}"]`;
        await zoomTo(page, sel);
        const at = await pickPoint(page, sel, "pin");
        const want = `${pad.ref}.${pad.pin}`;
        // Nothing is picked yet, so a title already naming the pad would prove nothing.
        expect(await page.locator(TITLE).textContent().catch(() => "")).not.toBe(want);
        await page.mouse.click(at.x, at.y);
        try {
          await page.waitForFunction(([t, w]) => document.querySelector(t)?.textContent === w, [TITLE, want] as const, { timeout: 20_000 });
        } catch {
          expect(await page.locator(TITLE).textContent(), `the click on ${want}`).toBe(want);
        }
      });
    }, 180_000);
  }
});
