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
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";
import { serveStatic, type StaticServer } from "./staticserver.js";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const prefix = "/agni/demo/";
let browser: Browser;
let site: StaticServer | undefined;
let base = "";
let out = "";
const prebuilt = process.env.AGNI_SITE_DIR ? resolve(process.env.AGNI_SITE_DIR) : "";

// seeded is every board the spec opens. The gate builds the first two; a prebuilt site that lists
// another mount must carry it too, so the published demo is checked seed by seed.
const seeded = [
  ["Sample Board", "gateway"],
  ["RoyalBlue54L Feather", "royalblue"],
  ["Jetson AGX Thor baseboard", "jetson"],
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
  site = await serveStatic(out, prefix);
  base = site.base;
  browser = await launch();
}, 300_000);

afterAll(async () => {
  await browser?.close();
  site?.close();
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

// serverOrFlagText lists every text node and hover text on the page that names a server or a
// command-line flag, hidden panels included, since a visitor to the static demo has neither (agni
// 894). Reading the DOM rather than innerText reaches the tabs that are not in front.
async function serverOrFlagText(page: Page): Promise<string[]> {
  await page.waitForSelector(".review .rv-empty, .review .rv-controls", { state: "attached", timeout: 60_000 });
  return page.evaluate(() => {
    const re = /\bserver\b|\bserve\b|--[a-z][a-z-]+/i;
    const out = new Set<string>();
    const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    while (walk.nextNode()) {
      const n = walk.currentNode;
      const t = (n.textContent ?? "").trim();
      if (t && re.test(t) && !n.parentElement?.closest("script, style")) out.add(t);
    }
    for (const el of document.querySelectorAll("[title], [placeholder], [aria-label]")) {
      for (const a of ["title", "placeholder", "aria-label"]) {
        const v = el.getAttribute(a);
        if (v && re.test(v)) out.add(`${a}: ${v}`);
      }
    }
    return [...out];
  });
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
        expect(await serverOrFlagText(page)).toEqual([]);
        expect(log.api).toEqual([]);
        expect(log.failed).toEqual([]);
        expect(log.errors).toEqual([]);
      });
    });
  }

  // A report saved from the viewer is written by the engine in the page, in the forms agni check writes,
  // and nothing leaves for a server to do it (agni issue 127).
  it("saves a check run as the report page and the results document", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      const rows = await checks(page);
      const saved = async (format: string) => {
        const [dl] = await Promise.all([page.waitForEvent("download"), page.click(`.checks-save[data-format="${format}"]`)]);
        return { name: dl.suggestedFilename(), text: readFileSync(await dl.path(), "utf8") };
      };
      const json = await saved("json");
      expect(json.name).toMatch(/\.agni-check\.json$/);
      const doc = JSON.parse(json.text);
      expect(doc.meta.producer).toBe("agni");
      expect(doc.meta.producerVersion ?? "").not.toBe("");
      expect(doc.design.contentHash ?? "").toMatch(/^sha256:/);
      expect(doc.findings.length).toBeGreaterThan(0);
      expect(doc.catalog.length).toBeGreaterThan(0);
      const html = await saved("html");
      expect(html.name).toMatch(/\.agni-check\.html$/);
      expect(html.text).toContain("<html");
      expect(rows).toBeGreaterThan(0);
      // Its rows link back into this viewer, under the demo's own prefix.
      expect(html.text).toMatch(new RegExp(`${prefix}designs/gateway/[^"]*/view\\?verdict=`));
      expect(log.api).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  });

  // The viewer runs the rules ListRules calls available, and ListRules called every datasheet-tier rule
  // unavailable without a model, so the gateway's project params never reached a check in the page
  // (agni issue 940). These two read the datasheet tier and fire on the gateway in the CLI.
  it("runs the gateway's datasheet-tier rules, which its project's params make available", async () => {
    await withPage(browser, async (page) => {
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      await checks(page);
      const table = (await page.locator(".checks-table").textContent()) ?? "";
      for (const rule of ["supply-exceeds-abs-max", "gateway-profiles/can-esd-missing"]) {
        expect(table).toContain(rule);
      }
    });
  });

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
