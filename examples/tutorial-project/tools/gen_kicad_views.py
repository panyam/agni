#!/usr/bin/env python3
"""Generate the gateway board's KiCad views FROM its EDIF netlist, so they cannot drift from it.

Reads gateway.edn, extracts cells/pins/instances/nets, and writes:
  symbols/gateway.kicad_sym  - one symbol per part type, pins numbered as the netlist numbers them
  gateway.kicad_sch          - instances placed on a grid, each pin stubbed and labelled

Netting is by LABEL, not by routing: every pin gets a short stub wire with a label carrying its
net name, and same-named labels union into one net. That is the real-schematic idiom for rails and
it removes the hand-routing problem entirely. Plain `label`, never `global_label`, because a global
label marks the net External and several rules deliberately skip external nets.
"""
import re, sys, pathlib, collections

src = pathlib.Path(sys.argv[1])
outdir = src.parent
text = src.read_text()

# --- parse the pieces we need out of the EDIF -------------------------------------------------
# cell -> ordered list of (port_name, designator)
cells = {}
for m in re.finditer(r'\(cell (\w+)\s*\(cellType GENERIC\)\s*\(designator "([^"]+)"\)(.*?)(?=\n    \(cell |\n  \(library |\Z)',
                     text, re.S):
    cell, prefix, body = m.group(1), m.group(2), m.group(3)
    pins = re.findall(r'\(port ([\w]+) \(direction (\w+)\) \(designator "([^"]+)"\)\)', body)
    cells[cell] = {"prefix": prefix, "pins": [(n, d, dr) for n, dr, d in pins]}

# instance id -> (refdes, cell, mpn, value)
insts = {}
for m in re.finditer(r'\(instance (\w+) \(viewRef V \(cellRef (\w+) \(libraryRef PARTS\)\)\) \(designator "([^"]+)"\)((?:\s*\(property [^\n]*)*)',
                     text):
    iid, cell, ref, props = m.group(1), m.group(2), m.group(3), m.group(4)
    mpn = (re.search(r'\(property MPN \(string "([^"]+)"\)\)', props) or [None, ""])[1]
    val = (re.search(r'\(property Value \(string "([^"]+)"\)\)', props) or [None, ""])[1]
    insts[iid] = {"ref": ref, "cell": cell, "mpn": mpn, "value": val}

# net -> [(instance_id, designator)]
nets = collections.OrderedDict()
for m in re.finditer(r'\(net (\w+) \(joined', text):
    name = m.group(1)
    # Read balanced parens from the '(net' onward. A regex cannot do this: the first portRef
    # closes with '))' and a non-greedy match stops there, silently yielding an empty net.
    i, depth = m.start(), 0
    while i < len(text):
        if text[i] == '(':
            depth += 1
        elif text[i] == ')':
            depth -= 1
            if depth == 0:
                break
        i += 1
    body = text[m.start():i + 1]
    conns = re.findall(r'\(portRef (\w+) \(instanceRef (\w+)\)\)', body)
    nets[name] = [(iid, d) for d, iid in conns]
    if not conns:
        sys.exit(f"net {name} parsed with no connections")

print(f"parsed {len(cells)} cells, {len(insts)} instances, {len(nets)} nets", file=sys.stderr)

# --- symbol library ----------------------------------------------------------------------------
# Pins run down the left edge at 2.54mm pitch. Symbol Y is up, schematic Y is down, so a pin at
# symbol y maps to schematic (cx + px, cy - py). Body is a plain box sized to the pin count.
PITCH = 2.54
def sym_body(cell, info):
    n = len(info["pins"])
    top = (n - 1) * PITCH / 2
    h = max(top + PITCH, PITCH)
    lines = [f'\t(symbol "{cell}"',
             '\t\t(pin_names (offset 0.254))',
             f'\t\t(property "Reference" "{info["prefix"]}")',
             f'\t\t(property "Value" "{cell}")',
             f'\t\t(symbol "{cell}_0_1"',
             f'\t\t\t(rectangle (start 0 {h:.2f}) (end 7.62 {-h:.2f})',
             '\t\t\t\t(fill (type background))))',
             f'\t\t(symbol "{cell}_1_1"']
    for i, (pname, desig, direction) in enumerate(info["pins"]):
        y = top - i * PITCH
        etype = {"INPUT": "input", "OUTPUT": "output", "INOUT": "bidirectional"}.get(direction, "passive")
        lines.append(f'\t\t\t(pin {etype} line (at -2.54 {y:.2f} 0) (length 2.54) '
                     f'(name "{pname}") (number "{desig}"))')
    lines[-1] += '))'
    return "\n".join(lines)

sym = ["(kicad_symbol_lib", '\t(version 20241209)', '\t(generator "agni-tutorial")']
for cell in sorted(cells):
    sym.append(sym_body(cell, cells[cell]))
sym.append(")")
(outdir / "symbols").mkdir(exist_ok=True)
(outdir / "symbols" / "gateway.kicad_sym").write_text("\n".join(sym) + "\n")

# --- schematic ---------------------------------------------------------------------------------
# Placement grid. Wide spacing so the stubs and labels never overlap a neighbouring symbol.
COLS, DX, DY, X0, Y0 = 4, 55.0, 38.0, 35.0, 25.0
pin_net = {}
for nname, conns in nets.items():
    for iid, desig in conns:
        pin_net[(iid, desig)] = nname

sch = ["(kicad_sch", '\t(version 20250114)', '\t(generator "agni-tutorial")', '\t(paper "A4")',
       '\t(title_block', '\t\t(title "Gateway ECU (tutorial board)")', '\t\t(rev "A"))',
       '\t(lib_symbols)']
wires, labels, emitted = [], [], 0
for idx, iid in enumerate(insts):
    inf = insts[iid]
    cx, cy = X0 + (idx % COLS) * DX, Y0 + (idx // COLS) * DY
    sch.append(f'\t(symbol (lib_id "gateway:{inf["cell"]}") (at {cx} {cy} 0) (unit 1) (uuid "{iid.lower()}")')
    sch.append(f'\t\t(property "Reference" "{inf["ref"]}" (at {cx} {cy - 12} 0)')
    sch.append('\t\t\t(effects (font (size 1.27 1.27))))')
    if inf["value"]:
        sch.append(f'\t\t(property "Value" "{inf["value"]}" (at {cx} {cy - 9.5} 0)')
        sch.append('\t\t\t(effects (font (size 1.27 1.27))))')
    if inf["mpn"]:
        sch.append(f'\t\t(property "MPN" "{inf["mpn"]}" (at {cx} {cy - 7} 0)')
        sch.append('\t\t\t(effects (font (size 1.27 1.27)) (hide yes))))')
    else:
        sch[-1] += ')'
    top = (len(cells[inf["cell"]]["pins"]) - 1) * PITCH / 2
    for i, (_, desig, _) in enumerate(cells[inf["cell"]]["pins"]):
        net = pin_net.get((iid, desig))
        if not net:
            continue
        # A pin's CONNECT point is its `at`, not `at` extended by `length`. Getting that wrong
        # puts every stub 2.54mm off its pin, so nothing joins and each pin becomes its own net.
        px, py = cx - 2.54, cy - (top - i * PITCH)
        ex = px - 7.62                                   # stub runs left, clear of the body
        emitted += 1
        wires.append(f'\t(wire (pts (xy {px:.2f} {py:.2f}) (xy {ex:.2f} {py:.2f}))\n\t\t(uuid "w{idx}-{desig}"))')
        labels.append(f'\t(label "{net}" (at {ex:.2f} {py:.2f} 180)\n'
                      f'\t\t(effects (font (size 1.27 1.27)) (justify right)))')
sch += wires + labels
sch.append(")")
(outdir / "gateway.kicad_sch").write_text("\n".join(sch) + "\n")
total = sum(len(c) for c in nets.values())
if emitted != total:
    sys.exit(f"emitted {emitted} stubs for {total} connections -- some pin was not placed")
print(f"stubs {emitted} == connections {total}", file=sys.stderr)
print(f"wrote {outdir/'gateway.kicad_sch'} and {outdir/'symbols'/'gateway.kicad_sym'}", file=sys.stderr)
