#!/usr/bin/env bash
# Regenerate KiCad's own ERC and DRC reports for the public demo's seeds (agni issue 967).
#
# For each seed this runs kicad-cli's ERC over the schematic and, where the seed has a board, its DRC
# over the board, and writes hack/seed-triage/<seed>.kicad.oracle: one line per violation, with the
# subject agni would name for it resolved through KiCad's own netlist. TestSeedTriageAgainstKiCad reads
# those files, so CI needs no KiCad. Do not hand-edit an output; the point of the file is that we did
# not choose it.
#
# Usage: hack/kicad_oracle.sh [path/to/kicad-cli]   (make samples-oracle first)
set -euo pipefail
cli="${1:-${KICAD_CLI:-kicad-cli}}"
command -v "$cli" >/dev/null || { echo "kicad-cli not found; pass its path or set KICAD_CLI" >&2; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
version="$("$cli" version)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# <seed> <design dir> <schematic> <board, or - for none>
seeds=(
  "gateway examples/tutorial-project/designs/gateway gateway.kicad_sch gateway.kicad_pcb"
  "jetson-agx-thor-baseboard tools/samples/boards/jetson-agx-thor-baseboard jetson-agx-thor-baseboard.kicad_sch jetson-agx-thor-baseboard.kicad_pcb"
  "royalblue54l-feather tools/samples/boards/royalblue54L-feather RoyalBlue54L-Feather.kicad_sch RoyalBlue54L-Feather.kicad_pcb"
  "royalblue54l-nfc-antenna tools/samples/boards/royalblue54L-feather/nfc_antenna/RoyalBlue54L-NFC-Antenna RoyalBlue54L-NFC-Antenna.kicad_sch RoyalBlue54L-NFC-Antenna.kicad_pcb"
)

for line in "${seeds[@]}"; do
  read -r seed dir sch pcb <<<"$line"
  [ -d "$root/$dir" ] || { echo "$dir is missing; run make samples-oracle" >&2; exit 1; }
  # KiCad writes backups and caches beside what it opens, so it runs over a copy.
  rm -rf "$tmp/d"; cp -R "$root/$dir" "$tmp/d"
  "$cli" sch erc --format json --severity-all -o "$tmp/erc.json" "$tmp/d/$sch" >/dev/null
  "$cli" sch export netlist --format kicadsexpr -o "$tmp/net" "$tmp/d/$sch" >/dev/null
  drc=-
  if [ "$pcb" != - ]; then
    "$cli" pcb drc --format json --severity-all -o "$tmp/drc.json" "$tmp/d/$pcb" >/dev/null
    drc="$tmp/drc.json"
  fi
  python3 -I - "$seed" "$dir" "$sch" "$pcb" "$version" "$tmp/erc.json" "$drc" "$tmp/net" "$tmp/d" \
    >"$root/hack/seed-triage/$seed.kicad.oracle" <<'PY'
import glob, json, re, sys

seed, rel, sch, pcb, version, erc, drc, netlist, copy = sys.argv[1:]
# Warnings about the symbol and footprint libraries a machine has installed, not about the design.
NOISE = {"lib_symbol_issues", "lib_footprint_issues", "footprint_link_issues"}
# Violations between two items, whose subject is the pair, spelled as agni spells a pair finding.
PAIRED = {"clearance", "shorting_items"}

text = open(netlist).read()
pin_net = {}
for net in re.finditer(r'\(net\s*\(code "[^"]*"\)\s*\(name "((?:[^"\\]|\\.)*)"\)(.*?)\n\t\t\)', text, re.S):
    for node in re.finditer(r'\(ref "([^"]*)"\)\s*\(pin "([^"]*)"\)', net.group(2)):
        pin_net[node.groups()] = net.group(1).replace('\\"', '"')

# A power symbol is left out of the netlist, and its value names the global net it joins.
power_net = {}
for f in glob.glob(copy + "/**/*.kicad_sch", recursive=True):
    s = open(f).read()
    for m in re.finditer(r'\n\t\(symbol\s*\(lib_id "power:[^"]*"\)(.*?)\n\t\)', s, re.S):
        value = re.search(r'\(property "Value" "([^"]*)"', m.group(1))
        for ref in re.findall(r'\(property "Reference" "([^"]*)"|\(reference "([^"]*)"\)', m.group(1)):
            if value:
                power_net[ref[0] or ref[1]] = value.group(1)

def item_net(desc):
    m = re.match(r'Symbol (\S+) (?:Hidden pin|Pin) (\S+)', desc)
    if m:
        ref, pin = m.groups()
        return pin_net.get((ref, pin)) or power_net.get(ref)
    m = re.search(r'\[([^\]]*)\]', desc)
    if m and m.group(1) and m.group(1) != "<no net>":
        return m.group(1)
    return None

def subject(vtype, items):
    nets = [n for n in (item_net(i["description"]) for i in items) if n]
    if vtype in PAIRED and len(set(nets)) == 2:
        return "|".join("net:" + n for n in sorted(set(nets)))
    return "net:" + nets[0] if nets else "-"

def clean(s):
    return " ".join(s.split())

rows = []
for tool, path in (("erc", erc), ("drc", drc)):
    if path == "-":
        continue
    d = json.load(open(path))
    vs = [v for sh in d.get("sheets", []) for v in sh["violations"]] + d.get("violations", [])
    vs += [dict(v, type="unconnected_items") for v in d.get("unconnected_items", [])]
    for v in vs:
        if v["type"] in NOISE:
            continue
        rows.append("\t".join([tool, v["type"], v["severity"], subject(v["type"], v["items"]), clean(v["description"])]
                              + [clean(i["description"]) for i in v["items"]]))

print(f"# KiCad's own ERC and DRC over the {seed} seed (agni issue 967), from {rel}.")
print("# Do not hand-edit: hack/kicad_oracle.sh writes this file, and the point of it is that we did not")
print("# choose the answer. TestSeedTriageAgainstKiCad compares the seed's triage record with it.")
print("#")
print(f"# {version}")
print(f"# kicad-cli sch erc --format json --severity-all {sch}")
if pcb != "-":
    print(f"# kicad-cli pcb drc --format json --severity-all {pcb}")
print("#")
print("# Library warnings are left out. A line is one violation: tool, type, severity, the subject in")
print("# agni's spelling (a net, two nets joined by | for a pair, or - for none), then KiCad's description")
print("# and its items, tab-separated. A pin is resolved to its net through KiCad's own netlist.")
for r in sorted(rows):
    print(r)
PY
  echo "wrote hack/seed-triage/$seed.kicad.oracle ($(grep -vc '^#' "$root/hack/seed-triage/$seed.kicad.oracle") violations)"
done
