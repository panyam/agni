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
// seeds opened, --timing breaks each design's Run checks down by stage and rule, and --control sends
// one of a dropped design's names from the page, which step 5 must then report as a failure (its
// positive control).
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";
import { breakdownDetail, checksBreakdown, classify, exitCode, leaks, loadDetail, loadSplit, markersFrom, pendingFiles, slowestFetch, sumSplits, table } from "./exercise-report.ts";
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
// --only names the seeds to walk by mount, and "drops" for the dropped designs, so `--only jetson`
// walks the Jetson seed alone.
const only = flag("--only")?.split(",");
const control = args.includes("--control");
// --no-background-load walks a demo built before agni issue 1061, whose viewer loads nothing ahead
// of Run checks and so never logs the line backgroundLoad waits for.
const backgroundLoads = !args.includes("--no-background-load");
// --timing opens every design with ?timing=1, so the page asks each request where its time went
// (agni issue 914), and adds a row breaking Run checks down for each design.
// --timing-memory does the same with ?timing=memory, so each stage also carries the worker's memory
// as it ended (agni issue 994), and the checks load row says what the jobs worker held after the run.
const timedMemory = args.includes("--timing-memory");
const timed = timedMemory || args.includes("--timing");
const timedURL = (href) => {
  const u = new URL(href, base);
  if (timed) u.searchParams.set("timing", timedMemory ? "memory" : "1");
  return u.href;
};
if (!siteDir === !url) {
  console.error("usage: node browser/exercise.mjs (--site <built demo folder> | --url <deployed demo>) [--prefix /agni/demo/] [--out report.md] [--only m1,m2,drops] [--control]");
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
  const log = { requests: [], failed: [], errors: [], timings: [], fetches: [], loads: [] };
  page.on("console", (m) => {
    const t = m.text();
    if (t.startsWith("agni timing ")) log.timings.push(t);
    if (t.startsWith("agni fetch ")) log.fetches.push(t);
    if (t.startsWith("agni loaded ") || t.startsWith("agni load skipped ")) log.loads.push(t);
  });
  const seen = new Map();
  page.on("request", (r) => {
    const s = { url: r.url(), method: r.method(), body: r.postData() ?? "", startMs: performance.now() };
    seen.set(r, s);
    log.requests.push(s);
  });
  const ended = (r) => {
    const s = seen.get(r);
    if (s) s.endMs = performance.now();
  };
  page.on("requestfinished", ended);
  page.on("requestfailed", ended);
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
// catalog arrives runs nothing (agni 868), so the walk waits for the button's rule count. A sheet
// counts as drawn when the readout reports what it drew, SVG elements or WebGL primitives. Any other
// change is a status line (a browser without WebGL2 says so the moment the canvas mounts), and
// timing that hid a 38-second blank page on the Jetson seed (agni issue 943).
async function opened(page) {
  await poll(async () => /^(SVG — \d+ elements|sheet \S+ — \d+ primitives)/.test((await page.locator("#readout").textContent()) ?? ""));
  const drawn = performance.now();
  await poll(async () => /\(\d+\)/.test((await page.locator(".checks-run").textContent()) ?? ""));
  return drawn;
}

// checks runs every rule and measures the longest gap between the page thread's frames while they
// run, which is how a visitor feels a blocked page. The time runs from the click until the button
// resets, so on a seed it includes reading the whole design, which drawing the first sheet does not.
async function checks(page, design, name = "checks", log = undefined, expectRun = false) {
  return step(name, design, async () => {
    await page.evaluate(() => {
      const g = (window.__gap = { max: 0, last: performance.now(), on: true, t0: performance.now() });
      const tick = (t) => {
        g.max = Math.max(g.max, t - g.last);
        g.last = t;
        if (g.on) requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    const t = performance.now();
    const label = async () => (await page.locator(".checks-run").textContent())?.trim();
    const runs = () => (log ? log.timings.filter((l) => l.startsWith("agni timing CheckDesign: ")).length : 0);
    const before = runs();
    await page.click(".checks-run");
    if (expectRun && log && timed) {
      // A page still finishing its open (a reload restoring its panels) can ignore the click and start
      // no run, so click again, at most once a second, until this run's CheckDesign has answered.
      let clicked = performance.now();
      await poll(async () => {
        if (runs() > before) return true;
        if (performance.now() - clicked > 1000 && (await label()) === "Run checks") {
          await page.click(".checks-run");
          clicked = performance.now();
        }
        return false;
      }, 180_000);
    }
    await poll(async () => (await label()) === "Run checks");
    const ms = since(t);
    // t0 and t1 are on the page's clock, which the timing lines' starts are on too.
    const { gap, t0, t1 } = await page.evaluate(() => {
      window.__gap.on = false;
      return { gap: window.__gap.max, t0: window.__gap.t0, t1: performance.now() };
    });
    const n = await page.locator(".check-locate").count();
    let asked = "";
    if (log && timed) {
      const b = checksBreakdown(log.timings, t0, t1);
      asked = `; ${b.requests.length} requests: ${b.requests.map((r) => `${r.rpc} ${Math.round(r.wallMs)} ms`).join(", ") || "none"}`;
    }
    return { ms, detail: `${n} finding rows, longest frame gap ${Math.round(gap)} ms${asked}`, n, t0, t1 };
  });
}

// checksTiming adds three rows for the design's last Run checks (agni issues 968, 1060). "checks
// breakdown" accounts for the whole click-to-reset time: every request the run made, when it
// started, how long the page waited and how much of that the engine spent, and the time the page
// spent outside every request. "checks load" is its CheckDesign's one-time part, the wait outside
// the engine (the design's files reaching the worker) and the stages that read, restore and build,
// with which cache tier answered each read. "checks rules" is the evaluation alone, what a change to
// a rule or to jaala moves. Only with --timing.
function checksTiming(log, design, run, suffix = "") {
  if (!timed) return;
  const b = checksBreakdown(log.timings, run.t0, run.t1);
  add(`checks breakdown${suffix}`, design, b.requests.length ? "ok" : "fail", b.requests.length ? breakdownDetail(b) : "the page logged no timed request during Run checks", b.totalMs);
  const line = log.timings.filter((t) => t.startsWith("agni timing CheckDesign: ")).pop();
  const s = line && loadSplit(line);
  if (!s) {
    add(`checks load${suffix}`, design, "fail", "the page logged no CheckDesign timing, so ?timing did not reach the engine");
    return;
  }
  const ms = (us) => `${Math.round(Number(us ?? 0) / 1000)} ms`;
  const t = JSON.parse(line.slice("agni timing CheckDesign: ".length));
  const peak = Math.max(0, ...(t.stages ?? []).map((st) => Number(st.sysBytes ?? 0)));
  const held = peak ? `; the checking worker held ${Math.round(peak / (1 << 20))} MB` : "";
  add(`checks load${suffix}`, design, "ok", loadDetail(s) + held, (s.outsideMs ?? 0) + s.loadMs);
  const rules = (t.rules ?? []).slice(0, 3);
  add(`checks rules${suffix}`, design, "ok", `${t.rulesEvaluated ?? "?"} rules; slowest ${rules.map((r) => `${r.rule} ${ms(r.micros)}`).join(", ") || "none"}`, s.rulesMs);
}

// openLoad is the "1. load" row for a seed: the design files the open fetched and, with --timing,
// what every request before Run checks spent loading the design rather than answering (agni issue
// 1060). It is paid once per visit, where every later question runs on what it loaded. Its time is
// the engine's loading alone, since "1. open" already gives the wall time.
function openLoad(log, design, beforeMs) {
  // Read now, before Run checks fetches the check worker's copy of the files.
  const files = log.fetches.map((l) => /^agni fetch .+?: (\{.*\})$/.exec(l)).filter(Boolean).map((m) => JSON.parse(m[1]));
  const mb = (files.reduce((n, f) => n + Number(f.bytes ?? 0), 0) / (1 << 20)).toFixed(1);
  const fetched = `${files.length} design files fetched, ${mb} MB`;
  if (!timed) {
    add("1. load", design, "ok", fetched);
    return;
  }
  const splits = log.timings.map(loadSplit).filter((s) => s && s.startMs !== undefined && s.startMs < beforeMs);
  if (!splits.length) {
    add("1. load", design, "fail", `${fetched}; the page logged no timed request before Run checks`);
    return;
  }
  const s = sumSplits(splits);
  add("1. load", design, "ok", `${fetched}; ${splits.length} requests: ${loadDetail(s)}`, s.loadMs);
}

// backgroundLoad waits for the load the viewer starts once the first sheet is drawn (agni issue
// 1061), the (n+1)th the page has logged, and reports it as a row. The walk then runs checks on a
// loaded design, as a reader who looks at the first sheet before pressing Run does. A board the
// engine kept nowhere a later worker can restore it from fails the row, since the load then bought
// nothing.
async function backgroundLoad(log, design, n, name = "1. background load") {
  return step(name, design, async () => {
    if (!backgroundLoads) return { status: "not available", detail: "walked with --no-background-load" };
    await poll(async () => log.loads.length > n, 180_000);
    const line = log.loads[n];
    if (line.startsWith("agni load skipped ")) return { status: "not available", detail: "the design has no board" };
    const m = /in (\d+) ms: board (.+)$/.exec(line);
    if (!m) throw new Error(`unreadable load line: ${line}`);
    return { status: m[2] === "not persisted" ? "fail" : "ok", ms: Number(m[1]), detail: `board ${m[2]}` };
  });
}

// checksAgain clicks Run checks a second time on the same page. The viewer keeps each rule's verdict
// and asks only for rules it lacks, and the worker keeps each rule's results on the design's model
// (agni 390), so this is the wait a reader has once the design is loaded. With --timing it lists the
// requests the click made.
async function checksAgain(page, design, log) {
  return checks(page, design, "checks again", log);
}

// walkDesign is step 4 on one open design: a finding into the drawing, a query, a trace and a saved
// report.
async function walkDesign(page, design, log) {
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
  // A click on a net asks for its first rows and says how many it left out (agni issue 896). The
  // busiest net is the case that used to answer whole, so the time runs from the ask to its count.
  await step("4. ask about the busiest net", design, async () => {
    await page.fill("textarea.query-text", "net.pin_count(?n, ?c) => ?n, ?c order by ?c desc limit 1");
    await page.click("button.query-run");
    await page.waitForSelector(".query-row, .query-empty, .query-error", { timeout: 120_000 });
    if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").textContent());
    const locate = page.locator(".query-row td:nth-child(2) .query-locate").first();
    if ((await locate.count()) === 0) return { status: "not available", detail: "no net to select" };
    const net = (await locate.textContent())?.trim() ?? "";
    await locate.click();
    await page.waitForSelector(".query-ask:not([disabled])", { timeout: 60_000 });
    const before = (await page.locator(".query-count").textContent()) ?? "";
    const t = performance.now();
    await page.click(".query-ask");
    // Waited for in the page on every frame, since the walk's 50 ms poll is coarser than the answer.
    await page.waitForFunction(
      (prev) => {
        const c = document.querySelector(".query-count")?.textContent ?? "";
        if (document.querySelector(".query-error, .query-empty")) return true;
        return c !== prev && /result\(s\)/.test(c);
      },
      before,
      { polling: "raf", timeout: 120_000 },
    );
    const ms = since(t);
    if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").textContent());
    if (await page.locator(".query-empty").count()) return { status: "fail", detail: `${net}: the net's question answered nothing (#975 where pin.net is empty)`, ms };
    const count = (await page.locator(".query-count").textContent()) ?? "";
    const head = count.match(/^\s*(\d+( of \d+)? result\(s\))/)?.[1] ?? count.trim();
    return { detail: `${net}: ${head}`, ms };
  });
  // The worker keeps what it read between questions (agni issue 974). One replaced after every
  // request fetched the design's files again for the next, so a question asked twice must fetch none.
  await step("4. ask again", design, async () => {
    const before = log.requests.length;
    // Every design has a net, so this answers rows wherever the walk got this far, and its count
    // differs from the last step's. The last step may have answered nothing, which shows no count
    // and leaves an empty-answer note that must not satisfy the wait.
    await page.fill("textarea.query-text", "net.pin_count(?n, ?c) => ?n, ?c order by ?c desc limit 2");
    const counted = page.locator(".query-count");
    const shown = (await counted.count()) ? ((await counted.textContent()) ?? "") : "";
    const t = performance.now();
    await page.click("button.query-run");
    await page.waitForFunction(
      (prev) => {
        const c = document.querySelector(".query-count")?.textContent ?? "";
        return document.querySelector(".query-error") || (c !== prev && /result\(s\)/.test(c));
      },
      shown,
      { polling: "raf", timeout: 120_000 },
    );
    const ms = since(t);
    if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").textContent());
    const refetched = log.requests.slice(before).filter((r) => r.url.includes("/raw/")).length;
    if (refetched) return { status: "fail", detail: `fetched ${refetched} of the design's files again (#974)`, ms };
    return { detail: "no file fetched again", ms };
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
  await step("4. save the report", design, async () => {
    if ((await page.locator(".checks-save").count()) === 0) return { status: "not available", detail: "this viewer has no Save report (#127)" };
    const save = async (format) => {
      const [dl] = await Promise.all([page.waitForEvent("download", { timeout: 120_000 }), page.click(`.checks-save[data-format="${format}"]`)]);
      return { name: dl.suggestedFilename(), text: readFileSync(await dl.path(), "utf8") };
    };
    const html = await save("html");
    const json = await save("json");
    if (!html.text.includes("<html")) throw new Error(`${html.name} is not an HTML page`);
    const doc = JSON.parse(json.text);
    if (!doc.meta?.producerVersion) throw new Error(`${json.name} names no producer version`);
    const kb = (t) => `${Math.max(1, Math.round(t.length / 1024))} KB`;
    return { detail: `${html.name} ${kb(html.text)}, ${json.name} ${kb(json.text)} with ${doc.findings?.length ?? 0} findings, version ${doc.meta.producerVersion}` };
  });
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
    await page.goto(timedURL(seed.href), { waitUntil: "domcontentloaded" });
    const drawn = await opened(page).catch((e) => {
      throw new Error(`${e.message}, ${pendingFiles(log.requests, prefix, performance.now())}`);
    });
    const slow = slowestFetch(log.fetches);
    return { ms: drawn - t, detail: `first sheet drawn; rule catalog after ${Math.round(since(t))} ms; ${seed.meta.trim()}${slow ? `; ${slow}` : ""}` };
  });
  // The load is read before Run checks, whose own loading is the checks load row's.
  if (ok) openLoad(log, seed.title, await page.evaluate(() => performance.now()));
  if (ok) await backgroundLoad(log, seed.title, 0);
  const run = ok && (await checks(page, seed.title));
  if (run) {
    checksTiming(log, seed.title, run);
    await checksAgain(page, seed.title, log);
    await walkDesign(page, seed.title, log);
    await step("1. reopen after a reload", seed.title, async () => {
      const t2 = performance.now();
      // The viewer rewrites its URL as it is used, dropping ?timing, so the reload names it again.
      await page.goto(timedURL(page.url()), { waitUntil: "domcontentloaded" });
      const drawn = await opened(page);
      return { ms: drawn - t2, detail: "first sheet drawn after a reload" };
    });
    // A fresh page over the store the first visit filled: the load and then the check restore the
    // board rather than parsing it.
    await backgroundLoad(log, seed.title, 1, "background load after a reload");
    const again = await checks(page, seed.title, "checks after a reload", log, true);
    if (again) checksTiming(log, seed.title, again, " after a reload");
  }
  network(log, seed.title, undefined);
  await page.close();
}

// drop opens the page's drop target, hands it files through one of its inputs, and opens the first
// design it proposes.
async function drop(design, input, files, markers) {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  const log = record(page);
  await page.goto(timedURL("designs/local/view/"), { waitUntil: "domcontentloaded" });
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
    if (ok) await backgroundLoad(log, design, 0, "2. background load");
    const run = ok && (await checks(page, design));
    if (run) {
      checksTiming(log, design, run);
      await walkDesign(page, design, log);
    }
  }
  if (control) await page.evaluate((m) => fetch(`${location.origin}/collect?n=${encodeURIComponent(m)}`).catch(() => undefined), markers[0]);
  network(log, design, markers);
  await page.close();
}

const text = (paths) => paths.filter((p) => /\.(edn|eds|kicad_sch|kicad_pcb)$/.test(p)).map((p) => readFileSync(p, "latin1")).join("\n");
const gatewayMarkers = markersFrom(text(walk(gateway)));

// 2. The tutorial board as a folder, then as a zip.
const drops = !only || only.includes("drops");
if (drops) await drop("gateway, as a folder", "#drop-folder-input", gateway, gatewayMarkers);
if (drops) await drop("gateway, as a zip", "#drop-input", { name: "gateway.zip", mimeType: "application/zip", buffer: storedZip(gateway) }, gatewayMarkers);

// 3. An EDIF netlist with its schematic export.
add("3. real EDIF pair", "", "not available", "#928, the samples corpus holds no .edn with its .eds");
if (drops) await drop("companion-demo .edn + .eds (synthetic)", "#drop-input", companionPair, markersFrom(text(companionPair)));

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
