// The board measuring tool reads millimetres in the board's own frame, in both renderers (agni issue
// 133). The board is core/render/testdata/dimensions.kicad_pcb, whose pad centres are held to
// kicad-cli's by TestBoardPadsSitWhereKiCadPutsThem. J1's two pins sit 2.54 mm apart on a header
// turned to 270 degrees, at (80, -65) and (80, -67.54) mm.
//
// The spec finds the pads the way a reader would, by the readout. It moves the cursor to two points,
// reads where the tool says they are, and from that places J1's pins on screen. Each click lands
// near a pin rather than on its centre, so the reading is exact only if the tool snaps.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { PANE, openBeside } from "./picks.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

const PIN_1 = { x: 80, y: -65 };
const PIN_2 = { x: 80, y: -67.54 };

function boardPoint(text: string): { x: number; y: number } {
  const m = text.match(/x (-?[\d.]+)\s+y (-?[\d.]+) mm/);
  if (!m) throw new Error(`the readout says ${JSON.stringify(text)}`);
  return { x: Number(m[1]), y: Number(m[2]) };
}

// screenOf places board points on screen from two cursor readings in the stage.
async function screenOf(page: Page, stage: string): Promise<(p: { x: number; y: number }) => { x: number; y: number }> {
  const box = await page.locator(stage).boundingBox();
  if (!box) throw new Error(`${stage} has no box`);
  const a = { x: box.x + box.width * 0.4, y: box.y + box.height * 0.45 };
  const b = { x: box.x + box.width * 0.6, y: box.y + box.height * 0.6 };
  const cursor = page.locator(`${stage} [data-measure-cursor]`);
  await page.mouse.move(a.x, a.y);
  const ma = boardPoint(await cursor.textContent() ?? "");
  await page.mouse.move(b.x, b.y);
  const mb = boardPoint(await cursor.textContent() ?? "");
  const sx = (b.x - a.x) / (mb.x - ma.x);
  const sy = (b.y - a.y) / (mb.y - ma.y);
  return (p) => ({ x: a.x + (p.x - ma.x) * sx, y: a.y + (p.y - ma.y) * sy });
}

async function measureJ1(page: Page, stage: string): Promise<string> {
  const at = await screenOf(page, stage);
  const p1 = at(PIN_1);
  const p2 = at(PIN_2);
  // Off each pin's centre by a couple of pixels, still on its 1.7 mm land.
  await page.mouse.click(p1.x + 2, p1.y + 2);
  await page.mouse.click(p2.x - 2, p2.y + 1);
  return (await page.locator(`${stage} [data-measure-span]`).textContent()) ?? "";
}

describe("the board measuring tool", () => {
  for (const mode of ["svg", "webgl"] as const) {
    it(`measures J1's pitch to the micrometre in ${mode}`, async () => {
      await withPage(browser, async (page) => {
        await page.goto(`${base()}/files/render/dimensions.kicad_pcb/view?mode=${mode}`, { waitUntil: "networkidle" });
        const button = page.locator(".canvas-measure");
        await button.waitFor({ state: "visible", timeout: 30_000 });
        await button.click();
        expect(await button.getAttribute("aria-pressed")).toBe("true");

        expect(await measureJ1(page, ".canvas-wrap")).toBe("2.540 mm  (Δx 0.000, Δy -2.540)");
        expect(await page.locator(".canvas-wrap [data-measure]").getAttribute("data-measure-points")).toBe("2");
        expect(await page.locator(".canvas-wrap .measure-lines line").count()).toBe(1);
        // Measuring selects nothing. Properties still names the design.
        expect(await page.locator('[data-component="properties"] .pr-title').textContent()).toBe("dimensions.kicad_pcb");

        await page.locator(".canvas-wrap [data-action=measure-unit]").click();
        expect(await page.locator(".canvas-wrap [data-measure-span]").textContent()).toBe("100.0 mil  (Δx 0.0, Δy -100.0)");

        await page.keyboard.press("Escape");
        expect(await button.getAttribute("aria-pressed")).toBe("false");
        expect(await page.locator(".canvas-wrap [data-measure]").isHidden()).toBe(true);
      });
    }, 120_000);
  }

  it("measures in a board view beside the canvas", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/files/render/dimensions.kicad_pcb/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      await openBeside(page, "Board");
      const stage = `${PANE} .sheetview-stage`;
      const button = page.locator(`${PANE} [data-action=measure]`);
      await button.click();
      expect(await button.getAttribute("aria-pressed")).toBe("true");
      expect(await measureJ1(page, stage)).toBe("2.540 mm  (Δx 0.000, Δy -2.540)");
      // The canvas's own tool stays off, so one view's clicks measure in that view alone.
      expect(await page.locator(".canvas-wrap [data-measure]").isHidden()).toBe(true);
      // A press on the unit button switches units and sets no point, as a reader's mouse would.
      const unit = await page.locator(`${stage} [data-action=measure-unit]`).boundingBox();
      if (!unit) throw new Error("the unit button is not on screen");
      // The pointer arrives first, and the cursor's readout changes as it does.
      await page.mouse.move(unit.x + unit.width / 2, unit.y + unit.height / 2);
      await page.mouse.down();
      await page.mouse.up();
      expect(await page.locator(`${stage} [data-measure-span]`).textContent()).toBe("100.0 mil  (Δx 0.0, Δy -100.0)");
    });
  }, 120_000);

  it("hides on a schematic", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/files/kicad/bus.kicad_sch/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      expect(await page.locator(".canvas-measure").isHidden()).toBe(true);
    });
  }, 120_000);
});
