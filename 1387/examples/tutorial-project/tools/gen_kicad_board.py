#!/usr/bin/env python3
"""Generate the gateway board's .kicad_pcb FROM its EDIF netlist, so the copper cannot drift from
the netlist it claims to implement.

One footprint per component, pads carrying the netlist's net assignments, and a routed segment per
net. Segments are 0.25mm (comfortably above the 0.127mm fabrication floor) except ONE deliberately
at 0.1mm, so the board-tier checklist item has a real defect to find rather than passing vacuously.
"""
import re, sys, pathlib, collections

src = pathlib.Path(sys.argv[1])
text = src.read_text()

cells, insts = {}, {}
for m in re.finditer(r'\(cell (\w+)\s*\(cellType GENERIC\)\s*\(designator "([^"]+)"\)(.*?)(?=\n    \(cell |\n  \(library |\Z)', text, re.S):
    cells[m.group(1)] = re.findall(r'\(port ([\w]+) \(direction (\w+)\) \(designator "([^"]+)"\)\)', m.group(3))
for m in re.finditer(r'\(instance (\w+) \(viewRef V \(cellRef (\w+) \(libraryRef PARTS\)\)\) \(designator "([^"]+)"\)((?:\s*\(property [^\n]*)*)', text):
    props = m.group(4)
    insts[m.group(1)] = {"ref": m.group(3), "cell": m.group(2),
                         "value": (re.search(r'\(property Value \(string "([^"]+)"\)\)', props) or [None, ""])[1]}
nets = collections.OrderedDict()
for m in re.finditer(r'\(net (\w+) \(joined', text):
    i, depth = m.start(), 0
    while i < len(text):
        if text[i] == '(': depth += 1
        elif text[i] == ')':
            depth -= 1
            if depth == 0: break
        i += 1
    nets[m.group(1)] = [(iid, d) for d, iid in re.findall(r'\(portRef (\w+) \(instanceRef (\w+)\)\)', text[m.start():i+1])]

pin_net = {(iid, d): n for n, cs in nets.items() for iid, d in cs}
netno = {n: i + 1 for i, n in enumerate(nets)}          # 0 is the unconnected net
FP = {"CAP": "Capacitor_SMD:C_0402", "RES": "Resistor_SMD:R_0402", "TVS": "Diode_SMD:D_SOD-323",
      "XTAL": "Crystal:Crystal_SMD_3225", "TESTPOINT": "TestPoint:TestPoint_Pad_D1.0mm",
      "CONN4": "Connector_PinHeader:J_1x04"}

out = ["(kicad_pcb", '\t(version 20240108)', '\t(generator "agni-tutorial")',
       '\t(title_block', '\t\t(title "Sample Board"))',
       '\t(layers', '\t\t(0 "F.Cu" signal)', '\t\t(2 "B.Cu" signal)', '\t\t(44 "Edge.Cuts" user)', '\t)',
       '\t(net 0 "")']
for n, i in netno.items():
    out.append(f'\t(net {i} "{n}")')
out.append('\t(gr_rect (start 0 0) (end 120 90)\n\t\t(layer "Edge.Cuts")\n\t\t(uuid "edge"))')

for idx, (iid, inf) in enumerate(insts.items()):
    x, y = 10 + (idx % 5) * 22, 12 + (idx // 5) * 18
    fp = FP.get(inf["cell"], "Package_SO:SOIC-8_3.9x4.9mm_P1.27mm")
    out.append(f'\t(footprint "{fp}"\n\t\t(layer "F.Cu")\n\t\t(uuid "fp-{inf["ref"].lower()}")\n\t\t(at {x} {y} 0)')
    out.append(f'\t\t(property "Reference" "{inf["ref"]}"\n\t\t\t(at 0 -2 0))')
    out.append(f'\t\t(property "Value" "{inf["value"] or inf["cell"]}"\n\t\t\t(at 0 2 0))')
    pads = []
    for i, (_, _, desig) in enumerate(cells[inf["cell"]]):
        net = pin_net.get((iid, desig))
        if not net:
            continue
        px = -1.0 + i * 1.0
        pads.append(f'\t\t(pad "{desig}" smd roundrect\n\t\t\t(at {px} 0)\n\t\t\t(size 0.6 0.7)\n'
                    f'\t\t\t(layers "F.Cu" "F.Paste" "F.Mask")\n'
                    f'\t\t\t(net {netno[net]} "{net}")\n\t\t\t(uuid "p-{inf["ref"].lower()}-{desig}"))')
    if pads:
        pads[-1] += ')'
        out += pads
    else:
        out[-1] += ')'

# One routed segment per net. CAN1_CANH is deliberately sub-floor so the board-tier item fails
# for a real, findable reason rather than passing because nothing was routed.
THIN = "CAN1_CANH"
for i, n in enumerate(nets):
    y = 78 + (i % 6) * 1.5
    w = 0.1 if n == THIN else 0.25
    out.append(f'\t(segment (start 5 {y}) (end {15 + i * 6} {y}) (width {w}) (layer "F.Cu") '
               f'(net {netno[n]}) (uuid "seg-{i}"))')
out.append(")")
dest = src.parent / "gateway.kicad_pcb"
dest.write_text("\n".join(out) + "\n")
print(f"wrote {dest}: {len(insts)} footprints, {len(nets)} nets, 1 sub-floor segment ({THIN})", file=sys.stderr)
