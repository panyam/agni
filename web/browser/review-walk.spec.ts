// The browser-review walk, mission #845's exercise, as a spec (agni issue 1118). It opens the tutorial
// gateway design and goes through what a reviewer does: pick each checklist and run it, run the checks,
// click a finding into the drawing, open a query set, click a cell, Ask, and save the checklist page.
//
// It runs in the gate with the rest of the suite, so a viewer change that breaks the walk fails on its
// own PR. `make exercise-browser-review` runs this file alone and writes its steps as a markdown table
// for the mission log (EXERCISE_OUT, which the target prints), and HEADED=1 shows it in a visible
// browser, stopping before each step to explain it until STEP_BY_STEP=0 (walkguide.ts).
//
// Every assertion is about structure (the picker matches the CLI, a click lights some of the drawing
// and not all of it, a set opens one tab per query, the saved page matches the CLI's), never about
// today's counts, so a fixture change does not fail the walk. The counts go in the table instead.
// Each step records a row and the walk carries on past a broken one, so one run reports every broken
// step; the test then fails naming them.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch } from "./browser.js";
import { growQueryPanel } from "./panels.js";
import { table, type Row } from "./exercise-report.js";
import { pause, stepByStep } from "./walkguide.js";

const base = (): string => inject("baseUrl");
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const project = join(repoRoot, "examples/tutorial-project");
const design = join(project, "designs/gateway");
const auditSet = join(repoRoot, "examples/netlist-audit/audit.yaml");
const label = "gateway (tut)";

// The CLI half runs a binary built from this tree, from an empty directory with HOME pointed at it, so
// a developer's ~/.config/agni/agni.yaml cannot add its mounts or web_dir to the run.
let browser: Browser;
let scratch: string;
let agniBin: string;
beforeAll(async () => {
  scratch = mkdtempSync(join(tmpdir(), "agni-review-walk-"));
  agniBin = join(scratch, "agni");
  execFileSync("go", ["build", "-o", agniBin, "./cmd/agni"], { cwd: repoRoot, stdio: "inherit" });
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

function agni(...args: string[]): string {
  return execFileSync(agniBin, args, {
    cwd: scratch,
    env: { ...process.env, HOME: scratch, XDG_CONFIG_HOME: scratch },
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

const highlighted = (page: Page): Promise<number> => page.locator("#svg-view .highlight-overlay svg *").count();
const drawn = (page: Page): Promise<number> =>
  page.evaluate(() => document.querySelectorAll("#svg-view svg:not(.highlight-overlay svg) *").length);

// settled waits until the highlight stops changing for 600 ms, since a click repaints in more than one
// frame, and returns the count it settled on.
async function settled(page: Page): Promise<number> {
  let last = -1;
  let same = 0;
  for (let i = 0; i < 100; i++) {
    const n = await highlighted(page);
    if (n === last) {
      if (++same >= 6) return n;
    } else {
      same = 0;
      last = n;
    }
    await page.waitForTimeout(100);
  }
  return last;
}

// lit checks that a click lit part of the drawing and not the whole of it, and describes the count.
async function lit(page: Page, what: string): Promise<string> {
  const n = await settled(page);
  const of = await drawn(page);
  if (n === 0) throw new Error(`${what} lit nothing`);
  if (n > of / 4) throw new Error(`${what} lit ${n} of ${of} drawn elements, which floods the board`);
  return `${n} of ${of} drawn elements lit`;
}

async function runChecklist(page: Page, name: string): Promise<string> {
  await page.selectOption("select.rv-checklist", { label: name });
  await page.click("button.rv-run");
  await page.waitForFunction(
    () => !document.querySelector(".rv-running") && document.querySelector(".rv-summary"),
    null,
    { timeout: 60_000 },
  );
  if (await page.locator(".rv-error").count()) throw new Error(await page.locator(".rv-error").innerText());
  return (await page.locator(".rv-summary").innerText()).replace(/\s+/g, " ").trim();
}

// normalise drops the line a saved page stamps with the time it was written, the only part two saves of
// one run differ in.
const normalise = (html: string): string => html.replace(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC/g, "<time>");

describe("the browser-review walk", () => {
  it("goes from a checklist to a saved report", async () => {
    const rows: Row[] = [];
    const out = process.env.EXERCISE_OUT;
    const page = await browser.newPage({ viewport: { width: 1400, height: 1000 }, acceptDownloads: true });
    const pageErrors: string[] = [];
    const failed: string[] = [];
    page.on("pageerror", (e) => pageErrors.push(String(e)));
    page.on("response", (r) => {
      if (r.status() >= 400) failed.push(`${r.status()} ${new URL(r.url()).pathname}`);
    });

    const step = async (name: string, body: () => Promise<string>): Promise<void> => {
      if (stepByStep) {
        const last = rows[rows.length - 1];
        await pause(page, name, last ? `${last.step}: ${last.status}, ${last.detail}` : "");
      }
      const t0 = Date.now();
      try {
        const detail = await body();
        rows.push({ step: name, design: label, status: "ok", ms: Date.now() - t0, detail });
      } catch (e) {
        const why = (e instanceof Error ? e.message : String(e)).split("\n")[0];
        rows.push({ step: name, design: label, status: "fail", ms: Date.now() - t0, detail: why });
        if (out) await page.screenshot({ path: join(out, `review-walk-${name.replace(/\W+/g, "-")}.png`) }).catch(() => {});
      }
    };

    try {
      await step("open", async () => {
        await page.goto(`${base()}/designs/tut/designs/gateway/view`, { waitUntil: "networkidle" });
        await page.waitForSelector(".query textarea.query-text", { timeout: 60_000 });
        await page.waitForFunction(() => /\(\d+\)/.test(document.querySelector(".checks-run")?.textContent ?? ""), null, {
          timeout: 60_000,
        });
        return `${await drawn(page)} drawn elements, ${(await page.locator(".checks-run").innerText()).trim()}`;
      });

      let checklists: string[] = [];
      await step("checklist picker", async () => {
        await page.waitForSelector("select.rv-checklist option[value]", { state: "attached", timeout: 30_000 });
        const picker = (await page.$$eval("select.rv-checklist option[value]", (os) =>
          os.map((o) => o.textContent?.trim() ?? "").filter((t) => t !== ""),
        )).sort();
        checklists = agni("checklists", design)
          .split("\n")
          .map((l) => l.trim().split(/\s+/)[0])
          .filter((n) => n !== "")
          .sort();
        expect(picker, "the picker should list exactly what `agni checklists` lists").toEqual(checklists);
        return `${picker.join(", ")}, as \`agni checklists\` lists`;
      });

      for (const name of checklists) {
        await step(`run checklist ${name}`, () => runChecklist(page, name));
      }

      await step("run checks", async () => {
        await page.click(".checks-run");
        await page.waitForFunction(() => document.querySelector(".checks-run")?.textContent?.trim() === "Run checks", null, {
          timeout: 120_000,
        });
        const findings = await page.locator(".check-locate").count();
        if (findings === 0) throw new Error("the run listed no findings");
        const after = await settled(page);
        if (after !== 0) throw new Error(`a run with nothing clicked lit ${after} elements`);
        return `${findings} findings, nothing lit`;
      });

      await step("click a finding", async () => {
        const first = page.locator(".check-locate").first();
        const name = (await first.innerText()).replace(/\s+/g, " ").trim();
        await first.click();
        return `${name}: ${await lit(page, "the finding")}`;
      });

      await step("open a query set", async () => {
        const queries = (readFileSync(auditSet, "utf8").match(/^ {2}- name:/gm) ?? []).length;
        const before = await page.locator(".query-tab-go").count();
        await page.click(".query-sets-open");
        await page.waitForSelector(".query-set-item, .query-sets-none", { timeout: 30_000 });
        await page.setInputFiles(".query-set-input", auditSet);
        await page.waitForFunction((n) => document.querySelectorAll(".query-tab-go").length >= n, before + queries, {
          timeout: 30_000,
        });
        await growQueryPanel(page);
        const titles = (await page.$$eval(".query-tab-go", (bs) => bs.map((b) => b.getAttribute("title") ?? ""))).slice(before);
        const errors: string[] = [];
        for (const t of titles) {
          await page.click(`.query-tab-go[title="${t}"]`);
          await page.waitForSelector(".query-row, .query-empty, .query-error", { timeout: 30_000 });
          if (await page.locator(".query-error").count()) errors.push(t);
        }
        expect(titles.length, "the set should open one tab per query").toBe(queries);
        expect(errors, "no tab of the set should answer an error").toEqual([]);
        return `${titles.length} tabs, none in error`;
      });

      await step("click a query cell", async () => {
        await page.click(`.query-tab-go[title="Parts on nets"]`);
        const cell = page.locator(".query-row .query-locate").first();
        await cell.waitFor({ timeout: 30_000 });
        const name = (await cell.innerText()).trim();
        await cell.click();
        return `${name}: ${await lit(page, "the cell")}`;
      });

      await step("ask", async () => {
        const tabs = await page.locator(".query-tab-go").count();
        const button = page.locator(".query-ask:not([disabled])");
        await button.waitFor({ timeout: 30_000 });
        const question = (await button.innerText()).trim();
        await button.click();
        await page.waitForFunction((n) => document.querySelectorAll(".query-tab-go").length > n, tabs, { timeout: 30_000 });
        await page.waitForSelector(".query-row, .query-empty", { timeout: 30_000 });
        if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").innerText());
        return `"${question}" opened its own tab with ${await page.locator(".query-row").count()} rows`;
      });

      await step("save the report", async () => {
        const name = checklists.includes("house") ? "house" : checklists[0];
        await runChecklist(page, name);
        const [download] = await Promise.all([page.waitForEvent("download", { timeout: 30_000 }), page.click(".rv-save")]);
        const saved = readFileSync((await download.path())!, "utf8");
        const cli = agni("review", design, "--checklist", name, "--format", "html", "--server", base(), "--mount", `tut=${project}`);
        if (out) {
          writeFileSync(join(out, `review-walk-viewer-${name}.html`), saved);
          writeFileSync(join(out, `review-walk-cli-${name}.html`), cli);
        }
        expect(normalise(saved), "the saved page should be the page `agni review --format html` writes").toBe(normalise(cli));
        return `${download.suggestedFilename()} matches \`agni review --checklist ${name} --format html\``;
      });

      await step("agni trial", async () => {
        const report = agni("trial", design);
        const steps = [...report.matchAll(/^\| ([a-z][^|]+?) \| (ok|fail[^|]*|[a-z ]+) \| [^|]* \|$/gm)].filter(
          (m) => m[1] !== "step",
        );
        if (steps.length === 0) throw new Error("the trial report listed no steps");
        const bad = steps.filter((m) => m[2] !== "ok").map((m) => `${m[1]}: ${m[2]}`);
        if (bad.length) throw new Error(bad.join("; "));
        return `${steps.length} steps ok`;
      });

      await step("page health", async () => {
        if (pageErrors.length || failed.length) throw new Error([...pageErrors, ...failed].join("; "));
        return "no page errors, no 4xx or 5xx responses";
      });
      if (stepByStep) await pause(page, "done", `${rows.filter((r) => r.status === "ok").length} of ${rows.length} steps ok`);
    } finally {
      await page.close();
      const text = table(rows);
      if (out) {
        mkdirSync(out, { recursive: true });
        writeFileSync(join(out, "browser-review.md"), `${text}\n`);
      }
    }

    expect(rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`)).toEqual([]);
    expect(rows.length).toBeGreaterThan(8);
  }, stepByStep ? 24 * 3600_000 : 600_000);
});
