// The library page uploads designs to a server's store, and they come back after a restart (agni
// issue 1719). It drives the page as a person does: two Jetson baseboards uploaded as two designs of
// one series through the new-design form, one schematic replaced on its design's page, the server
// restarted over the same --store-dir, and the newer design opened in the viewer.
//
// It uploads the Jetson revisions 1.1.2 and 1.1.3 when `make samples-revisions` fetched them, and
// otherwise the oracle corpus's Jetson board twice, so the gate runs it without the revisions.
// SCREENSHOT_DIR keeps a screenshot of each page it reaches.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { cpSync, existsSync, mkdirSync, mkdtempSync, appendFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const samples = join(repoRoot, "tools", "samples");
const revisions = join(samples, "revisions");
// LIBRARY_ORACLE=1 uploads the oracle corpus's board even where the revisions were fetched, as the
// gate does without them.
const useRevisions = process.env.LIBRARY_ORACLE !== "1" && existsSync(join(revisions, "jetson-agx-thor-baseboard-1.1.2"));
const older = useRevisions ? join(revisions, "jetson-agx-thor-baseboard-1.1.2") : join(samples, "boards", "jetson-agx-thor-baseboard");
const newer = useRevisions ? join(revisions, "jetson-agx-thor-baseboard-1.1.3") : older;
const shots = process.env.SCREENSHOT_DIR ?? "";

let browser: Browser;
let server: Server;
let port: number;
let store: string;

async function serve(): Promise<Server> {
  const base = `http://127.0.0.1:${port}`;
  return start({
    what: "agni serve (library)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--store-dir", store, "--web-dir", "web"],
    cwd: repoRoot,
    base,
    health: base,
  });
}

async function shot(page: Page, name: string): Promise<void> {
  if (!shots) return;
  mkdirSync(shots, { recursive: true });
  await page.screenshot({ path: join(shots, name), fullPage: true });
}

// upload makes a design through the list page's form from a folder.
async function upload(page: Page, id: string, folder: string): Promise<void> {
  await page.goto(`${server.base}/library/`);
  const form = page.getByTestId("new-design");
  await form.locator('input[name="id"]').fill(id);
  await form.locator('input[name="series"]').fill("jetson");
  await page.getByTestId("new-files-folder").setInputFiles(folder);
  await form.locator("text=files picked").waitFor();
  await form.locator("button.lb-submit").click();
  await page.waitForURL(`${server.base}/library/${id}`, { timeout: 120_000 });
  await page.locator("tr[data-artifact]").nth(1).waitFor({ timeout: 60_000 });
}

// artifactFiles asks the server for an artifact's files by hash, over Connect's JSON codec.
async function artifactFiles(name: string): Promise<Record<string, string>> {
  const r = await fetch(`${server.base}/agni.v1.webapi.DesignService/GetArtifact`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name }),
  });
  const a = (await r.json()) as { files: { path: string; sha256: string }[] };
  return Object.fromEntries(a.files.map((f) => [f.path, f.sha256]));
}

beforeAll(async () => {
  store = mkdtempSync(join(tmpdir(), "agni-library-"));
  port = await freePort();
  server = await serve();
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await server?.stop();
});

describe("the library page", () => {
  it("uploads two designs of a series, replaces a schematic, and keeps both across a restart", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${server.base}/`);
      await shot(page, "landing.png");
      await page.goto(`${server.base}/library/`);
      await expect.poll(() => page.getByTestId("library-status").textContent()).toBe("No designs yet.");

      await upload(page, "bb-1.1.2", older);
      await upload(page, "bb-1.1.3", newer);
      const types = await page.locator("tr[data-artifact] td:nth-child(2)").allTextContents();
      expect(types.sort()).toEqual(["board", "schematic"]);
      await shot(page, "design.png");

      await page.goto(`${server.base}/library/`);
      await page.locator('[data-design="designs/bb-1.1.3"]').waitFor();
      expect(await page.locator(".lb-series").getAttribute("data-series")).toBe("jetson");
      expect(await page.locator('[data-design="designs/bb-1.1.3"] .lb-latest').count()).toBe(1);
      expect(await page.locator('[data-design="designs/bb-1.1.2"] .lb-latest').count()).toBe(0);
      await shot(page, "library.png");

      // Replace 1.1.3's schematic with the files it holds, one sheet edited. A folder's stray sheets
      // stay out, as the server left them out of the artifact.
      const before = await artifactFiles("designs/bb-1.1.3/artifacts/schematic");
      const edited = mkdtempSync(join(tmpdir(), "agni-library-sch-"));
      for (const f of Object.keys(before)) cpSync(join(newer, f), join(edited, f));
      appendFileSync(join(edited, "power.kicad_sch"), "\n");
      await page.goto(`${server.base}/library/bb-1.1.3`);
      await page.getByTestId("replace-schematic-folder").setInputFiles(edited);
      await expect.poll(async () => (await artifactFiles("designs/bb-1.1.3/artifacts/schematic"))["power.kicad_sch"], { timeout: 60_000 }).not.toBe(before["power.kicad_sch"]);
      const after = await artifactFiles("designs/bb-1.1.3/artifacts/schematic");
      expect(Object.keys(after).sort()).toEqual(Object.keys(before).sort());

      // The server restarts over the same store, and the page reads everything back.
      await server.stop();
      server = await serve();
      await page.goto(`${server.base}/library/`);
      await page.locator('[data-design="designs/bb-1.1.3"]').waitFor({ timeout: 30_000 });
      expect(await page.locator(".lb-row").count()).toBe(2);
      expect(await page.locator('[data-design="designs/bb-1.1.3"] .lb-latest').count()).toBe(1);

      // The newer design opens in the viewer through its store path.
      await page.locator('[data-design="designs/bb-1.1.3"] a.lb-open').click();
      await page.waitForURL(/\/designs\/store\/bb-1\.1\.3\/schematic\/.*\/view/);
      await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 120_000 });
      await shot(page, "viewer.png");
    });
  }, 600_000);
});
