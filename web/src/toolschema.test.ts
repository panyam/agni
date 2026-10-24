import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import type { DescMessage } from "@bufbuild/protobuf";
import { BytesValueSchema, DescriptorProtoSchema, FieldSchema, Int64ValueSchema, SourceCodeInfoSchema, StructSchema, UninterpretedOptionSchema, ValueSchema } from "@bufbuild/protobuf/wkt";
import { ViewerLocationSchema, ViewerPageService } from "./gen/agni/v1/viewer/page_pb.js";
import { messageSchema, readOnly, toolName, toolSchema } from "./toolschema.js";

const here = dirname(fileURLToPath(import.meta.url));
const fixture = JSON.parse(readFileSync(join(here, "..", "..", "internal", "viewerrooms", "testdata", "viewer_tools.json"), "utf8")) as {
  tools: { name: string; readOnly: boolean; inputSchema: unknown }[];
  messages: Record<string, unknown>;
};

// The same types internal/viewerrooms/tools_test.go puts in the fixture's messages.
const MESSAGES: DescMessage[] = [StructSchema, ValueSchema, Int64ValueSchema, BytesValueSchema, UninterpretedOptionSchema, SourceCodeInfoSchema, FieldSchema];

describe("tool schemas, twinned with internal/viewerrooms via viewer_tools.json", () => {
  it("builds every viewer tool the bridge declares, byte for byte", () => {
    const built = ViewerPageService.methods.map((m) => ({ name: toolName(m), readOnly: readOnly(m), inputSchema: toolSchema(m) }));
    expect(built.length).toBeGreaterThan(0);
    expect(built).toEqual(fixture.tools);
  });

  it("maps the kinds the viewer's messages do not use as the Go walk does", () => {
    expect(Object.keys(fixture.messages).length).toBe(MESSAGES.length);
    for (const m of MESSAGES) expect(messageSchema(m), m.typeName).toEqual(fixture.messages[m.typeName]);
  });
});

describe("messageSchema", () => {
  it("names a property by its JSON name", () => {
    const props = messageSchema(ViewerLocationSchema).properties as Record<string, unknown>;
    expect(props).toHaveProperty("traceHops");
    expect(props).not.toHaveProperty("trace_hops");
  });

  it("writes a message met inside itself as a bare object", () => {
    const props = messageSchema(DescriptorProtoSchema).properties as Record<string, { items: unknown }>;
    expect(props.nestedType.items).toEqual({ type: "object" });
  });
});
