#!/usr/bin/env bash
# Regenerate KiCad's own 3D placement of each part on a sample board (agni issue 1643).
#
# kicad-cli exports the board as a GLB, with each part's 3D model placed on it and named by its
# reference. This writes core/render/testdata/<board>.kicad3d.tsv: one row per part with a model, giving
# the side it sits on, the model's own height, and its outline seen from above, as the convex hull of
# its vertices in agni's board frame (millimetres, Y up). TestBodiesAgreeWithKiCad3D holds the stack
# view's part blocks to these rows, so CI needs no KiCad. Do not hand-edit an output; the point of the
# file is that we did not choose it.
#
# Usage: hack/kicad_3d_oracle.sh [path/to/kicad-cli]   (make samples-oracle first)
set -euo pipefail
cli="${1:-${KICAD_CLI:-kicad-cli}}"
command -v "$cli" >/dev/null || { echo "kicad-cli not found; pass its path or set KICAD_CLI" >&2; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
version="$("$cli" version)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# <name> <design dir> <board file>
boards=(
  "glasgow tools/samples/boards/glasgow-revC3 glasgow.kicad_pcb"
)

for line in "${boards[@]}"; do
  read -r name dir pcb <<<"$line"
  [ -d "$root/$dir" ] || { echo "$dir is missing; run make samples-oracle" >&2; exit 1; }
  # KiCad writes a .kicad_prl beside what it opens, so it runs over a copy.
  rm -rf "$tmp/d"; cp -R "$root/$dir" "$tmp/d"
  "$cli" pcb export glb -o "$tmp/b.glb" "$tmp/d/$pcb" >/dev/null 2>&1
  python3 -I - "$tmp/b.glb" "$dir/$pcb" "$version" >"$root/core/render/testdata/$name.kicad3d.tsv" <<'PY'
import json, struct, sys

# glTF is Y up in metres. KiCad's export puts board X on x, the height off the board's bottom face on
# y, and KiCad's Y, which points down the board, on z. agni's board frame is Y up, so its y is -z.

def qmul(a, b):
    ax, ay, az, aw = a; bx, by, bz, bw = b
    return (aw*bx + ax*bw + ay*bz - az*by, aw*by - ax*bz + ay*bw + az*bx,
            aw*bz + ax*by - ay*bx + az*bw, aw*bw - ax*bx - ay*by - az*bz)

def transform(n):
    if 'matrix' in n:
        m = n['matrix']
        return lambda v: (m[0]*v[0] + m[4]*v[1] + m[8]*v[2] + m[12],
                          m[1]*v[0] + m[5]*v[1] + m[9]*v[2] + m[13],
                          m[2]*v[0] + m[6]*v[1] + m[10]*v[2] + m[14])
    t = n.get('translation', [0, 0, 0]); q = n.get('rotation', [0, 0, 0, 1]); s = n.get('scale', [1, 1, 1])
    def f(v):
        p = qmul(qmul(q, (v[0]*s[0], v[1]*s[1], v[2]*s[2], 0.0)), (-q[0], -q[1], -q[2], q[3]))
        return (p[0] + t[0], p[1] + t[1], p[2] + t[2])
    return f

glb, source, version = sys.argv[1], sys.argv[2], sys.argv[3]
d = open(glb, 'rb').read()
jlen = struct.unpack('<I', d[12:16])[0]
j = json.loads(d[20:20 + jlen])
binary = d[20 + jlen + 8:]
nodes, meshes, acc, views = j['nodes'], j.get('meshes', []), j['accessors'], j['bufferViews']

def vertices(a):
    """The accessor's VEC3 float positions."""
    if a.get('componentType') != 5126 or a.get('type') != 'VEC3':
        raise SystemExit(f'unexpected position accessor {a}')
    v = views[a['bufferView']]
    base = v.get('byteOffset', 0) + a.get('byteOffset', 0)
    stride = v.get('byteStride', 12)
    return [struct.unpack_from('<3f', binary, base + i * stride) for i in range(a['count'])]

def points(i, chain, out):
    nd = nodes[i]
    chain = chain + [transform(nd)]
    if 'mesh' in nd:
        for p in meshes[nd['mesh']]['primitives']:
            for c in vertices(acc[p['attributes']['POSITION']]):
                for f in reversed(chain):
                    c = f(c)
                out.append(c)
    for ch in nd.get('children', []):
        points(ch, chain, out)

def hull(pts):
    """The convex hull of 2D points, counter-clockwise, by Andrew's monotone chain."""
    pts = sorted(set(pts))
    if len(pts) < 3:
        return pts
    def cross(o, a, b):
        return (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0])
    lo, hi = [], []
    for p in pts:
        while len(lo) >= 2 and cross(lo[-2], lo[-1], p) <= 0:
            lo.pop()
        lo.append(p)
    for p in reversed(pts):
        while len(hi) >= 2 and cross(hi[-2], hi[-1], p) <= 0:
            hi.pop()
        hi.append(p)
    return lo[:-1] + hi[:-1]

parts, board = {}, []
for root in j['scenes'][j.get('scene', 0)]['nodes']:
    for ch in nodes[root].get('children', []):
        name = nodes[ch].get('name', '')
        pts = []
        points(ch, [transform(nodes[root])], pts)
        if name and not name.startswith('=>'):
            parts.setdefault(name, []).extend(pts)
        else:
            board.extend(pts)
top = max(p[1] for p in board) * 1000 if board else 0.0

print(f'# KiCad {version}, kicad-cli pcb export glb of {source}, by hack/kicad_3d_oracle.sh')
print('# ref, side, the model\'s own height in mm, and its outline seen from above as x,y pairs in mm')
for ref in sorted(parts):
    pts = parts[ref]
    zs = [p[1] * 1000 for p in pts]
    back = max(zs) <= top / 2
    # The model's own height. KiCad seats a model on its pad, some 0.085 mm off a board body that
    # leaves out the copper and the mask, so a height measured from the face would not compare with
    # a part's stated height.
    height = max(zs) - min(zs)
    ring = hull([(round(p[0] * 1000, 3), round(-p[2] * 1000, 3)) for p in pts])
    print(f'{ref}\t{"back" if back else "front"}\t{height:.3f}\t' + ' '.join(f'{x:.3f},{y:.3f}' for x, y in ring))
PY
  echo "wrote core/render/testdata/$name.kicad3d.tsv ($(grep -vc '^#' "$root/core/render/testdata/$name.kicad3d.tsv") parts)"
done
