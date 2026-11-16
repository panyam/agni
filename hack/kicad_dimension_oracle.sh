#!/usr/bin/env bash
# Regenerate KiCad's own pad centres and board extent for the dimension oracle (agni issue 133).
#
# kicad-cli exports each board's IPC-D-356 netlist, which states every pad's centre and side, and its
# GenCAD file, whose $BOARD section states the outline. IPC-D-356 measures from the board's auxiliary
# (drill and place) origin, so GenCAD is asked for the same frame and to state that origin, and the
# rows are shifted back by it. Glasgow's origin sits at (50, 120) mm. This writes
# core/render/testdata/<board>.kicaddim.tsv in agni's board frame (millimetres, Y up), with an
# `extent` row for the outline's bounds and a `pad` row per pad. TestBoardPadsSitWhereKiCadPutsThem
# holds the packed board to these rows, so CI needs no KiCad. Do not hand-edit an output; the point
# of the file is that we did not choose it.
#
# Usage: hack/kicad_dimension_oracle.sh [path/to/kicad-cli]   (make samples-oracle first, for Glasgow)
set -euo pipefail
cli="${1:-${KICAD_CLI:-kicad-cli}}"
command -v "$cli" >/dev/null || { echo "kicad-cli not found; pass its path or set KICAD_CLI" >&2; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
version="$("$cli" version)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# <name> <design dir> <board file>
boards=(
  "dimensions core/render/testdata dimensions.kicad_pcb"
  "glasgow tools/samples/boards/glasgow-revC3 glasgow.kicad_pcb"
)

for line in "${boards[@]}"; do
  read -r name dir pcb <<<"$line"
  [ -f "$root/$dir/$pcb" ] || { echo "$dir/$pcb is missing; run make samples-oracle" >&2; exit 1; }
  # KiCad writes a .kicad_prl beside what it opens, so it runs over a copy of the board alone.
  rm -rf "$tmp/d"; mkdir "$tmp/d"; cp "$root/$dir/$pcb" "$tmp/d/"
  "$cli" pcb export ipcd356 -o "$tmp/b.d356" "$tmp/d/$pcb" >/dev/null 2>&1
  "$cli" pcb export gencad --use-drill-origin --store-origin-coord -o "$tmp/b.cad" "$tmp/d/$pcb" >/dev/null 2>&1
  python3 -I - "$tmp/b.d356" "$tmp/b.cad" "$dir/$pcb" "$version" >"$root/core/render/testdata/$name.kicaddim.tsv" <<'PY'
import math, re, sys

d356, cad, source, version = sys.argv[1:5]
INCH = 25.4

# IPC-D-356 states coordinates in a unit its header names. KiCad writes "UNITS CUST 0", which is
# inches at 0.0001, so a pad centre is good to 2.54 micrometres. KiCad's Y points down the board, and
# the export negates it, so the file is already Y up like agni's board frame.
scale = None
pads = []
for line in open(d356, encoding='latin-1'):
    if line.startswith('P  UNITS'):
        u = line.split()
        if u[2:4] == ['CUST', '0'] or u[2:4] == ['CUST', '2']:
            scale = INCH / 10000
        elif u[2:4] == ['CUST', '1']:
            scale = 0.001
        else:
            raise SystemExit(f'{d356}: unknown units {line.strip()}')
    if not (line.startswith('317') or line.startswith('327')):
        continue
    if scale is None:
        raise SystemExit(f'{d356}: a pad record comes before the units')
    ref, pin = line[20:26].strip(), line[27:31].strip()
    if ref == 'VIA' and pin == '':
        continue  # a via, which the oracle does not hold
    m = re.search(r'A(\d\d)X([+-]?\d+)Y([+-]?\d+)', line)
    if not m:
        raise SystemExit(f'{d356}: no access or position in {line.rstrip()}')
    # The access code is a layer number. 00 is a through pad and 01 the top. KiCad puts surface
    # pads on the outer layers only, so any other number is the bottom, the board's last layer.
    side = {'00': 'through', '01': 'top'}.get(m.group(1), 'bottom')
    pads.append((ref, pin, side, int(m.group(2)) * scale, int(m.group(3)) * scale))

# GenCAD's $BOARD section draws the outline as LINE, ARC and CIRCLE records, in inches.
units, origin, xs, ys, inboard = None, None, [], [], False
def take(x, y):
    xs.append(x); ys.append(y)
for line in open(cad, encoding='latin-1'):
    w = line.split()
    if not w:
        continue
    if w[0] == 'UNITS':
        units = w[1]
    elif w[0] == 'ORIGIN':
        # The auxiliary origin, negated, in agni's frame: a point in either file is its row minus this.
        origin = (float(w[1]) * INCH, float(w[2]) * INCH)
    elif w[0] == '$BOARD':
        inboard = True
    elif w[0] == '$ENDBOARD':
        inboard = False
    elif inboard and w[0] == 'LINE':
        x1, y1, x2, y2 = map(float, w[1:5]); take(x1, y1); take(x2, y2)
    elif inboard and w[0] == 'CIRCLE':
        cx, cy, r = map(float, w[1:4]); take(cx - r, cy - r); take(cx + r, cy + r)
    elif inboard and w[0] == 'ARC':
        # ARC x1 y1 x2 y2 xc yc runs counter-clockwise from the first point to the second.
        x1, y1, x2, y2, xc, yc = map(float, w[1:7])
        take(x1, y1); take(x2, y2)
        r = math.hypot(x1 - xc, y1 - yc)
        a1, a2 = math.atan2(y1 - yc, x1 - xc), math.atan2(y2 - yc, x2 - xc)
        sweep = (a2 - a1) % (2 * math.pi) or 2 * math.pi
        for k in range(4):
            a = k * math.pi / 2
            if (a - a1) % (2 * math.pi) <= sweep:
                take(xc + r * math.cos(a), yc + r * math.sin(a))
if units != 'INCH':
    raise SystemExit(f'{cad}: GenCAD units {units}, want INCH')
if not xs:
    raise SystemExit(f'{cad}: no $BOARD outline')
if origin is None:
    raise SystemExit(f'{cad}: no ORIGIN in the header')
ox, oy = origin

print(f'# {source}: pad centres from IPC-D-356 and the outline from GenCAD, by {version}')
print('# extent\tmin x\tmin y\tmax x\tmax y (mm, Y up)')
print('# pad\tref\tpin\tside\tx\ty (mm, Y up)')
print(f'extent\t{min(xs)*INCH-ox:.4f}\t{min(ys)*INCH-oy:.4f}\t{max(xs)*INCH-ox:.4f}\t{max(ys)*INCH-oy:.4f}')
for ref, pin, side, x, y in sorted(pads, key=lambda p: (p[0], p[1], p[3], p[4])):
    print(f'pad\t{ref}\t{pin}\t{side}\t{x-ox:.4f}\t{y-oy:.4f}')
PY
  echo "$name: $(grep -c '^pad' "$root/core/render/testdata/$name.kicaddim.tsv") pads"
done

# The fixture also goes through KiCad's IPC-2581 writer, so the same rows hold agni's IPC-2581 read
# of the same board (TestIPC2581PadsSitWhereKiCadPutsThem). The export is KiCad's writing of a board
# we authored, so it is committed beside the fixture.
rm -rf "$tmp/d"; mkdir "$tmp/d"; cp "$root/core/render/testdata/dimensions.kicad_pcb" "$tmp/d/"
"$cli" pcb export ipc2581 --version C --units mm -o "$tmp/d/dimensions.xml" "$tmp/d/dimensions.kicad_pcb" >/dev/null 2>&1
# The writer stamps the time and the path it read, and neither is ours to commit.
sed -E -e 's#(timeStamp|dateTime)="[^"]*"#\1=""#g' -e "s#$tmp/d/##g" "$tmp/d/dimensions.xml" \
  >"$root/core/render/testdata/dimensions.ipc2581.xml"
echo "dimensions.ipc2581.xml: $(wc -c <"$root/core/render/testdata/dimensions.ipc2581.xml") bytes"
