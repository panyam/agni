// The viewer's clients speak binary Connect (agni issue 1502). A few places in the page read or answer
// a request before an engine does: mountingFetch finds the design a request names, the server engine
// refuses a dropped design, and a static site answers WorkspaceService itself. Each reads a request
// through this module, which decodes either codec against the method's own schema.

import { fromBinary, fromJson, toBinary, toJson, type DescMessage, type DescService, type JsonValue, type MessageShape } from "@bufbuild/protobuf";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { DiffService } from "./gen/agni/v1/webapi/diff_pb.js";
import { LexiconService } from "./gen/agni/v1/webapi/lexicon_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";
import { SnapshotService } from "./gen/agni/v1/webapi/snapshot_pb.js";
import { TableService } from "./gen/agni/v1/webapi/tables_pb.js";
import { ViewerRoomService } from "./gen/agni/v1/webapi/viewerroom_pb.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";

const SERVICES: DescService[] = [
  CheckService,
  DesignService,
  DiffService,
  LexiconService,
  ProjectService,
  QueryService,
  ReviewService,
  SnapshotService,
  TableService,
  ViewerRoomService,
  WorkspaceService,
];

// inputs and outputs map "pkg.Service/Method" to the method's request and response schemas.
const inputs = new Map<string, DescMessage>();
const outputs = new Map<string, DescMessage>();
for (const s of SERVICES) {
  for (const m of s.methods) {
    inputs.set(`${s.typeName}/${m.name}`, m.input);
    outputs.set(`${s.typeName}/${m.name}`, m.output);
  }
}

const procedure = (url: URL): string => url.pathname.split("/").filter(Boolean).slice(-2).join("/");

// isBinary reports whether a request or response carries the binary codec.
export function isBinary(r: Request | Response): boolean {
  return (r.headers.get("content-type") ?? "").startsWith("application/proto");
}

// requestSchema returns the request schema of the procedure a URL names, from its last two path
// segments, or undefined for a procedure the viewer does not call.
export function requestSchema(url: URL): DescMessage | undefined {
  return inputs.get(procedure(url));
}

// responseSchema is requestSchema's twin for the procedure's response.
export function responseSchema(url: URL): DescMessage | undefined {
  return outputs.get(procedure(url));
}

// requestJson returns a request's body as its JSON form, whichever codec it came in. It returns
// undefined for a body it cannot read: an empty one, one that is neither codec, or a binary one for a
// procedure the viewer does not call.
export async function requestJson(req: Request): Promise<Record<string, unknown> | undefined> {
  try {
    if (isBinary(req)) {
      const schema = requestSchema(new URL(req.url));
      if (!schema) return undefined;
      const bytes = new Uint8Array(await req.clone().arrayBuffer());
      return toJson(schema, fromBinary(schema, bytes)) as Record<string, unknown>;
    }
    const v: unknown = JSON.parse(await req.clone().text());
    return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : undefined;
  } catch {
    return undefined;
  }
}

// readRequest decodes a request against a schema the caller knows, in the codec it came in. An empty
// JSON body reads as the empty message, as the server reads it.
export async function readRequest<D extends DescMessage>(req: Request, schema: D): Promise<MessageShape<D>> {
  if (isBinary(req)) return fromBinary(schema, new Uint8Array(await req.clone().arrayBuffer()));
  const text = (await req.clone().text()) || "{}";
  return fromJson(schema, JSON.parse(text) as JsonValue, { ignoreUnknownFields: true });
}

// readResponse decodes a successful response against its schema, in the codec it came in.
export async function readResponse<D extends DescMessage>(res: Response, schema: D): Promise<MessageShape<D>> {
  if (isBinary(res)) return fromBinary(schema, new Uint8Array(await res.arrayBuffer()));
  return fromJson(schema, (await res.json()) as JsonValue, { ignoreUnknownFields: true });
}

// answer replies to req with msg, in the codec req came in, so the client decodes it as it would the
// server's.
export function answer<D extends DescMessage>(req: Request, schema: D, msg: MessageShape<D>): Response {
  if (isBinary(req)) {
    return new Response(toBinary(schema, msg), { status: 200, headers: { "Content-Type": "application/proto" } });
  }
  return new Response(JSON.stringify(toJson(schema, msg)), { status: 200, headers: { "Content-Type": "application/json" } });
}
