// The designs page uploads designs to a server's store, and they come back after a restart (agni
// issues 1719, 1697). It drives the pages as a person does. Two Jetson baseboards go up as two designs
// of one series through the new-design form. The Jetson project's rules go up as a pack from its
// folder, and the newer design is validated against it, which keeps a report. The two designs go
// into a project with the pack as its default, and the newer one is diffed against the older. Its
// schematic is replaced on its page, which marks the report as run on earlier files. The server
// restarts over the same --store and --uploads, everything reads back, and the newer design opens
// in the viewer.
//
// It uploads the Jetson revisions 1.1.2 and 1.1.3 when `make samples-revisions` fetched them, and
// otherwise the oracle corpus's Jetson board twice, so the gate runs it without the revisions.
// SCREENSHOT_DIR keeps a screenshot of each page it reaches.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { cpSync, existsSync, mkdirSync, mkdtempSync, appendFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { launch, withPage } from "./browser.js";
import { hideDrawers, showPanel } from "./panels.js";
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
let uploads: string;

async function serve(): Promise<Server> {
  const base = `http://127.0.0.1:${port}`;
  return start({
    what: "agni serve (library)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--store", pathToFileURL(store).href, "--uploads", pathToFileURL(uploads).href, "--web-dir", "web"],
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
  await page.goto(`${server.base}/designs/`);
  const form = page.getByTestId("new-design");
  await form.locator('input[name="id"]').fill(id);
  await form.locator('input[name="series"]').fill("jetson");
  await page.getByTestId("new-files-folder").setInputFiles(folder);
  await form.locator("text=files picked").waitFor();
  await form.locator("button.lb-submit").click();
  await page.waitForURL(`${server.base}/designs/${id}`, { timeout: 120_000 });
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
  store = mkdtempSync(join(tmpdir(), "agni-library-store-"));
  uploads = mkdtempSync(join(tmpdir(), "agni-library-uploads-"));
  port = await freePort();
  server = await serve();
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await server?.stop();
});

describe("the designs page", () => {
  it("uploads two designs of a series, replaces a schematic, and keeps both across a restart", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${server.base}/`);
      await shot(page, "landing.png");
      await page.goto(`${server.base}/designs/`);
      await expect.poll(() => page.getByTestId("library-status").textContent()).toBe("No designs yet.");

      await upload(page, "bb-1.1.2", older);
      await upload(page, "bb-1.1.3", newer);
      const types = await page.locator("tr[data-artifact] td:nth-child(2)").allTextContents();
      expect(types.sort()).toEqual(["board", "schematic"]);
      await shot(page, "design.png");

      await page.goto(`${server.base}/designs/`);
      await page.locator('[data-design="designs/bb-1.1.3"]').waitFor();
      expect(await page.locator(".lb-series").getAttribute("data-series")).toBe("jetson");
      expect(await page.locator('[data-design="designs/bb-1.1.3"] .lb-latest').count()).toBe(1);
      expect(await page.locator('[data-design="designs/bb-1.1.2"] .lb-latest').count()).toBe(0);
      await shot(page, "library.png");

      // The Jetson project's rules go up as a pack from a copy of its folder holding a design too.
      // The server keeps the files project.yaml names and never asks for the design's.
      const project = mkdtempSync(join(tmpdir(), "agni-library-project-"));
      for (const f of ["project.yaml", "profiles", "params", "checklist"]) cpSync(join(repoRoot, "examples", "jetson-project", f), join(project, f), { recursive: true });
      mkdirSync(join(project, "boards", "big"), { recursive: true });
      cpSync(join(newer, "jetson-agx-thor-baseboard.kicad_pcb"), join(project, "boards", "big", "jetson-agx-thor-baseboard.kicad_pcb"));
      await page.goto(`${server.base}/packs/`);
      await page.getByTestId("pack-files-folder").setInputFiles(project);
      await page.getByTestId("pack-upload").locator("text=files picked").waitFor();
      await page.getByTestId("pack-upload").locator("button.lb-submit").click();
      await page.waitForURL(`${server.base}/packs/jetson`, { timeout: 60_000 });
      await expect.poll(() => page.locator("[data-testid=pack-tiers] tr").allTextContents()).toContain("Profilesprofiles");
      expect(await page.locator(".lb-files li").allTextContents()).not.toContain("boards/big/jetson-agx-thor-baseboard.kicad_pcb");
      // The power checklist is a folder of item files (agni issue 1812), and they go up with the pack.
      expect(await page.locator("[data-testid=pack-tiers] tr").allTextContents()).toContain("Checklistsreview, drc, power");
      expect(await page.locator(".lb-files li").allTextContents()).toEqual(expect.arrayContaining(["checklist/01-power/p1-bulk.md", "checklist/01-power/p2-decoupling.md"]));
      await shot(page, "pack.png");

      // Validate 1.1.3 against the pack, which keeps a report as a tab.
      await page.goto(`${server.base}/designs/bb-1.1.3`);
      await page.getByTestId("validate").locator("summary").click();
      await page.locator('input[name="pack"][value="packs/jetson"]').check();
      await page.getByTestId("validate").locator("button.lb-submit").click();
      await page.getByTestId("report").waitFor({ timeout: 120_000 });
      expect(await page.getByTestId("report-state").textContent()).toBe("current");
      const findings = Number(await page.getByTestId("report").getAttribute("data-total"));
      expect(findings).toBeGreaterThan(0);
      expect(await page.locator(".lb-rule").count()).toBeGreaterThan(0);
      await shot(page, "report.png");

      // A finding in the report opens the viewer on its own verdict, which the canvas lights.
      const finding = await page.locator(".lb-rule li a.lb-subject").first().getAttribute("href");
      expect(finding).toMatch(/^\/designs\/bb-1\.1\.3\/view\?pack=jetson&verdict=.+&rule=/);
      await page.goto(`${server.base}${finding}`);
      await page.waitForURL(/\/mounts\/uploads\/bb-1\.1\.3\/.*\/view\?.*verdict=/, { timeout: 120_000 });
      await page.waitForFunction(() => [...document.querySelectorAll("[data-highlight]")].some((e) => e.getAttribute("data-highlight") !== ""), null, { timeout: 120_000 });
      await shot(page, "finding.png");
      await page.goBack();
      await page.getByTestId("report").waitFor({ timeout: 60_000 });

      // The report opens the viewer on 1.1.3 read by name under the pack, and the viewer's own run
      // checks it under the pack's convention, which a read by file address lacks.
      await page.getByTestId("report-view").click();
      await page.waitForURL(/\/mounts\/uploads\/bb-1\.1\.3\/[0-9a-f]{12}\/schematic\/.*\/view\?.*design=designs%2Fbb-1\.1\.3.*pack=jetson/, { timeout: 120_000 });
      await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 120_000 });
      await showPanel(page, "checks");
      await page.click(".checks-run", { timeout: 120_000 });
      await page.locator("text=jetson/active-low-overbar").first().waitFor({ state: "attached", timeout: 120_000 });
      await hideDrawers(page);
      await shot(page, "viewer-under-pack.png");

      // A project holds both designs, with the pack as its default.
      await page.goto(`${server.base}/projects/`);
      await page.getByTestId("new-project").locator('input[name="id"]').fill("bench");
      await page.getByTestId("new-project").locator("button.lb-submit").click();
      await page.waitForURL(`${server.base}/projects/bench`);
      for (const d of ["designs/bb-1.1.2", "designs/bb-1.1.3"]) {
        await page.getByTestId("add-design").locator("select").selectOption(d);
        await page.getByTestId("add-design").locator("button").click();
        await page.locator(`.lb-row[data-design="${d}"]`).waitFor();
      }
      await page.getByTestId("add-pack").locator("select").selectOption("packs/jetson");
      await page.getByTestId("add-pack").locator("button").click();
      await page.locator('[data-testid=project-packs] li[data-pack="packs/jetson"]').waitFor();
      await shot(page, "project.png");

      // The newer design diffs against the older from its page.
      await page.goto(`${server.base}/designs/bb-1.1.3`);
      await page.getByTestId("diff-with").locator("select").selectOption("designs/bb-1.1.2");
      const diff = await page.getByTestId("diff-with").locator("a.lb-open").getAttribute("href");
      expect(diff).toMatch(/\/mounts\/uploads\/bb-1\.1\.3\/.*\/view\?against=uploads%2Fbb-1\.1\.2%2F/);

      // Replace 1.1.3's schematic with the files it holds, one sheet edited. A folder's stray sheets
      // stay out, as the server left them out of the artifact.
      const before = await artifactFiles("designs/bb-1.1.3/artifacts/schematic");
      const edited = mkdtempSync(join(tmpdir(), "agni-library-sch-"));
      for (const f of Object.keys(before)) cpSync(join(newer, f), join(edited, f));
      appendFileSync(join(edited, "power.kicad_sch"), "\n");
      await page.goto(`${server.base}/designs/bb-1.1.3`);
      await page.getByTestId("replace-schematic-folder").setInputFiles(edited);
      await expect.poll(async () => (await artifactFiles("designs/bb-1.1.3/artifacts/schematic"))["power.kicad_sch"], { timeout: 60_000 }).not.toBe(before["power.kicad_sch"]);
      const after = await artifactFiles("designs/bb-1.1.3/artifacts/schematic");
      expect(Object.keys(after).sort()).toEqual(Object.keys(before).sort());
      await page.reload();
      await expect.poll(() => page.getByTestId("report-state").textContent(), { timeout: 30_000 }).toBe("run on earlier design files");

      // The server restarts over the same store, and the page reads everything back.
      await server.stop();
      server = await serve();
      await page.goto(`${server.base}/designs/`);
      await page.locator('[data-design="designs/bb-1.1.3"]').waitFor({ timeout: 30_000 });
      expect(await page.locator(".lb-row").count()).toBe(2);
      expect(await page.locator('[data-design="designs/bb-1.1.3"] .lb-latest').count()).toBe(1);
      await page.goto(`${server.base}/projects/bench`);
      await page.locator('.lb-row[data-design="designs/bb-1.1.2"]').waitFor({ timeout: 30_000 });
      expect(await page.locator(".lb-row").count()).toBe(2);
      expect(await page.locator("[data-testid=project-packs] li").count()).toBe(1);
      await page.goto(`${server.base}/designs/bb-1.1.3`);
      await expect.poll(() => page.getByTestId("report-state").textContent(), { timeout: 30_000 }).toBe("run on earlier design files");
      expect(Number(await page.getByTestId("report").getAttribute("data-total"))).toBe(findings);

      // The newer design opens in the viewer through its view in the uploads mount.
      await page.goto(`${server.base}/designs/`);
      await page.locator('[data-design="designs/bb-1.1.3"] a.lb-open').click();
      await page.waitForURL(/\/mounts\/uploads\/bb-1\.1\.3\/[0-9a-f]{12}\/schematic\/.*\/view/);
      await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 120_000 });
      await shot(page, "viewer.png");
    });
  }, 600_000);
});
