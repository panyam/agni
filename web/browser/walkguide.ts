// The guided mode of the browser-review walk (agni issue 1118). With the browser headed, the walk stops
// before each step and shows a panel saying what the step does and what the product is doing, with
// the previous step's result, until the reader presses Next. The same text goes to the terminal.
//
// STEP_BY_STEP=0 turns it off for a headed run that should play straight through. A headless run
// never stops, whatever STEP_BY_STEP says, since nobody is there to press Next and CI would wait
// forever.

import { appendFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "playwright-core";

export const stepByStep = process.env.HEADED === "1" && process.env.STEP_BY_STEP !== "0";

// explanations says, per step, what a reviewer is doing and what the viewer does underneath. A
// checklist run is keyed by its prefix, since the checklist names come from the project.
const explanations: Record<string, string> = {
  open:
    "Open {board}. The viewer ingests the design once into a snapshot " +
    "(netlist, schematic sheets and board), draws the first sheet, and loads the rule catalog. The " +
    "number on Run checks is how many rules have not run yet.",
  "checklist picker":
    "The Review panel's picker lists the checklists the project declares, including ones inherited " +
    "through 'extends'. It asks ReviewService.ListChecklists, the same rpc 'agni checklists' asks, so " +
    "the two must list the same names.",
  "run checklist":
    "Run one checklist. Each item is a question bound to a rule, a query or a manual check, and the " +
    "engine scores it: pass, fail, provisional (a fail resting on datasheet values below the " +
    "confidence floor) or not automated. Coverage is how many items the engine could answer.",
  "run checks":
    "Run every rule in the catalog over the design. The findings list fills, and nothing is lit on the " +
    "drawing yet, because the run-wide field stays off until you choose something to look at.",
  "click a finding":
    "Click the first finding. The viewer locates its subjects (nets, parts, pins) on the sheet and " +
    "lights them. It should light a few elements, never the whole board.",
  "open a query set":
    "Open examples/netlist-audit/audit.yaml as a query set: a netlist audit workbook's tables as " +
    "Datalog queries, each opening in its own tab over the one design read. No tab should answer an " +
    "error.",
  "click a query cell":
    "Click a part in the 'Parts on nets' table. A query answer carries the entity behind each cell, so " +
    "the click lights that part on the drawing.",
  ask:
    "Ask follows up on the selected part with a canned question, which opens as a new query tab. Its " +
    "cells click into the drawing the same way.",
  "save the report":
    "Run each checklist and save its report. The viewer asks ReviewService.RenderReviewReport for " +
    "the page, the same rpc 'agni review --format html' calls, so each saved file must match the CLI's " +
    "byte for byte apart from the time it was written, links included.",
  "agni trial":
    "Outside the browser, 'agni trial' walks the same design the way a release trial on someone " +
    "else's machine would, and reports each step without any design content.",
  "page health":
    "Finally, the page itself: no uncaught errors and no failed requests during the whole walk.",
  done:
    "The walk is over, and the page is no longer paused: explore the viewer as you like. Press Next to " +
    "close the browser; the step table prints in the terminal.",
};

// explain is the text for one step, with {board} spelled as the walk's board.
export function explain(step: string, board = "the design"): string {
  return (explanations[step] ?? explanations[step.replace(/ \S+$/, "")] ?? "").replace("{board}", board);
}

// intro fills the page before the walk opens anything, since the browser starts on an empty page and
// a panel over a blank screen reads as a walk that failed to load. It names the board, says what the
// walk does and how to drive it, lists the steps, and waits for Next like every other stop.
export async function intro(page: Page, board: string, steps: readonly string[]): Promise<void> {
  process.stdout.write(`\n── the browser-review walk over ${board}\nPress Next in the browser at each stop.\n`);
  await page.setContent(`<!doctype html><html><head><meta charset="utf-8"><title>agni browser-review walk</title></head>
<body style="margin:0;font:15px/1.5 system-ui,sans-serif;background:#f8fafc;color:#111827">
<main style="max-width:720px;margin:48px 48px 48px 500px;padding:0 24px">
<h1 style="font-size:24px;margin:0 0 8px">The browser-review walk</h1>
<p style="margin:0 0 16px;color:#374151">A reviewer's path through agni's viewer, on ${board}.</p>
<p>The walk opens the design, runs its project's checklists and the full rule catalog, clicks a finding and a
query answer into the schematic, asks a follow-up question, and saves the checklist page, checking each
against the command line as it goes. It stops before every step with a panel in the top left saying what
the step does and what agni does underneath. Press <b>Next</b> there, or Enter, to run it.</p>
<ol style="padding-left:20px;color:#374151">${steps.map((s) => `<li>${s}</li>`).join("")}</ol>
<p style="color:#6b7280;font-size:13px">STEP_BY_STEP=0 plays the walk straight through; the step table prints in the terminal when it ends.</p>
</main></body></html>`);
  await pause(page, "open", "", board);
}

// pause shows the panel for the next step and waits for Next. It removes the panel before returning,
// so the panel never covers anything the step clicks.
//
// The page is FROZEN while a panel waits (agni issue 1118): a clear shield takes the clicks, and
// scrolling and keys other than the ones that continue are swallowed, so the step runs on exactly the
// page the walk left. A reader exploring between stops otherwise changed the selection, the zoom and
// the scroll under the walk, and the next step clicked into a page it did not expect. The last stop,
// "done", leaves the page open, so the viewer can be explored freely before Next closes it.
export async function pause(page: Page, step: string, previous: string, board?: string): Promise<void> {
  const text = explain(step, board);
  const explore = step === "done";
  process.stdout.write(`\n── ${step}\n${text}\n`);
  await page.evaluate(
    ({ step, text, previous, explore }) => {
      const w = window as unknown as { __walkNext?: boolean; __walkRelease?: () => void };
      w.__walkNext = false;
      const at = { x: window.scrollX, y: window.scrollY };
      const shield = document.createElement("div");
      const swallow = (e: Event): void => {
        if (!(e.target instanceof Node) || !box.contains(e.target)) {
          e.preventDefault();
          e.stopPropagation();
        }
      };
      if (!explore) {
        shield.id = "walk-shield";
        shield.title = "The page is paused while the walk explains the next step. Press Next to continue.";
        shield.style.cssText = "position:fixed;inset:0;z-index:2147483646;background:rgba(15,23,42,.06);cursor:not-allowed";
        document.body.appendChild(shield);
        window.addEventListener("wheel", swallow, { capture: true, passive: false });
        window.addEventListener("touchmove", swallow, { capture: true, passive: false });
      }
      w.__walkRelease = () => {
        shield.remove();
        window.removeEventListener("wheel", swallow, true);
        window.removeEventListener("touchmove", swallow, true);
        window.scrollTo(at.x, at.y);
      };
      const box = document.createElement("div");
      box.id = "walk-guide";
      // Top left, because a headed window on a laptop is often shorter than the 1000 px page, so
      // anything pinned to the bottom sat below the window and the Next button could not be seen.
      box.style.cssText =
        "position:fixed;left:16px;top:16px;z-index:2147483647;max-width:440px;max-height:calc(100vh - 32px);" +
        "overflow:auto;padding:12px 14px;background:#1f2937;color:#f9fafb;font:13px/1.45 system-ui,sans-serif;" +
        "border-radius:8px;box-shadow:0 6px 24px rgba(0,0,0,.35)";
      const add = (tag: string, content: string, css: string): HTMLElement => {
        const el = document.createElement(tag);
        el.textContent = content;
        el.style.cssText = css;
        box.appendChild(el);
        return el;
      };
      const head = add("div", "", "display:flex;align-items:center;gap:10px;margin-bottom:6px");
      const next = document.createElement("button");
      next.textContent = "Next \u2192";
      next.style.cssText = "padding:4px 14px;font:inherit;font-weight:600;cursor:pointer";
      const title = document.createElement("span");
      title.textContent = `Next: ${step}`;
      title.style.cssText = "font-weight:600";
      head.append(next, title);
      add("div", text, "margin-bottom:8px");
      if (previous) add("div", `Last: ${previous}`, "opacity:.75;font-size:12px");
      add(
        "div",
        explore
          ? "The page is yours now: explore the viewer, then press Next to close the browser."
          : "The page is paused while you read. Enter, Space or \u2192 also continues.",
        "opacity:.6;font-size:11px;margin-top:6px",
      );
      const go = (): void => {
        w.__walkNext = true;
        window.removeEventListener("keydown", onKey, true);
      };
      const onKey = (e: KeyboardEvent): void => {
        if (e.key === "Enter" || e.key === " " || e.key === "ArrowRight") {
          e.preventDefault();
          go();
        } else if (!explore) {
          e.preventDefault();
          e.stopPropagation();
        }
      };
      next.onclick = go;
      window.addEventListener("keydown", onKey, true);
      document.body.appendChild(box);
      next.focus();
    },
    { step, text, previous, explore },
  );
  await page.waitForFunction(() => (window as unknown as { __walkNext?: boolean }).__walkNext === true, null, {
    timeout: 0,
  });
  await page.evaluate(() => {
    document.getElementById("walk-guide")?.remove();
    (window as unknown as { __walkRelease?: () => void }).__walkRelease?.();
  });
}

// While a step runs in a headed walk the page shows a loader and takes no clicks, because a step can
// take several seconds on Jetson and a reader clicking in that time changes the page under the walk.
// The walk's own clicks are real input too, so nothing at the browser level can tell them from a
// reader's (Input.setIgnoreInputEvents drops both). Instead the loader is a shield the walk lifts
// for each of its own actions (act) and raises again after, which also restores it after a
// navigation replaces the page. A headless run shows nothing and pays nothing.
const headed = process.env.HEADED === "1";
let running = "";

export async function busy(page: Page, step: string): Promise<void> {
  if (!headed) return;
  running = step;
  await page.evaluate(showBusy, step).catch(() => {});
}

export async function idle(page: Page): Promise<void> {
  if (!headed) return;
  running = "";
  await page.evaluate(() => document.getElementById("walk-busy")?.remove()).catch(() => {});
}

// act runs one of the walk's own page actions with the loader lifted, so the click lands on the
// viewer rather than on the shield.
export async function act<T>(page: Page, action: () => Promise<T>): Promise<T> {
  if (!headed || !running) return action();
  await page
    .evaluate(() => {
      const b = document.getElementById("walk-busy");
      if (b) b.style.pointerEvents = "none";
    })
    .catch(() => {});
  try {
    return await action();
  } finally {
    await page.evaluate(showBusy, running).catch(() => {});
    await trace(page);
  }
}

// trace records where the page and its window sit after each of the walk's actions, in a headed
// run, to EXERCISE_OUT/headed-trace.txt. A view that jumps can move without the page scrolling at
// all (the window re-fitted to the screen, or the visual viewport panned), which nothing measured
// inside the page shows, so this is the evidence a report of one needs.
async function trace(page: Page): Promise<void> {
  const out = process.env.EXERCISE_OUT;
  if (!out) return;
  const at = await page
    .evaluate(() => ({
      scrollY: window.scrollY,
      viewportTop: window.visualViewport?.offsetTop ?? 0,
      viewportHeight: window.visualViewport?.height ?? 0,
      innerHeight: window.innerHeight,
      screenY: window.screenY,
      outerHeight: window.outerHeight,
      screenHeight: window.screen.availHeight,
    }))
    .catch(() => undefined);
  if (at) appendFileSync(join(out, "headed-trace.txt"), `${running}: ${JSON.stringify(at)}\n`);
}

// showBusy raises the loader, creating it when a navigation has removed it. It runs in the page.
function showBusy(step: string): void {
  let b = document.getElementById("walk-busy");
  if (!b) {
    const style = document.createElement("style");
    style.textContent = "@keyframes walk-spin{to{transform:rotate(360deg)}}";
    document.head.appendChild(style);
    b = document.createElement("div");
    b.id = "walk-busy";
    b.style.cssText = "position:fixed;inset:0;z-index:2147483645;background:rgba(15,23,42,.04);cursor:progress";
    const pill = document.createElement("div");
    pill.style.cssText =
      "position:absolute;top:12px;left:50%;transform:translateX(-50%);display:flex;align-items:center;gap:10px;" +
      "padding:8px 14px;background:#1f2937;color:#f9fafb;font:13px/1.4 system-ui,sans-serif;border-radius:999px;" +
      "box-shadow:0 6px 24px rgba(0,0,0,.3)";
    const spin = document.createElement("span");
    spin.style.cssText =
      "width:14px;height:14px;border:2px solid rgba(255,255,255,.3);border-top-color:#fff;border-radius:50%;" +
      "animation:walk-spin .8s linear infinite";
    const label = document.createElement("span");
    label.id = "walk-busy-label";
    pill.append(spin, label);
    b.appendChild(pill);
    document.body.appendChild(b);
    const swallow = (e: Event): void => {
      if (document.getElementById("walk-busy")) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    window.addEventListener("wheel", swallow, { capture: true, passive: false });
    window.addEventListener("keydown", swallow, true);
  }
  b.style.pointerEvents = "auto";
  const label = document.getElementById("walk-busy-label");
  if (label) label.textContent = `Running: ${step}… the page ignores clicks while the walk works.`;
}
