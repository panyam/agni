// A walk lights the nets it went on through, and not the ones it stopped on (agni issue 1562). A
// ground or a rail on a board is a pour and many segments, and lighting it, however faint, stacked to
// solid grey over the copper. The Trace tab still says the walk stopped there.

import { beforeAll, afterAll, describe, expect, inject, it } from "vitest";
import type { Browser, Request } from "playwright-core";
import { fromBinary, toJson, type DescMessage } from "@bufbuild/protobuf";
import { HighlightSheetRequestSchema } from "../src/gen/agni/v1/webapi/design_pb.js";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// bodyOf is a request's body in its JSON form, since the viewer speaks binary Connect.
const bodyOf = (req: Request, schema: DescMessage): string => {
  const buf = req.postDataBuffer();
  if (!buf) return "";
  if (!(req.headers()["content-type"] ?? "").startsWith("application/proto")) return buf.toString();
  return JSON.stringify(toJson(schema, fromBinary(schema, new Uint8Array(buf))));
};

describe("a walk on a board", () => {
  it("lights the net it went on through and not the ground it stopped on, and says it stopped there", async () => {
    await withPage(browser, async (page) => {
      const lit: { sheet: string; body: string }[] = [];
      await page.route("**/HighlightSheet", (route) => {
        const body = bodyOf(route.request(), HighlightSheetRequestSchema);
        lit.push({ sheet: (JSON.parse(body || "{}") as { sheet?: string }).sheet ?? "", body });
        return route.continue();
      });
      const litOn = (sheet: string, net: string) => lit.some((l) => l.sheet === sheet && l.body.includes(`"${net}"`));
      // C44 decouples FTDI_3V3 on the Jetson cut, so one hop of Small parts reaches that rail's net and
      // GND, and stops on GND. The link opens the schematic sheet that draws the walk.
      await page.goto(`${base()}/designs/probe/designs/jetson-cut/view?mode=svg&walk=C44&walk_kind=small-parts&walk_hops=1`, {
        waitUntil: "networkidle",
      });
      const ground = page.locator('.walk-net[data-walk-net="GND"]');
      await ground.waitFor({ state: "attached", timeout: 60_000 });
      expect(await ground.locator(".walk-ended").textContent(), "the Trace tab says why the walk stopped").toContain("ends here: ground");

      // The positive control on each sheet. The walk lights the net it went on through.
      const rail = "/USB Debug, PD/FTDI_3V3";
      await expect.poll(() => litOn("/USB Debug, PD", rail), { timeout: 30_000 }).toBe(true);
      expect(litOn("/USB Debug, PD", "GND"), "the schematic sheet lights GND").toBe(false);

      await page.keyboard.press("Control+g");
      await page.locator(".sheet-tile", { hasText: /^\s*Board\s*$/ }).first().click();
      await expect.poll(() => litOn("board", rail), { timeout: 30_000 }).toBe(true);
      expect(litOn("board", "GND"), "the board lights GND").toBe(false);
    });
  }, 120_000);
});
