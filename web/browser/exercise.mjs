// The public demo walk, mission #851's exercise (agni issue 878). It drives the built static demo, or
// the deployed one, in Chromium the way a visitor would, and prints one row per step. `make
// exercise-public-demo` builds the site and runs it; DEMO_URL points it at a deployed demo instead.
//
//   node browser/exercise.mjs --site <built demo folder> [--prefix /agni/demo/] [--out report.md]
//   node browser/exercise.mjs --url https://panyam.github.io/agni/demo/ [--out report.md]
//
// It is a report, not a test. A step with nothing to run against yet prints "not available" with the
// ticket that adds it, and the run exits non-zero only when a step that exists fails. The browser
// suite's site.spec.ts and drop.spec.ts stay the gate's assertions. --only <mount,...> limits the
// seeds opened, and --control sends one of a dropped design's names from the page, which step 5 must
// then report as a failure (its positive control).
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";
import { classify, exitCode, leaks, markersFrom, table } from "./exercise-report.ts";
import { serveStatic } from "./staticserver.ts";
import { storedZip, walk } from "./storedzip.ts";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const args = process.argv.slice(2);
const flag = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const siteDir = flag("--site");
const url = flag("--url");
const out = flag("--out");
const only = flag("--only")?.split(",");
const control = args.includes("--control");
if (!siteDir === !url) {
  console.error("usage: node browser/exercise.mjs (--site <built demo folder> | --url <deployed demo>) [--prefix /agni/demo/] [--out report.md] [--only m1,m2] [--control]");
  process.exit(2);
}

// SYNTHETIC names the seeds that are this repo's own fixtures, so a real EDIF seed is told apart
// from the tutorial's 19-component one.
const SYNTHETIC = new Set(["gateway"]);
const gateway = join(repo, "examples", "tutorial-project", "designs", "gateway");
const companionPair = ["companion-demo.edn", "companion-demo.eds"].map((f) => join(repo, "cmd", "agni", "testdata", "review", f));

const rows = [];
const add = (step, design, status, detail, ms) => {
  rows.push({ step, design, status, detail, ms });
  console.error(`  ${status.padEnd(13)} ${step}  ${design}  ${ms === undefined ? "" : `${Math.round(ms)} ms  `}${detail}`);
};

let site;
let base = url;
if (siteDir) {
  site = await serveStatic(resolve(siteDir), flag("--prefix") ?? "/agni/demo/");
  base = site.base;
}
if (!base.endsWith("/")) base += "/";
const prefix = new URL(base).pathname;
const browser = await chromium.launch().catch((e) => {
  console.error(`could not launch Chromium: ${e.message}\nInstall one with:  cd web && pnpm exec playwright-core install chromium`);
  process.exit(2);
});

// record keeps every request a page makes, with its body, for the network split and step 5.
function record(page) {
  const log = { requests: [], failed: [], errors: [] };
  page.on("request", (r) => log.requests.push({ url: r.url(), method: r.method(), body: r.postData() ?? "" }));
  page.on("response", (r) => {
    if (r.status() >= 400) log.failed.push(`${r.status()} ${new URL(r.url()).pathname}`);
  });
  page.on("pageerror", (e) => log.errors.push(String(e)));
  return log;
}

function split(requests) {
  const n = { api: 0, file: 0, asset: 0 };
  for (const r of requests) n[classify(new URL(r.url).pathname, prefix)]++;
  return n;
}

const since = (t) => performance.now() - t;
const poll = async (fn, timeout = 180_000) => {
  const end = Date.now() + timeout;
  for (;;) {
    const v = await fn();
    if (v) return v;
    if (Date.now() > end) throw new Error("timed out");
    await new Promise((ok) => setTimeout(ok, 50));
  }
};

// step runs one step, turning a thrown error into a failed row so the walk goes on.
async function step(name, design, body) {
  const t = performance.now();
  try {
    const r = await body();
    add(name, design, r.status ?? "ok", r.detail ?? "", r.ms ?? since(t));
    return r;
  } catch (e) {
    add(name, design, "fail", String(e.message ?? e).split("\n")[0], since(t));
    return undefined;
  }
}

// opened waits for a design's first sheet and its rule catalog. A click on Run checks before the
// catalog arrives runs nothing (agni 868), so the walk waits for the button's rule count.
async function opened(page) {
  await poll(async () => (await page.locator("#readout").textContent()) !== "no sheet loaded" && (await page.locator("#readout").count()) > 0);
  const drawn = performance.now();
  await poll(async () => /\(\d+\)/.test((await page.locator(".checks-run").textContent()) ?? ""));
  return drawn;
}

// checks runs every rule and measures the longest gap between the page thread's frames while they
// run, which is how a visitor feels a blocked page. The time runs from the click until the button
// resets, so on a seed it includes reading the whole design, which drawing the first sheet does not.
async function checks(page, design) {
  return step("checks", design, async () => {
    await page.evaluate(() => {
      const g = (window.__gap = { max: 0, last: performance.now(), on: true });
      const tick = (t) => {
        g.max = Math.max(g.max, t - g.last);
        g.last = t;
        if (g.on) requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    const t = performance.now();
    await page.click(".checks-run");
    await poll(async () => (await page.locator(".checks-run").textContent())?.trim() === "Run checks");
    const ms = since(t);
    const gap = await page.evaluate(() => {
      window.__gap.on = false;
      return window.__gap.max;
    });
    const n = await page.locator(".check-locate").count();
    return { ms, detail: `${n} finding rows, longest frame gap ${Math.round(gap)} ms`, n };
  });
}

// walkDesign is step 4 on one open design: a finding into the drawing, a query, a trace and a saved
// report.
async function walkDesign(page, design) {
  await step("4. finding into the drawing", design, async () => {
    if ((await page.locator(".check-locate").count()) === 0) return { status: "not available", detail: "no findings to click" };
    await page.locator(".check-locate").first().click();
    const n = await poll(() => page.locator("#svg-view .highlight-overlay svg *").count(), 60_000);
    return { detail: `${n} highlight elements` };
  });
  const ref = await step("4. query", design, async () => {
    await page.fill("textarea.query-text", 'component.class(?r, "resistor") => ?r');
    await page.click("button.query-run");
    await page.waitForSelector(".query-row, .query-empty, .query-error", { timeout: 120_000 });
    if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").textContent());
    const n = await page.locator(".query-row").count();
    if (n === 0) return { detail: "no resistors" };
    // An entity cell carries its sheet badges beside the name, so the name is the locate button's.
    const cell = page.locator(".query-row td:nth-child(2)").first();
    const button = cell.locator(".query-locate");
    const first = ((await button.count()) ? await button.textContent() : await cell.textContent())?.trim() ?? "";
    return { detail: `${n} resistors, first ${first}`, first };
  });
  await step("4. trace", design, async () => {
    if (!ref?.first) return { status: "not available", detail: "no resistor to trace across" };
    await page.locator(".dv-default-tab-content", { hasText: /^Trace$/ }).first().click();
    const pins = page.locator(".trace-pin");
    await pins.nth(0).fill(`${ref.first}.1`);
    await pins.nth(1).fill(`${ref.first}.2`);
    await page.click(".trace-run");
    const kind = await poll(async () => {
      for (const k of ["trace-route", "trace-noroute", "trace-unresolved", "trace-error"]) if ((await page.locator(`.${k}`).count()) > 0) return k;
      return "";
    }, 120_000);
    const head = (await page.locator(".trace-headline, .trace-error").first().textContent())?.trim() ?? "";
    return { status: kind === "trace-route" || kind === "trace-noroute" ? "ok" : "fail", detail: `${ref.first}.1 to ${ref.first}.2: ${head}` };
  });
  add("4. save the report", design, "not available", "#127, no report export in the viewer");
}

// network is step 5 for one page: the split of what it fetched, failing on any analysis call, and,
// for a dropped design, on any request carrying one of its names.
function network(log, design, markers) {
  const n = split(log.requests);
  const leaked = markers ? leaks(log.requests, markers) : [];
  const bad = [];
  if (n.api) bad.push(`${n.api} analysis calls, and a static site has nothing to answer them`);
  if (leaked.length) bad.push(...leaked);
  if (log.errors.length) bad.push(`page errors: ${log.errors.join("; ")}`);
  const detail = `${n.asset} assets, ${n.file} design files, ${n.api} API calls${markers ? `, ${markers.length} of the design's names watched` : ""}`;
  add("5. network", design, bad.length ? "fail" : "ok", bad.length ? `${detail}; ${bad.join("; ")}` : detail);
}

// 1. The landing page and each seed.
let seeds = [];
await step("1. landing page", "", async () => {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  const log = record(page);
  await page.goto(base, { waitUntil: "networkidle" });
  seeds = await page.$$eval(".st-row", (rs) =>
    rs.map((r) => ({ title: r.querySelector(".st-link")?.textContent?.trim() ?? "", href: r.querySelector(".st-link")?.getAttribute("href") ?? "", meta: r.querySelector(".st-meta")?.textContent ?? "" })),
  );
  await page.close();
  if (log.failed.length) throw new Error(`failed loads: ${log.failed.join(", ")}`);
  return { detail: `${seeds.length} seeds: ${seeds.map((s) => s.title).join(", ")}` };
});
const mountOf = (href) => href.match(/designs\/([^/]+)\//)?.[1] ?? href;
if (!seeds.some((s) => /EDIF/i.test(s.meta) && !SYNTHETIC.has(mountOf(s.href)))) add("1. real EDIF seed", "", "not available", "#928, the only EDIF seed is the synthetic tutorial board");
add("1. seed a server answers", "", "not available", "#855, the demo has no server");

for (const seed of seeds) {
  const mount = mountOf(seed.href);
  if (only && !only.includes(mount)) continue;
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  const log = record(page);
  const t = performance.now();
  const ok = await step("1. open", seed.title, async () => {
    await page.goto(new URL(seed.href, base).href, { waitUntil: "domcontentloaded" });
    const drawn = await opened(page);
    return { ms: drawn - t, detail: `first sheet drawn; rule catalog after ${Math.round(since(t))} ms; ${seed.meta.trim()}` };
  });
  if (ok && (await checks(page, seed.title))) {
    await walkDesign(page, seed.title);
    await step("1. reopen after a reload", seed.title, async () => {
      const t2 = performance.now();
      await page.reload({ waitUntil: "domcontentloaded" });
      const drawn = await opened(page);
      return { ms: drawn - t2, detail: "first sheet drawn after a reload" };
    });
  }
  network(log, seed.title, undefined);
  await page.close();
}

// drop opens the page's drop target, hands it files through one of its inputs, and opens the first
// design it proposes.
async function drop(design, input, files, markers) {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  const log = record(page);
  await page.goto(new URL("designs/local/view/", base).href, { waitUntil: "domcontentloaded" });
  const button = input === "#drop-folder-input" ? "#drop-folder" : "#drop-open";
  const ready = await page
    .waitForSelector(`${button}:not([hidden])`, { timeout: 60_000 })
    .then(() => true)
    .catch(() => false);
  if (!ready) {
    add("2. drop", design, "not available", input === "#drop-folder-input" ? "this demo predates Open folder (#878)" : "the page shows no Open files");
    await page.close();
    return;
  }
  const t = performance.now();
  const proposed = await step("2. drop and propose", design, async () => {
    await page.setInputFiles(input, files);
    await poll(() => page.locator("#drop-dialog").evaluate((d) => d.open), 120_000);
    const names = await page.locator(".drop-design-name").allTextContents();
    return { detail: `proposed ${names.map((n) => n.trim()).join("; ")}` };
  });
  if (proposed) {
    await page.locator(".drop-design-open").first().click();
    const ok = await step("2. open", design, async () => {
      await opened(page);
      return { ms: since(t), detail: `drop to first sheet drawn; ${new URL(page.url()).pathname}` };
    });
    if (ok && (await checks(page, design))) await walkDesign(page, design);
  }
  if (control) await page.evaluate((m) => fetch(`${location.origin}/collect?n=${encodeURIComponent(m)}`).catch(() => undefined), markers[0]);
  network(log, design, markers);
  await page.close();
}

const text = (paths) => paths.filter((p) => /\.(edn|eds|kicad_sch|kicad_pcb)$/.test(p)).map((p) => readFileSync(p, "latin1")).join("\n");
const gatewayMarkers = markersFrom(text(walk(gateway)));

// 2. The tutorial board as a folder, then as a zip.
await drop("gateway, as a folder", "#drop-folder-input", gateway, gatewayMarkers);
await drop("gateway, as a zip", "#drop-input", { name: "gateway.zip", mimeType: "application/zip", buffer: storedZip(gateway) }, gatewayMarkers);

// 3. An EDIF netlist with its schematic export.
add("3. real EDIF pair", "", "not available", "#928, the samples corpus holds no .edn with its .eds");
await drop("companion-demo .edn + .eds (synthetic)", "#drop-input", companionPair, markersFrom(text(companionPair)));

await browser.close();
site?.close();

let commit = "";
try {
  commit = execFileSync("git", ["-C", repo, "describe", "--always", "--dirty"], { encoding: "utf8" }).trim();
} catch {
  // Outside a checkout the report names no commit.
}
const report = `Public demo walk @ ${commit || "unknown"} against ${url ?? `the demo built at ${siteDir}`}, ${new Date().toISOString()}\n\n${table(rows)}\n`;
console.log(report);
if (out) {
  mkdirSync(dirname(resolve(out)), { recursive: true });
  writeFileSync(out, report);
  console.error(`wrote ${resolve(out)}`);
}
process.exit(exitCode(rows));
