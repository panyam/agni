// A test's fake Connect server answers the way agni serve does: in the codec the request came in
// (agni issue 1502). Tests write requests and replies in their JSON form, and these helpers do the
// encoding. Used only by tests.
import { fromJson, toBinary, type JsonValue } from "@bufbuild/protobuf";
import { isBinary, requestJson, responseSchema } from "./wirecodec.js";

// bodyText is a request's body in its JSON form, as a string a test can search.
export async function bodyText(req: Request): Promise<string> {
  return JSON.stringify((await requestJson(req)) ?? {});
}

// reply answers req with a reply written in JSON form, encoded in req's codec against the
// procedure's response schema.
export function reply(req: Request, json: unknown, init: ResponseInit = {}): Response {
  const schema = responseSchema(new URL(req.url));
  if (isBinary(req) && schema) {
    const msg = fromJson(schema, (json ?? {}) as JsonValue, { ignoreUnknownFields: true });
    return new Response(toBinary(schema, msg), { status: 200, ...init, headers: { "content-type": "application/proto" } });
  }
  return new Response(JSON.stringify(json ?? {}), { status: 200, ...init, headers: { "content-type": "application/json" } });
}
