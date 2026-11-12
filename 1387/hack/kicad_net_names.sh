#!/usr/bin/env bash
# Regenerate KiCad's own net names for the KiCad reader's schematic fixtures (agni issue 1019).
#
# For every readers/kicad/testdata/*.kicad_sch that no other fixture includes as a sub-sheet, this
# asks kicad-cli for the netlist and writes <stem>.netnames beside it: one line per net, its name and
# the ref.pin pairs on it. TestFixtureNetNamesAreKiCads compares the reader against those files, so
# CI needs no KiCad. Do not hand-edit an output; the point of the file is that we did not choose it.
#
# Usage: hack/kicad_net_names.sh [path/to/kicad-cli]
set -euo pipefail
cli="${1:-${KICAD_CLI:-kicad-cli}}"
command -v "$cli" >/dev/null || { echo "kicad-cli not found; pass its path or set KICAD_CLI" >&2; exit 1; }
dir="$(cd "$(dirname "$0")/../readers/kicad/testdata" && pwd)"
version="$("$cli" version)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# A file another fixture names as a Sheetfile is a sub-sheet, read only through its root.
children="$(grep -ho '(property "Sheetfile" "[^"]*"' "$dir"/*.kicad_sch | sed 's/.*"Sheetfile" "\(.*\)"/\1/' | sort -u)"

# sch.kicad_sch places its symbols with no (at ...), so every pin sits at the origin and KiCad and
# the reader disagree about connectivity rather than about names.
skip="sch.kicad_sch"

for sch in "$dir"/*.kicad_sch; do
  base="$(basename "$sch")"
  if grep -qx "$base" <<<"$children" || grep -qx "$base" <<<"$skip"; then continue; fi
  if ! "$cli" sch export netlist --format kicadsexpr -o "$tmp/out.net" "$sch" >/dev/null 2>&1; then
    echo "skip $base: kicad-cli could not export it" >&2; continue
  fi
  out="$dir/${base%.kicad_sch}.netnames"
  {
    echo "# KiCad's own net names for $base, from \`kicad-cli sch export netlist\` ($version)."
    echo "# Regenerate with hack/kicad_net_names.sh; do not hand-edit. Each line is a net name, a tab,"
    echo "# then the ref.pin pairs on it, so the test can pair our nets with KiCad's by their pins."
    awk '
      /^\t\(nets/ { innets = 1; next }
      !innets { next }
      /^\t\t\(net$/ { if (name != "") print name "\t" pins; name = ""; pins = ""; next }
      /^\t\t\t\(name "/ { sub(/^\t\t\t\(name "/, ""); sub(/"\)$/, ""); name = $0; next }
      /^\t\t\t\t\(ref "/ { ref = $0; sub(/^\t\t\t\t\(ref "/, "", ref); sub(/"\)$/, "", ref); next }
      /^\t\t\t\t\(pin "/ { p = $0; sub(/^\t\t\t\t\(pin "/, "", p); sub(/"\)$/, "", p); pins = pins (pins == "" ? "" : " ") ref "." p; next }
      END { if (name != "") print name "\t" pins }
    ' "$tmp/out.net" | LC_ALL=C sort
  } > "$out"
  echo "wrote $(basename "$out") ($(grep -vc '^#' "$out") nets)"
done
