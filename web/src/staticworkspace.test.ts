import { describe, expect, it } from "vitest";
import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { FileKind, WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { staticWorkspace } from "./staticworkspace.js";

// site is what `agni site` writes for two mounts, in the protojson it writes: the gateway holds a
// design two folders down beside a folder of profiles, and the notes mount holds no design at all.
const site: Record<string, unknown> = {
  "mounts.json": {
    mounts: [
      { name: "gateway", root: "", uri: "mount://gateway" },
      { name: "notes", root: "", uri: "mount://notes" },
    ],
    prunedMounts: 0,
  },
  "dirs/gateway.json": {
    entries: [
      { name: "designs", isDir: true, format: "", uri: "mount://gateway/designs", kind: "FILE_KIND_UNSPECIFIED" },
      { name: "profiles", isDir: true, format: "", uri: "mount://gateway/profiles", kind: "FILE_KIND_UNSPECIFIED" },
      { name: "README.md", isDir: false, format: "", uri: "mount://gateway/README.md", kind: "FILE_KIND_UNSPECIFIED" },
      { name: "gateway", isDir: true, format: "", uri: "mount://gateway/designs/gateway", kind: "FILE_KIND_UNSPECIFIED" },
      { name: "gateway-rev-c.edn", isDir: false, format: "edif", uri: "mount://gateway/designs/gateway/gateway-rev-c.edn", kind: "FILE_KIND_DESIGN" },
      { name: "gateway.edn", isDir: false, format: "edif", uri: "mount://gateway/designs/gateway/gateway.edn", kind: "FILE_KIND_DESIGN" },
      { name: "can.yaml", isDir: false, format: "", uri: "mount://gateway/profiles/can.yaml", kind: "FILE_KIND_UNSPECIFIED" },
    ],
  },
  "dirs/notes.json": {
    entries: [{ name: "part.pdf", isDir: false, format: "", uri: "mount://notes/part.pdf", kind: "FILE_KIND_DATASHEET" }],
  },
};

// engine is what the page's engine answers: the browser mount a dropped design is read from, and the
// gateway, whose files the open design brought into it.
const engine: Record<string, unknown> = {
  ListMounts: {
    mounts: [
      { name: "gateway", root: "gateway", uri: "mount://gateway" },
      { name: "local", root: "local", uri: "mount://local" },
    ],
  },
  ListDir: { entries: [{ name: "board.kicad_sch", format: "kicad", uri: "mount://local/drop-1/board.kicad_sch", kind: "FILE_KIND_DESIGN" }] },
};

function client() {
  const reads: string[] = [];
  const passed: string[] = [];
  const read = async (path: string) => {
    reads.push(path);
    if (!(path in site)) throw new Error(`${path}: 404`);
    return site[path];
  };
  const next: typeof globalThis.fetch = async (input, init) => {
    const req = new Request(input, init);
    passed.push(`${req.url.split("/").pop()} ${await req.text()}`);
    return new Response(JSON.stringify(engine[req.url.split("/").pop()!] ?? {}), { headers: { "Content-Type": "application/json" } });
  };
  const transport = createConnectTransport({ baseUrl: "https://demo.example/agni/demo/", fetch: staticWorkspace(read, next) });
  return { ws: createClient(WorkspaceService, transport), reads, passed };
}

const names = (entries: { name: string }[]) => entries.map((e) => e.name);

describe("staticWorkspace", () => {
  it("lists the site's mounts and then the engine's others, and with opens leaves out a site mount holding nothing of that kind", async () => {
    const { ws } = client();
    const all = await ws.listMounts({});
    expect(names(all.mounts)).toEqual(["gateway", "notes", "local"]);
    // The site names the gateway once, with its own entry rather than the engine's.
    expect(all.mounts[0]).toMatchObject({ uri: "mount://gateway", root: "" });
    expect(all.prunedMounts).toBe(0);
    const designs = await ws.listMounts({ opens: [FileKind.DESIGN] });
    expect(names(designs.mounts)).toEqual(["gateway", "local"]);
    expect(designs.prunedMounts).toBe(1);
  });

  it("lists one folder at a time, down to a nested design, in the order the site wrote", async () => {
    const { ws } = client();
    const root = await ws.listDir({ uri: "mount://gateway" });
    expect(names(root.entries)).toEqual(["designs", "profiles", "README.md"]);
    expect(root.entries[0].isDir).toBe(true);
    expect((await ws.listDir({ uri: "mount://gateway/designs" })).entries.map((e) => e.uri)).toEqual(["mount://gateway/designs/gateway"]);
    const nested = await ws.listDir({ uri: "mount://gateway/designs/gateway" });
    expect(names(nested.entries)).toEqual(["gateway-rev-c.edn", "gateway.edn"]);
    expect(nested.entries[0]).toMatchObject({ isDir: false, format: "edif", kind: FileKind.DESIGN, uri: "mount://gateway/designs/gateway/gateway-rev-c.edn" });
  });

  it("leaves out a folder with nothing the caller opens beneath it, and keeps every file", async () => {
    const { ws } = client();
    const root = await ws.listDir({ uri: "mount://gateway", opens: [FileKind.DESIGN] });
    expect(names(root.entries)).toEqual(["designs", "README.md"]);
    expect(names((await ws.listDir({ uri: "mount://gateway/profiles", opens: [FileKind.DESIGN] })).entries)).toEqual(["can.yaml"]);
  });

  it("answers not-found for a folder a carried mount does not hold, and asks the engine about another mount", async () => {
    const { ws, passed } = client();
    const err = await ws.listDir({ uri: "mount://gateway/nowhere" }).then(
      () => undefined,
      (e: unknown) => ConnectError.from(e),
    );
    expect(err?.code).toBe(Code.NotFound);
    const local = await ws.listDir({ uri: "mount://local/drop-1", opens: [FileKind.DESIGN] });
    expect(names(local.entries)).toEqual(["board.kicad_sch"]);
    expect(passed).toEqual(['ListDir {"uri":"mount://local/drop-1","opens":["FILE_KIND_DESIGN"]}']);
  });

  it("reads each file once and passes every other request on", async () => {
    const { ws, reads, passed } = client();
    await ws.listDir({ uri: "mount://gateway" });
    await ws.listDir({ uri: "mount://gateway/designs" });
    await ws.listMounts({ opens: [FileKind.DESIGN] });
    expect(reads.sort()).toEqual(["dirs/gateway.json", "dirs/notes.json", "mounts.json"]);
    expect(passed).toEqual(['ListMounts {"opens":["FILE_KIND_DESIGN"]}']);
    await ws.listDesignFiles({ uri: "mount://gateway/designs/gateway" });
    expect(passed[1]).toBe('ListDesignFiles {"uri":"mount://gateway/designs/gateway"}');
  });
});
