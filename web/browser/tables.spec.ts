// The Tables panel against a real server (agni issue 898). A revision comparison's tables in the
// viewer are the rows `agni diff --rename-approx --format csv` writes for the same two files, and a
// net in them locates on the drawing.
//
// It runs against the served page rather than the static demo because the static demo's Compare picker
// lists no designs yet (agni issue 978), so there is no comparison to open there.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// cli runs the agni this checkout builds, with no personal agni.yaml in the way.
function cli(...args: string[]): string {
  const home = mkdtempSync(join(tmpdir(), "agni-home-"));
  return execFileSync("go", ["run", "./cmd/agni", ...args], {
    cwd: repo,
    env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home },
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

// csvRows splits csv text into lines with the header first. Every cell here is plain, so a line is a
// row; the comparison is of lines as written.
const csvRows = (text: string): string[] => text.trimEnd().split("\n");

describe("the Tables panel under Compare", () => {
  it("shows the comparison as agni diff --rename-approx writes it, and locates a net from it", async () => {
    const want = csvRows(
      cli(
        "diff",
        "examples/tutorial-project/designs/gateway/gateway.edn",
        "examples/tutorial-project/designs/gateway/gateway-rev-c.edn",
        "--rename-approx",
        "--format",
        "csv",
      ),
    );
    expect(want.length).toBeGreaterThan(2);
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view`, { waitUntil: "networkidle" });
      await page.click("#compare button");
      await page.locator("#compare-tree").getByText("gateway-rev-c.edn").click();
      await page.locator(".dv-default-tab-content", { hasText: /^Tables$/ }).first().click();
      await page.click('.tables-source[data-source="compare"]', { timeout: 60_000 });
      await page.waitForSelector(".tables-row", { timeout: 60_000 });
      await page.check(".tables-near-renames");
      // The near-rename pass pairs rev C's renamed net, so the table changes; wait for that answer.
      await page.waitForFunction((n) => document.querySelectorAll(".tables-row").length === n, want.length - 1, { timeout: 60_000 });
      const header = await page.locator(".tables-table th").evaluateAll((ths) => ths.map((th) => th.getAttribute("data-column")).join(","));
      const rows = await page.locator(".tables-row").evaluateAll((trs) => trs.map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent).join(",")));
      expect([header, ...rows]).toEqual(want);
      expect(rows.some((r) => r.startsWith("net-renamed-approx,"))).toBe(true);

      await page.locator(".tables-locate", { hasText: /^I2C_SCL$/ }).first().click();
      await page.waitForFunction(() => document.querySelectorAll("#svg-view .highlight-overlay svg *").length > 0, null, { timeout: 30_000 });
    });
  });
});
