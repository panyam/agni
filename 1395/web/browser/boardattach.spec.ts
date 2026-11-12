// A reader attaches a board the design does not declare, and the page reads it as the design's
// board (agni issue 1367). The design is examples/common/designs/odb/gateway.edn, an EDIF netlist,
// beside gateway-layout, its ODB++ job, with no design.yaml naming the pair. Only a browser shows the
// picker, the reload with ?board=, and the browser engine bringing in files from outside the design.

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

const DESIGN = "common/odb/gateway.edn";
const JOB = "common/odb/gateway-layout";
// The server answers the job's matrix entry as the board to attach, the file a reader enters it by.
const BOARD = `${JOB}/matrix/matrix`;

async function open(page: Page, query = ""): Promise<void> {
  await page.goto(`${base()}/designs/${DESIGN}/view${query}`, { waitUntil: "networkidle" });
  await page.waitForSelector(".sheet-tile", { timeout: 60_000 });
}

const tiles = (page: Page) => page.$$eval(".sheet-tile-name", (ns) => ns.map((n) => n.textContent?.trim() ?? ""));

describe("attaching a board the design does not declare", () => {
  for (const engine of ["server", "wasm"]) {
    it(`picks the ODB++ job, reads it as the board, and keeps it (${engine} engine)`, async () => {
      await withPage(browser, async (page) => {
        const q = engine === "wasm" ? "?engine=wasm" : "";
        await open(page, q);
        expect(await tiles(page)).not.toContain("Board");

        await page.locator(".sheet-board-pick").click();
        await page.waitForSelector(`.board-picker-row[data-uri="mount://${JOB}"] .board-picker-use`, { timeout: 30_000 });
        await Promise.all([page.waitForURL(/[?&]board=/, { timeout: 30_000 }), page.locator(`.board-picker-row[data-uri="mount://${JOB}"] .board-picker-use`).click()]);
        await page.waitForSelector(".sheet-tile", { timeout: 60_000 });
        expect(new URL(page.url()).searchParams.get("board")).toBe(BOARD);
        await page.waitForFunction(() => [...document.querySelectorAll(".sheet-tile-name")].some((n) => n.textContent?.trim() === "Board"), null, { timeout: 60_000 });
        expect(await page.locator(".sheet-board-attached").innerText()).toContain(JOB);

        // The board draws, from the job's own files, in a view beside the canvas.
        const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: /^Board$/ }) });
        await row.locator(".sheet-tile-beside").click();
        await page.waitForSelector("[data-sheetview] .picks-front", { timeout: 60_000 });
        expect(await page.locator("[data-sheetview] .picks-front [data-ref]").count()).toBeGreaterThan(3);

        // A reload keeps it, and the design's own board takes it away again.
        await page.reload({ waitUntil: "networkidle" });
        await page.waitForFunction(() => [...document.querySelectorAll(".sheet-tile-name")].some((n) => n.textContent?.trim() === "Board"), null, { timeout: 60_000 });
        await Promise.all([page.waitForURL((u) => !u.searchParams.has("board"), { timeout: 30_000 }), page.locator(".sheet-board-clear").click()]);
        await page.waitForSelector(".sheet-tile", { timeout: 60_000 });
        expect(await tiles(page)).not.toContain("Board");
      });
    }, 180_000);
  }
});
