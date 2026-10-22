// The Tables panel against a real server (agni issue 898). A revision comparison's tables in the
// viewer are the rows `agni diff --rename-approx --format csv` writes for the same two files, and a
// net in them locates on the drawing. site.spec.ts checks the same comparison on the static demo.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser } from "playwright-core";
import { launch, withPage } from "./browser.js";
import { expectCompareTables, gatewayDiffRows } from "./comparetables.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("the Tables panel under Compare", () => {
  it("shows the comparison as agni diff --rename-approx writes it, and locates a net from it", async () => {
    const want = gatewayDiffRows();
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view`, { waitUntil: "networkidle" });
      await page.click("#compare button");
      await page.locator("#compare-tree").getByText("gateway-rev-c.edn").click();
      await expectCompareTables(page, want);
    });
  });

  // The link `agni diff --server` and an MCP diff answer print (agni issue 1126) opens the same
  // comparison with no click on the Compare picker.
  it("opens the same comparison from a link naming the revision to compare against", async () => {
    const want = gatewayDiffRows();
    await withPage(browser, async (page) => {
      const against = encodeURIComponent("tut/designs/gateway/gateway-rev-c.edn");
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view?against=${against}`, { waitUntil: "networkidle" });
      await expectCompareTables(page, want);
    });
  });

  // A link from a diff that paired near-renames opens tables that pair them, with the option already
  // ticked, so the rows match `agni diff --rename-approx` before the reader touches anything.
  it("pairs near-renames when the link says the diff did", async () => {
    const want = gatewayDiffRows();
    await withPage(browser, async (page) => {
      const against = encodeURIComponent("tut/designs/gateway/gateway-rev-c.edn");
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view?against=${against}&rename_approx=1`, { waitUntil: "networkidle" });
      await page.locator(".dv-default-tab-content", { hasText: /^Tables$/ }).first().click();
      await page.click('.tables-source[data-source="compare"]', { timeout: 60_000 });
      await page.waitForSelector(".tables-row", { timeout: 60_000 });
      expect(await page.isChecked(".tables-near-renames")).toBe(true);
      await page.waitForFunction((n) => document.querySelectorAll(".tables-row").length === n, want.length - 1, { timeout: 60_000 });
      const rows = await page.locator(".tables-row").evaluateAll((trs) => trs.map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent).join(",")));
      expect(rows).toEqual(want.slice(1));
    });
  });
});
