// The wire-fixture replayer (agni issues 1630 and 1633). It starts `agni serve` on the cross-probe
// project mounted as probe, replays clients/wirefixtures/fixtures.json in order through AgniClient,
// and compares each answer with the recording. It leaves out each fixture's ignore fields and each
// error's message, since a message is prose. It replays over Connect and over gRPC, as the gate's
// TestWireFixtures does for Go.
//
// AGNI_BIN names the agni binary to serve. Without it the test builds one with `go build`.

import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  createRegistry,
  fromBinary,
  fromJson,
  toJson,
  type DescFile,
  type DescMessage,
  type JsonValue,
  type Message,
  type Registry,
} from "@bufbuild/protobuf";
import { reflect, type ReflectMessage } from "@bufbuild/protobuf/reflect";
import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { Code, ConnectError, blobsMissing, snapshotUnavailable, textPosition, type AgniClient } from "../src/index.js";
import { createAgniClient, type NodeClientOptions } from "../src/node.js";
import { file_agni_v1_param_service } from "../src/gen/agni/v1/param/service_pb.js";
import { file_agni_v1_viewer_page } from "../src/gen/agni/v1/viewer/page_pb.js";
import { file_agni_v1_webapi_checks } from "../src/gen/agni/v1/webapi/checks_pb.js";
import { file_agni_v1_webapi_design_view } from "../src/gen/agni/v1/webapi/design_view_pb.js";
import { file_agni_v1_webapi_designs } from "../src/gen/agni/v1/webapi/designs_pb.js";
import { file_agni_v1_webapi_diff } from "../src/gen/agni/v1/webapi/diff_pb.js";
import { file_agni_v1_webapi_lexicon } from "../src/gen/agni/v1/webapi/lexicon_pb.js";
import { file_agni_v1_webapi_project } from "../src/gen/agni/v1/webapi/project_pb.js";
import { file_agni_v1_webapi_query } from "../src/gen/agni/v1/webapi/query_pb.js";
import { file_agni_v1_webapi_review } from "../src/gen/agni/v1/webapi/review_pb.js";
import { file_agni_v1_webapi_server } from "../src/gen/agni/v1/webapi/server_pb.js";
import { file_agni_v1_webapi_snapshot } from "../src/gen/agni/v1/webapi/snapshot_pb.js";
import { file_agni_v1_webapi_tables } from "../src/gen/agni/v1/webapi/tables_pb.js";
import { file_agni_v1_webapi_viewerroom } from "../src/gen/agni/v1/webapi/viewerroom_pb.js";
import { file_agni_v1_webapi_workspace } from "../src/gen/agni/v1/webapi/workspace_pb.js";

const repo = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const fixturesFile = join(repo, "clients/wirefixtures/fixtures.json");

interface Fixture {
  name: string;
  service: string;
  method: string;
  request: JsonValue;
  response?: JsonValue;
  error?: { code: string; details?: { type: string; value: JsonValue }[] };
  ignore?: string[];
}

const fixtures: Fixture[] = JSON.parse(readFileSync(fixturesFile, "utf8")).fixtures;

// registry holds every message the services name, with the files they import, so a fixture's
// request, answer and error details all decode by their full names.
const registry: Registry = createRegistry(
  ...withDependencies([
    file_agni_v1_webapi_checks,
    file_agni_v1_webapi_design_view,
    file_agni_v1_webapi_designs,
    file_agni_v1_webapi_diff,
    file_agni_v1_webapi_lexicon,
    file_agni_v1_webapi_project,
    file_agni_v1_webapi_query,
    file_agni_v1_webapi_review,
    file_agni_v1_webapi_server,
    file_agni_v1_webapi_snapshot,
    file_agni_v1_webapi_tables,
    file_agni_v1_webapi_viewerroom,
    file_agni_v1_webapi_workspace,
    file_agni_v1_param_service,
    file_agni_v1_viewer_page,
  ]),
);

function withDependencies(files: DescFile[]): DescFile[] {
  const seen = new Map<string, DescFile>();
  const visit = (f: DescFile) => {
    if (seen.has(f.name)) return;
    seen.set(f.name, f);
    f.dependencies.forEach(visit);
  };
  files.forEach(visit);
  return [...seen.values()];
}

let server: ChildProcess | undefined;
let baseUrl = "";
let work = "";
let log = "";

beforeAll(async () => {
  work = mkdtempSync(join(tmpdir(), "agni-ts-client-"));
  const agni = process.env.AGNI_BIN || buildAgni(join(work, "agni"));
  const port = await freePort();
  baseUrl = `http://127.0.0.1:${port}`;
  // An empty working directory, so the server serves the API alone and no workspace of the repo's.
  const cwd = mkdtempSync(join(work, "cwd-"));
  server = spawn(agni, ["serve", "--addr", `127.0.0.1:${port}`, "--mount", `probe=${join(repo, "examples/cross-probe-project")}`], {
    cwd,
    stdio: ["ignore", "pipe", "pipe"],
  });
  server.stdout?.on("data", (b) => (log += b));
  server.stderr?.on("data", (b) => (log += b));
  await healthy(baseUrl, 60_000);
});

afterAll(async () => {
  if (server && server.exitCode === null) {
    const exited = new Promise((r) => server!.once("exit", r));
    server.kill();
    await exited;
  }
  if (work) rmSync(work, { recursive: true, force: true });
});

function buildAgni(out: string): string {
  execFileSync("go", ["build", "-o", out, "./cmd/agni"], { cwd: repo, stdio: "inherit" });
  return out;
}

function freePort(): Promise<number> {
  return new Promise((res, rej) => {
    const s = createServer();
    s.once("error", rej);
    s.listen(0, "127.0.0.1", () => {
      const addr = s.address();
      s.close(() => (typeof addr === "object" && addr ? res(addr.port) : rej(new Error("no port"))));
    });
  });
}

async function healthy(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (server?.exitCode !== null) throw new Error(`agni serve exited: ${log}`);
    try {
      if ((await fetch(`${url}/healthz`)).ok) return;
    } catch {
      // Not listening yet.
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`agni serve did not answer at ${url}: ${log}`);
}

// call makes a fixture's call through the client's field for its service, such as client.design
// for agni.v1.webapi.DesignViewService.
async function call(client: AgniClient, f: Fixture): Promise<{ output: DescMessage; response?: Message; error?: ConnectError }> {
  const service = registry.getService(f.service);
  if (!service) throw new Error(`${f.name}: no service ${f.service}`);
  const method = service.methods.find((m) => m.name === f.method);
  if (!method) throw new Error(`${f.name}: ${f.service} has no rpc ${f.method}`);
  const field = service.name.replace(/Service$/, "");
  const stub = (client as unknown as Record<string, Record<string, (req: Message) => Promise<Message>>>)[
    field.charAt(0).toLowerCase() + field.slice(1)
  ];
  if (!stub) throw new Error(`${f.name}: AgniClient has no client for ${f.service}`);
  const request = fromJson(method.input, f.request, { registry });
  try {
    return { output: method.output, response: await stub[method.localName](request) };
  } catch (e) {
    return { output: method.output, error: ConnectError.from(e) };
  }
}

// comparable is a message's JSON with the ignored fields cleared.
function comparable(schema: DescMessage, m: Message, ignore: string[] = []): JsonValue {
  const r = reflect(schema, m);
  for (const p of ignore) clearPath(r, p);
  return toJson(schema, m, { registry });
}

// clearPath clears a field named by its proto name, dotted through singular messages.
function clearPath(r: ReflectMessage, path: string): void {
  const [head, ...rest] = path.split(".");
  const field = r.fields.find((f) => f.name === head);
  if (!field) return;
  if (rest.length === 0) {
    r.clear(field);
  } else if (field.fieldKind === "message" && r.isSet(field)) {
    clearPath(r.get(field), rest.join("."));
  }
}

// codeName is a code as Connect's JSON names it, invalid_argument for Code.InvalidArgument.
function codeName(code: Code): string {
  return Code[code].replace(/[A-Z]/g, (c, i) => (i > 0 ? "_" : "") + c.toLowerCase());
}

// detailReaders are the client's readers for each detail a fixture's error may carry.
const detailReaders: Record<string, (err: unknown) => unknown> = {
  "agni.v1.webapi.TextPosition": textPosition,
  "agni.v1.webapi.SnapshotUnavailable": snapshotUnavailable,
  "agni.v1.webapi.BlobsMissing": blobsMissing,
};

const transports: Record<string, NodeClientOptions> = {
  connect: { token: "wire-fixtures" },
  grpc: { protocol: "grpc", token: "wire-fixtures" },
};

test("the fixture file holds answers and errors to replay", () => {
  // A replayer that loops over an empty file passes with nothing compared.
  expect(fixtures.length).toBeGreaterThanOrEqual(7);
  expect(fixtures.filter((f) => f.error).length).toBeGreaterThanOrEqual(2);
  expect(fixtures.filter((f) => f.response).length).toBeGreaterThanOrEqual(5);
});

for (const [transport, opts] of Object.entries(transports)) {
  describe(`over ${transport}`, () => {
    let client: AgniClient;
    let replayed = 0;
    beforeAll(() => {
      client = createAgniClient(baseUrl, opts);
    });
    afterAll(() => {
      // Every fixture ran, in order, rather than the loop stopping short.
      expect(replayed).toBe(fixtures.length);
    });

    for (const f of fixtures) {
      test(f.name, async () => {
        const got = await call(client, f);
        replayed++;
        if (f.error) {
          expect(got.error, `${f.name} answered, want the error ${f.error.code}`).toBeDefined();
          const err = got.error!;
          expect(codeName(err.code)).toBe(f.error.code);
          const want = f.error.details ?? [];
          const details = err.details.filter((d): d is { type: string; value: Uint8Array } => "type" in d);
          expect(details.map((d) => d.type)).toEqual(want.map((d) => d.type));
          want.forEach((d, i) => {
            const schema = registry.getMessage(d.type);
            if (!schema) throw new Error(`${f.name}: no message ${d.type}`);
            expect(toJson(schema, fromBinary(schema, details[i].value), { registry })).toEqual(
              toJson(schema, fromJson(schema, d.value, { registry }), { registry }),
            );
            // The client's own reader finds the same detail.
            expect(detailReaders[d.type]?.(err), `no reader found ${d.type}`).toBeDefined();
          });
          return;
        }
        if (got.error) throw new Error(`${f.name} failed with ${codeName(got.error.code)}: ${got.error.message}`);
        expect(comparable(got.output, got.response!, f.ignore)).toEqual(
          comparable(got.output, fromJson(got.output, f.response!, { registry }), f.ignore),
        );
      });
    }
  });
}
