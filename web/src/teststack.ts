// jetsonRows is the Jetson cut's stack for tests (agni issue 1593). Used only by tests.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { fromJsonString } from "@bufbuild/protobuf";
import { BoardStackViewSchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import type { StackRowIn } from "./stacklayout.js";

// The Jetson cut's stack as the engine serves it, with each row's depth (agni issue 1635). It is the
// twin core/render's TestBoardStackTwin writes, so these tests draw at the depths render.BoardStack
// places the rows at. Silk and paste give no thickness, and the rest sum to the board's 1.711 mm.
const twin = join(dirname(fileURLToPath(import.meta.url)), "..", "..", "core", "render", "testdata", "jetson_cut_stack_twin.json");

export const jetsonRows = (): StackRowIn[] =>
  fromJsonString(BoardStackViewSchema, readFileSync(twin, "utf8")).rows.map((r) => ({
    name: r.name,
    kind: r.kind,
    layer: r.layer,
    color: r.color,
    topMm: r.topMm,
    bottomMm: r.bottomMm,
    thicknessDefaulted: r.thicknessDefaulted,
  }));
