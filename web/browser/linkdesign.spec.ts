// The designs page links a design to a repo, moves it to a newer tag, and opens both revisions (agni
// issue 1727). It drives the pages as a person does. The new-design form adds a GitHub mount, walks
// the repo to the schematic, and links it at the older ref. The design's page moves the schematic to
// the newer ref. The page then lists two revisions, and each opens in the viewer on its own files.
//
// In the gate it reads a fake GitHub (tools/fakegithub/main.go) whose repo holds a small KiCad schematic
// at two tags, with one net renamed between them. `make exercise-linked-sources` runs it against
// real GitHub and Antmicro's Jetson baseboard instead, through these variables:
//
//	LINK_WALK_SERVER  an agni serve with --store and --uploads that reads GitHub
//	LINK_WALK_REPO    owner/repo
//	LINK_WALK_FILE    the schematic's path in the repo
//	LINK_WALK_OLD     the tag or commit to link first
//	LINK_WALK_NEW     the tag or commit to move to
//	LINK_WALK_DESIGN  the design id to make, bb when unset
//	LINK_WALK_MOUNT   the mount to add, board when unset
//
// SCREENSHOT_DIR keeps a screenshot of each page it reaches.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { launch, withPage } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const shots = process.env.SCREENSHOT_DIR ?? "";
const given = process.env.LINK_WALK_SERVER ?? "";
const repo = process.env.LINK_WALK_REPO ?? "acme/board";
const file = process.env.LINK_WALK_FILE ?? "hw/board.kicad_sch";
// The refs to link and move to: the fake's two tags in the gate (agni issue 1781), and Antmicro's two
// commits in the Jetson run, since Antmicro tags no revisions.
const oldRef = process.env.LINK_WALK_OLD ?? "1.1.2";
const newRef = process.env.LINK_WALK_NEW ?? "1.1.3";
// commits maps each ref to the commit it names. A commit names itself.
let commits: Record<string, string> = {};
const commitOf = (ref: string): string => commits[ref] ?? ref;
const design = process.env.LINK_WALK_DESIGN ?? "bb";
const mountName = process.env.LINK_WALK_MOUNT ?? "board";

let browser: Browser;
let base: string;
const servers: Server[] = [];

// fakeRepo writes the fixture's two revisions, starts the fake GitHub over them, and returns its
// addresses. 1.1.3 renames the net SIG to DATA, so the two revisions hold different files.
async function fakeRepo(): Promise<{ api: string; raw: string; commits: Record<string, string> }> {
  const dir = mkdtempSync(join(tmpdir(), "agni-link-walk-"));
  const sch = readFileSync(join(repoRoot, "readers", "kicad", "testdata", "wirenet.kicad_sch"), "utf8");
  for (const [rev, text] of [
    [oldRef, sch],
    [newRef, sch.replace('"SIG"', '"DATA"')],
  ]) {
    mkdirSync(join(dir, rev, dirname(file)), { recursive: true });
    writeFileSync(join(dir, rev, file), text);
  }
  const bin = join(dir, "fakegithub");
  execFileSync("go", ["build", "-o", bin, "./tools/fakegithub"], { cwd: repoRoot, stdio: "inherit" });
  const port = await freePort();
  const answer = `http://127.0.0.1:${port}`;
  servers.push(
    await start({
      what: "fakegithub",
      command: bin,
      args: ["--addr", `127.0.0.1:${port}`, "--repo", repo, "--commit", `${oldRef}=${join(dir, oldRef)}`, "--commit", `${newRef}=${join(dir, newRef)}`],
      cwd: repoRoot,
      base: answer,
      health: answer,
    }),
  );
  return (await (await fetch(answer)).json()) as { api: string; raw: string; commits: Record<string, string> };
}

async function shot(page: Page, name: string): Promise<void> {
  if (!shots) return;
  mkdirSync(shots, { recursive: true });
  await page.screenshot({ path: join(shots, name), fullPage: true });
}

// readAt matches a source cell's ref and version. A tag names the commit it read, which the cell
// shortens to 12 digits. A commit ref is its own version, so the cell names none.
function readAt(ref: string): string {
  const quoted = ref.replace(/[.]/g, "\\.");
  return commitOf(ref) === ref ? `@${quoted}, pinned` : `@${quoted} at ${commitOf(ref).slice(0, 12)}, pinned`;
}

// revisionAt is the revision row named by ref. A link at a pinned ref names its revision by it.
function revisionAt(page: Page, ref: string) {
  return page.locator(`tr[data-revision="${ref}"]`);
}

// openRevision follows the viewer link on the revision row named by ref, after checking the row
// holds the commit ref names, and returns the address the viewer landed on once it drew a sheet.
async function openRevision(page: Page, ref: string): Promise<string> {
  const row = revisionAt(page, ref);
  await row.waitFor({ timeout: 60_000 });
  expect(await row.locator(".lb-versions").textContent()).toBe(commitOf(ref).slice(0, 12));
  expect(await row.locator("a.lb-open").getAttribute("href")).toBe(`/designs/${design}%40${encodeURIComponent(ref)}/view`);
  await row.locator("a.lb-open").click();
  await page.waitForURL((u) => u.pathname.startsWith("/mounts/") && u.searchParams.get("design") === `designs/${design}@${ref}`, { timeout: 120_000 });
  await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 120_000 });
  await shot(page, `viewer-${ref}.png`);
  return new URL(page.url()).pathname;
}

beforeAll(async () => {
  if (given) {
    base = given.replace(/\/$/, "");
  } else {
    const gh = await fakeRepo();
    commits = gh.commits;
    const port = await freePort();
    base = `http://127.0.0.1:${port}`;
    const store = mkdtempSync(join(tmpdir(), "agni-link-store-"));
    const uploads = mkdtempSync(join(tmpdir(), "agni-link-uploads-"));
    servers.push(
      await start({
        what: "agni serve (link walk)",
        command: inject("agniBin"),
        args: ["serve", "--addr", `127.0.0.1:${port}`, "--store", pathToFileURL(store).href, "--uploads", pathToFileURL(uploads).href, "--github-api", gh.api, "--github-raw", gh.raw, "--web-dir", "web"],
        cwd: repoRoot,
        base,
        health: base,
      }),
    );
  }
  browser = await launch();
}, 180_000);

afterAll(async () => {
  await browser?.close();
  for (const s of servers.reverse()) await s.stop();
});

describe("linking a design from a repo", () => {
  it(`links ${repo}'s schematic at ${oldRef}, moves it to ${newRef}, and opens both revisions`, async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base}/designs/`);
      const form = page.getByTestId("new-design");
      await form.locator('input[name="id"]').fill(design);
      await form.locator('input[name="series"]').fill("board");
      await form.locator('input[name="from"][value="link"]').check();

      // Add the repo as a mount from inside the form. The form then chooses it.
      const add = form.getByTestId("add-mount");
      await add.locator("summary").click();
      await add.locator('input[name="name"]').fill(mountName);
      await add.locator('input[name="repo"]').fill(`https://github.com/${repo}`);
      await add.locator('input[name="ref"]').fill(oldRef);
      await add.locator('button[type="submit"]').click();
      // A refusal shows in the form's message, so the poll reports that instead of an empty choice.
      const chosen = async () => (await form.locator('select[name="mount"]').inputValue()) || ((await add.locator(".mount-add-msg").count()) ? await add.locator(".mount-add-msg").textContent() : "");
      await expect.poll(chosen, { timeout: 300_000 }).toBe(mountName);

      // Walk to the schematic, one folder at a time.
      const parts = file.split("/");
      for (let i = 1; i < parts.length; i++) await form.locator(`button[data-path="${parts.slice(0, i).join("/")}"]`).click();
      await form.locator(`button[data-path="${file}"]`).click();
      expect(await form.locator('input[name="link-ref"]').inputValue()).toBe(oldRef);
      expect(await form.locator('input[name="artifact"]').inputValue()).toBe("schematic");
      await shot(page, "new-design-link.png");
      await form.locator("button.lb-submit").click();
      await page.waitForURL(`${base}/designs/${design}`, { timeout: 300_000 });

      const source = page.locator('tr[data-artifact="schematic"] .lb-source');
      await expect.poll(() => source.textContent(), { timeout: 60_000 }).toMatch(new RegExp(`^${mountName}/${file.replace(/[.]/g, "\\.")}${readAt(oldRef)}`));
      await revisionAt(page, oldRef).waitFor({ timeout: 60_000 });
      await shot(page, "design-linked.png");

      // Move the schematic to the newer ref on its row.
      await page.getByTestId("ref-schematic").fill(newRef);
      await page.getByTestId("move-schematic").click();
      await expect.poll(() => source.textContent(), { timeout: 300_000 }).toMatch(new RegExp(readAt(newRef)));
      await revisionAt(page, newRef).waitFor({ timeout: 60_000 });
      expect(await page.locator("tr[data-revision]").count()).toBe(2);
      expect(await revisionAt(page, newRef).textContent()).toMatch(/current/);
      expect(await revisionAt(page, oldRef).textContent()).not.toMatch(/current/);
      await shot(page, "design-moved.png");

      // Each revision opens in the viewer, on its own files.
      const older = await openRevision(page, oldRef);
      await page.goto(`${base}/designs/${design}`);
      const newer = await openRevision(page, newRef);
      expect(newer).not.toBe(older);
    });
  }, 900_000);
});
