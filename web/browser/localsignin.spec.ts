// A person signs in at /signin with a local account and lands in the viewer (agni issue 1879). The
// spec starts `agni serve --auth tokens --local-accounts` on localhost, so it needs no TLS, and adds
// one account with `agni users add` before the server starts. It opens a design while signed out,
// which sends the page to the sign-in form. A wrong password shows the form again with its error. The
// right one follows the redirect back to the design, and the viewer draws its sheet.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser } from "playwright-core";
import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { launch, withPage } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const password = "carol's long password";

let browser: Browser;
let server: Server;

beforeAll(async () => {
  const designs = mkdtempSync(join(tmpdir(), "agni-local-designs-"));
  copyFileSync(join(repoRoot, "readers", "kicad", "testdata", "wirenet.kicad_sch"), join(designs, "wirenet.kicad_sch"));
  const store = pathToFileURL(mkdtempSync(join(tmpdir(), "agni-local-store-"))).href;
  const uploads = pathToFileURL(mkdtempSync(join(tmpdir(), "agni-local-uploads-"))).href;
  execFileSync(inject("agniBin"), ["users", "add", "carol", "--store", store, "--password-stdin"], { input: `${password}\n`, cwd: repoRoot });
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  server = await start({
    what: "agni serve (local sign-in)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--auth", "tokens", "--local-accounts", "--store", store, "--uploads", uploads, "--mount", `demo=${designs}`, "--web-dir", "web"],
    cwd: repoRoot,
    base,
    health: `${base}/healthz`,
  });
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await server?.stop();
});

describe("signing in with a local account", () => {
  it("keeps a wrong password on the form, and takes the right one into the viewer", async () => {
    await withPage(browser, async (page) => {
      const design = "/mounts/demo/wirenet.kicad_sch/view?mode=svg";
      await page.goto(`${server.base}${design}`);
      await page.waitForURL((u) => u.pathname === "/signin" && u.searchParams.get("next") === design);

      await page.locator("#username").fill("carol");
      await page.locator("#password").fill("not carol's password");
      await page.locator('form[action="/signin/local"] button[type="submit"]').click();
      await page.waitForURL((u) => u.pathname === "/signin/local");
      expect(await page.locator(".error").textContent()).toBe("That username and password don't match an account on this server.");
      expect(await page.locator("#username").count()).toBe(1);
      expect((await page.context().cookies()).map((c) => c.name)).not.toContain("agni_session");

      await page.locator("#username").fill("carol");
      await page.locator("#password").fill(password);
      await page.locator('form[action="/signin/local"] button[type="submit"]').click();
      await page.waitForURL((u) => u.pathname.startsWith("/mounts/demo/"), { timeout: 60_000 });
      await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 60_000 });
      const session = (await page.context().cookies()).find((c) => c.name === "agni_session");
      expect(session?.httpOnly).toBe(true);
      expect(await page.locator("#username").count()).toBe(0);
    });
  }, 180_000);
});
