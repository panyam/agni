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
import { agniBin } from "./agnibin.js";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch, withPage } from "./browser.js";
import { serveStatic, type StaticServer } from "./staticserver.js";
import { expectCompareTables, gatewayDiffRows } from "./comparetables.js";

// The design's summary line, which Properties shows at the design level (agni issue 902). It names
// the read, so "faithful" in it says the schematic was drawn rather than an auto-layout.
const DESIGN_SUMMARY = '[data-component="properties"] dd[data-row="read"]';

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
      agniBin(),
      [
        "site", out, "--base", prefix,
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
        expect(await page.locator(DESIGN_SUMMARY).textContent()).toContain("faithful");
        expect(await serverOrFlagText(page)).toEqual([]);
        expect(log.api).toEqual([]);
        expect(log.failed).toEqual([]);
        expect(log.errors).toEqual([]);
      });
    });
  }

  // A diff link (`?against=`, agni issue 1126) opens the same comparison on a static host, where the
  // page's own engine holds both revisions' files and no server lists anything.
  it("opens a comparison from a link naming the revision to compare against", async () => {
    const want = gatewayDiffRows();
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      await page.waitForSelector(DESIGN_SUMMARY, { timeout: 60_000 });
      const view = new URL(page.url());
      view.searchParams.set("against", "gateway/designs/gateway/gateway-rev-c.edn");
      await page.goto(view.toString(), { waitUntil: "domcontentloaded" });
      await expect
        .poll(() => page.locator(".diff-file").allTextContents(), { timeout: 60_000 })
        .toEqual(["A gateway:designs/gateway", "B gateway:designs/gateway/gateway-rev-c.edn"]);
      expect(await page.locator(".diff-error").count()).toBe(0);
      await expectCompareTables(page, want);
      expect(log.api).toEqual([]);
      expect(log.failed).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  }, 180_000);

  // The Compare picker lists the site's designs from the files `agni site` wrote, since a static host
  // has no server to list a mount and the page's engine holds only the files it was given (agni
  // issue 978).
  it("compares the Sample Board against another revision picked from the Compare picker", async () => {
    const want = gatewayDiffRows();
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      await page.waitForSelector(DESIGN_SUMMARY, { timeout: 60_000 });
      await page.click("#compare button");
      const tree = page.locator("#compare-tree");
      // The tree opens down to the folder holding the open design, which holds its other revisions.
      await tree.locator("li li li > button.node.dir", { hasText: "gateway" }).click({ timeout: 60_000 });
      await tree.getByText("gateway-rev-c.edn").click();
      await expect
        .poll(() => page.locator(".diff-file").allTextContents(), { timeout: 60_000 })
        .toEqual(["A gateway:designs/gateway", "B gateway:designs/gateway/gateway-rev-c.edn"]);
      // The pair was compared, so the legend counts its changes, and nothing failed.
      await page.waitForSelector(".diff-legend .diff-chip", { timeout: 60_000 });
      expect(await page.locator(".diff-error").count()).toBe(0);
      await expectCompareTables(page, want);
      expect(log.api).toEqual([]);
      expect(log.failed).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  });

  // A report saved from the viewer is written by the engine in the page, in the forms agni check writes,
  // and nothing leaves for a server to do it (agni issue 127).
  // The Lexicon panel answers from the engine in the page, so the demo has it (agni issue 1267).
  it("tries a vocabulary edit with nothing to call but files", async () => {
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      await page.locator(".dv-default-tab-content", { hasText: /^Lexicon$/ }).first().click({ timeout: 120_000 });
      await page.locator('.lex-group[data-vocabulary="net.control"] summary').click({ timeout: 120_000 });
      await page.locator('.lex-group[data-vocabulary="net.control"] .lex-off-switch input').check();
      await page.locator('.lex-group[data-vocabulary="net.control"] .lex-row-replaced').first().waitFor({ state: "attached" });
      await page.click(".lex-try");
      await page.waitForSelector('.lex-result[data-table="finding_changes"]', { timeout: 120_000 });
      expect(await page.locator('.lex-result[data-table="role_changes"] .lex-change').count()).toBeGreaterThan(0);
      expect(log.api).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  }, 300_000);

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

  // A run saved as csv is Tabulate's tables encoded by the engine in the page, so the findings file
  // holds the rows `agni check --format csv` writes for the same design (agni issue 956). The viewer
  // assembles its run rule by rule in the panel's order, so the rows are compared as a set.
  it("saves a check run as csv, the rows agni check --format csv writes", async () => {
    const home = mkdtempSync(join(tmpdir(), "agni-home-"));
    const want = execFileSync(agniBin(), ["check", "examples/tutorial-project/designs/gateway", "--format", "csv"], {
      cwd: repo,
      env: { ...process.env, HOME: home, XDG_CONFIG_HOME: home },
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    })
      .trimEnd()
      .split("\n");
    expect(want.length).toBeGreaterThan(2);
    await withPage(browser, async (page) => {
      const log = record(page);
      await page.goto(base, { waitUntil: "domcontentloaded" });
      await page.click("text=Sample Board");
      await checks(page);
      const files = new Map<string, string>();
      const two = new Promise<void>((done) => {
        page.on("download", async (dl) => {
          files.set(dl.suggestedFilename(), readFileSync(await dl.path(), "utf8"));
          if (files.size === 2) done();
        });
      });
      await page.click('.checks-save[data-format="csv"]');
      await two;
      expect([...files.keys()].sort()).toEqual(["gateway.findings.csv", "gateway.verdicts.csv"]);
      const got = files.get("gateway.findings.csv")!.trimEnd().split("\n");
      expect(got[0]).toBe(want[0]);
      expect(got.slice(1).sort()).toEqual(want.slice(1).sort());
      expect(files.get("gateway.verdicts.csv")!.split("\n")[0]).toBe("verdict_id,url,rule,outcome,subjects,statement,context,terms,reason");
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
      await expect.poll(() => page.locator(DESIGN_SUMMARY).textContent(), { timeout: 60_000 }).toContain("faithful");
      // A static host serves only the pages `agni site` wrote, so a dropped design's path rides in the
      // query of the browser mount's one viewer page (agni issue 1072).
      const at = new URL(page.url());
      expect(at.pathname).toBe(`${prefix}designs/local/view/`);
      expect(at.searchParams.get("d") ?? "").toMatch(/^drop-[a-z0-9]+\/companion-demo\.edn$/);
      expect(log.failed).toEqual([]);
      expect(log.errors).toEqual([]);
    });
  });
});
