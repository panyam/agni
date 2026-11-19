// A mounted folder's boards become designs only when a person adds them (agni issue 1736). The walk
// mounts a folder holding Jetson's two releases with no design.yaml, sees the designs page list none,
// browses to the folder, ticks one release, adds it, and opens the design it made. It uses the
// oracle corpus's Jetson board twice when `make samples-revisions` has not fetched the releases, so
// the gate runs it without them. That copy holds two sheets its root never reaches, so each folder
// proposes three boards, and the walk ticks the root by its entry. SCREENSHOT_DIR keeps a screenshot
// of each page it reaches.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { launch, withPage } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const samples = join(repoRoot, "tools", "samples");
const revisions = join(samples, "revisions");
const useRevisions = existsSync(join(revisions, "jetson-agx-thor-baseboard-1.1.2"));
const releases: Record<string, string> = useRevisions
  ? { "rev-1.1.2": join(revisions, "jetson-agx-thor-baseboard-1.1.2"), "rev-1.1.3": join(revisions, "jetson-agx-thor-baseboard-1.1.3") }
  : { "rev-a": join(samples, "boards", "jetson-agx-thor-baseboard"), "rev-b": join(samples, "boards", "jetson-agx-thor-baseboard") };
const shots = process.env.SCREENSHOT_DIR ?? "";

let browser: Browser;
let server: Server;
let lab: string;

async function shot(page: Page, name: string): Promise<void> {
  if (!shots) return;
  mkdirSync(shots, { recursive: true });
  await page.screenshot({ path: join(shots, name) });
}

beforeAll(async () => {
  lab = mkdtempSync(join(tmpdir(), "agni-lab-"));
  for (const [rel, from] of Object.entries(releases)) {
    const to = join(lab, "jetson", rel);
    mkdirSync(to, { recursive: true });
    for (const f of readdirSync(from)) {
      if (f === "design.yaml" || f === "README.md" || f === "LICENSE") continue;
      cpSync(join(from, f), join(to, f), { recursive: true });
    }
  }
  const store = mkdtempSync(join(tmpdir(), "agni-lab-store-"));
  const uploads = mkdtempSync(join(tmpdir(), "agni-lab-uploads-"));
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  server = await start({
    what: "agni serve (add designs)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--store", pathToFileURL(store).href, "--uploads", pathToFileURL(uploads).href, "--mount", `lab=${lab}`, "--web-dir", "web"],
    cwd: repoRoot,
    base,
    health: base,
  });
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await server?.stop();
});

describe("adding a mounted folder's boards as designs", () => {
  it("lists no design for the mount until one release is ticked and added", async () => {
    const [first, second] = Object.keys(releases);
    await withPage(browser, async (page) => {
      await page.goto(`${server.base}/designs/`);
      await expect.poll(() => page.getByTestId("library-status").textContent()).toBe("No designs yet.");

      await page.goto(`${server.base}/mounts/lab/jetson/`);
      const panel = page.getByTestId("folder-designs");
      const board = (rel: string) => panel.locator(`[data-proposal="jetson/${rel}"][data-entry*="/jetson-agx-thor-baseboard."]`);
      await board(second).waitFor({ timeout: 60_000 });
      expect(await board(first).count()).toBe(1);
      expect(await panel.locator(`[data-proposal="jetson/${first}"]`).count()).toBe(await panel.locator(`[data-proposal="jetson/${second}"]`).count());
      await shot(page, "folder-before.png");

      await board(second).locator("input").check();
      await panel.locator('input[name="series"]').fill("jetson");
      await page.getByTestId("add-designs").click();
      const made = panel.locator("[data-design]");
      await made.first().waitFor({ timeout: 120_000 });
      expect(await made.count()).toBe(1);
      const name = (await made.getAttribute("data-design")) ?? "";
      expect(await panel.locator(`[data-proposal="jetson/${second}"]`).count()).toBe(0);
      expect(await board(first).count()).toBe(1);
      await shot(page, "folder-after.png");

      await made.locator("a").click();
      await page.waitForURL(`${server.base}/${name}`);
      await page.locator("tr[data-artifact]").first().waitFor({ timeout: 60_000 });
      const types = await page.locator("tr[data-artifact] td:nth-child(2)").allTextContents();
      expect(types.sort()).toEqual(["board", "schematic"]);
      await shot(page, "design.png");
    });
  }, 600_000);
});
