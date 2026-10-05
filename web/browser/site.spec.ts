// The static demo (agni issue 856). `agni site` writes the demo as plain files, and this serves them
// from a plain file server under /agni/demo/, the prefix the docs' Pages site gives it, with no agni
// server anywhere. Every asset has to load under the prefix, because a page served at the wrong root
// still returns 200 and renders with no CSS, and every answer has to come from the engine in the
// page, since there is nothing else to answer.
//
// With AGNI_SITE_DIR set, the spec serves that already-built demo instead of building its own. The
// docs workflow sets it, so the files checked are the files Pages publishes, and a seed the site
// carries beyond the gate's two is opened as well.

import { beforeAll, afterAll, describe, expect, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { createReadStream, existsSync, mkdtempSync, readdirSync, rmSync, statSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { tmpdir } from "node:os";
import { extname, join, normalize, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const prefix = "/agni/demo/";
let browser: Browser;
let server: Server;
let base = "";
let out = "";
const prebuilt = process.env.AGNI_SITE_DIR ? resolve(process.env.AGNI_SITE_DIR) : "";

const types: Record<string, string> = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".wasm": "application/wasm",
};

// seeded is every board the spec opens. The gate builds the first two; a prebuilt site that lists
// another mount must carry it too, so the published demo is checked seed by seed.
const seeded = [
  ["Sample Board", "gateway"],
  ["RoyalBlue54L-Feather", "royalblue"],
  ["jetson-agx-thor-baseboard", "jetson"],
] as const;

beforeAll(async () => {
  if (prebuilt) {
    if (!existsSync(join(prebuilt, "index.html"))) throw new Error(`AGNI_SITE_DIR=${prebuilt} holds no index.html`);
    out = prebuilt;
  } else {
    out = mkdtempSync(join(tmpdir(), "agni-site-"));
    const home = mkdtempSync(join(tmpdir(), "agni-home-"));
    execFileSync(
      "go",
      [
        "run", "./cmd/agni", "site", out, "--base", prefix,
        "--seed", "gateway=examples/tutorial-project",
        "--seed", "royalblue=tools/samples/boards/royalblue54L-feather",
      ],
      { cwd: repo, env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home }, stdio: "pipe" },
    );
  }
  // A plain file server: a path under the prefix maps to the built folder, a folder serves its
  // index.html, and anything else is a 404, as a static host answers.
  server = createServer((req, res) => {
    const path = decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname);
    if (!path.startsWith(prefix)) {
      res.writeHead(404).end();
      return;
    }
    let file = normalize(join(out, path.slice(prefix.length)));
    if (!file.startsWith(out)) {
      res.writeHead(404).end();
      return;
    }
    if (existsSync(file) && statSync(file).isDirectory()) file = join(file, "index.html");
    if (!existsSync(file)) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
    createReadStream(file).pipe(res);
  });
  await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
  const addr = server.address();
  base = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}${prefix}`;
  browser = await launch();
}, 300_000);

afterAll(async () => {
  await browser?.close();
  server?.close();
  if (out && !prebuilt) rmSync(out, { recursive: true, force: true });
});

function record(page: Page) {
  const log = { failed: [] as string[], api: [] as string[], errors: [] as string[] };
  page.on("response", (r) => {
    if (r.status() >= 400) log.failed.push(`${r.status()} ${new URL(r.url()).pathname}`);
  });
  page.on("request", (r) => {
    if (new URL(r.url()).pathname.includes("agni.v1.webapi.")) log.api.push(new URL(r.url()).pathname);
  });
  page.on("pageerror", (e) => log.errors.push(String(e)));
  return log;
}

async function checks(page: Page): Promise<number> {
  await page.click(".checks-run", { timeout: 60_000 });
  await page.waitForSelector(".check-locate", { timeout: 60_000 });
  return page.locator(".check-locate").count();
}

describe("the static demo under /agni/demo/", () => {
  it("lists the seeded boards with every asset loading under the prefix", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "networkidle" });
      expect(await page.locator(".st-link").count()).toBeGreaterThanOrEqual(2);
      // The page's own styles applied, which a page served at the wrong root loses while still 200.
      expect(await page.evaluate(() => getComputedStyle(document.querySelector(".st-app")!).maxWidth)).toBe("860px");
      expect(await page.textContent(".st-private")).toContain("never uploaded");
      expect(log.failed).toEqual([]);
    });
  });

  it("carries a listing for each seed it opens, and opens each seed it carries", () => {
    const listed = readdirSync(join(out, "files")).map((f) => f.replace(/\.json$/, "")).sort();
    const opened = seeded.map(([, mount]) => mount).filter((m) => listed.includes(m)).sort();
    expect(opened).toEqual(listed);
    expect(listed).toEqual(expect.arrayContaining(["gateway", "royalblue"]));
  });

  for (const [title, mount] of seeded) {
    it(`opens and checks ${title} in the page, with nothing to call but files`, async (ctx) => {
      if (!existsSync(join(out, "files", `${mount}.json`))) ctx.skip();
      await withPage(browser, async (page) => {
        const log = record(page);
        await page.goto(base, { waitUntil: "domcontentloaded" });
        await page.click(`text=${title}`);
        expect(await checks(page)).toBeGreaterThan(0);
        expect(new URL(page.url()).pathname).toContain(`${prefix}designs/${mount}/`);
        expect(await page.locator("#design-summary").textContent()).toContain("faithful");
        expect(log.api).toEqual([]);
        expect(log.failed).toEqual([]);
        expect(log.errors).toEqual([]);
      });
    });
  }

  it("opens a dropped design from the landing page's Open files", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Open files…");
      await page.waitForSelector("#drop-open:not([hidden])", { timeout: 60_000 });
      const dir = join(repo, "cmd", "agni", "testdata", "review");
      await page.setInputFiles("#drop-input", [join(dir, "companion-demo.edn"), join(dir, "companion-demo.eds")]);
      await expect.poll(() => page.locator("#drop-dialog").evaluate((d) => (d as HTMLDialogElement).open), { timeout: 60_000 }).toBe(true);
      await page.locator(".drop-design-open").first().click();
      await expect.poll(() => page.locator("#design-summary").textContent(), { timeout: 60_000 }).toContain("faithful");
      expect(new URL(page.url()).pathname).toMatch(new RegExp(`^${prefix}designs/local/drop-[a-z0-9]+/companion-demo\\.edn/view`));
      expect(log.failed).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  });
});
