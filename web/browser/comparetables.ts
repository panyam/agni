// The Tables panel under Compare (agni issue 898), shared by the served page (tables.spec.ts) and the
// static demo (site.spec.ts), which opens the same comparison since its Compare picker lists the
// site's designs (agni issue 978). A revision comparison's tables in the viewer are the rows
// `agni diff --rename-approx --format csv` writes for the same two files, and a net in them locates
// on the drawing.

import { expect } from "vitest";
import type { Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));

// gatewayDiffRows are the csv rows the CLI writes comparing the gateway with its rev C, header first,
// from the agni this checkout builds with no personal agni.yaml in the way. Every cell is plain, so a
// line is a row, and the comparison is of lines as written.
export function gatewayDiffRows(): string[] {
  const home = mkdtempSync(join(tmpdir(), "agni-home-"));
  const csv = execFileSync(
    "go",
    [
      "run", "./cmd/agni", "diff",
      "examples/tutorial-project/designs/gateway/gateway.edn",
      "examples/tutorial-project/designs/gateway/gateway-rev-c.edn",
      "--rename-approx", "--format", "csv",
    ],
    { cwd: repo, env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home }, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] },
  );
  return csv.trimEnd().split("\n");
}

// expectCompareTables checks, on a page with the gateway's comparison against rev C open, that the
// Tables panel's comparison holds want's rows once near renames are asked for, and that a net in
// them locates on the drawing.
export async function expectCompareTables(page: Page, want: string[]): Promise<void> {
  expect(want.length).toBeGreaterThan(2);
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
}
