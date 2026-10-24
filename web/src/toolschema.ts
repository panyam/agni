import { ScalarType, type DescField, type DescMessage, type DescMethod } from "@bufbuild/protobuf";
import { MethodOptions_IdempotencyLevel } from "@bufbuild/protobuf/wkt";

// toolschema.ts turns a proto service's methods into the tools an agent calls, as
// internal/viewerrooms/tools.go does in Go (agni issue 1154). The bridge declares the Go walk's tools
// and refuses a tab's tool whose schema differs from that by a byte, so the two walks are held to one
// fixture, internal/viewerrooms/testdata/viewer_tools.json. The mapping is the one mcpkit#1609 sets
// out for protogen, which this file stands in for until protogen ships it.

// JSONSchema is a JSON Schema as a plain object.
export type JSONSchema = Record<string, unknown>;

// toolName is a method's tool: GetLocation is viewer_get_location.
export function toolName(method: DescMethod, prefix = "viewer_"): string {
  return prefix + method.name.replace(/[A-Z]/g, (c, i: number) => (i > 0 ? "_" : "") + c.toLowerCase());
}

// readOnly says a method is marked NO_SIDE_EFFECTS, so an agent holding only read access may call it.
export function readOnly(method: DescMethod): boolean {
  return method.idempotency === MethodOptions_IdempotencyLevel.NO_SIDE_EFFECTS;
}

// toolSchema is a tool's input schema: the request message's, with a required design beside its
// fields, which the bridge maps to the room and the tab drops before it decodes the request.
export function toolSchema(method: DescMethod): JSONSchema {
  const s = messageSchema(method.input);
  (s.properties as JSONSchema).design = { type: "string" };
  s.required = ["design"];
  return s;
}

// messageSchema is the JSON Schema of a message as protojson writes it: properties under their JSON
// names, 64-bit integers as strings, bytes as base64, enums by name, the well-known types in their
// own JSON forms, and nothing required.
export function messageSchema(desc: DescMessage): JSONSchema {
  return walk(desc, new Set());
}

// WELL_KNOWN are the types protojson writes in a JSON form of their own rather than as their fields.
const WELL_KNOWN: Record<string, () => JSONSchema> = {
  "google.protobuf.Timestamp": () => ({ type: "string", format: "date-time" }),
  "google.protobuf.Duration": () => ({ type: "string" }),
  "google.protobuf.FieldMask": () => ({ type: "string" }),
  "google.protobuf.Struct": () => ({ type: "object" }),
  "google.protobuf.Value": () => ({}),
  "google.protobuf.ListValue": () => ({ type: "array" }),
  "google.protobuf.Any": () => ({ type: "object" }),
  "google.protobuf.BoolValue": () => ({ type: "boolean" }),
  "google.protobuf.StringValue": () => ({ type: "string" }),
  "google.protobuf.BytesValue": () => ({ type: "string", contentEncoding: "base64" }),
  "google.protobuf.Int32Value": () => ({ type: "integer" }),
  "google.protobuf.UInt32Value": () => ({ type: "integer" }),
  "google.protobuf.Int64Value": () => ({ type: "string" }),
  "google.protobuf.UInt64Value": () => ({ type: "string" }),
  "google.protobuf.FloatValue": () => ({ type: "number" }),
  "google.protobuf.DoubleValue": () => ({ type: "number" }),
};

function walk(desc: DescMessage, visiting: Set<string>): JSONSchema {
  const wk = WELL_KNOWN[desc.typeName];
  if (wk) return wk();
  // A message met again inside itself is written as a bare object rather than recursed into.
  if (visiting.has(desc.typeName)) return { type: "object" };
  visiting.add(desc.typeName);
  const properties: JSONSchema = {};
  for (const f of desc.fields) properties[f.jsonName] = fieldSchema(f, visiting);
  visiting.delete(desc.typeName);
  return { type: "object", properties };
}

function fieldSchema(f: DescField, visiting: Set<string>): JSONSchema {
  switch (f.fieldKind) {
    case "map":
      return { type: "object", additionalProperties: element(f.mapKind, f.scalar, f.enum, f.message, visiting) };
    case "list":
      return { type: "array", items: element(f.listKind, f.scalar, f.enum, f.message, visiting) };
    default:
      return element(f.fieldKind, f.scalar, f.enum, f.message, visiting);
  }
}

// element is one value of a field, so a list's element or a map's value.
function element(
  kind: "scalar" | "enum" | "message",
  scalar: ScalarType | undefined,
  en: { values: readonly { name: string }[] } | undefined,
  message: DescMessage | undefined,
  visiting: Set<string>,
): JSONSchema {
  if (kind === "enum" && en) return { type: "string", enum: en.values.map((v) => v.name) };
  if (kind === "message" && message) return walk(message, visiting);
  switch (scalar) {
    case ScalarType.BOOL:
      return { type: "boolean" };
    case ScalarType.STRING:
      return { type: "string" };
    case ScalarType.BYTES:
      return { type: "string", contentEncoding: "base64" };
    case ScalarType.INT32:
    case ScalarType.SINT32:
    case ScalarType.SFIXED32:
    case ScalarType.UINT32:
    case ScalarType.FIXED32:
      return { type: "integer" };
    case ScalarType.INT64:
    case ScalarType.SINT64:
    case ScalarType.SFIXED64:
    case ScalarType.UINT64:
    case ScalarType.FIXED64:
      return { type: "string" };
    case ScalarType.FLOAT:
    case ScalarType.DOUBLE:
      return { type: "number" };
  }
  return {};
}
