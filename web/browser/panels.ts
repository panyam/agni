// Helpers several browser specs share for driving the viewer's docked panels.

import type { Page } from "playwright-core";

// growQueryPanel enlarges the docked query panel, the same thing dragging its sash does. The dock
// opens it a couple of rows tall, which is not enough for the results table to be on screen at all.
// Drop it from a spec once #759 sizes the dock's panels.
export async function growQueryPanel(page: Page): Promise<void> {
  await page.evaluate(() => {
    const overlay = document.querySelector(".query")?.closest<HTMLElement>(".dv-render-overlay");
    if (!overlay) throw new Error("query panel is not in a dock overlay");
    overlay.style.top = "420px";
    overlay.style.height = "560px";
    window.dispatchEvent(new Event("resize"));
  });
  await page.waitForTimeout(200);
}
