#!/usr/bin/env bash
# Regenerates the ODB++ fixtures from the synthetic KiCad boards they are exported from: the
# gateway's two layouts under examples/common/designs/odb, which the odb-verification example reads,
# and the rest under readers/odb/testdata, with KiCad's own exporter (agni issue 1250). Needs kicad-cli 9 or later.
# Precision 6 writes coordinates to the nanometre, where the default rounds them to 10 microns.
# The exporter stamps the time it ran into two files of each job, so the script rewrites those
# stamps to a fixed value and a regeneration with no board change leaves the tree unchanged.
set -euo pipefail
cd "$(dirname "$0")/.."
command -v kicad-cli >/dev/null || { echo "kicad-cli not found; install KiCad 9 or later" >&2; exit 1; }
out=readers/odb/testdata
examples=examples/common/designs/odb
boards=(
  examples/tutorial-project/designs/gateway/gateway.kicad_pcb
  examples/tutorial-project/designs/gateway/gateway-rev-b.kicad_pcb
  examples/common/designs/demo-board.kicad_pcb
  examples/common/designs/mixer.kicad_pcb
  cmd/agni/testdata/conformance/drc.fires.kicad_pcb
  readers/kicad/testdata/board.kicad_pcb
)
for b in "${boards[@]}"; do
  name=$(basename "$b" .kicad_pcb)
  dir=$out
  case $name in gateway*) dir=$examples; name=$name-layout ;; esac
  rm -rf "${dir:?}/$name"
  # kicad-cli writes the board's local settings (.kicad_prl) beside it when there is none, and one in
  # the tutorial's fixture folder restamps every capture over it, so a new one is removed.
  prl="${b%.kicad_pcb}.kicad_prl"
  had_prl=0; [ -e "$prl" ] && had_prl=1
  kicad-cli pcb export odb --compression none --precision 6 -o "$dir/$name" "$b" >/dev/null
  find "$dir/$name" -type f -print0 | xargs -0 perl -pi -e \
    's/^# \d{4}-\d\d-\d\dT\d\d:\d\d:\d\d$/# 2026-01-01T00:00:00/; s/^(CREATION|SAVE)_DATE=.*/$1_DATE=20260101.000000/'
  [ "$had_prl" = 1 ] || rm -f "$prl"
  echo "wrote $dir/$name"
done

# The same board with every features file compressed, as Valor-lineage producers write a job
# (`features.Z`). compress(1) ships with macOS and is `ncompress` on Linux.
if command -v compress >/dev/null; then
  rm -rf "$out/board-compressed"
  cp -R "$out/board" "$out/board-compressed"
  find "$out/board-compressed" -name features -exec compress -f {} \;
  echo "wrote $out/board-compressed"
fi
