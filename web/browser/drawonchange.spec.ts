// A WebGL view draws only when what it shows changes (agni issue 1479). Before, every canvas redrew
// on every animation frame, so a board beside the schematic redrew the whole board while the reader
// panned the schematic. Only a browser has WebGL2 and frames, so only a browser shows the counts.
// Each canvas counts the frames it drew in data-draws (canvas.ts).

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");
const CANVAS = "#view";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// draws reads how many frames a canvas has drawn.
const draws = (page: Page, sel: string) => page.$eval(sel, (c) => Number((c as HTMLElement).dataset.draws ?? "0"));

// settle waits for a canvas to stop drawing, the count holding over a few frames.
async function settle(page: Page, sel: string): Promise<number> {
  let last = -1;
  for (let i = 0; i < 40; i++) {
    const n = await draws(page, sel);
    if (n === last && n > 0) return n;
    last = n;
    await page.waitForTimeout(150);
  }
  throw new Error(`${sel} kept drawing, at ${last} frames`);
}

// wheel turns the mouse wheel over a canvas, a step a frame, as a hand on a mouse would.
async function wheel(page: Page, sel: string, steps: number): Promise<void> {
  const b = (await page.locator(sel).boundingBox())!;
  await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, i < steps / 2 ? -120 : 120);
    await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => r(null))));
  }
}

// openBoardView opens the board beside the canvas in a view of its own, and returns its WebGL canvas.
async function openBoardView(page: Page): Promise<string> {
  const before = await page.$$eval("[data-sheetview]", (vs) => vs.map((v) => (v as HTMLElement).dataset.sheetview!));
  const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: /^Board$/ }) });
  await row.locator(".sheet-tile-beside").click();
  const id = await page.waitForFunction(
    (known) => [...document.querySelectorAll<HTMLElement>("[data-sheetview]")].map((v) => v.dataset.sheetview!).find((v) => !known.includes(v)) ?? false,
    before,
    { timeout: 30_000 },
  );
  const gl = `[data-sheetview="${await id.jsonValue()}"] .sheetview-gl`;
  await page.waitForSelector(`[data-sheetview="${await id.jsonValue()}"][data-renderer="webgl"]`, { timeout: 30_000 });
  await page.waitForFunction((s) => Number((document.querySelector(s) as HTMLElement | null)?.dataset.draws ?? "0") > 0, gl, { timeout: 30_000 });
  return gl;
}

describe("a WebGL view draws only when it changes", () => {
  it("leaves the board alone while the reader pans the schematic, and draws it when the board moves", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/view?mode=webgl`, { waitUntil: "networkidle" });
      await page.waitForFunction((s) => Number((document.querySelector(s) as HTMLElement | null)?.dataset.draws ?? "0") > 0, CANVAS, { timeout: 30_000 });
      await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 60_000 });
      const board = await openBoardView(page);

      // Nobody touches either drawing, so neither draws.
      const canvasStill = await settle(page, CANVAS);
      const boardStill = await settle(page, board);
      await page.waitForTimeout(500);
      expect(await draws(page, CANVAS)).toBe(canvasStill);
      expect(await draws(page, board)).toBe(boardStill);

      // Panning the schematic draws the schematic and never the board.
      await wheel(page, CANVAS, 20);
      expect(await draws(page, CANVAS)).toBeGreaterThan(canvasStill);
      expect(await draws(page, board)).toBe(boardStill);

      // Panning the board draws it.
      await wheel(page, board, 6);
      expect(await draws(page, board)).toBeGreaterThan(boardStill);
    });
  }, 120_000);

  it("still draws a locked view when its peer moves the camera", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/view?mode=webgl`, { waitUntil: "networkidle" });
      await page.waitForFunction((s) => Number((document.querySelector(s) as HTMLElement | null)?.dataset.draws ?? "0") > 0, CANVAS, { timeout: 30_000 });
      await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 60_000 });
      const first = await openBoardView(page);
      const second = await openBoardView(page);
      // The second view opens as a tab over the first, so it floats, and both are on screen.
      await page.locator(second.replace(" .sheetview-gl", " .sheetview-float")).click();
      await page.waitForFunction((s) => (document.querySelector(s) as HTMLElement | null)?.clientWidth, first, { timeout: 10_000 });
      // The floating view can lie over the first, so the link and the wheel go to each element itself.
      for (const gl of [first, second]) {
        await page.$eval(gl.replace(" .sheetview-gl", " .sheetview-link"), (el) => {
          const s = el as HTMLSelectElement;
          s.value = "locked";
          s.dispatchEvent(new Event("change"));
        });
      }
      const still = await settle(page, second);
      await page.$eval(first, (c) => {
        const r = c.getBoundingClientRect();
        for (let i = 0; i < 6; i++) c.dispatchEvent(new WheelEvent("wheel", { deltaY: -120, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2, bubbles: true, cancelable: true }));
      });
      await expect.poll(() => draws(page, second), { timeout: 10_000 }).toBeGreaterThan(still);
    });
  }, 120_000);
});
