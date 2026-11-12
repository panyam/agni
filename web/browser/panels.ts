// Helpers several browser specs share for driving the viewer's docked panels.

import type { Frame, Page } from "playwright-core";

// growQueryPanel enlarges the docked query panel to 42% of the window's height. It drags the sash
// above the panel, as a reader would. The first version of this helper moved the panel's overlay
// instead. That left the real boundary where the layout put it, and dockview draws the sash on that
// boundary. So once the panel opened taller, the sash sat across the result rows and took their
// clicks (agni issue 759).
export async function growQueryPanel(page: Page): Promise<void> {
  const sash = await page.evaluate(() => {
    const tab = Array.from(document.querySelectorAll(".dv-tab")).find((t) => t.textContent?.trim().startsWith("Query"));
    const group = tab?.closest(".dv-groupview")?.getBoundingClientRect();
    if (!group) throw new Error("the query panel is not in a dock group");
    const x = group.left + group.width / 2;
    const hit = Array.from(document.querySelectorAll(".dv-sash"))
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

// showPanel brings a dock panel into view by its id, as a reader would (agni issue 1542). Sheets,
// Checks and Review open from their header buttons. Rules, Lexicon and Coverage share the right
// drawer with Checks, so the drawer opens from Checks's button and then the panel's tab. A panel in
// the grid needs nothing.
export async function showPanel(page: Page | Frame, id: string): Promise<void> {
  // A design folder's page redirects to itself as it opens, which destroys the context an evaluate
  // runs in, so a step that lands mid-redirect runs again on the new page.
  for (let tries = 3; ; tries--) {
    try {
      return await showPanelOnce(page, id);
    } catch (e) {
      if (tries <= 1 || !/context was destroyed|navigat/i.test(String(e))) throw e;
      await page.waitForLoadState("domcontentloaded");
    }
  }
}

async function showPanelOnce(page: Page | Frame, id: string): Promise<void> {
  const visible = (id: string) => {
    const hole = document.querySelector(`[data-dock-panel="${id}"]`);
    return !!hole && hole.getBoundingClientRect().width > 0 && getComputedStyle(hole).visibility !== "hidden" && !hole.closest("#panel-park");
  };
  // The dock builds its drawer buttons once it has laid out, so wait for them before asking.
  await page.locator(".dv-groupview, .dock-drawer-btn").first().waitFor({ state: "attached", timeout: 120_000 });
  if (await page.evaluate(visible, id)) return;
  const inGrid = await page.evaluate((id) => !!document.querySelector(`[data-dock-panel="${id}"]`)?.closest(".dv-groupview"), id);
  const button = page.locator(`.dock-drawer-btn[data-panel="${id}"]`);
  if (inGrid) {
    await page.locator(".dv-default-tab-content", { hasText: new RegExp(`^${GRID_TITLES[id] ?? id}$`) }).first().click();
  } else if (await button.count()) {
    await button.click();
  } else {
    if (!(await page.locator(`.dock-drawer-tab[data-panel="${id}"]`).isVisible())) await page.locator('.dock-drawer-btn[data-panel="checks"]').click();
    await page.locator(`.dock-drawer-tab[data-panel="${id}"]`).click();
  }
  await page.waitForFunction(visible, id, { timeout: 10_000 });
}

// GRID_TITLES names each grid panel's tab, for showPanel to bring one forward in its stack.
const GRID_TITLES: Record<string, string> = {
  canvas: "Canvas", properties: "Properties", parts: "Parts", query: "Query", trace: "Trace",
  tables: "Tables", diff: "Diff", changes: "Changes",
};

// hideDrawers closes any open drawer with a click on its scrim, as a reader would, so a click on the
// drawing lands on it rather than on the scrim.
export async function hideDrawers(page: Page): Promise<void> {
  const scrims = page.locator(".dock-drawer-scrim:visible");
  for (let n = await scrims.count(); n > 0; n = await scrims.count()) {
    // The drawer sits over its own edge of the scrim, so the click goes to the far edge.
    const scrim = scrims.first();
    const box = (await scrim.boundingBox())!;
    const left = ((await scrim.getAttribute("class")) ?? "").includes("dock-drawer-scrim-left");
    await scrim.click({ position: { x: left ? box.width - 4 : 4, y: box.height / 2 } });
  }
}
