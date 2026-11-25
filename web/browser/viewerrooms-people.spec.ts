// Mission #1176's exercise: two people share one `agni serve --mcp --viewer-rooms`, each with their
// own agent and their own viewer tab on one design. ada and bob are local accounts (`agni users add`),
// so the exercise needs no identity provider outside the server. Each signs in to the viewer in their
// own browser context, and each agent signs in with `POST /signin/local`, as `agni login --with local`
// does, and sends its token on /mcp.
//
// A step that waits on a ticket is "not available" while it fails. Once it passes, it FAILS the walk
// (exercise-report.ts, waitedOn), so the PR that makes a step pass also drops its wait. make
// exercise-viewer-rooms passes the waited-on tickets GitHub reports closed as CLOSED_TICKETS, and a
// step waiting only on closed tickets must pass. The gate runs it offline, and the waits stand.
//
// Steps:
//   one reader      single-user serve on localhost, no sign-in, an agent drives a tab (a control)
//   sign in         ada and bob sign in and each draws the design, and each agent signs in on /mcp
//   join            each tab joins the design's room and its reader lets agents in
//   own tab         ada's agent lists and moves ada's tab, and bob's stays put
//   other's tab     ada's agent is refused on bob's tab though bob's switch is on
//   refusal says why  the refusal says the tab is another person's, apart from a switch left off
//   pair            bob's tab offers a pairing code, ada's agent pairs with it and moves bob's tab,
//                   bob revokes, and ada's agent's next call is refused
//   ask             a question typed in ada's tab reaches ada's agent and not bob's

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, BrowserContext, Page } from "playwright-core";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { launch } from "./browser.js";
import { showPanel } from "./panels.js";
import { freePort, start, type Server } from "./childserver.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";
import { Agent, text, type CallResult } from "./mcpagent.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const DESIGN = "kicad/hier_numeric_root.kicad_sch";
const PASSWORDS = { ada: "ada's long password", bob: "bob's long password" } as const;
type Person = keyof typeof PASSWORDS;

// The steps wait on two tickets. #1903 puts the bridge's pairing in front of the reader, and #1905
// makes a refusal on another person's tab say so. closed_tickets.sh reads the numbers off each
// waitsOn list.

let browser: Browser;
let shared: Server;
let local: Server;

async function serve(what: string, extra: string[]): Promise<Server> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  return start({
    what,
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--mount", "kicad=readers/kicad/testdata", "--web-dir", "web", "--mcp", "--viewer-rooms", ...extra],
    cwd: repoRoot,
    base,
    health: `${base}/healthz`,
  });
}

beforeAll(async () => {
  const store = pathToFileURL(mkdtempSync(join(tmpdir(), "agni-rooms-store-"))).href;
  const uploads = pathToFileURL(mkdtempSync(join(tmpdir(), "agni-rooms-uploads-"))).href;
  for (const [name, password] of Object.entries(PASSWORDS)) {
    execFileSync(inject("agniBin"), ["users", "add", name, "--store", store, "--password-stdin"], { input: `${password}\n`, cwd: repoRoot });
  }
  shared = await serve("agni serve (two people, viewer rooms)", ["--auth", "tokens", "--local-accounts", "--store", store, "--uploads", uploads]);
  local = await serve("agni serve (one reader, viewer rooms)", []);
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await shared?.stop();
  await local?.stop();
});

const closed = new Set((process.env.CLOSED_TICKETS ?? "").split(",").filter((t) => t !== "").map((t) => `#${t.replace(/^#/, "")}`));

const activeSheet = (page: Page): Promise<string> =>
  page.evaluate(() => (globalThis as unknown as { __agniViewerPage: { location(): { sheet: string } } }).__agniViewerPage.location().sheet);

// drawDesign opens the design in page and waits for it to draw.
async function drawDesign(page: Page, base: string): Promise<void> {
  await page.goto(`${base}/mounts/${DESIGN}/view?devpage`);
  await page
    .locator(".sheet-tile-name")
    .first()
    .waitFor({ state: "attached", timeout: 60_000 })
    .catch(() => {
      throw new Error(`the design drew no sheet tile at ${page.url()}`);
    });
}

// roomLog collects what page logs about its room and the relay into said.
function roomLog(page: Page, said: string[] = []): string[] {
  page.on("console", (m) => {
    if (/room|relay/i.test(m.text())) said.push(m.text().replace(/token=[\w.-]+/, "token=..."));
  });
  return said;
}

// letAgentsIn waits for the tab's room switch and turns it on. A tab shows the switch once it is in
// its design's room, so a tab that failed to join fails here, with what the page logged about it.
async function letAgentsIn(page: Page, said: string[]): Promise<void> {
  const box = page.locator("#agents-switch input[type=checkbox]");
  await box.waitFor({ state: "visible", timeout: 15_000 }).catch(() => {
    throw new Error(`the tab shows no "Let agents drive this tab" switch, so it joined no room${said.length ? `: ${said.join("; ")}` : ""}`);
  });
  await box.check();
}

// signIn signs a person in to the viewer through the sign-in form, in their own browser context, and
// returns the page, with what it logs about its room collected in said.
async function signIn(ctx: BrowserContext, who: Person, said: string[]): Promise<Page> {
  const page = await ctx.newPage();
  roomLog(page, said);
  await page.goto(`${shared.base}/mounts/${DESIGN}/view?devpage`);
  await page.waitForURL((u) => u.pathname === "/signin", { timeout: 30_000 });
  await page.locator("#username").fill(who);
  await page.locator("#password").fill(PASSWORDS[who]);
  await page.locator('form[action="/signin/local"] button[type="submit"]').click();
  await page.waitForURL((u) => u.pathname.startsWith("/mounts/"), { timeout: 60_000 });
  await page.locator(".sheet-tile-name").first().waitFor({ state: "attached", timeout: 60_000 });
  return page;
}

// agentFor signs an agent in as who, the way `agni login --with local` does.
async function agentFor(who: Person): Promise<Agent> {
  const res = await fetch(`${shared.base}/signin/local`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ username: who, password: PASSWORDS[who] }),
  });
  if (!res.ok) throw new Error(`${who}'s agent could not sign in: HTTP ${res.status} ${await res.text()}`);
  const { token } = (await res.json()) as { token: string };
  const agent = new Agent(`${shared.base}/mcp`, "session", token);
  await agent.connect();
  return agent;
}

type Listed = { page: string; allowed: boolean };
const listed = async (agent: Agent): Promise<Listed[]> => {
  const r = await agent.call("list_pages", { design: DESIGN });
  if (r.isError) throw new Error(`list_pages: ${text(r)}`);
  return JSON.parse(text(r)) as Listed[];
};

// agreedTabs asks until the agent lists want tabs that agreed, since the bridge hears of a tab's
// switch through relay events that can arrive after the click (agni issue 1798).
async function agreedTabs(agent: Agent, want: number): Promise<string[]> {
  let ids: string[] = [];
  for (const deadline = Date.now() + 10_000; Date.now() < deadline; await new Promise((r) => setTimeout(r, 100))) {
    ids = (await listed(agent)).filter((p) => p.allowed).map((p) => p.page);
    if (ids.length >= want) break;
  }
  return ids;
}

// sheets holds two sheet ids of the design, its root and IO/1, read once off a tab by hand.
const sheets: string[] = [];

async function learnSheets(page: Page): Promise<void> {
  if (sheets.length > 0) return;
  const root = await activeSheet(page);
  await showPanel(page, "overview");
  await page.locator(".sheet-tile", { hasText: "IO/1" }).click();
  await page.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO/1", null, { timeout: 30_000 });
  sheets.push(root, await activeSheet(page));
}

// otherSheet returns one of the two sheets that page is not showing.
async function otherSheet(page: Page): Promise<string> {
  const here = await activeSheet(page);
  return sheets.find((s) => s !== here) ?? "";
}

describe("two people on one server, each driving their own viewer tab", () => {
  it("keeps each person's agent to that person's tabs", async () => {
    const rows: Row[] = [];
    const passed = new Set<string>();
    const out = process.env.EXERCISE_OUT;
    if (out) mkdirSync(out, { recursive: true });

    const step = async (name: string, opts: { waitsOn?: string[]; needs?: string[] }, body: () => Promise<string>): Promise<void> => {
      const waitsOn = opts.waitsOn ?? [];
      const missing = (opts.needs ?? []).filter((s) => !passed.has(s));
      if (missing.length > 0) {
        rows.push({ step: name, design: DESIGN, status: "not available", detail: `not reached, after ${missing.join(", ")}` });
        return;
      }
      const t0 = Date.now();
      let ok = false;
      let detail: string;
      try {
        detail = await body();
        ok = true;
        passed.add(name);
      } catch (e) {
        detail = (e instanceof Error ? e.message : String(e)).split("\n")[0];
      }
      const open = waitsOn.filter((t) => !closed.has(t));
      const shut = waitsOn.filter((t) => closed.has(t));
      if (shut.length > 0) detail += ` (${shut.join(", ")} closed)`;
      rows.push({ step: name, design: DESIGN, ms: Date.now() - t0, ...waitedOn(ok, open, detail) });
    };

    const contexts: Record<Person, BrowserContext> = { ada: await browser.newContext({ viewport: { width: 1400, height: 1000 } }), bob: await browser.newContext({ viewport: { width: 1400, height: 1000 } }) };
    const tabs = {} as Record<Person, Page>;
    const agents = {} as Record<Person, Agent>;
    const ids = {} as Record<Person, string>;
    const said = {} as Record<Person, string[]>;
    let refusal = "";
    try {
      await step("one reader", {}, async () => {
        const page = await contexts.ada.newPage();
        try {
          const logged = roomLog(page);
          await drawDesign(page, local.base);
          await letAgentsIn(page, logged);
          await learnSheets(page);
          const agent = new Agent(`${local.base}/mcp`, "session");
          await agent.connect();
          const to = await otherSheet(page);
          const moved = await agent.call("viewer_show_sheet", { design: DESIGN, sheet: to });
          if (moved.isError) throw new Error(`viewer_show_sheet: ${text(moved)}`);
          await page.waitForFunction((s) => (globalThis as unknown as { __agniViewerPage: { location(): { sheet: string } } }).__agniViewerPage.location().sheet === s, to, { timeout: 30_000 });
          return "with no sign-in, an agent moved the one reader's tab";
        } finally {
          await page.close();
        }
      });

      await step("sign in", {}, async () => {
        for (const who of ["ada", "bob"] as const) {
          said[who] = [];
          tabs[who] = await signIn(contexts[who], who, said[who]);
          agents[who] = await agentFor(who);
          if (!(await agents[who].listTools()).includes("viewer_show_sheet")) throw new Error(`${who}'s agent is offered no viewer tools on /mcp`);
        }
        return "ada and bob signed in and each drew the design, and each agent signed in on /mcp";
      });

      await step("join", { needs: ["sign in"] }, async () => {
        await letAgentsIn(tabs.ada, said.ada);
        ids.ada = (await agreedTabs(agents.ada, 1))[0] ?? "";
        if (!ids.ada) throw new Error("ada's agent lists no tab that agreed after ada turned the switch on");
        await letAgentsIn(tabs.bob, said.bob);
        ids.bob = (await agreedTabs(agents.bob, 1)).find((id) => id !== ids.ada) ?? "";
        if (!ids.bob) throw new Error("bob's agent lists no tab of bob's that agreed");
        return "each tab joined the design's room and let agents in";
      });

      await step("own tab", { needs: ["join"] }, async () => {
        const seen = (await agreedTabs(agents.ada, 1)).sort();
        if (seen.join() !== ids.ada) throw new Error(`ada's agent lists ${seen.length} agreed tabs (${seen.join(", ")}), and should list ada's alone`);
        const bobBefore = await activeSheet(tabs.bob);
        const to = await otherSheet(tabs.ada);
        const moved = await agents.ada.call("viewer_show_sheet", { design: DESIGN, sheet: to, page: ids.ada });
        if (moved.isError) throw new Error(`viewer_show_sheet on ada's tab: ${text(moved)}`);
        if ((await activeSheet(tabs.ada)) !== to) throw new Error("ada's tab did not move");
        if ((await activeSheet(tabs.bob)) !== bobBefore) throw new Error("bob's tab moved too");
        return "ada's agent lists ada's tab alone and moves it, and bob's tab stays put";
      });

      await step("other's tab", { needs: ["join"] }, async () => {
        const before = await activeSheet(tabs.bob);
        const r: CallResult = await agents.ada.call("viewer_show_sheet", { design: DESIGN, sheet: await otherSheet(tabs.bob), page: ids.bob });
        if (!r.isError) throw new Error("ada's agent moved bob's tab");
        if ((await activeSheet(tabs.bob)) !== before) throw new Error("bob's tab moved, though the call was refused");
        refusal = text(r);
        return `refused: ${refusal}`;
      });

      await step("refusal says why", { waitsOn: ["#1905"], needs: ["other's tab"] }, async () => {
        if (!/another person|someone else/i.test(refusal)) throw new Error(`the refusal does not say the tab is another person's: ${refusal}`);
        return refusal;
      });

      await step("pair", { needs: ["join"] }, async () => {
        const offer = tabs.bob.locator("#agents-switch button.agents-pair");
        await offer.click({ timeout: 5_000 });
        const code = (await tabs.bob.locator('#agents-switch [data-pair-code]:not([data-pair-code=""])').getAttribute("data-pair-code", { timeout: 5_000 })) ?? "";
        const paired = await agents.ada.call("pair", { design: DESIGN, code });
        if (paired.isError) throw new Error(`pair: ${text(paired)}`);
        const to = await otherSheet(tabs.bob);
        const moved = await agents.ada.call("viewer_show_sheet", { design: DESIGN, sheet: to, page: ids.bob });
        if (moved.isError) throw new Error(`after pairing, viewer_show_sheet on bob's tab: ${text(moved)}`);
        if ((await activeSheet(tabs.bob)) !== to) throw new Error("bob's tab did not move after pairing");
        await tabs.bob.locator("#agents-switch .agents-grants button.agents-revoke").first().click({ timeout: 5_000 });
        const after = await agents.ada.call("viewer_show_sheet", { design: DESIGN, sheet: await otherSheet(tabs.bob), page: ids.bob });
        if (!after.isError) throw new Error("ada's agent still moves bob's tab after bob revoked the grant");
        return "bob paired ada's agent by code, it moved bob's tab, and its next call after the revoke was refused";
      });

      await step("ask", { needs: ["join"] }, async () => {
        const adaHears = agents.ada.call("next_request", { design: DESIGN, wait_seconds: 15 });
        const bobHears = agents.bob.call("next_request", { design: DESIGN, wait_seconds: 5 });
        await new Promise((r) => setTimeout(r, 500));
        const form = tabs.ada.locator("form.agents-ask");
        await form.locator("input[type=text]").fill("which sheet holds the IO block?");
        await form.locator("button[type=submit]").click();
        type Heard = CallResult & { structuredContent?: { event?: { name: string; data: { text: string } } } };
        const [a, b] = (await Promise.all([adaHears, bobHears])) as [Heard, Heard];
        if (a.structuredContent?.event?.data.text !== "which sheet holds the IO block?") throw new Error(`ada's agent did not hear ada's question: ${text(a)}`);
        if (b.structuredContent?.event) throw new Error(`bob's agent heard ada's question: ${b.structuredContent.event.data.text}`);
        return "ada's question reached ada's agent, and bob's agent heard nothing";
      });
    } finally {
      const report = `## Shared viewer rooms, mission #1176\n\n${table(rows)}\n`;
      process.stdout.write(`\n${report}\n`);
      if (out) writeFileSync(join(out, "viewer-rooms.md"), report);
      await contexts.ada.close();
      await contexts.bob.close();
    }
    const broken = rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`);
    expect(exitCode(rows), broken.join("\n")).toBe(0);
  }, 300_000);
});
