// The guided mode of the browser-review walk (agni issue 1118). With the browser headed, the walk stops
// before each step and shows a panel saying what the step does and what the product is doing, with
// the previous step's result, until the reader presses Next. The same text goes to the terminal.
//
// STEP_BY_STEP=0 turns it off for a headed run that should play straight through. A headless run
// never stops, whatever STEP_BY_STEP says, since nobody is there to press Next and CI would wait
// forever.

import type { Page } from "playwright-core";

export const stepByStep = process.env.HEADED === "1" && process.env.STEP_BY_STEP !== "0";

// explanations says, per step, what a reviewer is doing and what the viewer does underneath. A
// checklist run is keyed by its prefix, since the checklist names come from the project.
const explanations: Record<string, string> = {
  open:
    "Open the tutorial project's gateway board. The viewer ingests the design once into a snapshot " +
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
    "Run the house checklist and save the report. The viewer asks ReviewService.RenderReviewReport for " +
    "the page, the same rpc 'agni review --format html' calls, so the saved file must match the CLI's " +
    "byte for byte apart from the time it was written.",
  "agni trial":
    "Outside the browser, 'agni trial' walks the same design the way a release trial on someone " +
    "else's machine would, and reports each step without any design content.",
  "page health":
    "Finally, the page itself: no uncaught errors and no failed requests during the whole walk.",
  done: "The walk is over. Press Next to close the browser; the step table prints in the terminal.",
};

export function explain(step: string): string {
  return explanations[step] ?? explanations[step.replace(/ \S+$/, "")] ?? "";
}

// pause shows the panel for the next step and waits for Next. It removes the panel before returning,
// so the panel never covers anything the step clicks.
export async function pause(page: Page, step: string, previous: string): Promise<void> {
  const text = explain(step);
  process.stdout.write(`\n── ${step}\n${text}\n`);
  await page.evaluate(
    ({ step, text, previous }) => {
      const w = window as unknown as { __walkNext?: boolean };
      w.__walkNext = false;
      const box = document.createElement("div");
      box.id = "walk-guide";
      box.style.cssText =
        "position:fixed;left:16px;bottom:16px;z-index:2147483647;max-width:440px;padding:12px 14px;" +
        "background:#1f2937;color:#f9fafb;font:13px/1.45 system-ui,sans-serif;border-radius:8px;" +
        "box-shadow:0 6px 24px rgba(0,0,0,.35)";
      const add = (tag: string, content: string, css: string): HTMLElement => {
        const el = document.createElement(tag);
        el.textContent = content;
        el.style.cssText = css;
        box.appendChild(el);
        return el;
      };
      if (previous) add("div", previous, "opacity:.75;margin-bottom:8px;font-size:12px");
      add("div", `Next: ${step}`, "font-weight:600;margin-bottom:4px");
      add("div", text, "margin-bottom:10px");
      const next = add("button", "Next", "padding:4px 14px;font:inherit;cursor:pointer");
      next.onclick = () => {
        w.__walkNext = true;
      };
      document.body.appendChild(box);
      next.focus();
    },
    { step, text, previous },
  );
  await page.waitForFunction(() => (window as unknown as { __walkNext?: boolean }).__walkNext === true, null, {
    timeout: 0,
  });
  await page.evaluate(() => document.getElementById("walk-guide")?.remove());
}
