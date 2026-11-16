// The stack view's two looks (agni issue 1653). The layer look draws each layer apart in the
// editor's colours, for questions about the stack. The board look draws the board as it is made: an
// opaque mask on each outer face, outer copper seen through it, pads in their finish, and white
// silkscreen. The inner layers, the dielectrics and the drawing aids hide.
//
// The board look reads only the board's layer rows and the packed sheet's keys, so it recolours what
// the server packed and repacks nothing.

import type { BoardLook } from "./gen/agni/v1/geom/geom_packed_pb.js";

export type StackLook = "layers" | "board";

// LookRowIn is the part of a BoardLayerRow the board look reads.
export interface LookRowIn {
  kind: string;
  side: string;
  groups: readonly number[];
}

// Face is the board face a group draws on in the board look.
export type Face = "top" | "bottom";

// BoardLookPlan says how the board look draws each packed group. color names a Style colour by its
// BoardLook field, face the board face the group draws on, and hidden the groups it hides. A group in
// none of them draws as the layer look draws it.
export interface BoardLookPlan {
  color: Map<number, keyof Omit<BoardLook, "$typeName" | "$unknown">>;
  face: Map<number, Face>;
  hidden: Set<number>;
}

// boardLookPlan plans the board look from the board's layer rows. The copper rows go first, so a
// zone group an outer copper row shares with the Zones row draws with its copper, and a zone group no
// outer row names hides with the inner layers. Through copper draws on the top face under the mask,
// as a tented via does, with its pads in their finish (padPrimitives).
export function boardLookPlan(rows: readonly LookRowIn[]): BoardLookPlan {
  const plan: BoardLookPlan = { color: new Map(), face: new Map(), hidden: new Set() };
  const place = (groups: readonly number[], face: Face, color: "maskOverCopper" | "silk") => {
    for (const g of groups) {
      if (plan.face.has(g) || plan.hidden.has(g)) continue;
      plan.face.set(g, face);
      plan.color.set(g, color);
    }
  };
  const hide = (groups: readonly number[]) => {
    for (const g of groups) if (!plan.face.has(g)) plan.hidden.add(g);
  };
  const rank = (r: LookRowIn) => (r.kind === "copper" ? 0 : r.kind === "zones" || r.kind === "pours" ? 2 : 1);
  for (const r of [...rows].sort((a, b) => rank(a) - rank(b))) {
    switch (r.kind) {
      case "copper":
        if (r.side === "front") place(r.groups, "top", "maskOverCopper");
        else if (r.side === "back") place(r.groups, "bottom", "maskOverCopper");
        else hide(r.groups);
        break;
      case "through":
        place(r.groups, "top", "maskOverCopper");
        break;
      case "silk":
        place(r.groups, r.side === "back" ? "bottom" : "top", "silk");
        break;
      case "edge":
        for (const g of r.groups) if (!plan.face.has(g)) plan.face.set(g, "top");
        break;
      default:
        // Zones on an inner layer, fab, courtyard and other drawing aids.
        hide(r.groups);
    }
  }
  return plan;
}

// KeyIn is the part of a packed key padPrimitives reads.
export interface KeyIn {
  primitive: number;
  pin: string;
}

// padPrimitives lists the primitives that draw a pad on a group the board look puts under the mask.
// A pad's key names its pin, and a track's names none, so the two tell apart without a repack.
export function padPrimitives(keys: readonly KeyIn[], groupOf: (primitive: number) => number, plan: BoardLookPlan): number[] {
  const out: number[] = [];
  for (const k of keys) {
    if (!k.pin) continue;
    if (plan.color.get(groupOf(k.primitive)) === "maskOverCopper") out.push(k.primitive);
  }
  return out;
}
