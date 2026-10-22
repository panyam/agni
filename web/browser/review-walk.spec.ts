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
import { launch, viewportFor } from "./browser.js";
import { table, type Row } from "./exercise-report.js";
import { act, busy, idle, intro, pause, stepByStep } from "./walkguide.js";

const base = (): string => inject("baseUrl");
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
// WALK_DESIGN picks the board. The gate walks the tutorial project's small synthetic gateway, and
// `make exercise-browser-review` defaults to Jetson, Antmicro's baseboard in examples/jetson-project,
// whose files `make -C examples/jetson-project board` copies in from the fetched samples corpus.
const walks = {
  gateway: {
    mount: "tut",
    project: "examples/tutorial-project",
    path: "designs/gateway",
    about: "the tutorial project's Sample Board, a small synthetic gateway with two checklists and a project query set",
  },
  jetson: {
    mount: "jetson",
    project: "examples/jetson-project",
    path: "designs/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module, sixteen schematic sheets and about 1,100 parts, under the Jetson review project's checklist and conventions",
  },
};
const which = (process.env.WALK_DESIGN || "gateway") as keyof typeof walks;
const walk = walks[which];
if (!walk) throw new Error(`WALK_DESIGN=${which} names no walk; use one of ${Object.keys(walks).join(", ")}`);
const project = join(repoRoot, walk.project);
const design = join(project, walk.path);
const auditSet = join(repoRoot, "examples/netlist-audit/audit.yaml");
const label = `${walk.path.split("/").pop()} (${walk.mount})`;

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
  await act(page, () => page.selectOption("select.rv-checklist", { label: name }));
  await act(page, () => page.click("button.rv-run"));
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
// openTab brings a query tab to the front. Choosing a tab is not what the walk measures, and a real
// click on one depends on nothing being painted over it: in a headed run the tab strip, scrolled to its
// far end by the set's last tab, timed out bringing the first one back (agni issue 1118). So the walk
// scrolls the tab into view and clicks it in the page.
async function openTab(page: Page, title: string): Promise<void> {
  const tab = page.locator(`.query-tab-go[title="${title}"]`);
  await tab.waitFor({ timeout: 30_000 });
  await tab.evaluate((el) => {
    el.scrollIntoView({ block: "nearest", inline: "nearest" });
    (el as HTMLElement).click();
  });
}

// why is the line of a failed step's error worth a table cell: Playwright's first line says only that
// a call timed out, and the reason (what intercepted the click, what was not visible) is in its log.
function why(e: unknown): string {
  const text = (e instanceof Error ? e.message : String(e)).replace(/\u001b\[[0-9;]*m/g, "");
  const lines = text.split("\n").map((l) => l.trim()).filter((l) => l !== "");
  const reason = [...lines].reverse().find((l) => /intercepts pointer events|not visible|outside of the viewport|not stable|not enabled|detached|waiting for/.test(l));
  return reason && reason !== lines[0] ? `${lines[0]} (${reason.replace(/^-\s*/, "")})` : lines[0];
}

const normalise = (html: string): string => html.replace(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC/g, "<time>");

describe("the browser-review walk", () => {
  it("goes from a checklist to a saved report", async () => {
    const rows: Row[] = [];
    const out = process.env.EXERCISE_OUT;
    if (out) mkdirSync(out, { recursive: true });
    const page = await browser.newPage({ viewport: viewportFor({ width: 1400, height: 1000 }), acceptDownloads: true });
    const pageErrors: string[] = [];
    const failed: string[] = [];
    page.on("pageerror", (e) => pageErrors.push(String(e)));
    page.on("response", (r) => {
      if (r.status() >= 400) failed.push(`${r.status()} ${new URL(r.url()).pathname}`);
    });

    // The intro page stands in for the first stop, so "open" pauses there rather than over a blank page.
    const step = async (name: string, body: () => Promise<string>): Promise<void> => {
      if (stepByStep && rows.length > 0) {
        const last = rows[rows.length - 1];
        await pause(page, name, `${last.step}: ${last.status}, ${last.detail}`, walk.about);
      }
      const t0 = Date.now();
      await busy(page, name);
      try {
        const detail = await body();
        rows.push({ step: name, design: label, status: "ok", ms: Date.now() - t0, detail });
      } catch (e) {
        rows.push({ step: name, design: label, status: "fail", ms: Date.now() - t0, detail: why(e) });
        if (out) await page.screenshot({ path: join(out, `review-walk-${name.replace(/\W+/g, "-")}.png`) }).catch(() => {});
      }
    };

    try {
      if (stepByStep) {
        await intro(page, walk.about, [
          "open the design",
          "match the checklist picker against 'agni checklists'",
          "run each checklist",
          "run the rule catalog",
          "click a finding into the schematic",
          "open a netlist audit as query tabs",
          "click a part from a query answer",
          "ask a follow-up question",
          "save each checklist page and compare it with the CLI's",
          "run 'agni trial'",
          "check the page logged no errors",
        ]);
      }
      await step("open", async () => {
        await act(page, () => page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view`, { waitUntil: "networkidle" }));
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
        await act(page, () => page.click(".checks-run"));
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
        await act(page, () => first.click());
        return `${name}: ${await lit(page, "the finding")}`;
      });

      await step("open a query set", async () => {
        const queries = (readFileSync(auditSet, "utf8").match(/^ {2}- name:/gm) ?? []).length;
        const before = await page.locator(".query-tab-go").count();
        await act(page, () => page.click(".query-sets-open"));
        await page.waitForSelector(".query-set-item, .query-sets-none", { timeout: 30_000 });
        await act(page, () => page.setInputFiles(".query-set-input", auditSet));
        await page.waitForFunction((n) => document.querySelectorAll(".query-tab-go").length >= n, before + queries, {
          timeout: 30_000,
        });
        const titles = (await page.$$eval(".query-tab-go", (bs) => bs.map((b) => b.getAttribute("title") ?? ""))).slice(before);
        const errors: string[] = [];
        for (const t of titles) {
          await act(page, () => openTab(page, t));
          await page.waitForSelector(".query-row, .query-empty, .query-error", { timeout: 30_000 });
          if (await page.locator(".query-error").count()) errors.push(t);
        }
        expect(titles.length, "the set should open one tab per query").toBe(queries);
        expect(errors, "no tab of the set should answer an error").toEqual([]);
        return `${titles.length} tabs, none in error`;
      });

      await step("click a query cell", async () => {
        await act(page, () => openTab(page, "Parts on nets"));
        await page.locator(".query-row .query-locate").first().waitFor({ timeout: 30_000 });
        // A virtual symbol (#FLG01, #PWR03) is a row with nothing to draw, so the walk takes the first
        // placed part. A part drawn on another sheet is selected without the canvas moving, and its
        // cell's sheet chip is how a reader goes there, so the walk clicks that too.
        const cell = page.locator(".query-row td:nth-child(2)").filter({ has: page.locator(".query-locate") }).filter({ hasNotText: /^\s*#/ }).first();
        const name = (await cell.locator(".query-locate").innerText()).trim();
        await act(page, () => cell.locator(".query-locate").click());
        if ((await settled(page)) === 0 && (await cell.locator(".sheet-badge").count()) > 0) {
          const sheet = (await cell.locator(".sheet-badge").first().innerText()).trim();
          await act(page, () => cell.locator(".sheet-badge").first().click());
          return `${name}, on sheet ${sheet}: ${await lit(page, "the cell")}`;
        }
        return `${name}: ${await lit(page, "the cell")}`;
      });

      await step("ask", async () => {
        const tabs = await page.locator(".query-tab-go").count();
        const button = page.locator(".query-ask:not([disabled])");
        await button.waitFor({ timeout: 30_000 });
        const question = (await button.innerText()).trim();
        await act(page, () => button.click());
        await page.waitForFunction((n) => document.querySelectorAll(".query-tab-go").length > n, tabs, { timeout: 30_000 });
        await page.waitForSelector(".query-row, .query-empty", { timeout: 30_000 });
        if (await page.locator(".query-error").count()) throw new Error(await page.locator(".query-error").innerText());
        return `"${question}" opened its own tab with ${await page.locator(".query-row").count()} rows`;
      });

      await step("save the report", async () => {
        // Every checklist, since a page whose items carry no finding has no link to compare, and the
        // links are where the viewer and the CLI once disagreed (agni issue 1120).
        const saved: string[] = [];
        for (const name of checklists) {
          await runChecklist(page, name);
          const [download] = await Promise.all([page.waitForEvent("download", { timeout: 30_000 }), act(page, () => page.click(".rv-save"))]);
          const viewer = readFileSync((await download.path())!, "utf8");
          const cli = agni("review", design, "--checklist", name, "--format", "html", "--server", base(), "--mount", `${walk.mount}=${project}`);
          if (out) {
            writeFileSync(join(out, `review-walk-viewer-${name}.html`), viewer);
            writeFileSync(join(out, `review-walk-cli-${name}.html`), cli);
          }
          expect(normalise(viewer), `the saved ${name} page should be the page \`agni review --format html\` writes`).toBe(normalise(cli));
          const links = (viewer.match(/href="http[^"]*\/view\?/g) ?? []).length;
          saved.push(`${name} (${links} links)`);
        }
        return `${saved.join(", ")} match \`agni review --format html\``;
      });

      await step("agni trial", async () => {
        const report = agni("trial", design);
        const steps = [...report.matchAll(/^\| ([a-z][^|]+?) \| (ok|fail[^|]*|[a-z ]+) \| [^|]* \|$/gm)].filter(
          (m) => m[1] !== "step",
        );
        if (steps.length === 0) throw new Error("the trial report listed no steps");
        // A step the design has nothing for (a project with no query sets) says skipped, which is an
        // answer rather than a failure.
        const bad = steps.filter((m) => m[2] !== "ok" && m[2] !== "skipped").map((m) => `${m[1]}: ${m[2]}`);
        if (bad.length) throw new Error(bad.join("; "));
        const skipped = steps.filter((m) => m[2] === "skipped").map((m) => m[1]);
        return `${steps.length - skipped.length} steps ok${skipped.length ? `, skipped: ${skipped.join(", ")}` : ""}`;
      });

      await step("page health", async () => {
        if (pageErrors.length || failed.length) throw new Error([...pageErrors, ...failed].join("; "));
        return "no page errors, no 4xx or 5xx responses";
      });
      if (stepByStep) await pause(page, "done", `${rows.filter((r) => r.status === "ok").length} of ${rows.length} steps ok`);
    } finally {
      await page.close();
      const text = table(rows);
      if (out) writeFileSync(join(out, "browser-review.md"), `${text}\n`);
    }

    expect(rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`)).toEqual([]);
    expect(rows.length).toBeGreaterThan(8);
  }, stepByStep ? 24 * 3600_000 : 600_000);
});
