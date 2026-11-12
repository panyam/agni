// The Trace panel offers as end pins only those a trace from the start pin reaches, within the hop
// count the reader picks (agni issue 1541). jsdom runs no engine, so only a browser against a server
// shows the panel asking DesignService.ReachablePins and narrowing its list to the answer. The design
// is the gate's cut of Jetson, where R119.1 reaches seven pins within one crossing and nine within
// six, D21.2 two crossings away through R120.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
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

const listed = (page: Page) => page.$$eval(".trace-pick-value", (ns) => ns.map((n) => n.textContent ?? ""));

async function hops(page: Page, n: number): Promise<void> {
  await page.selectOption("select.trace-hops", String(n));
}

describe("the Trace panel's end pin list", () => {
  it("offers only the pins a trace from the start pin reaches, within the hops picked", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg`, { waitUntil: "networkidle" });
      await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
      await page.locator(".dv-default-tab-content", { hasText: /^Trace$/ }).first().click();
      const [from, to] = [page.locator("input.trace-pin").nth(0), page.locator("input.trace-pin").nth(1)];
      // The pickers fill from the Properties inventory, which arrives after the first draw.
      await page.waitForFunction(() => (document.querySelector("input.trace-pin") as HTMLInputElement | null)?.placeholder.includes("."), null, { timeout: 30_000 });
      await from.fill("R119.1");

      await to.click();
      await page.waitForFunction(() => document.querySelectorAll(".trace-pick-value").length > 0 && [...document.querySelectorAll(".trace-pick-value")].some((n) => n.textContent === "D21"), null, { timeout: 30_000 });
      const wide = await listed(page);
      expect(wide).toEqual(["C44", "C133", "C134", "C135", "D20", "D21", "R119", "R120"]);
      expect(wide).not.toContain("TP76");

      await hops(page, 1);
      await to.click();
      await page.waitForFunction(() => ![...document.querySelectorAll(".trace-pick-value")].some((n) => n.textContent === "D21"), null, { timeout: 30_000 });
      expect(await listed(page)).toEqual(["C44", "C133", "C134", "C135", "D20", "R119", "R120"]);

      await hops(page, 6);
      await to.fill("D21.");
      await page.waitForFunction(() => [...document.querySelectorAll(".trace-pick-value")].some((n) => n.textContent === "D21.2"), null, { timeout: 30_000 });
      expect(await page.locator(".trace-pick-detail").first().innerText()).toContain("2 crossings");
      await to.fill("D21.2");
      await page.click("button.trace-run");
      await page.waitForFunction(() => (document.querySelector(".trace")?.textContent ?? "").includes("R120"), null, { timeout: 30_000 });
      expect(await page.locator(".trace-unresolved").count()).toBe(0);
    });
  }, 120_000);
});
