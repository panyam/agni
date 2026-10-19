// The Load page's boot test (agni issue 1072): the real load.ts against the real LoadPage.html under
// jsdom, on the server engine, so the page's wiring is exercised as a visitor's browser runs it.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";

const servedFields: Record<string, string> = {
  "{{ .Engine }}": "server",
  "{{ .WasmMaxBytes }}": "134217728",
  "{{ .Base }}": "/",
  "{{ .Host }}": "",
  "{{ .Build }}": "v-test",
};

function pageBody(): string {
  const src = readFileSync(join(process.cwd(), "templates/LoadPage.html"), "utf8");
  const m = /\{\{ define "Body" \}\}([\s\S]*?)\n\{\{ end \}\}/.exec(src);
  if (!m) throw new Error("LoadPage.html has no Body block");
  let body = m[1];
  for (const [field, value] of Object.entries(servedFields)) body = body.split(field).join(value);
  const left = /\{\{[^}]*\}\}/.exec(body);
  if (left) throw new Error(`LoadPage.html's Body has a field this test does not fill: ${left[0]}`);
  return body;
}

const REPLIES: Record<string, unknown> = {
  ListDesignFiles: { mount: "m", files: [{ path: "d/b.edn", sha256: "sha256:1" }], totalSize: "10" },
  GetDesign: { layout: "grid", sheets: [{ id: "s1", name: "Top" }] },
  Ingest: { name: "snapshots/abc", tiers: { boardRead: true, boardPersisted: false } },
};

async function boot(url: string): Promise<{ called: { method: string; body: string }[] }> {
  vi.resetModules();
  const called: { method: string; body: string }[] = [];
  (globalThis as unknown as { fetch: unknown }).fetch = async (input: unknown, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(String(input), init);
    const method = req.url.split("/").pop() ?? "";
    called.push({ method, body: await req.clone().text() });
    return new Response(JSON.stringify(REPLIES[method] ?? {}), { status: 200, headers: { "content-type": "application/json" } });
  };
  window.history.replaceState(null, "", url);
  document.body.innerHTML = pageBody();
  await import("./load.js");
  return { called };
}

afterEach(() => {
  window.localStorage.clear();
});

describe("the Load page boots", () => {
  it("reads a design's drawing and board, shows each step done and records the read", async () => {
    const info = vi.spyOn(console, "info").mockImplementation(() => {});
    const { called } = await boot("/designs/m/d/b.edn/load?verdict=v&rule=r");
    await vi.waitFor(() => expect(window.localStorage.getItem("agni.loaded")).not.toBeNull(), 5000);
    expect(document.getElementById("load-design")?.hidden).toBe(false);
    expect(document.getElementById("load-drop")?.hidden).toBe(true);
    const states = [...document.querySelectorAll<HTMLElement>("#load-steps .ld-step")].map((li) => [li.dataset.step, li.dataset.state]);
    // The server engine fetches nothing into the page, so there is no files step.
    expect(states).toEqual([["drawing", "done"], ["board", "done"]]);
    const methods = called.map((c) => c.method);
    expect(methods).toContain("GetDesign");
    expect(called.some((c) => c.method === "Ingest" && c.body.includes('"readBoard":true'))).toBe(true);
    const record = JSON.parse(window.localStorage.getItem("agni.loaded") ?? "[]") as { key: string; stamp: string }[];
    expect(record).toEqual([{ key: "server v-test mount://m/d/b.edn", stamp: "d/b.edn sha256:1" }]);
    expect(info).toHaveBeenCalledWith(expect.stringMatching(/^agni loaded mount:\/\/m\/d\/b\.edn in \d+ ms: board not persisted$/));
    info.mockRestore();
  });

  it("says why a design could not be read and offers the viewer anyway", async () => {
    REPLIES.Ingest = undefined;
    const fetchBefore = globalThis.fetch;
    try {
      vi.resetModules();
      (globalThis as unknown as { fetch: unknown }).fetch = async (input: unknown, init?: RequestInit) => {
        const req = input instanceof Request ? input : new Request(String(input), init);
        const method = req.url.split("/").pop() ?? "";
        if (method === "Ingest" && (await req.clone().text()).includes('"readBoard":true')) {
          return new Response(JSON.stringify({ code: "invalid_argument", message: "no netlist" }), { status: 400, headers: { "content-type": "application/json" } });
        }
        const reply = method === "Ingest" ? { name: "snapshots/abc" } : (REPLIES[method] ?? {});
        return new Response(JSON.stringify(reply), { status: 200, headers: { "content-type": "application/json" } });
      };
      window.history.replaceState(null, "", "/designs/m/d/b.edn/load?sheet=s1");
      document.body.innerHTML = pageBody();
      await import("./load.js");
      await vi.waitFor(() => expect(document.getElementById("load-failed")?.hidden).toBe(false), 5000);
      expect(document.getElementById("load-failed")?.textContent).toContain("no netlist");
      const anyway = document.getElementById("load-anyway") as HTMLAnchorElement;
      expect(anyway.hidden).toBe(false);
      expect(new URL(anyway.href).pathname + new URL(anyway.href).search).toBe("/designs/m/d/b.edn/view?sheet=s1&loaded=1");
      expect(window.localStorage.getItem("agni.loaded")).toBeNull();
    } finally {
      (globalThis as unknown as { fetch: unknown }).fetch = fetchBefore;
      REPLIES.Ingest = { name: "snapshots/abc", tiers: { boardRead: true, boardPersisted: false } };
    }
  });

  it("shows the drop page at the browser mount's address, and says when it keeps nothing", async () => {
    await boot("/designs/local/load/");
    await vi.waitFor(() => expect(document.querySelector("#saved-drops li")).not.toBeNull(), 5000);
    expect(document.getElementById("load-drop")?.hidden).toBe(false);
    expect(document.getElementById("load-design")?.hidden).toBe(true);
    // jsdom has no IndexedDB, which some private windows also lack, and the page says so.
    expect(document.querySelector("#saved-drops li")?.textContent).toContain("keeps no dropped files");
  });
});
