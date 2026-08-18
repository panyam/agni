// Launches the Chromium playwright already has on disk, and hands a spec one page.
//
// playwright-core rather than the full playwright package, and no browser download of its own: this
// suite is deliberately outside `make testall` (see the Makefile's browsertest target), so it is
// allowed to depend on a browser being present and to say so plainly when one is not. The gate
// stays hermetic and a machine with no Chromium never turns CI red for a reason unrelated to the
// change under test.

import { chromium, type Browser, type Page } from "playwright-core";

// launch opens a browser, or explains how to get one. playwright-core ships no installer of its
// own, so the message names the command that actually fetches a browser.
export async function launch(): Promise<Browser> {
  try {
    return await chromium.launch();
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
