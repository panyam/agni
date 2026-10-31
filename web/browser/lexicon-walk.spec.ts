// Mission #1264's step 4 (agni issue 1267): the Lexicon panel on the MNT Reform motherboard, against a
// real server. The rows Try shows are the rows `agni lexicon --try` writes for the same edit, and the
// saved file is the candidate that command read. Apply then carries the edit to Properties.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";
import { agniBin } from "./agnibin.js";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const board = "tools/samples/boards/mnt-reform-motherboard30";
const view = "/designs/boards/mnt-reform-motherboard30/reform2-motherboard30.kicad_sch/view";

// cliTry is the two tables `agni lexicon --try` answers with, each as its rows' cells joined.
function cliTry(candidate: string): Record<string, string[]> {
  const home = mkdtempSync(join(tmpdir(), "agni-home-"));
  const out = execFileSync(agniBin(), ["lexicon", board, "--try", candidate, "--format", "json"], {
    cwd: repo,
    env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home },
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"],
    maxBuffer: 1 << 26,
  });
  const tables = JSON.parse(out).tables as { name: string; rows?: { cells?: string[] }[] }[];
  return Object.fromEntries(tables.map((t) => [t.name, (t.rows ?? []).map((r) => (r.cells ?? []).join("|"))]));
}

const panelRows = (page: Page, table: string) =>
  page.locator(`.lex-result[data-table="${table}"] .lex-change`).evaluateAll((trs) => trs.map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent).join("|")));

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("the Lexicon panel", () => {
  it("tries an edit, matches agni lexicon --try row for row, saves the same file, and applies it", async () => {
    const want = cliTry("hack/exercise-naming-usbh.yaml");
    expect(want.role_changes.length).toBeGreaterThan(0);
    expect(want.finding_changes.length).toBeGreaterThan(0);
    await withPage(browser, async (page) => {
      await page.goto(`${inject("baseUrl")}${view}`, { waitUntil: "domcontentloaded" });
      await page.locator(".dv-default-tab-content", { hasText: /^Lexicon$/ }).first().click({ timeout: 120_000 });
      const rail = page.locator('.lex-group[data-vocabulary="net.rail"]');
      await rail.locator("summary").click({ timeout: 120_000 });
      await page.fill('[aria-label="convention name"]', "exercise-naming");
      await page.press('[aria-label="convention name"]', "Tab");
      await rail.locator('[aria-label="add to net.rail"]').fill("^USBH_3V3$");
      await rail.locator(".lex-add button").click();
      await rail.locator(".lex-row-edited .lex-value", { hasText: "^USBH_3V3$" }).waitFor();

      await page.click(".lex-try");
      await page.waitForSelector('.lex-result[data-table="finding_changes"]', { timeout: 120_000 });
      expect(await panelRows(page, "role_changes")).toEqual(want.role_changes);
      expect(await panelRows(page, "finding_changes")).toEqual(want.finding_changes);

      // A row selects what it names.
      await page.locator(".lex-change").first().click();
      await expect.poll(() => page.locator("#properties-panel").textContent(), { timeout: 30_000 }).toContain("USBH_3V3");

      const [dl] = await Promise.all([page.waitForEvent("download"), page.click(".lex-save")]);
      expect(dl.suggestedFilename()).toBe("exercise-naming.yaml");
      const saved = readFileSync(await dl.path(), "utf8");
      expect(saved).toBe('name: exercise-naming\nlexicon:\n  net:\n    rail:\n      patterns:\n        - ^USBH_3V3$\n');
      const file = join(mkdtempSync(join(tmpdir(), "agni-saved-")), "saved.yaml");
      writeFileSync(file, saved);
      expect(cliTry(file)).toEqual(want);

      // Apply reaches the bar and Properties, whose roles are read under the convention on screen.
      await page.click(".lex-apply");
      await expect.poll(() => page.locator(".convbar-active").textContent(), { timeout: 60_000 }).toBe("exercise-naming");
      await expect.poll(() => page.locator("#properties-panel").textContent(), { timeout: 60_000 }).toContain("the name matches the rail pattern ^USBH_3V3$");
    });
  }, 300_000);
});
