// The datasheet-params panel's view-side types (WS9-035). The presenter fetches GetComponentParams
// on design load and pushes a PartsState. The panel renders one row per datasheet-backed component,
// each expanding into its parameter tree, and a click emits onLocate(refDes), the same locate path a
// finding or a coverage signal uses.
import type { GetComponentParamsResponse } from "./gen/agni/v1/webapi/checks_pb.js";
import type { PartSpec } from "./gen/agni/v1/param/param_pb.js";

// PartRow is one design component joined to its datasheet spec.
export interface PartRow {
  refDes: string;
  mpn: string;
  spec: PartSpec | undefined;
  // corpus names where the spec came from, "project" or "shared".
  corpus: string;
}

export interface PartsState {
  parts: PartRow[];
  // corpusConnected tells "nothing was asked" from "the corpus has no spec for these parts", which
  // an empty parts list cannot say on its own.
  corpusConnected: boolean;
}

export interface PartsView {
  setState: (s: PartsState) => void;
}

export function emptyParts(): PartsState {
  return { parts: [], corpusConnected: false };
}

// partsFromResponse maps the GetComponentParams wire response into the panel's view state.
export function partsFromResponse(resp: GetComponentParamsResponse): PartsState {
  return {
    parts: resp.components.map((c) => ({ refDes: c.refDes, mpn: c.mpn, spec: c.spec, corpus: c.corpus })),
    corpusConnected: resp.corpusConnected,
  };
}
