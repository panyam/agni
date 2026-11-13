// The view-side half of the highlight layer (core/render/highlight.go). A HighlightSpec selects
// components, nets, and pins and the color and alpha to draw them with, independent of how the
// sheet was rendered. The WebGL path resolves specs locally against the PackedSheet keys it
// already holds, because resolveHighlights mirrors render.HighlightPacked exactly and the
// HighlightSheet PACKED rpc would return the same indices. The SVG path has no local geometry to
// join, so the presenter fetches the HighlightSheet SVG overlay and composites it above the sheet.
// See docsite/content/architecture/web-app.md#the-render-contract.

import type { Primitive, PrimitiveKey } from "./packed.js";
import { HighlightShape } from "./gen/agni/v1/geom/geom_packed_pb.js";

export { HighlightShape };

// DEFAULT_HIGHLIGHT_COLOR matches render.DefaultHighlightColor, a saturated magenta that
// stands out against the schematic palette. Used when a spec names no color.
export const DEFAULT_HIGHLIGHT_COLOR = "#ed1cb8";

// BASE_HIGHLIGHT_COLOR and BASE_HIGHLIGHT_ALPHA draw the FIELD of findings while one of them is
// focused. The desaturated slate recedes behind the focus layer's magenta, so a reader who never
// opens the Highlight menu can tell the clicked finding from the other forty on the sheet (agni
// issue 348). A different HUE rather than a dimmer magenta, because the layers are figure and
// ground, and a same-hue dimmer layer reads as the same highlight further away.
//
// It has no Go twin. DEFAULT_HIGHLIGHT_COLOR has one, because the server falls back to it when a
// spec names no color. The client always sets this EXPLICITLY and sends it inside the spec, so a
// server constant would be dead code. C12's literal ban covers core/render/*.go only, and
// highlightstyle.ts already holds client-owned style values.
export const BASE_HIGHLIGHT_COLOR = "#6b7f99";
export const BASE_HIGHLIGHT_ALPHA = 0.55;

// PinTarget names one pin of one placed component (geom.PinRef).
export interface PinTarget {
  refDes: string;
  pin: string;
}

// HighlightSpec is the view-side shape of geom.HighlightSpec. The selection is the union of
// its components (all their symbol + pin primitives), nets (their wires), and pins. Several
// specs form independent layers with their own styles; a later spec wins where they overlap.
export interface HighlightSpec {
  components?: string[];
  nets?: string[];
  // netIds selects wires by per-instance net id (PrimitiveKey in packed.ts), so a focus can target
  // ONE of two nets that share a name (WS9). A spec listing only netIds never matches by name.
  netIds?: string[];
  // netIdNames names the nets that netIds lists, for a wire keyed by name alone. A drawing keys a
  // wire by name alone when two of the design's nets share that name (agni issue 1443), so the wire
  // lights for either net, as the server's HighlightSheet lights it. It never matches a wire that
  // carries an id, so a focus on one of two same-named nets stays on that net wherever ids join.
  netIdNames?: string[];
  // busIds selects a bus trunk/entry by source id (geom PrimitiveKey.bus_id), so a bus finding
  // highlights its own bus (WS7-042b). A bus has no net, so it matches ONLY by bus id.
  busIds?: string[];
  pins?: PinTarget[];
  color?: string; // "#rrggbb"; empty -> DEFAULT_HIGHLIGHT_COLOR
  alpha?: number; // (0,1]; unset/0 -> fully opaque (outline) / 0.3 (bounding shapes) / 0.4 (path)
  shape?: HighlightShape; // unset -> OUTLINE (the WS9-016 re-stroke behavior)
  strokeScale?: number; // PATH width and bounding-shape stroke multiplier (WS9-044, agni 1536); unset/0 -> 1
}

// FocusStyle is the part of a user highlight style that withFocusShape stamps onto the focus
// specs (WS9-044), namely the marker color, opacity, and width scale.
export interface FocusStyle {
  color: string;
  alpha: number;
  scale: number;
}

// withFocusShape stamps each focus-layer spec with an emphasis shape chosen by the subject KIND
// it carries (WS9-040). A net gets PATH, a translucent wider re-stroke of its wire path, because a
// bounding rect around a net that fans out to several pins frames everything between its
// endpoints. A component or pin keeps the bounding rect. Each focus spec is built from a single
// subject (findingSpec, entitySpecs), so the presence of net names or ids alone selects the shape.
// selectFinding and locateEntity both go through here. An optional user style (WS9-044) sets the
// color on any focus, the opacity and width scale on a PATH marker, and the width scale on a box's
// stroke (agni issue 1536). Without one the specs keep the built-in look.
export function withFocusShape(specs: HighlightSpec[], style?: FocusStyle): HighlightSpec[] {
  return specs.map((s) => {
    const isNet = (s.nets && s.nets.length > 0) || (s.netIds && s.netIds.length > 0);
    // A bus keeps OUTLINE and recolors its own trunk. It is already thick on both backends, and
    // PATH would mis-tessellate its GL triangle quads (WS7-042b).
    const isBus = s.busIds !== undefined && s.busIds.length > 0;
    const shape = isNet ? HighlightShape.PATH : isBus ? HighlightShape.OUTLINE : HighlightShape.BOUNDING_RECT;
    if (!style) return { ...s, shape };
    if (shape === HighlightShape.PATH) return { ...s, shape, color: style.color, alpha: style.alpha, strokeScale: style.scale };
    if (shape === HighlightShape.BOUNDING_RECT) return { ...s, shape, color: style.color, strokeScale: style.scale }; // a box keeps its default fill opacity
    return { ...s, shape, color: style.color };
  });
}

// ResolvedHighlight is one spec resolved against a sheet, with its normalized style and matched
// primitive indices. entities holds the same indices grouped per matched entity (one group per
// component, net, or listed pin). The bounding shapes frame each group, so a rect draws around
// one component and never around the spec's whole union.
export interface ResolvedHighlight {
  color: string;
  alpha: number;
  shape: HighlightShape; // UNSPECIFIED resolves to OUTLINE
  strokeScale: number; // PATH width and bounding-shape stroke multiplier (WS9-044, agni 1536); 1 when unset or <= 0
  primitives: Set<number>;
  entities: number[][];
  // parts says, for each entity in entities, whether it is a whole component. Only a part's frame
  // takes the span floor (frameFloor), so a picked pin keeps a small frame (agni issue 1536).
  parts: boolean[];
  // pins says, for each entity in entities, whether it is one listed pin, so WebGL can draw it the
  // dot the SVG overlay draws (pinMarkRadius).
  pins: boolean[];
  // subjects names each matched entity once in check.SubjectParam's spelling (`component:R1`,
  // `net:SDA`, `pin:U7.3`, `bus:<id>`), in the order the keys first matched it. A net matched by its
  // id is named by its name when the key carries one. The canvas writes them to data-lit, so a
  // browser spec can compare what WebGL lit with what SVG lit (agni issue 1330).
  subjects: string[];
}

// normalizeAlpha treats out-of-range/unset alpha as fully opaque, matching the server.
export function normalizeAlpha(a: number | undefined): number {
  return a === undefined || a <= 0 || a >= 1 ? 1 : a;
}

// normalizeFillAlpha is the bounding-shape rule, matching render.highlightFillAlpha. Unset or
// out-of-range means 0.3 rather than opaque, since an opaque fill would hide the entity.
export function normalizeFillAlpha(a: number | undefined): number {
  return a === undefined || a <= 0 || a >= 1 ? 0.3 : a;
}

// normalizePathAlpha is the PATH highlighter rule, matching render.highlightPathAlpha. Unset or
// out-of-range means 0.4, so the wire shows through the focus marker.
export function normalizePathAlpha(a: number | undefined): number {
  return a === undefined || a <= 0 || a >= 1 ? 0.4 : a;
}

// resolveHighlights resolves specs against a sheet's primitive keys, mirroring
// render.HighlightPacked. A listed component matches every primitive keyed to its ref_des
// (symbol graphics and pins), a listed net matches its wire primitives, and a listed pin
// matches just that (ref_des, pin) primitive. One (possibly empty) result per spec, in order.
// A sheet can place one ref-des twice, so a component or pin is one entity per placement
// (PrimitiveKey.placement) and frames where each placement sits, as the SVG overlay does (agni
// issue 1477). It is still one subject.
export function resolveHighlights(keys: PrimitiveKey[], specs: HighlightSpec[]): ResolvedHighlight[] {
  return specs.map((spec) => {
    const comps = new Set(spec.components ?? []);
    const nets = new Set(spec.nets ?? []);
    const netIds = new Set(spec.netIds ?? []);
    const netIdNames = new Set(spec.netIdNames ?? []);
    const buses = new Set(spec.busIds ?? []);
    const pins = new Set((spec.pins ?? []).map((p) => `${p.refDes} ${p.pin}`));
    const primitives = new Set<number>();
    const byEntity = new Map<string, number[]>();
    const subjects = new Set<string>();
    for (const k of keys) {
      let entity: string;
      let subject: string;
      if (k.refDes !== "" && comps.has(k.refDes)) {
        entity = `c ${k.refDes} ${k.placement ?? 0}`;
        subject = `component:${k.refDes}`;
      } else if (k.netId !== "" && netIds.has(k.netId)) {
        entity = `n ${k.netId}`; // per-instance: two same-named nets are two entities
        subject = `net:${k.net || k.netId}`;
      } else if (k.net !== "" && (nets.has(k.net) || (k.netId === "" && netIdNames.has(k.net)))) {
        entity = `n ${k.net}`;
        subject = `net:${k.net}`;
      } else if (k.busId !== "" && buses.has(k.busId)) {
        entity = `b ${k.busId}`; // a bus joins by source id, having no net (WS7-042b)
        subject = `bus:${k.busId}`;
      } else if (k.pin !== "" && pins.has(`${k.refDes} ${k.pin}`)) {
        entity = `p ${k.refDes} ${k.pin} ${k.placement ?? 0}`;
        subject = `pin:${k.refDes}.${k.pin}`;
      } else {
        continue;
      }
      subjects.add(subject);
      primitives.add(k.primitive);
      const group = byEntity.get(entity);
      if (group) {
        group.push(k.primitive);
      } else {
        byEntity.set(entity, [k.primitive]);
      }
    }
    const shape = spec.shape || HighlightShape.OUTLINE;
    // OUTLINE re-strokes opaque, PATH re-strokes translucent (WS9-040), the bounding shapes fill
    // translucent. PATH and OUTLINE both recolor the entity's primitives. They also differ by
    // width, but the GL backend cannot apply a width to 1px lines, so only their default alpha
    // differs.
    const alpha =
      shape === HighlightShape.PATH
        ? normalizePathAlpha(spec.alpha)
        : shape === HighlightShape.OUTLINE
          ? normalizeAlpha(spec.alpha)
          : normalizeFillAlpha(spec.alpha);
    return {
      color: spec.color || DEFAULT_HIGHLIGHT_COLOR,
      alpha,
      shape,
      strokeScale: spec.strokeScale && spec.strokeScale > 0 ? spec.strokeScale : 1,
      primitives,
      entities: [...byEntity.values()],
      parts: [...byEntity.keys()].map((k) => k[0] === "c"),
      pins: [...byEntity.keys()].map((k) => k[0] === "p"),
      subjects: [...subjects],
    };
  });
}

// EntityFrame is one entity's framing geometry in world coordinates. It holds the padded bbox
// for a bounding rect, and the circumscribed circle. The circle is centred on the raw bbox and
// its radius is the half-diagonal plus the pad, so it never clips the entity.
export interface EntityFrame {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  cx: number;
  cy: number;
  r: number;
}

// entityFrame pads a raw world bbox into its frame. The pad is 10% of the larger side, and at
// least minPad and 8 world units. A zero-area entity (a single pin point) frames by the floor
// alone, and 8 units of a nanometre sheet is nothing, so the caller passes minPad scaled to its
// drawing (agni issue 1374). TWINNED with render.framedBounds. The fixtures in highlights.test.ts
// and core/render/highlight_test.go assert the same numbers, so change both together.
export function entityFrame(minX: number, minY: number, maxX: number, maxY: number, minPad = 0): EntityFrame {
  const w = maxX - minX;
  const h = maxY - minY;
  const pad = Math.max(8, minPad, 0.1 * Math.max(w, h));
  return {
    minX: minX - pad,
    minY: minY - pad,
    maxX: maxX + pad,
    maxY: maxY + pad,
    cx: (minX + maxX) / 2,
    cy: (minY + maxY) / 2,
    // Explicit sqrt, not Math.hypot. JS engines do not guarantee a correctly rounded
    // Math.hypot, while mul/add/sqrt are correctly rounded in both JS and Go, so the twinned
    // fixtures can assert exact cross-backend equality.
    r: Math.sqrt((w / 2) * (w / 2) + (h / 2) * (h / 2)) + pad,
  };
}

// entityBounds unions the vertex ranges of an entity's primitives into its raw world
// bbox. The SVG side computes shape bounds from the geometry, and the packed vertices are
// that geometry tessellated, so both backends frame the same box. Null when the entity has no
// vertices.
export function entityBounds(
  vertices: Int32Array,
  primitives: Primitive[],
  entity: number[],
): { minX: number; minY: number; maxX: number; maxY: number } | null {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const i of entity) {
    const p = primitives[i];
    if (!p) continue;
    for (let v = p.firstVertex; v < p.firstVertex + p.count; v++) {
      const x = vertices[v * 2];
      const y = vertices[v * 2 + 1];
      if (x < minX) minX = x;
      if (y < minY) minY = y;
      if (x > maxX) maxX = x;
      if (y > maxY) maxY = y;
    }
  }
  return minX > maxX ? null : { minX, minY, maxX, maxY };
}

// FRAME_FLOOR_FRACTION and FRAME_STROKE_FRACTION size a part's frame against the sheet it sits on,
// as fractions of the sheet's smaller world span. The floor is the least pad around a part, so a
// small part on a dense board frames at about twice its old width. The stroke is the frame's
// outline width, before the user's strokeScale (agni issue 1536).
export const FRAME_FLOOR_FRACTION = 0.015;
export const FRAME_STROKE_FRACTION = 0.003;

// frameFloor, frameStroke and frameStrokeAlpha are twinned with the functions of the same names in
// core/render/highlight.go. highlights.test.ts and highlight_frame_test.go assert the same numbers
// for one fixture, so WebGL and SVG size a frame alike.

// frameFloor is the least world pad around a part on a sheet whose smaller world span is minSpan.
export function frameFloor(minSpan: number): number {
  return FRAME_FLOOR_FRACTION * minSpan;
}

// frameStroke is a frame's stroke width in world units, scaled by the spec's strokeScale.
export function frameStroke(minSpan: number, strokeScale: number): number {
  return FRAME_STROKE_FRACTION * minSpan * strokeScale;
}

// frameStrokeAlpha is a frame stroke's opacity for a normalized fill alpha. The default fill of 0.3
// and anything above it give an opaque stroke, and a fainter fill gives a stroke as faint in
// proportion.
export function frameStrokeAlpha(fill: number): number {
  return Math.min(1, fill / 0.3);
}

// boardNetAlpha is the opacity a net lit on the board repaints its copper at, for a normalized path
// alpha, twinned with render.boardNetAlpha (agni issue 1543). The viewer's default path alpha of 0.4
// gives opaque copper, since a translucent track over a track of the same width cannot be seen, and
// a reader who asks for a fainter highlight still gets one.
export function boardNetAlpha(path: number): number {
  return Math.min(1, path / 0.4);
}

// rectRing tessellates a stroke of the given width centred on a frame's padded bbox, as four
// quads, so the GL overlay draws the opaque outline an SVG stroke draws.
export function rectRing(f: EntityFrame, width: number): number[] {
  const h = width / 2;
  const [ox0, oy0, ox1, oy1] = [f.minX - h, f.minY - h, f.maxX + h, f.maxY + h];
  const [ix0, iy0, ix1, iy1] = [f.minX + h, f.minY + h, f.maxX - h, f.maxY - h];
  const quad = (x0: number, y0: number, x1: number, y1: number) => [x0, y0, x1, y0, x1, y1, x0, y0, x1, y1, x0, y1];
  return [...quad(ox0, oy0, ox1, iy0), ...quad(ox0, iy1, ox1, oy1), ...quad(ox0, iy0, ix0, iy1), ...quad(ix1, iy0, ox1, iy1)];
}

// circleRing tessellates a stroke of the given width centred on a frame's circle, as an annulus.
export function circleRing(f: EntityFrame, width: number): number[] {
  const ro = f.r + width / 2;
  const ri = Math.max(f.r - width / 2, 0);
  const out: number[] = [];
  for (let i = 0; i < CIRCLE_SEGMENTS; i++) {
    const a0 = (2 * Math.PI * i) / CIRCLE_SEGMENTS;
    const a1 = (2 * Math.PI * (i + 1)) / CIRCLE_SEGMENTS;
    const [c0, s0, c1, s1] = [Math.cos(a0), Math.sin(a0), Math.cos(a1), Math.sin(a1)];
    const o0 = [f.cx + ro * c0, f.cy + ro * s0];
    const o1 = [f.cx + ro * c1, f.cy + ro * s1];
    const i0 = [f.cx + ri * c0, f.cy + ri * s0];
    const i1 = [f.cx + ri * c1, f.cy + ri * s1];
    out.push(...i0, ...o0, ...o1, ...i0, ...o1, ...i1);
  }
  return out;
}

// CIRCLE_SEGMENTS is the framing circle's tessellation for the GL overlay (the SVG side
// draws a true <circle>); 48 keeps it visually round at any zoom the viewer reaches.
const CIRCLE_SEGMENTS = 48;

// rectTriangles tessellates a frame's padded bbox into two triangles (an x,y pair list)
// for the GL overlay path.
export function rectTriangles(f: EntityFrame): number[] {
  return [f.minX, f.minY, f.maxX, f.minY, f.maxX, f.maxY, f.minX, f.minY, f.maxX, f.maxY, f.minX, f.maxY];
}

// circleTriangles tessellates a frame's circumscribed circle into a triangle fan around
// its center.
export function circleTriangles(f: EntityFrame): number[] {
  const out: number[] = [];
  for (let i = 0; i < CIRCLE_SEGMENTS; i++) {
    const a0 = (2 * Math.PI * i) / CIRCLE_SEGMENTS;
    const a1 = (2 * Math.PI * (i + 1)) / CIRCLE_SEGMENTS;
    out.push(f.cx, f.cy, f.cx + f.r * Math.cos(a0), f.cy + f.r * Math.sin(a0), f.cx + f.r * Math.cos(a1), f.cy + f.r * Math.sin(a1));
  }
  return out;
}

// PIN_MARK_FRACTION sizes a lit pin's dot on a schematic as a fraction of the sheet's larger world
// span. The SVG overlay draws a lit pin as a circle of radius pinRPx*1.6 = 4 px on a drawing whose
// longer side is sheetMaxPx = 1600 px (core/render/svg.go), and 4/1600 is 0.0025. A pin's own
// primitive is a dot a pixel or two across in WebGL, so recolouring it alone left a lit pin close
// to invisible.
export const PIN_MARK_FRACTION = 0.0025;

// pinMarkRadius is a lit pin's dot radius in world units on a sheet whose larger span is maxSpan.
export function pinMarkRadius(maxSpan: number): number {
  return PIN_MARK_FRACTION * maxSpan;
}

// PATH_WIDTH_FRACTION sizes the WebGL PATH highlighter's world-space HALF-width (WS9-043) as a
// fraction of the sheet's smaller world span. Browser GL lines are always 1px, so the marker's
// width comes from filled quads (as the board copper tracks and the bounding shapes do), sized
// in world units so it scales with zoom like every other GL overlay. A fraction rather than an
// absolute value keeps it unit-agnostic. KiCad geometry is in nanometers but a reader may use
// another unit, so a fixed constant would be invisible at one scale and huge at another. The
// user's strokeScale (WS9-044) multiplies it in canvas.ts.
export const PATH_WIDTH_FRACTION = 0.0015;

// pathQuads tessellates a matched wire entity into world-space triangle quads of the given
// half-width (WS9-043), so the WebGL overlay can draw a PATH highlighter at true width instead
// of a 1px recolor. Each segment of a wire's line-strip primitives becomes two triangles offset
// perpendicular by halfWidth; segments are independent (no miter join), which reads fine for a
// translucent halo. Output is a flat x,y list in world units, matching the overlay vertex space.
export function pathQuads(vertices: Int32Array, primitives: Primitive[], entity: number[], halfWidth: number): number[] {
  const tris: number[] = [];
  for (const i of entity) {
    const p = primitives[i];
    if (!p) continue;
    const end = p.firstVertex + p.count;
    for (let v = p.firstVertex; v + 1 < end; v++) {
      const ax = vertices[v * 2];
      const ay = vertices[v * 2 + 1];
      const bx = vertices[(v + 1) * 2];
      const by = vertices[(v + 1) * 2 + 1];
      const dx = bx - ax;
      const dy = by - ay;
      const len = Math.sqrt(dx * dx + dy * dy);
      if (len === 0) continue;
      const nx = (-dy / len) * halfWidth;
      const ny = (dx / len) * halfWidth;
      // Two triangles covering the segment's offset quad. Winding is irrelevant because the
      // overlay draws unculled.
      tris.push(ax + nx, ay + ny, ax - nx, ay - ny, bx + nx, by + ny);
      tris.push(ax - nx, ay - ny, bx - nx, by - ny, bx + nx, by + ny);
    }
  }
  return tris;
}
