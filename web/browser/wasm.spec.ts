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
  // Run stays disabled until the rule catalog arrives, and a click waits for it to enable (agni 868).
  await page.click(".checks-run", { timeout: 60_000 });
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

      // Two lanes (agni issue 911): the checks ran on a worker of their own, so each worker fetched
      // the design's files, and a query never waits behind a check. The Load page read the design on
      // two workers of its own before the viewer opened (agni issue 1072), its drawing on one and its
      // board on another that ended, so the files were fetched four times and two workers are left.
      expect(page.workers().filter((w) => new URL(w.url()).pathname.endsWith("/agni-worker.js"))).toHaveLength(2);
      expect(wasm.network.filter((p) => p === "/raw/tut/designs/gateway/gateway.edn")).toHaveLength(4);
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

  // agni issue 911: every worker keeps what it read in the origin's private file system, and a
  // reloaded page restores from it. A read stores its result again, which rewrites the blob, so a
  // reload that restored leaves every blob as it was.
  it("restores a reloaded design from the browser's cache instead of reading it again", async () => {
    await withPage(browser, async (page) => {
      await openAndCheck(page, "?engine=wasm");
      const before = await storedBlobs(page);
      expect(Object.keys(before).length, "the first open stored nothing, so an unchanged store proves nothing").toBeGreaterThanOrEqual(2);

      // Opening the same address again starts new workers over the same storage, as a reload does.
      const again = await openAndCheck(page, "?engine=wasm");
      expect(again.rows.length).toBeGreaterThan(0);
      expect(await storedBlobs(page)).toEqual(before);
    });
  });
});

// agni issue 1072: the viewer opens only a design the Load page read, so a link straight to /view is
// read on /load first and lands where it pointed, and once read a reload opens it directly.
describe("a link to a design this browser has not read", () => {
  it("goes through the Load page, lands on the address it named, and a reload skips the Load page", async () => {
    await withPage(browser, async (page) => {
      const pages: string[] = [];
      page.on("framenavigated", (f) => {
        if (f === page.mainFrame()) pages.push(new URL(f.url()).pathname);
      });
      // A trace link carries its question in the address, which the viewer keeps as its own state, so
      // landing where it pointed shows as the same question in the final address.
      await page.goto(`${base()}${gateway}?engine=wasm&trace=R1.1%2CR1.2`, { waitUntil: "domcontentloaded" });
      await page.waitForSelector(".checks-run", { timeout: 60_000 });
      await expect.poll(() => new URL(page.url()).searchParams.get("trace"), { timeout: 60_000 }).toBe("R1.1,R1.2");
      const at = new URL(page.url());
      expect(at.pathname).toBe(gateway);
      expect(at.searchParams.get("engine")).toBe("wasm");
      expect(at.searchParams.has("loaded")).toBe(false);
      expect(pages).toContain(gateway.replace(/\/view$/, "/load"));

      pages.length = 0;
      await page.reload({ waitUntil: "domcontentloaded" });
      await page.waitForSelector(".checks-run", { timeout: 60_000 });
      expect(pages).toEqual([gateway]);
    });
  });
});

// storedBlobs lists the engine's cache, wasmhost/agni in the origin's private file system, by name
// and last write.
async function storedBlobs(page: Page): Promise<Record<string, number>> {
  return page.evaluate(async () => {
    const root = await navigator.storage.getDirectory();
    const dir = await (await root.getDirectoryHandle("wasmhost")).getDirectoryHandle("agni");
    const out: Record<string, number> = {};
    for await (const [name, h] of (dir as unknown as { entries(): AsyncIterable<[string, FileSystemFileHandle]> }).entries()) {
      if (!name.startsWith(".")) out[name] = (await h.getFile()).lastModified;
    }
    return out;
  });
}

// agni issue 918: a check run can be stopped on either engine, the button comes back, the panel says
// the run was cancelled, and the next run answers.
describe("cancelling Run checks", () => {
  // held keeps every matching request waiting until release, so a run is still in flight when the
  // test presses Cancel. A request the page aborted meanwhile is gone, so continuing it may throw.
  function hold(page: Page, pattern: string) {
    let open!: () => void;
    const released = new Promise<void>((r) => (open = r));
    const handler = async (route: import("playwright-core").Route) => {
      await released;
      await route.continue().catch(() => undefined);
    };
    return {
      on: () => page.route(pattern, handler),
      release: async () => {
        open();
        await page.unroute(pattern, handler);
      },
    };
  }

  async function cancelThenRun(page: Page, held: ReturnType<typeof hold>): Promise<void> {
    await page.waitForFunction(() => /\(\d+\)/.test(document.querySelector(".checks-run")?.textContent ?? ""), null, { timeout: 60_000 });
    await held.on();
    await page.click(".checks-run");
    await expect.poll(() => page.locator(".checks-run").textContent(), { timeout: 30_000 }).toBe("Cancel");
    // Past the busy overlay's delay, which once covered the whole page during a run, Cancel included.
    await page.waitForTimeout(1000);
    await page.click(".checks-run");
    await expect.poll(() => page.locator(".checks-run").textContent(), { timeout: 30_000 }).toMatch(/^Run checks \(\d+\)$/);
    expect(await page.locator(".checks-cancelled").textContent()).toMatch(/^Cancelled\. \d+ selected rules? (was|were) not run\.$/);
    expect(await page.locator(".check-locate").count()).toBe(0);
    await held.release();
    await page.click(".checks-run");
    await page.waitForSelector(".check-locate", { timeout: 60_000 });
    expect(await page.locator(".checks-cancelled").count()).toBe(0);
  }

  it("stops a run on the server engine, and the next run answers", async () => {
    await withPage(browser, async (page) => {
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(String(e)));
      await page.goto(`${base()}${gateway}`, { waitUntil: "domcontentloaded" });
      await cancelThenRun(page, hold(page, "**/agni.v1.webapi.CheckService/CheckDesign"));
      expect(errors).toEqual([]);
    });
  });

  // On the browser engine the run is on the jobs lane, and the jobs worker fetches the design's files
  // before it answers, which is where the run is held.
  it("stops a run on the browser engine, and the next run answers", async () => {
    await withPage(browser, async (page) => {
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(String(e)));
      await page.goto(`${base()}${gateway}?engine=wasm`, { waitUntil: "domcontentloaded" });
      await page.waitForSelector(".query textarea.query-text", { timeout: 60_000 });
      await cancelThenRun(page, hold(page, "**/raw/**"));
      expect(errors).toEqual([]);
    });
  });
});
