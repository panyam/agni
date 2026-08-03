// The datasheet-params panel's view-side types (WS9-035). The presenter fetches GetComponentParams
// on design load and pushes a PartsState; the panel renders one row per datasheet-backed component,
// each expanding into its parameter tree, and emits onLocate(refDes) when a component is clicked —
// the same locate path a finding or a coverage signal uses.
import type { GetComponentParamsResponse } from "./gen/agni/v1/webapi/checks_pb.js";
import type { PartSpec } from "./gen/agni/v1/param/param_pb.js";

// PartRow is one design component joined to its datasheet spec.
export interface PartRow {
  refDes: string;
  mpn: string;
  spec: PartSpec | undefined;
}

export interface PartsState {
  parts: PartRow[];
}

export interface PartsView {
  setState: (s: PartsState) => void;
}

export function emptyParts(): PartsState {
  return { parts: [] };
}

// partsFromResponse maps the GetComponentParams wire response into the panel's view state.
export function partsFromResponse(resp: GetComponentParamsResponse): PartsState {
  return { parts: resp.components.map((c) => ({ refDes: c.refDes, mpn: c.mpn, spec: c.spec })) };
}
