# KiCad s-expression grammar (ingested subset)

What the `kicad` package parses out of KiCad files and how each construct maps to the
neutral IR (`agni.v1.ir`). This is the **subset we read**, not the full KiCad format,
which is large and still evolving. Same posture as the EDIF primers, which document the
contract between the format and the reader.

There are two layers:
1. The **generic s-expression** layer produces tokens and a tree. It is trivial and not
   format-specific.
2. The **KiCad AST** layer holds the named constructs the readers expect (`pcb.go`,
   `sch.go`), each with its meaning and its IR mapping.

The notation is EBNF-ish. `x*` is zero or more, `x?` is optional, `|` is alternation,
`"lit"` is a literal head symbol, and `STRING`/`NUMBER`/`SYMBOL` are atoms. The reader looks
constructs up by head rather than position, so the order of children does not matter, and
`...` means other children we ignore.

## 1. Generic s-expression (internal/sexpr, wrapped by sexpr.go)

```
node   = list | atom
list   = "(" node* ")"
atom   = STRING | SYMBOL | NUMBER
STRING = '"' ( char | escape )* '"'          ; escape = \" \\ \n \t
SYMBOL = { any char except whitespace, ( ) " }
```

- A **list** is an ordered node sequence. Its first element is the *head* (e.g. `footprint`),
  and the `sexpr.Node` helpers `Head()`, `Arg(i)`, `Child(name)` and `Children(name)` navigate
  by it.
- An **atom** is a leaf. Quotes and escapes are resolved, so `"F.Cu"` and `F.Cu` both yield the
  text `F.Cu`. The reader never distinguishes quoted from bare, and only the text matters.

## 2. The board file (.kicad_pcb, pcb.go)

The board is the physical and connectivity view, holding placed footprints and the nets their
pads sit on. This is where connectivity is explicit and exact.

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

- **`kicad_pcb`** is one board, read into one `ir.Design` (`source_format="kicad-pcb"`).
- **`title_block.title`** is the board's human title, carried as `Design.name` (may be empty).
- **`net-decl` `(net N "name")`** is the board's numbered net table. The reader does **not**
  rely on this table. It builds nets from the pads instead (see `net-ref`), which also works
  for KiCad 10 boards that drop the table. Net number `0` or an empty name means no net.
- **`footprint "Lib:Name"`** is one placed physical component. The string is the footprint id
  (`library:name`). It becomes one `ir.Component` keyed by its `Reference` property (`ref_des`),
  with `footprint_ref` set to the footprint id, and one deduped
  `ir.Footprint{name=id, library=Lib}`. A footprint with no `Reference` (graphic-only) is
  skipped, and so is one whose reference is the unannotated-placeholder form (`REF**`, trailing
  `?`). A placeholder is annotation state rather than an identity, and keying on it merges
  distinct parts (WS1-024; the corpus cimos board carries 26 `REF**` footprints). `uuid` goes to
  `Provenance.native_id` (`native_id_kind="kicad-uuid"`).
- **`property "Reference"|"Value"`** are component fields. `Reference` becomes `ref_des` and
  `Value` becomes `Component.attributes["Value"]`.
- **`pad "N" type shape`** is a copper land, and the pad number `N` is the pin identifier. It
  becomes an `ir.Connection{component_ref=ref_des, pin_ref=N}` on the pad's net.
- **`net-ref` (a pad's `(net …)` child)** says which net the pad is on. The net **name** is the
  last argument, so both `(net 3 "GND")` (older) and `(net "GND")` (KiCad 10) yield `GND`. An
  absent child means an unconnected pad with no connection. The reader creates or *finds*
  `ir.Net{name}` and adds the connection, deduped per net by `ref.pad`.

This netlist reader does not extract geometry, because geometry is always a keyed sidecar and
never the netlist IR (C7/C8). That covers component placement (`at`), pad `size` and shape,
copper, layers, zones, vias and 3D models. The **board-geometry sidecar** exists
(`ReadBoardGeometry` in pcb_geom.go, WS1-006) and recovers all of it except 3D models into
`geom.BoardGeometry`, while the netlist reader keeps only the logical netlist plus footprint refs.
A PCB component has **no `ComponentSection`s**, because units are a schematic concept (see §3).

### 2b. Board-geometry productions (pcb_geom.go, WS1-006)

The sidecar reader walks the same tree for the physical layout. Coordinates go out in
nanometers, Y-flipped to the geom contract's Y-up frame. Rotations are carried verbatim, and
the proto documents the composition rule.

```
layers     = "(" "layers" ("(" NUMBER STRING SYMBOL ... ")")* ")"   ; number, name, kind
gr_line    = "(" "gr_line" start end ... layer ")"                  ; Edge.Cuts -> outline path
gr_rect    = "(" "gr_rect" start end ... layer ")"                  ; -> closed 4-edge path
gr_arc     = "(" "gr_arc" start mid end ... layer ")"               ; -> 16-segment polyline
segment    = "(" "segment" start end width layer net-ref ")"        ; one track run
via        = "(" "via" at size drill "(" "layers" STRING STRING ")" net-ref ")"
zone       = "(" "zone" net-ref net_name? layer(s) ... polygon ")"  ; authored outline only
```

- **`layers` table** becomes `BoardLayer{number,name,kind}` rows, plus the number-to-name map
  that pre-KiCad-10 copper `net-ref`s resolve through.
- **copper `net-ref`** comes in three forms. `(net N "name")` carries the name last. `(net N)`
  resolves through the top-level net table, which this reader DOES read, unlike the netlist
  reader. `(net "name")` is KiCad 10, which drops the table; it turned up on the corpus
  pic_programmer board, where number-only resolution lost all 370 segments. Net 0 or an
  unresolvable net means no net, and that copper is dropped since the IR has no key for it.
- **`footprint`** becomes `ComponentPlacement{ref_des, at, rotation, layer}` with
  footprint-local `Pad`s whose number matches `ir.Connection.pin_ref`. Reference-less
  footprints are skipped exactly as the netlist reader skips them, so the two artifacts agree
  on the component set.
- **`zone`** carries its net, its layer and the authored `polygon` outline. `filled_polygon`
  is derived fill data and is not carried (C6 bound).

## 3. The schematic file (.kicad_sch, sch.go, WS1-005 PR2)

The schematic is the logical view, holding the part-type library, the placed symbols (with
units), and the sheet hierarchy. Connectivity here is implicit in wires, pins and labels, so
the file states no nets. The reader computes them from that geometry (`schNets` in
sch_nets.go) through the same solver the xschem and gEDA readers use.

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

- **`kicad_sch`** is one schematic sheet file, and it contributes to an `ir.Design`
  (`source_format="kicad-sch"`).
- **`lib_symbols`** is the embedded part-type library, holding the definitions used on this
  sheet. It becomes an `ir.PartLibrary`.
- **`lib-symbol` `(symbol "library:name" …)`** is one part-type definition, read into an
  `ir.PartType` whose `name` is the `library:name` id. Its `property "Reference"` value (e.g.
  `U`, `R`, `#PWR`) is the reference-designator prefix, stored as
  `PartType.designator_prefix`.
- **`sub-symbol` `(symbol "name_U_S" …)`** is a unit or style variant that holds the actual
  pins, since KiCad splits a symbol's graphics by unit `U` and body style `S`. The reader pools
  the pins across all sub-symbols onto the one `PartType`.
- **`pin elec-type graphic`** is a terminal. `elec-type` (input/output/bidirectional/passive/
  power_in/power_out/no_connect/…) is normalized to `ir.Pin.direction` (`PinDirection`), with
  the raw spelling kept in `attributes["direction_raw"]` when it has no clean mapping (C9). The
  pin becomes `ir.Pin{name=pin-name, designator=pin-number, direction}`.
- **`placed-symbol` `(symbol (lib_id "library:name") (unit N) …)`** is one placement of a part
  on the sheet. The distinctive case is a multi-unit part (e.g. a quad gate), which appears as
  **several placed symbols sharing one `Reference` with different `unit` numbers**. The reader
  groups placed symbols by `Reference` into one `ir.Component`, and each placement becomes one
  `ir.ComponentSection{index=unit-1, part_ref=lib-id}`. `uuid` goes to the section's
  `Provenance.native_id`.
  Symbols whose `Reference` starts with `#` (KiCad virtual power and flag symbols like `#PWR`,
  `#FLG`) are skipped as components, because they are connectivity anchors and not physical
  parts. Their PINS still reach the netlist (WS1-014). A power symbol's pin with a power
  electrical type (`power_in`/`power_out`) becomes a typed virtual connection on the net,
  `ir.Connection{component_ref="#PWR05", attributes={"direction": "power_in"}}`, so power
  rules see driver evidence (a `PWR_FLAG`'s `power_out` IS the assertion that the net is
  driven) while the component list stays physical. Only power directions travel, and a virtual
  pin never fabricates signal-direction facts. The name-anchor semantics (net naming, the
  WS1-017 external and global attrs) are unchanged and ride alongside.
- **`sheet` (with `Sheetname`/`Sheetfile` properties and `(pin "NAME" ...)` ports)** is a
  hierarchical sub-sheet reference. The netlist hierarchy walk (WS1-018,
  `ReadSchematicHierarchyNets`) follows `Sheetfile` through the caller-supplied opener (C1)
  and reads the whole tree into ONE design, one instance per placement. Components resolve
  their per-instance ref-des from the matching `instances` path entry, so a reused file is two
  instances with distinct refs. Local labels are qualified by the instance's sheet path
  (`/amp1/SIG`, which is KiCad's own net-name convention and matches board files). Global
  labels and power symbols unify design-wide, and a sheet `pin` port unions the parent net with
  the child's same-named `hierarchical_label` per instance edge. `ir.Sheet{id, name}` uses the
  geometry walk's hierarchical ids (`/`, `/<Sheetname>`, ...) so netlist and geometry agree on
  sheet identity. A single-sheet read (`ReadSchematic`) keeps the flat one-file semantics.
- **Connection points.** Labels and junction dots attach anywhere ALONG a wire, and the reader
  splits the segment there. Pins, power-symbol pins included, connect only where a wire ENDS on
  their connect point, and crossing wires without a junction stay separate. This is pinned
  against `kicad-cli sch export netlist`, since the showcase board has a GND pin sitting
  mid-span on the USB_D- wire, unconnected. Label and net-name text is unescaped from KiCad's
  brace forms (`VPP{slash}MCLR` is `VPP/MCLR`, and two labels in either spelling are one net).
  An endpoint left strictly INSIDE another segment's body after that split is the undotted
  T-tap. It is drawn as connected and is electrically two nets, and it emits the
  no_junction_endpoints diagnostic (WS1-012). The wire-no-junction rule reports it, and
  dangling-endpoint deliberately skips those points. The full solving algorithm, covering the
  netgraph unions, name ranks, instance scoping and the walk, is in
  [net solving](../../docsite/content/architecture/net-solving.md).
- **External symbol libraries (WS1-016).** A `lib_id` whose library is missing from the
  embedded `lib_symbols` resolves from `.kicad_sym` files. The project's `sym-lib-table`
  beside the schematic comes first (`${KIPRJMOD}` is its directory, and no flag is needed),
  then each `--symbol-path` directory by `<Library>.kicad_sym` nickname. Embedded definitions
  always win, and unresolved libraries keep the placeholder behavior. Same-library `extends`
  chains flatten, so a derived symbol with no units of its own inherits the parent's.
  Cross-library extends is ledgered. Resolution is held equal to the embedded read by test,
  because kicad-cli cannot oracle this one (headless, it ignores the project table entirely).

Symbol, wire and label drawing geometry belongs in a **geometry sidecar** and not in the
netlist IR (C7/C8), so this reader does not carry it. Schematic-page geometry is what the
`agni.v1.geom` sidecar models, and `ReadSchematicGeometry` in schematic_geometry.go (with
`ReadSchematicHierarchy` for a sheet tree) populates it for KiCad, as the EDIF `.eds` reader
does for EDIF. The netlist reader also drops symbol graphics and text.
