// keyColumns writes keys in the PackedKeys column layout (agni issue 1532), so a test can build a
// PackedSheet message the way the server sends one. Used only by tests. packed.test.ts holds it to
// the Go packer's twin fixture, so it cannot drift from render.PackedKeys.
import { create } from "@bufbuild/protobuf";
import { PackedKeysSchema, type PackedKeys } from "./gen/agni/v1/geom/geom_packed_pb.js";

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
  const strings = [""];
  const index = new Map([["", 0]]);
  const of = (s = "") => {
    let i = index.get(s);
    if (i === undefined) {
      i = strings.length;
      index.set(s, i);
      strings.push(s);
    }
    return i;
  };
  const view = new DataView(new ArrayBuffer(keys.length * 7 * 4));
  keys.forEach((k, i) => {
    [k.primitive, of(k.refDes), of(k.net), of(k.netId), of(k.pin), of(k.busId), k.placement ?? 0].forEach((w, j) => view.setUint32((i * 7 + j) * 4, w, true));
  });
  return create(PackedKeysSchema, { strings, records: new Uint8Array(view.buffer) });
}
