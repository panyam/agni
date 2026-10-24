// An agent drives viewer tabs through `agni serve --mcp --viewer-rooms` (agni issue 1154). The unit
// tests hold the tab's tools and switch apart from a relay (viewerroom.test.ts), and the Go tests run
// the relay and the bridge with Go pages (internal/viewerrooms). This spec is where a real tab, the
// served shell's switch, the relay's WebSocket under /relay/ and an agent on /mcp meet, on both MCP
// wires.
//
// Three tabs: A1 and A2 on one design, B1 on another. A2 and B1 turn the switch on and A1 does not.

import { afterAll, beforeAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { launch } from "./browser.js";
import { freePort, start, type Server } from "./childserver.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const DESIGN_A = "kicad/hier_numeric_root.kicad_sch";
const DESIGN_B = "kicad/hier_bus_root.kicad_sch";

let browser: Browser;
let server: Server;

beforeAll(async () => {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  server = await start({
    what: "agni serve (viewer rooms)",
    command: inject("agniBin"),
    args: ["serve", "--addr", `127.0.0.1:${port}`, "--mount", "kicad=readers/kicad/testdata", "--web-dir", "web", "--mcp", "--viewer-rooms"],
    cwd: repoRoot,
    base,
    health: base,
  });
  browser = await launch();
}, 120_000);

afterAll(async () => {
  await browser?.close();
  await server?.stop();
});

type CallResult = { isError?: boolean; content?: { text?: string }[]; structuredContent?: { location?: { mount: string; path: string; sheet: string } } };

// Agent is an MCP client on one wire, spoken by hand so the spec needs no SDK: "session" initializes
// and sends its session id, "stateless" (the 2026-07-28 wire) sends the protocol version on every
// request instead.
class Agent {
  private session = "";
  private id = 0;
  constructor(
    private readonly url: string,
    readonly wire: "session" | "stateless",
  ) {}

  async connect(): Promise<void> {
    if (this.wire === "stateless") return;
    const res = await this.post("initialize", { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "spec-agent", version: "0" } });
    this.session = res.headers.get("mcp-session-id") ?? "";
    expect(this.session, "the server gave the session no id").not.toBe("");
    await this.post("notifications/initialized", undefined, true);
  }

  async listTools(): Promise<string[]> {
    const r = (await this.rpc("tools/list", {})) as { tools: { name: string }[] };
    return r.tools.map((t) => t.name);
  }

  async call(name: string, args: Record<string, unknown>): Promise<CallResult> {
    return (await this.rpc("tools/call", { name, arguments: args })) as CallResult;
  }

  private async rpc(method: string, params: Record<string, unknown>): Promise<unknown> {
    const res = await this.post(method, params);
    const body = await res.text();
    // A streamable HTTP answer is JSON or one SSE event carrying it.
    const json = res.headers.get("content-type")?.includes("text/event-stream")
      ? body.split("\n").filter((l) => l.startsWith("data:")).map((l) => l.slice(5).trim()).pop() ?? ""
      : body;
    const msg = JSON.parse(json) as { result?: unknown; error?: { message: string } };
    if (msg.error) throw new Error(`${method}: ${msg.error.message}`);
    return msg.result;
  }

  private async post(method: string, params: Record<string, unknown> | undefined, notification = false): Promise<Response> {
    const headers: Record<string, string> = { "content-type": "application/json", accept: "application/json, text/event-stream" };
    if (this.session) headers["mcp-session-id"] = this.session;
    let p = params;
    if (this.wire === "stateless") {
      // The stateless wire routes on headers as well as the body (SEP-2243).
      headers["mcp-protocol-version"] = "2026-07-28";
      headers["mcp-method"] = method;
      if (method === "tools/call" && typeof params?.name === "string") headers["mcp-name"] = params.name;
      p = {
        ...params,
        _meta: {
          "io.modelcontextprotocol/protocolVersion": "2026-07-28",
          "io.modelcontextprotocol/clientInfo": { name: "spec-agent", version: "0" },
          "io.modelcontextprotocol/clientCapabilities": {},
        },
      };
    }
    const body = notification ? { jsonrpc: "2.0", method } : { jsonrpc: "2.0", id: ++this.id, method, params: p };
    const res = await fetch(this.url, { method: "POST", headers, body: JSON.stringify(body) });
    if (!res.ok && res.status !== 202) throw new Error(`${method}: HTTP ${res.status} ${await res.text()}`);
    return res;
  }
}

const text = (r: CallResult): string => (r.content ?? []).map((c) => c.text ?? "").join("");

async function openTab(design: string, agents: boolean): Promise<Page> {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  await page.goto(`${server.base}/designs/${design}/view?devpage`);
  await page.locator(".sheet-tile-name").first().waitFor({ timeout: 60_000 });
  // The switch shows once the tab is in its design's room.
  const box = page.locator("#agents-switch input[type=checkbox]");
  await box.waitFor({ state: "visible", timeout: 30_000 });
  expect(await box.isChecked(), "a new tab starts with agents kept out").toBe(false);
  if (agents) await box.check();
  return page;
}

const activeTile = (page: Page): Promise<string> => page.locator(".sheet-tile.active .sheet-tile-name").textContent().then((t) => t ?? "");

describe.each(["session", "stateless"] as const)("an agent on the %s wire", (wire) => {
  let tabs: Record<string, Page>;

  beforeAll(async () => {
    tabs = { A1: await openTab(DESIGN_A, false), A2: await openTab(DESIGN_A, true), B1: await openTab(DESIGN_B, true) };
  }, 180_000);

  afterAll(async () => {
    for (const p of Object.values(tabs ?? {})) await p.close();
  });

  it("drives the tab of the design it names, and only a tab whose reader let it", async () => {
    const agent = new Agent(`${server.base}/mcp`, wire);
    await agent.connect();
    expect(await agent.listTools()).toEqual(expect.arrayContaining(["viewer_get_location", "viewer_show_sheet", "viewer_locate_verdict", "list_pages"]));

    const a = await agent.call("viewer_get_location", { design: DESIGN_A });
    expect(a.isError, text(a)).toBeFalsy();
    expect(a.structuredContent?.location).toMatchObject({ mount: "kicad", path: "hier_numeric_root.kicad_sch" });
    const b = await agent.call("viewer_get_location", { design: DESIGN_B });
    expect(b.isError, text(b)).toBeFalsy();
    expect(b.structuredContent?.location).toMatchObject({ mount: "kicad", path: "hier_bus_root.kicad_sch" });

    // A sheet id, read off A1 by hand, for the agent to open in A2.
    const rootTile = await activeTile(tabs.A1);
    await tabs.A1.locator(".sheet-tile", { hasText: "IO/1" }).click();
    await tabs.A1.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO/1", null, { timeout: 30_000 });
    const ioSheet = await tabs.A1.evaluate(() => (globalThis as unknown as { __agniViewerPage: { location(): { sheet: string } } }).__agniViewerPage.location().sheet);
    await tabs.A1.locator(".sheet-tile", { hasText: rootTile }).click();
    await tabs.A1.waitForFunction((t) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === t, rootTile, { timeout: 30_000 });

    const moved = await agent.call("viewer_show_sheet", { design: DESIGN_A, sheet: ioSheet });
    expect(moved.isError, text(moved)).toBeFalsy();
    expect(moved.structuredContent?.location?.sheet).toBe(ioSheet);
    await tabs.A2.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO/1", null, { timeout: 30_000 });
    expect(await activeTile(tabs.A1), "A1 kept agents out, so it stays where its reader left it").toBe(rootTile);
    expect(new URL(tabs.A2.url()).searchParams.get("sheet") ?? tabs.A2.url(), "the tab's address follows the move").toContain(ioSheet);

    // A sheet the design lacks comes back as the page's own error, with its code.
    const bad = await agent.call("viewer_show_sheet", { design: DESIGN_A, sheet: "no-such-sheet" });
    expect(bad.isError).toBe(true);
    expect(text(bad)).toContain("InvalidArgument");

    // Turning A2's switch off leaves design A with no tab that agreed.
    await tabs.A2.locator("#agents-switch input[type=checkbox]").uncheck();
    await expect.poll(async () => (await agent.call("viewer_get_location", { design: DESIGN_A })).isError, { timeout: 10_000 }).toBe(true);
    await tabs.A2.locator("#agents-switch input[type=checkbox]").check();
  }, 120_000);
});
