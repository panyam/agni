// Dropping files on the viewer (agni issue 854). Files go into the browser mount through the page's
// "Open files" input, which is the same path a drag takes, the engine proposes the designs they make,
// and opening one reads it in the page. The server under test mounts the tutorial project as `tut`,
// so the dropped copy can be compared with the served one.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { crc32 } from "node:zlib";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");
const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const tutorial = join(repo, "examples", "tutorial-project");
const gateway = join(tutorial, "designs", "gateway");
const apiPrefix = "/agni.v1.webapi.";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((n) => {
    const p = join(dir, n);
    return statSync(p).isDirectory() ? walk(p) : [p];
  });
}

// storedZip writes a zip with no compression, enough for a browser to read and small enough to keep
// here rather than add a dependency.
function storedZip(root: string): Buffer {
  const locals: Buffer[] = [];
  const centrals: Buffer[] = [];
  let offset = 0;
  for (const p of walk(root)) {
    const name = Buffer.from(relative(root, p).split(sep).join("/"));
    const data = readFileSync(p);
    const crc = crc32(data);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(data.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(name.length, 26);
    locals.push(local, name, data);
    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(data.length, 20);
    central.writeUInt32LE(data.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt32LE(offset, 42);
    centrals.push(central, name);
    offset += 30 + name.length + data.length;
  }
  const cd = Buffer.concat(centrals);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(centrals.length / 2, 8);
  end.writeUInt16LE(centrals.length / 2, 10);
  end.writeUInt32LE(cd.length, 12);
  end.writeUInt32LE(offset, 16);
  return Buffer.concat([...locals, cd, end]);
}

function record(page: Page): { network: string[]; errors: string[] } {
  const log = { network: [] as string[], errors: [] as string[] };
  page.on("request", (r) => log.network.push(new URL(r.url()).pathname));
  page.on("pageerror", (e) => log.errors.push(String(e)));
  return log;
}

async function runChecks(page: Page): Promise<string[]> {
  await expect.poll(() => page.locator(".checks-run").textContent(), { timeout: 60_000 }).toMatch(/\(\d+\)/);
  await page.click(".checks-run");
  await page.waitForSelector(".check-locate", { timeout: 60_000 });
  return page.locator(".check-locate").allTextContents();
}

describe("dropping files on the viewer", () => {
  it("opens a dropped zip's declared design and finds what the server finds in the same files", async () => {
    let served: string[] = [];
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/view`, { waitUntil: "domcontentloaded" });
      served = await runChecks(page);
    });
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(`${base()}/designs/local/view?engine=wasm`, { waitUntil: "domcontentloaded" });
      await page.waitForSelector("#drop-open:not([hidden])", { timeout: 60_000 });
      await page.setInputFiles("#drop-input", { name: "tutorial.zip", mimeType: "application/zip", buffer: storedZip(tutorial) });

      const dialog = page.locator("#drop-dialog");
      await expect.poll(() => dialog.evaluate((d) => (d as HTMLDialogElement).open), { timeout: 60_000 }).toBe(true);
      expect(await dialog.textContent()).toContain("declared by its design.yaml");
      await dialog.locator(".drop-design-open").first().click();

      const dropped = await runChecks(page);
      expect(served.length, "the served run found nothing, so agreeing with it proves nothing").toBeGreaterThan(0);
      expect([...dropped].sort()).toEqual([...served].sort());
      expect(page.url()).toMatch(/\/designs\/local\/drop-[a-z0-9]+\/tutorial\.zip\/designs\/gateway/);

      // The design never touched the server: no analysis call, no listing, no file read.
      expect(log.network.filter((p) => p.startsWith(apiPrefix) && !p.endsWith("/ListMounts"))).toEqual([]);
      expect(log.network.some((p) => p.startsWith("/raw/"))).toBe(false);
      expect(log.errors).toEqual([]);
    });
  });

  it("proposes a design.yaml for loose files and opens the design it declares", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(`${base()}/designs/local/view?engine=wasm`, { waitUntil: "domcontentloaded" });
      await page.waitForSelector("#drop-open:not([hidden])", { timeout: 60_000 });
      const loose = walk(gateway).filter((p) => !p.endsWith("design.yaml") && !p.includes(`${sep}symbols${sep}`));
      await page.setInputFiles("#drop-input", loose);

      const dialog = page.locator("#drop-dialog");
      await expect.poll(() => dialog.evaluate((d) => (d as HTMLDialogElement).open), { timeout: 60_000 }).toBe(true);
      const yaml = await dialog.locator(".drop-design-yaml").first().inputValue();
      expect(yaml).toContain("entry: gateway.edn");
      expect(yaml).toContain("  - gateway.kicad_sch");
      expect(yaml).toContain("revisions:");
      await dialog.locator(".drop-design-open").first().click();

      expect((await runChecks(page)).length).toBeGreaterThan(0);
      expect(page.url()).toMatch(/\/designs\/local\/drop-[a-z0-9]+\/gateway\.edn\/view/);
      // The schematic is drawn because the written design.yaml names gateway.kicad_sch as a companion.
      // A netlist read without it would draw an auto-layout.
      await expect.poll(() => page.locator("#design-summary").textContent(), { timeout: 30_000 }).toContain("faithful");
      expect(log.network.filter((p) => p.startsWith(apiPrefix) && !p.endsWith("/ListMounts"))).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  });

  it("refuses a dropped design on the server engine instead of sending it", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(`${base()}/designs/local/drop-x/board.edn/view`, { waitUntil: "domcontentloaded" });
      const note = page.locator("#engine-note");
      await expect.poll(() => note.isVisible(), { timeout: 60_000 }).toBe(true);
      expect(await note.textContent()).toContain("needs the browser engine");
      await page.waitForTimeout(1000);
      const naming = log.network.filter((p) => p.startsWith(apiPrefix) && !p.startsWith(`${apiPrefix}WorkspaceService/`) && !p.endsWith("/ListRules") && !p.endsWith("/ListRelations"));
      expect(naming).toEqual([]);
    });
  });
});
