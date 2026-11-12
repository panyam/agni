// keyColumns and labelColumns write keys and labels in the PackedKeys and PackedLabels column
// layouts (agni issue 1532), so a test can build a PackedSheet message the way the server sends one.
// Used only by tests. packed.test.ts holds both to the Go packer's twin fixture, so they cannot drift
// from render.PackedKeys and render.PackedLabels.
import { create } from "@bufbuild/protobuf";
import { PackedKeysSchema, PackedLabelsSchema, type PackedKeys, type PackedLabels } from "./gen/agni/v1/geom/geom_packed_pb.js";

// strings interns strings for one column message. Index 0 is always "".
function strings(): { list: string[]; of: (s?: string) => number } {
  const list = [""];
  const index = new Map([["", 0]]);
  return {
    list,
    of: (s = "") => {
      let i = index.get(s);
      if (i === undefined) {
        i = list.length;
        index.set(s, i);
        list.push(s);
      }
      return i;
    },
  };
}

interface KeyInit {
  primitive: number;
  refDes?: string;
  net?: string;
  netId?: string;
  pin?: string;
  busId?: string;
  placement?: number;
}

export function keyColumns(keys: KeyInit[]): PackedKeys {
  const t = strings();
  const view = new DataView(new ArrayBuffer(keys.length * 7 * 4));
  keys.forEach((k, i) => {
    [k.primitive, t.of(k.refDes), t.of(k.net), t.of(k.netId), t.of(k.pin), t.of(k.busId), k.placement ?? 0].forEach((w, j) => view.setUint32((i * 7 + j) * 4, w, true));
  });
  return create(PackedKeysSchema, { strings: t.list, records: new Uint8Array(view.buffer) });
}

interface LabelInit {
  x?: number;
  y?: number;
  text?: string;
  height?: number;
  rotationDeg?: number;
  justify?: string;
  color?: string;
  maxWidth?: number;
  width?: number;
  group?: number;
  mirror?: boolean;
  runs?: { text: string; kind: number }[];
}

export function labelColumns(labels: LabelInit[]): PackedLabels {
  const t = strings();
  const view = new DataView(new ArrayBuffer(labels.length * 11 * 4));
  const runs: { label: number; runs: { text: string; kind: number }[] }[] = [];
  labels.forEach((l, i) => {
    const at = (j: number) => (i * 11 + j) * 4;
    [l.x, l.y, l.height, l.rotationDeg, l.maxWidth, l.width].forEach((w, j) => view.setInt32(at(j), w ?? 0, true));
    [t.of(l.text), t.of(l.justify), t.of(l.color), l.group ?? 0, l.mirror ? 1 : 0].forEach((w, j) => view.setUint32(at(6 + j), w, true));
    if (l.runs && l.runs.length > 0) runs.push({ label: i, runs: l.runs });
  });
  return create(PackedLabelsSchema, { strings: t.list, records: new Uint8Array(view.buffer), runs });
}
