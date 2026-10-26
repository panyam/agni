// The guided mode of the browser-review walk (agni issue 1118). The walk opens the viewer with its
// review tour running (web/src/reviewtour.ts, `?tour=review&tourpace=host`), and each step a reader
// takes in the viewer is a tour step. With the browser headed the READER does those steps: the walk
// releases one, the page dims around the controls it needs, and the walk waits for the viewer to
// report the step done. The steps the walk does on its own (open the design, compare with the CLI,
// run `agni trial`) stop with a panel saying what they check, until the reader presses Next.
//
// STEP_BY_STEP=0 turns the reader's part off for a headed run that should play straight through, and
// a headless run never stops, whatever STEP_BY_STEP says, since nobody is there to press Next and CI
// would wait forever. Either way the walk still takes the tour, acting through its holes as a reader
// would, so a step whose hole misses its control, or whose action reports no event, fails the walk.

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
  "save the report":
    "Next the walk runs each checklist again and you save its page. The viewer asks " +
    "ReviewService.RenderReviewReport for it, the same rpc 'agni review --format html' calls, so each " +
    "saved file must match the CLI's byte for byte apart from the time it was written, links included.",
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
<p>The walk opens the design and then hands you the viewer's review tour: run a checklist and the full rule
catalog, click a finding and a query answer into the schematic, ask a follow-up question, select a block
and save the checklist page. At each of those the page dims around what the step needs, and you do it;
<b>Show me</b> does it for you, and Esc leaves the tour, after which the walk finishes the steps itself.
Behind you it checks each result against the command line. The steps it does alone stop with a panel in
the top left saying what they check; press <b>Next</b> there, or Enter.</p>
<ol style="padding-left:20px;color:#374151">${steps.map((s) => `<li>${s}</li>`).join("")}</ol>
<p style="color:#6b7280;font-size:13px">STEP_BY_STEP=0 plays the walk straight through, the walk taking the tour itself; the step table prints in the terminal when it ends.</p>
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

// The tour's handle on the page (TourHandle in web/src/reviewtour.ts), as the walk reads it.
interface TourState {
  step: string;
  index: number;
  ended: string;
}
const tourState = (page: Page): Promise<TourState | null> =>
  page.evaluate(() => {
    const t = (globalThis as { __agniTour?: TourState }).__agniTour;
    return t ? { step: t.step, index: t.index, ended: t.ended } : null;
  });

// tourAct takes one of the tour's steps. It releases the step, so the page dims around the step's
// controls, and then either waits for the reader to do it (stepByStep) or runs the walk's own
// action, which reaches the controls only through the holes, as a reader would. Both end when the
// viewer reports the step done and the tour moves on. A tour the reader left (Esc) makes the walk
// carry on alone, so a reader can stop taking part without stopping the walk.
export async function tourAct(page: Page, step: string, action: () => Promise<unknown>, timeout = 180_000): Promise<void> {
  await page.waitForFunction(() => (globalThis as { __agniTour?: unknown }).__agniTour !== undefined, null, { timeout: 30_000 });
  const at = (await tourState(page))!;
  if (at.ended) {
    await act(page, action);
    return;
  }
  if (at.step !== step) throw new Error(`the tour is on its "${at.step}" step where the walk expected "${step}"`);
  await page.evaluate(() => (globalThis as { __agniTour?: { release(): void } }).__agniTour?.release());
  // A released step must cut a hole the reader can reach without scrolling, since the dimmed page takes
  // no wheel. The walk's own clicks scroll their target into view first, so without this a step whose
  // control sat below its panel's fold passed here and left a reader in front of a page with no way in.
  await page
    .waitForFunction(() => Number(document.querySelector<HTMLElement>(".tsappkit-tour")?.dataset.tourHoles ?? "0") > 0, null, {
      timeout: 5_000,
    })
    .catch(() => {
      throw new Error(`the tour's "${step}" step shows no hole: its controls are off screen or not drawn`);
    });
  const moved = (wait: number) =>
    page.waitForFunction(
      (i) => {
        const t = (globalThis as { __agniTour?: TourState }).__agniTour!;
        return t.index !== i || t.ended !== "";
      },
      at.index,
      { timeout: wait },
    );
  if (!stepByStep) {
    await act(page, action);
    await moved(timeout).catch(() => {
      throw new Error(`the tour did not move past "${step}": the viewer reported no event that finishes it`);
    });
    return;
  }
  process.stdout.write(`   your turn: do what the tour's panel says, or press Show me\n`);
  await page.evaluate(() => {
    const b = document.getElementById("walk-busy");
    if (b) b.style.display = "none";
  });
  await moved(0);
  await page.evaluate(() => {
    const b = document.getElementById("walk-busy");
    if (b) b.style.display = "";
  });
  const after = (await tourState(page))!;
  if (after.index === at.index && after.ended !== "" && after.ended !== "done") {
    process.stdout.write("   you left the tour, so the walk does the rest itself\n");
    await act(page, action);
  }
}

// pressedAt is when the reader first pressed in the page during a tour step, so the walk times a step
// from the reader's action rather than from when it offered the step. Undefined when the walk did it.
export async function pressedAt(page: Page, step: string): Promise<number | undefined> {
  if (!stepByStep) return undefined;
  return page.evaluate((s) => (globalThis as { __agniTour?: { pressed: Record<string, number> } }).__agniTour?.pressed[s], step);
}

// lastEvent is the payload of the viewer's latest event of a type since the tour started, which says
// what the reader chose: which checklist, finding or cell.
export async function lastEvent<T>(page: Page, type: string): Promise<T | undefined> {
  return page.evaluate((t) => {
    const events = (globalThis as { __agniTour?: { events: { type: string; data: unknown }[] } }).__agniTour?.events ?? [];
    return [...events].reverse().find((e) => e.type === t)?.data as T | undefined;
  }, type);
}
