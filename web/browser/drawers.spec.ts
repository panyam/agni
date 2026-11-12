// The viewer's drawers and their shortcuts (agni issues 1549 and 1542). Sheets opens from the left,
// and Checks and Review from the right. jsdom lays nothing out and dispatches no browser shortcut,
// so only a browser shows that a button or a key puts a panel on screen and takes it off again.

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

async function open(page: Page): Promise<void> {
  await page.goto(`${base()}/designs/kicad/hier_bus_root.kicad_sch/view?mode=svg`, { waitUntil: "networkidle" });
  await page.waitForSelector(".query-mode", { timeout: 30_000 });
  await page.locator(".dock-drawer-btn").first().waitFor({ timeout: 30_000 });
}

// shown reports whether a panel's content is on screen, in the grid or in an open drawer.
const shown = (page: Page, id: string): Promise<boolean> =>
  page.evaluate((id) => {
    const hole = document.querySelector(`[data-dock-panel="${id}"]`);
    if (!hole || hole.closest("#panel-park")) return false;
    const r = hole.getBoundingClientRect();
    if (r.width === 0 || getComputedStyle(hole).visibility === "hidden") return false;
    return r.right > 0 && r.left < innerWidth;
  }, id);

describe("the drawer buttons", () => {
  for (const id of ["overview", "checks", "review"]) {
    it(`open and close the ${id} drawer`, async () => {
      await withPage(browser, async (page) => {
        await open(page);
        expect(await shown(page, id), "a drawer starts closed").toBe(false);
        const button = page.locator(`.dock-drawer-btn[data-panel="${id}"]`);
        await button.click();
        await expect.poll(() => shown(page, id)).toBe(true);
        expect(await button.getAttribute("class")).toContain("active");
        await button.click();
        await expect.poll(() => shown(page, id)).toBe(false);
        expect(await button.getAttribute("class")).not.toContain("active");
      });
    }, 90_000);
  }

  // goapplib issue 148. A drawer left open came back open on the next page, and its scrim took
  // every click on the drawing.
  it("brings a drawer left open back closed after a reload, with its tab still in front", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await page.locator('.dock-drawer-btn[data-panel="review"]').click();
      await expect.poll(() => shown(page, "review")).toBe(true);
      await page.reload({ waitUntil: "networkidle" });
      await page.locator(".dock-drawer-btn").first().waitFor({ timeout: 30_000 });
      expect(await shown(page, "review"), "the drawer comes back closed").toBe(false);
      expect(await page.locator(".dock-drawer-scrim:visible").count()).toBe(0);
      await page.locator('.dock-drawer-btn[data-panel="review"]').click();
      await expect.poll(() => shown(page, "review"), { message: "Review is still the drawer's front tab" }).toBe(true);
    });
  }, 90_000);

  it("closes the Sheets drawer once a sheet is picked, on that sheet", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await page.locator('.dock-drawer-btn[data-panel="overview"]').click();
      // The second tile is the sub-sheet, since the root opens first.
      const tile = page.locator(".sheet-tile").nth(1);
      const name = (await tile.locator(".sheet-tile-name").textContent())?.trim() ?? "";
      expect(name).not.toBe("");
      await tile.click();
      await page.waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() === n, name, { timeout: 30_000 });
      await expect.poll(() => shown(page, "overview")).toBe(false);
    });
  }, 90_000);
});

describe("the dock's shortcuts", () => {
  for (const mod of ["Meta", "Control"]) {
    it(`${mod}+G shows and hides the Sheets drawer`, async () => {
      await withPage(browser, async (page) => {
        await open(page);
        await page.locator("#svg-view").click({ position: { x: 5, y: 5 } });
        await page.keyboard.press(`${mod}+g`);
        await expect.poll(() => shown(page, "overview")).toBe(true);
        await page.keyboard.press(`${mod}+g`);
        await expect.poll(() => shown(page, "overview")).toBe(false);
      });
    }, 90_000);

    it(`${mod}+Shift+F brings Query forward from behind Trace and focuses the query box`, async () => {
      await withPage(browser, async (page) => {
        await open(page);
        await page.locator(".dv-default-tab-content", { hasText: /^Trace$/ }).first().click();
        await expect.poll(() => shown(page, "query")).toBe(false);
        await page.locator("#svg-view").click({ position: { x: 5, y: 5 } });
        await page.keyboard.press(`${mod}+Shift+f`);
        await expect.poll(() => shown(page, "query")).toBe(true);
        await expect.poll(() => page.evaluate(() => document.activeElement?.classList.contains("query-text"))).toBe(true);
      });
    }, 90_000);
  }

  it("closes an open drawer for Control+Shift+F, so the query box is not under it", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await page.locator('.dock-drawer-btn[data-panel="checks"]').click();
      await expect.poll(() => shown(page, "checks")).toBe(true);
      await page.keyboard.press("Control+Shift+f");
      await expect.poll(() => shown(page, "checks")).toBe(false);
      await expect.poll(() => page.evaluate(() => document.activeElement?.classList.contains("query-text"))).toBe(true);
    });
  }, 90_000);

  it("closes an open drawer on Escape and keeps the highlight, which the next Escape clears", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      const lit = () => page.evaluate(() => (document.querySelector("[data-highlight]") as HTMLElement | null)?.dataset.highlight ?? "");
      await page.keyboard.press("Control+f");
      await page.keyboard.type("R1");
      await page.keyboard.press("Enter");
      await page.locator(".query-row .query-locate", { hasText: /^R1$/ }).first().click({ timeout: 30_000 });
      await expect.poll(lit, { timeout: 30_000 }).toContain('"R1"');

      await page.locator('.dock-drawer-btn[data-panel="checks"]').click();
      await expect.poll(() => shown(page, "checks")).toBe(true);
      await page.keyboard.press("Escape");
      await expect.poll(() => shown(page, "checks")).toBe(false);
      expect(await lit()).toContain('"R1"');

      await page.keyboard.press("Escape");
      await expect.poll(lit).not.toContain('"R1"');
    });
  }, 90_000);

  it("leaves Control+G to a text field", async () => {
    await withPage(browser, async (page) => {
      await open(page);
      await page.locator("textarea.query-text").focus();
      await page.keyboard.press("Control+g");
      await page.waitForTimeout(300);
      expect(await shown(page, "overview")).toBe(false);
    });
  }, 90_000);
});
