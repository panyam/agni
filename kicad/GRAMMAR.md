# KiCad s-expression grammar (ingested subset)

What the `kicad` package parses out of KiCad files and how each construct maps to the
neutral IR (`agni.v1.ir`). This is the **subset we read**, not the full KiCad format
(which is large and evolving). Same posture as the EDIF primers: document the contract
between the format and the reader.

Two layers:
1. **Generic s-expression** — the token/tree layer `sexpr.go` produces. Trivial; not
   format-specific.
2. **KiCad AST** — the named constructs the readers expect (`pcb.go`, `sch.go`), each with
   its meaning and its IR mapping.

Notation: EBNF-ish. `x*` = zero or more, `x?` = optional, `|` = alternation, `"lit"` = a
literal head symbol, `STRING`/`NUMBER`/`SYMBOL` = atoms. Order of children is not
significant to the reader (we look constructs up by head, not position), so `...` means
"other children we ignore".

## 1. Generic s-expression (sexpr.go)

```
node   = list | atom
list   = "(" node* ")"
atom   = STRING | SYMBOL | NUMBER
STRING = '"' ( char | escape )* '"'          ; escape = \" \\ \n \t
SYMBOL = { any char except whitespace, ( ) " }
```

- **list** — an ordered node sequence. Its first element is the *head* (e.g. `footprint`);
  helpers `head()`, `arg(i)`, `child(name)`, `children(name)` navigate by it.
- **atom** — a leaf. Quotes and escapes are resolved, so `"F.Cu"` and `F.Cu` both yield the
  text `F.Cu`. The reader never distinguishes quoted from bare; only the text matters.

## 2. `.kicad_pcb` — the board (pcb.go)

The board is the physical/connectivity view: placed footprints and the nets their pads sit
on. This is where connectivity is explicit and exact.

```
pcb        = "(" "kicad_pcb" version? generator? title-block? net-decl* footprint* ... ")"
version    = "(" "version" NUMBER ")"
generator  = "(" "generator" STRING ")"
title-block= "(" "title_block" ( "(" "title" STRING ")" )? ... ")"
net-decl   = "(" "net" NUMBER STRING ")"                 ; a board-level net table entry
footprint  = "(" "footprint" STRING layer? uuid? at? property* pad* ... ")"
property   = "(" "property" STRING STRING ... ")"        ; key, value
uuid       = "(" "uuid" STRING ")"
pad        = "(" "pad" STRING SYMBOL SYMBOL ... net-ref? ... ")" ; number, type, shape
net-ref    = "(" "net" NUMBER STRING ")" | "(" "net" STRING ")"  ; number+name OR name-only
```

Meaning and IR mapping:

- **`kicad_pcb`** — one board. → one `ir.Design` (`source_format="kicad-pcb"`).
- **`title_block.title`** — human title of the board. → `Design.name` (may be empty).
- **`net-decl` `(net N "name")`** — the board's net table (numbered). The reader does **not**
  rely on this table; it builds nets from the pads instead (see `net-ref`), which also works
  for KiCad 10 boards that drop the table. Net number `0` / empty name = "no net".
- **`footprint "Lib:Name"`** — one placed physical component. The string is the footprint id
  (`library:name`). → one `ir.Component` keyed by its `Reference` property (`ref_des`), with
  `footprint_ref` = the footprint id, and one `ir.Footprint{name=id, library=Lib}` (deduped).
  A footprint with no `Reference` (graphic-only) is skipped, and so is one whose reference is the unannotated-placeholder form (`REF**`, trailing `?`) — a placeholder is annotation state, not an identity, and keying it merges distinct parts (WS1-024; the corpus cimos board carries 26 `REF**` footprints). `uuid` → `Provenance.native_id`
  (`native_id_kind="kicad-uuid"`).
- **`property "Reference"|"Value"`** — component fields. `Reference` → `ref_des`; `Value` →
  `Component.attributes["Value"]`.
- **`pad "N" type shape`** — a copper land. The pad number `N` is the pin identifier. → a
  `ir.Connection{component_ref=ref_des, pin_ref=N}` on the pad's net.
- **`net-ref` (a pad's `(net …)` child)** — which net the pad is on. The net **name** is the
  last argument, so both `(net 3 "GND")` (older) and `(net "GND")` (KiCad 10) yield `GND`.
  Absent child = unconnected pad (no connection). → creates/*finds* `ir.Net{name}` and adds
  the connection (deduped per net by `ref.pad`).

Not extracted by this netlist reader — geometry is always a keyed sidecar, never the
netlist IR (C7/C8): component placement (`at`), pad `size`/shape, copper, layers, zones,
vias, 3D models. The **board-geometry sidecar** now exists (`ReadBoardGeometry` in
pcb_geom.go, WS1-006) and recovers all of it except 3D models into `geom.BoardGeometry`;
the netlist reader keeps only the logical netlist plus footprint refs. A PCB component has
**no `ComponentSection`s** — units are a schematic concept (see §3).

### 2b. Board-geometry productions (pcb_geom.go, WS1-006)

The sidecar reader walks the same tree for the physical layout. Coordinates go out in
nanometers, Y-flipped to the geom contract's Y-up frame; rotations are carried verbatim
(the proto documents the composition rule).

```
layers     = "(" "layers" ("(" NUMBER STRING SYMBOL ... ")")* ")"   ; number, name, kind
gr_line    = "(" "gr_line" start end ... layer ")"                  ; Edge.Cuts -> outline path
gr_rect    = "(" "gr_rect" start end ... layer ")"                  ; -> closed 4-edge path
gr_arc     = "(" "gr_arc" start mid end ... layer ")"               ; -> 16-segment polyline
segment    = "(" "segment" start end width layer net-ref ")"        ; one track run
via        = "(" "via" at size drill "(" "layers" STRING STRING ")" net-ref ")"
zone       = "(" "zone" net-ref net_name? layer(s) ... polygon ")"  ; authored outline only
```

- **`layers` table** → `BoardLayer{number,name,kind}` rows, and the number→name map that
  pre-KiCad-10 copper `net-ref`s resolve through.
- **copper `net-ref`** — three forms: `(net N "name")` (name is last), `(net N)` (resolve
  via the top-level net table this reader DOES read, unlike the netlist reader), and
  `(net "name")` (KiCad 10, which drops the table; found on the corpus pic_programmer
  board where number-only resolution lost all 370 segments). Net 0 / unresolvable = no
  net; that copper is dropped since the IR has no key for it.
- **`footprint`** → `ComponentPlacement{ref_des, at, rotation, layer}` with
  footprint-local `Pad`s (number matches `ir.Connection.pin_ref`); Reference-less
  footprints are skipped exactly like the netlist reader, so the two artifacts agree on
  the component set.
- **`zone`** → net + layer + the authored `polygon` outline; `filled_polygon` (derived
  fill data) is not carried (C6 bound).

## 3. `.kicad_sch` — the schematic (sch.go, WS1-005 PR2)

The schematic is the logical view: the part-type library, the placed symbols (with units),
and the sheet hierarchy. Connectivity here is implicit (wires + labels) and is **not**
extracted; nets come from the board.

```
sch          = "(" "kicad_sch" version? generator? lib-symbols placed-symbol* sheet* ... ")"
lib-symbols  = "(" "lib_symbols" lib-symbol* ")"
lib-symbol   = "(" "symbol" STRING property* sub-symbol* ")"     ; STRING = "library:name"
sub-symbol   = "(" "symbol" STRING pin* ... ")"                  ; STRING = "name_unit_style"
pin          = "(" "pin" SYMBOL SYMBOL ... pin-name pin-number ")" ; elec-type, graphic-style
pin-name     = "(" "name" STRING ... ")"
pin-number   = "(" "number" STRING ... ")"
placed-symbol= "(" "symbol" lib-id at? unit? uuid? property* ... ")"
lib-id       = "(" "lib_id" STRING ")"                           ; references a lib-symbol
unit         = "(" "unit" NUMBER ")"
sheet        = "(" "sheet" at? uuid? property* ... ")"           ; Sheetname, Sheetfile props
```

Meaning and IR mapping:

- **`kicad_sch`** — one schematic sheet file. → contributes to an `ir.Design`
  (`source_format="kicad-sch"`).
- **`lib_symbols`** — the embedded part-type library (the definitions used on this sheet).
  → `ir.PartLibrary`.
- **`lib-symbol` `(symbol "library:name" …)`** — one part-type definition. → `ir.PartType`
  with `name` = the `library:name` id. Its `property "Reference"` value (e.g. `U`, `R`,
  `#PWR`) is the reference-designator prefix → `PartType.designator_prefix`.
- **`sub-symbol` `(symbol "name_U_S" …)`** — a unit/style variant that holds the actual
  pins (KiCad splits a symbol's graphics by unit `U` and body-style `S`). The reader pools
  the pins across all sub-symbols onto the one `PartType`.
- **`pin elec-type graphic`** — a terminal. `elec-type` (input/output/bidirectional/passive/
  power_in/power_out/no_connect/…) → normalized `ir.Pin.direction` (`PinDirection`), with the
  raw spelling kept in `attributes["direction_raw"]` when it has no clean mapping (C9). →
  `ir.Pin{name=pin-name, designator=pin-number, direction}`.
- **`placed-symbol` `(symbol (lib_id "library:name") (unit N) …)`** — one placement of a part
  on the sheet. The distinctive case: a multi-unit part (e.g. a quad gate) is **several
  placed symbols sharing one `Reference` with different `unit` numbers**. The reader groups
  placed symbols by `Reference` → one `ir.Component`, and each placement → one
  `ir.ComponentSection{index=unit-1, part_ref=lib-id}`. `uuid` → section `Provenance.native_id`.
  Symbols whose `Reference` starts with `#` (KiCad virtual power/flag symbols like `#PWR`,
  `#FLG`) are skipped as components: they are connectivity anchors, not physical parts.
  Their PINS still reach the netlist (WS1-014): a power symbol's pin with a power
  electrical type (`power_in`/`power_out`) becomes a typed virtual connection on the net —
  `ir.Connection{component_ref="#PWR05", attributes={"direction": "power_in"}}` — so power
  rules see driver evidence (a `PWR_FLAG`'s `power_out` IS the "net is driven" assertion)
  while the component list stays physical. Only power directions travel; a virtual pin
  never fabricates signal-direction facts. The name-anchor semantics (net naming, the
  WS1-017 external/global attrs) are unchanged and ride alongside.
- **`sheet` (+ `Sheetname`/`Sheetfile` properties, `(pin "NAME" ...)` ports)** — a
  hierarchical sub-sheet reference. The netlist hierarchy walk (WS1-018,
  `ReadSchematicHierarchyNets`) follows `Sheetfile` through the caller-supplied opener
  (C1) and reads the whole tree into ONE design, one instance per placement: components
  resolve their per-instance ref-des from the matching `instances` path entry (a reused
  file is two instances with distinct refs), local labels are qualified by the instance's
  sheet path (`/amp1/SIG` — KiCad's own net-name convention, matching board files), global
  labels and power symbols unify design-wide, and a sheet `pin` port unions the parent net
  with the child's same-named `hierarchical_label` per instance edge. `ir.Sheet{id, name}`
  uses the geometry walk's hierarchical ids (`/`, `/<Sheetname>`, ...) so netlist and
  geometry agree on sheet identity. A single-sheet read (`ReadSchematic`) keeps the flat
  one-file semantics.
- **Connection points.** Labels and junction dots attach anywhere ALONG a wire (the reader
  splits the segment there); pins — including power-symbol pins — connect only where a
  wire ENDS on their connect point, and crossing wires without a junction stay separate.
  Pinned against `kicad-cli sch export netlist` (the showcase board has a GND pin sitting
  mid-span on the USB_D- wire, unconnected). Label/net-name text is unescaped from KiCad's
  brace forms (`VPP{slash}MCLR` = `VPP/MCLR`; two labels in either spelling are one net).
  An endpoint left strictly INSIDE another segment's body after that split is the undotted
  T-tap — drawn as connected, electrically two nets — and emits the no_junction_endpoints
  diagnostic (WS1-012; the wire-no-junction rule reports it, and dangling-endpoint
  deliberately skips those points). The full solving algorithm — the netgraph unions, name
  ranks, instance scoping, and the walk — is [docs/22](../docs/22-net-solving-and-hierarchy.md).
- **External symbol libraries (WS1-016).** A `lib_id` whose library is missing from the
  embedded `lib_symbols` resolves from `.kicad_sym` files: the project's `sym-lib-table`
  beside the schematic first (`${KIPRJMOD}` = its directory; no flag needed), then each
  `--symbol-path` directory by `<Library>.kicad_sym` nickname. Embedded definitions always
  win; unresolved libraries keep the placeholder behavior. Same-library `extends` chains
  flatten (a derived symbol with no own units inherits the parent's); cross-library
  extends is ledgered. Resolution is held equal to the embedded read by test — kicad-cli
  cannot oracle this one (headless, it ignores the project table entirely).

Not extracted by this reader — it belongs in a **geometry sidecar**, not the netlist IR
(C7/C8): symbol, wire, and label drawing geometry, and therefore the schematic's implicit
connectivity (which we take from the board instead). Schematic-page geometry is exactly
what the `agni.v1.geom` sidecar models (fed today by the EDIF `.eds` reader); a KiCad
schematic-geometry reader would populate it. Also dropped: symbol graphics, text, and sheet
instance paths.
