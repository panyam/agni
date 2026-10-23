// A parent page on another origin drives a framed viewer over postMessage (agni issue 1152). The
// unit tests hold the envelope, the allowlist and the dispatcher apart from a browser; this spec is
// the one place a real cross-origin frame, a real MessageEvent.origin and the served
// --embed-origin meet, which jsdom cannot stand in for.
//
// The parent is examples/embed-viewer/parent.html, served from a second port so its origin differs
// from the viewer's, and it is driven through the `agniCall` it exposes.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Frame, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";
import { serveStatic, type StaticServer } from "./staticserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

let browser: Browser;
let parent: StaticServer;
let viewer: Server;

beforeAll(async () => {
  parent = await serveStatic(join(repoRoot, "examples", "embed-viewer"), "/");
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  viewer = await start({
    what: "agni serve (embed)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--mount", "kicad=readers/kicad/testdata", "--web-dir", "web", "--embed-origin", new URL(parent.base).origin],
    cwd: repoRoot,
    base,
    health: base,
  });
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await viewer?.stop();
  parent?.close();
});

type Reply = { ok: boolean; body?: { location?: { sheet: string; mount: string; path: string } }; error?: { code: string; message: string } };

const call = (page: Page, method: string, body: unknown = {}): Promise<Reply> =>
  page.evaluate(([m, b]) => (window as unknown as { agniCall: (m: string, b: unknown) => Promise<Reply> }).agniCall(m as string, b), [method, body] as const);

async function activeTile(frame: Frame): Promise<string> {
  return (await frame.locator(".sheet-tile.active .sheet-tile-name").textContent()) ?? "";
}

describe("a parent page on another origin", () => {
  it("drives the framed viewer's sheet and reads where it landed", async () => {
    await withPage(browser, async (page) => {
      const design = `${viewer.base}/designs/kicad/hier_numeric_root.kicad_sch/view`;
      await page.goto(`${parent.base}parent.html?viewer=${encodeURIComponent(design)}`);
      const frame = page.frameLocator("#viewer");
      await frame.locator(".sheet-tile-name").nth(4).waitFor({ timeout: 60_000 });
      const viewerFrame = page.frames().find((f) => f.url().startsWith(viewer.base));
      if (!viewerFrame) throw new Error("the viewer frame never loaded");

      // The viewer announces itself only to an origin it takes requests from, so a parent the server
      // did not name waits here rather than at its first call.
      await page
        .waitForFunction(() => document.getElementById("status")?.textContent?.startsWith("viewer ready"), null, { timeout: 30_000 })
        .catch(() => {
          throw new Error("the viewer never announced itself to the parent page; was the parent's origin passed to --embed-origin?");
        });

      const first = await call(page, "GetLocation");
      expect(first.ok, JSON.stringify(first)).toBe(true);
      expect(first.body?.location).toMatchObject({ mount: "kicad", path: "hier_numeric_root.kicad_sch" });
      const rootSheet = first.body?.location?.sheet ?? "";
      expect(rootSheet).not.toBe("");

      // The reader moves the viewer by hand, and the parent reads the move.
      await frame.locator(".sheet-tile", { hasText: "IO/1" }).click();
      await viewerFrame.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO/1", null, { timeout: 30_000 });
      const moved = await call(page, "GetLocation");
      expect(moved.body?.location?.sheet).not.toBe(rootSheet);

      // The parent moves it back, and the viewer's own page shows it.
      const back = await call(page, "ShowSheet", { sheet: rootSheet });
      expect(back.ok, JSON.stringify(back)).toBe(true);
      expect(back.body?.location?.sheet).toBe(rootSheet);
      expect(await activeTile(viewerFrame)).toBe("Hier numeric root");

      // A sheet the design lacks is refused with a code, and the page stays put.
      const bad = await call(page, "ShowSheet", { sheet: "no-such-sheet" });
      expect(bad).toMatchObject({ ok: false, error: { code: "invalid_argument" } });
      expect(await activeTile(viewerFrame)).toBe("Hier numeric root");
    });
  }, 120_000);
});

// A saved check report beside a viewer it links into (agni issue 1156). The report is written by this
// checkout's agni from a fixture and served from an origin no server named, so the only request the
// viewer takes from it is ApplyLocation with the link alone.
describe("a check report beside the viewer", () => {
  let site: StaticServer;
  let viewerBase: string;
  const design = "hier_numeric_root.kicad_sch";

  beforeAll(async () => {
    viewerBase = inject("baseUrl");
    const dir = mkdtempSync(join(tmpdir(), "agni-report-pane-"));
    const home = mkdtempSync(join(tmpdir(), "agni-report-home-"));
    execFileSync(
      inject("agniBin"),
      ["check", `readers/kicad/testdata/${design}`, "--mount", "kicad=readers/kicad/testdata", "--server", viewerBase, "--verdicts", "--format", "html", "-o", join(dir, "report.html")],
      { cwd: repoRoot, env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home }, stdio: "ignore" },
    );
    copyFileSync(join(repoRoot, "examples", "embed-viewer", "two-pane.html"), join(dir, "two-pane.html"));
    site = await serveStatic(dir, "/");
  }, 120_000);

  afterAll(() => site?.close());

  // A link about a net drawn on the IO sheet alone, so landing there is the viewer moving.
  const ioLink = 'a[target="agni-viewer"][href*="R2-Pad1"]';

  it("moves the viewer beside it without reloading it", async () => {
    await withPage(browser, async (page) => {
      const viewerUrl = `${viewerBase}/designs/kicad/${design}/view`;
      await page.goto(`${site.base}two-pane.html?viewer=${encodeURIComponent(viewerUrl)}&report=report.html`);
      const viewer = page.frameLocator("#viewer");
      await viewer.locator(".sheet-tile-name").nth(4).waitFor({ timeout: 60_000 });
      const viewerFrame = page.frames().find((f) => f.name() === "agni-viewer");
      if (!viewerFrame) throw new Error("the viewer frame never loaded");
      await viewerFrame.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "Hier numeric root", null, { timeout: 30_000 });
      // A mark the page keeps only while it is not reloaded.
      await viewerFrame.evaluate(() => ((window as unknown as { kept: number }).kept = 1));

      await page.frameLocator("#report").locator(ioLink).first().click();
      await viewerFrame.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO", null, { timeout: 30_000 });
      expect(await viewerFrame.evaluate(() => (window as unknown as { kept?: number }).kept), "the viewer reloaded rather than moving").toBe(1);
    });
  }, 120_000);

  it("opens the link in one named viewer tab when no viewer is open", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${site.base}report.html`);
      const opened = page.context().waitForEvent("page");
      await page.locator(ioLink).first().click();
      const tab = await opened;
      await tab.waitForURL(/\/designs\/kicad\/hier_numeric_root\.kicad_sch\/(view|load)/, { timeout: 30_000 });
      expect(page.url(), "the report itself navigated away").toBe(`${site.base}report.html`);
      await tab.close();
    });
  }, 120_000);
});
