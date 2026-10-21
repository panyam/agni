// Launches the Chromium playwright already has on disk, and hands a spec one page.
//
// playwright-core rather than the full playwright package, and no browser download of its own. The
// suite runs in `make testall` since PR 629, so CI installs a Chromium as a step and a developer
// installs one once (the error below names the command).
//
// HEADED=1 shows the browser instead of running it headless, and SLOWMO=<ms> pauses that long before
// each action (250 by default when headed), so `make browser-test HEADED=1` or `make
// exercise-browser-review HEADED=1` can be watched (agni issue 1118).

import { chromium, type Browser, type Page } from "playwright-core";

// launch opens a browser, or explains how to get one. playwright-core ships no installer of its
// own, so the message names the command that actually fetches a browser.
export async function launch(): Promise<Browser> {
  const headed = process.env.HEADED === "1";
  const slowMo = Number(process.env.SLOWMO ?? (headed ? 250 : 0)) || 0;
  try {
    return await chromium.launch({ headless: !headed, slowMo });
  } catch (e) {
    const why = e instanceof Error ? e.message : String(e);
    throw new Error(
      `could not launch Chromium for the browser tests.\n${why}\n\n` +
        `Install one with:  cd web && pnpm exec playwright-core install chromium`,
    );
  }
}

// withPage runs one body against a fresh page and always closes it.
//
// A page per assertion rather than one shared across the file, because these tests measure LAYOUT:
// a previous test's scroll position, expanded badge strip or open drawer is exactly the kind of
// leftover state that makes a geometry assertion pass for the wrong reason.
export async function withPage(browser: Browser, body: (page: Page) => Promise<void>): Promise<void> {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  try {
    await body(page);
  } finally {
    await page.close();
  }
}
