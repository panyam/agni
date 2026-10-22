// Helpers several browser specs share for driving the viewer's docked panels.

import type { Page } from "playwright-core";

// growQueryPanel enlarges the docked query panel by dragging the sash above it, as a reader would, to
// 42% of the window's height. Moving the panel's overlay instead (the first version of this helper)
// left the real boundary, and the sash dockview draws on it, where the layout put it, so once the
// panel opened taller the sash sat across the result rows and took their clicks (agni issue 759).
export async function growQueryPanel(page: Page): Promise<void> {
  const sash = await page.evaluate(() => {
    const tab = Array.from(document.querySelectorAll(".tab")).find((t) => t.textContent?.trim().startsWith("Query"));
    const group = tab?.closest(".groupview")?.getBoundingClientRect();
    if (!group) throw new Error("the query panel is not in a dock group");
    const x = group.left + group.width / 2;
    const hit = Array.from(document.querySelectorAll(".sash"))
      .map((s) => s.getBoundingClientRect())
      .find((r) => r.width > r.height && r.left <= x && r.right >= x && Math.abs(r.top + r.height / 2 - group.top) <= 8);
    if (!hit) throw new Error("no sash on the query panel's top edge");
    return { x, y: hit.top + hit.height / 2, target: Math.round(window.innerHeight * 0.42) };
  });
  if (sash.y <= sash.target) return;
  await page.mouse.move(sash.x, sash.y);
  await page.mouse.down();
  await page.mouse.move(sash.x, sash.target, { steps: 8 });
  await page.mouse.up();
  await page.waitForTimeout(200);
}
