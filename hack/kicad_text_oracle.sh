#!/usr/bin/env bash
# Regenerate KiCad's own drawing of each board text, for the board text oracle (agni issue 1366).
#
# For each board this exports every silkscreen and fab layer with kicad-cli, one layer at a time and
# at full page size, so the SVG's coordinates are the board's own in millimetres. kicad-cli wraps each
# text it strokes in <g class="stroked-text"><desc>TEXT</desc>, so each text's glyph box is the box
# around its own strokes. The output is readers/kicad/testdata/<board>.kicad.text.oracle, one line per
# text: layer, text, then the box's x0 y0 x1 y1 in mm, Y down. TestBoardTextAgainstKiCad reads those
# files, so CI needs no KiCad. Do not hand-edit an output.
#
# Usage: hack/kicad_text_oracle.sh [path/to/kicad-cli]   (make samples-oracle first, for Jetson)
set -euo pipefail
cli="${1:-${KICAD_CLI:-kicad-cli}}"
command -v "$cli" >/dev/null || { echo "kicad-cli not found; pass its path or set KICAD_CLI" >&2; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
version="$("$cli" version)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# <name> <board>
boards=(
  "board readers/kicad/testdata/board.kicad_pcb"
  "gateway examples/tutorial-project/designs/gateway/gateway.kicad_pcb"
  "jetson-agx-thor-baseboard tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb"
)
for line in "${boards[@]}"; do
  read -r name board <<<"$line"
  # kicad-cli writes a .kicad_prl beside a board it exports, so it reads a copy.
  mkdir -p "$tmp/$name"
  cp "$root/$board" "$tmp/$name/"
  copy="$tmp/$name/$(basename "$board")"
  out="$root/readers/kicad/testdata/$name.kicad.text.oracle"
  {
    echo "# kicad-cli $version, $board. layer, text, glyph box x0 y0 x1 y1 in mm, Y down."
    for layer in F.SilkS B.SilkS F.Fab B.Fab; do
      "$cli" pcb export svg --mode-single --layers "$layer" --exclude-drawing-sheet --page-size-mode 0 \
        -o "$tmp/$name/$layer.svg" "$copy" >/dev/null
      python3 -I - "$tmp/$name/$layer.svg" "$layer" <<'EOF'
import re, sys
svg, layer = sys.argv[1], sys.argv[2]
s = open(svg).read()
rows = []
for m in re.finditer(r'<g class="stroked-text"><desc>(.*?)</desc>(.*?)</g>', s, re.S):
    text = m.group(1).replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">").replace("&quot;", '"')
    nums = [float(v) for d in re.findall(r'd="([^"]+)"', m.group(2)) for v in re.findall(r"-?\d+(?:\.\d+)?", d)]
    xs, ys = nums[0::2], nums[1::2]
    if not xs:
        continue
    rows.append(f"{layer}\t{text}\t{min(xs):.3f}\t{min(ys):.3f}\t{max(xs):.3f}\t{max(ys):.3f}")
print("\n".join(sorted(rows)))
EOF
    done
  } | grep -v '^$' >"$out"
  echo "wrote $out ($(($(wc -l <"$out") - 1)) texts)"
done
