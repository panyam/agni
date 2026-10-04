// One design, one URL, either engine (agni issues 178 and 853).
//
// The tutorial gateway is opened at its own `/designs/tut/...` URL twice: once on the server engine,
// once with `?engine=wasm`, where the page loads agni.wasm into a Web Worker, brings the design's
// files over from the server, and routes every analysis client to the worker. Both runs must find
// the same thing. On wasm, the request log is the direct check that the analysis happened in the
// page: the only API calls reaching the network are WorkspaceService's, which list the design's
// files, and the bytes come from /raw/.
//
// What the engine answers is held to the server's, rule by rule, by TestWasmEngineAnswersAsTheServerDoes
// in Go. This asserts the page drives it.

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

const gateway = "/designs/tut/designs/gateway/view";
const apiPrefix = "/agni.v1.webapi.";
const workspacePrefix = "/agni.v1.webapi.WorkspaceService/";

// openAndCheck opens the gateway, runs every rule and a query, and returns the finding rows and every
// path the page requested.
async function openAndCheck(page: Page, query: string): Promise<{ rows: string[]; network: string[] }> {
  const network: string[] = [];
  page.on("request", (r) => network.push(new URL(r.url()).pathname));
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(`${base()}${gateway}${query}`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".query textarea.query-text", { timeout: 60_000 });
  await expect.poll(() => page.locator("#readout").textContent(), { timeout: 60_000 }).not.toBe("no sheet loaded");
  // Wait for the button to carry its rule count. The first sheet can draw before ListRules answers,
  // and a click before then runs no rules at all (agni 868).
  await expect.poll(() => page.locator(".checks-run").textContent(), { timeout: 60_000 }).toMatch(/\(\d+\)/);
  await page.click(".checks-run");
  await page.waitForSelector(".check-locate", { timeout: 60_000 });
  const rows = await page.locator(".check-locate").allTextContents();

  await page.fill("textarea.query-text", "net.has_test_point(?n) => ?n");
  await page.click("button.query-run");
  await page.waitForSelector(".query-row", { timeout: 60_000 });

  expect(errors).toEqual([]);
  return { rows, network };
}

describe("one design under both engines", () => {
  it("finds the same thing on the server and in the browser, and on wasm sends only listings and files", async () => {
    let server: { rows: string[]; network: string[] } = { rows: [], network: [] };
    await withPage(browser, async (page) => {
      server = await openAndCheck(page, "");
    });
    await withPage(browser, async (page) => {
      const wasm = await openAndCheck(page, "?engine=wasm");

      expect(server.rows.length, "the server run found nothing, so agreeing with it proves nothing").toBeGreaterThan(0);
      expect([...wasm.rows].sort()).toEqual([...server.rows].sort());

      // Positive controls: the server run did reach the network for its analysis, and the wasm run
      // fetched the engine and the design's files, so the log is recording this page.
      expect(server.network.some((p) => p.startsWith(`${apiPrefix}CheckService/`))).toBe(true);
      expect(wasm.network.some((p) => p.endsWith("/agni.wasm"))).toBe(true);
      expect(wasm.network.some((p) => p === "/raw/tut/designs/gateway/gateway.edn")).toBe(true);
      expect(wasm.network.some((p) => p === `${workspacePrefix}ListDesignFiles`)).toBe(true);

      const analysis = wasm.network.filter((p) => p.startsWith(apiPrefix) && !p.startsWith(workspacePrefix));
      expect(analysis).toEqual([]);
    });
  });

  // agni issue 852: a design over the page's size limit is analysed on the server instead, the page
  // says so, and the answers are the server's.
  it("sends a design over the browser's size limit to the server and says why", async () => {
    await withPage(browser, async (page) => {
      const over = await openAndCheck(page, "?engine=wasm&wasm-max-bytes=1000");
      expect(over.rows.length).toBeGreaterThan(0);
      expect(over.network.some((p) => p.startsWith(`${apiPrefix}CheckService/`))).toBe(true);
      expect(over.network.some((p) => p.startsWith("/raw/"))).toBe(false);
      const note = page.locator("#engine-note");
      await expect.poll(() => note.isVisible()).toBe(true);
      expect(await note.textContent()).toMatch(/Analysed on the server: its files are .* over the 1 KB the browser engine takes/);
    });
  });
});
