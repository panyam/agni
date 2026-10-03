// The viewer on the in-browser engine (agni issue 178, mission 851).
//
// `?engine=wasm` loads agni.wasm into a Web Worker, mounts the tutorial project from the seed, and
// routes every Connect client to the worker. The server under test still serves the page and its
// static files, and it does NOT mount the tutorial project, so a panel that reached the network for
// its answer would get a not-found and the assertions below would fail. The request log is the
// direct check: no API path leaves the page, which is the property the mission promises a visitor
// whose design must not leave their machine.
//
// What the engine answers is held to the server's by TestWasmEngineAnswersAsTheServerDoes in Go.
// This asserts the page drives it: the drawing loads, checks run to findings, and a query answers.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser } from "playwright-core";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

const apiPrefix = "/agni.v1.webapi.";

describe("the viewer on the in-browser engine", () => {
  it("opens, checks and queries a design with no API request leaving the page", async () => {
    await withPage(browser, async (page) => {
      const network: string[] = [];
      page.on("request", (r) => network.push(new URL(r.url()).pathname));
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(String(e)));

      await page.goto(`${base()}/designs/tut/designs/gateway/view?engine=wasm&seed=/static/seed/tut.json`, {
        waitUntil: "domcontentloaded",
      });
      await page.waitForSelector(".query textarea.query-text", { timeout: 60_000 });
      // The drawing loaded, so GetDesign and GetSheet were answered.
      await expect
        .poll(() => page.locator("#readout").textContent(), { timeout: 60_000 })
        .not.toBe("no sheet loaded");

      // Wait for the button to carry its rule count. On this engine the first sheet can draw before
      // ListRules answers, and a click before then runs no rules at all (agni 868).
      await expect.poll(() => page.locator(".checks-run").textContent(), { timeout: 60_000 }).toMatch(/\(\d+\)/);
      await page.click(".checks-run");
      await page.waitForSelector(".check-locate", { timeout: 60_000 });
      expect(await page.locator(".check-locate").count()).toBeGreaterThan(0);

      await page.fill("textarea.query-text", "net.has_test_point(?n) => ?n");
      await page.click("button.query-run");
      await page.waitForSelector(".query-row", { timeout: 60_000 });

      expect(errors).toEqual([]);
      // Positive control: the engine's own assets were fetched, so the log is recording this page.
      expect(network.some((p) => p.endsWith("/agni.wasm"))).toBe(true);
      expect(network.filter((p) => p.startsWith(apiPrefix))).toEqual([]);
    });
  });
});
