# 22 — Net solving & the hierarchy walk

Enforceable rules in [/CONSTRAINTS.md](../CONSTRAINTS.md); a `CN` reference (e.g. C9) points to constraint N there.

How a schematic's implicit connectivity becomes `ir.Net`s: the shared solver
(`internal/netgraph`), the KiCad connection-point semantics layered on it
(`kicad/sch_nets.go`), and the multi-sheet hierarchy walk that reads a whole project as
one design (`kicad/sch_hier_nets.go`, WS1-018). Written because the walk's PR (109) was
hard to review without this picture in one place.

## The problem

Schematic formats do not store nets. A `.kicad_sch` stores wires (line segments), symbol
placements (whose pins are at symbol-local coordinates), and labels (text at a point).
Connectivity is implied by geometry: things that touch are connected, and things that
share a name are connected. The same is true of xschem and gEDA. So nets are *computed*
by a solver, and every schematic reader feeds the same one.

KiCad adds a second problem: a multi-sheet design is a FILE PER SHEET. The root
references children by `(sheet ... (property "Sheetfile" ...))`, and one child file can
be instantiated several times. Reading one file therefore reads one sheet of the design,
not the design.

## The solver: internal/netgraph

`Build(wires, anchors, pins, terminals)` takes four flat inputs on an integer grid (the
caller quantizes its native units so points meant to coincide compare equal):

- **Wire**: a segment between two grid points, optionally carrying an inline net name
  (xschem `lab=`) and the source's wire id (for diagnostics).
- **Anchor**: a named point: a label, a power tap, a port. Contributes a name, not a
  pin. Carries `Driver` (a PWR_FLAG-style "this net is fed" assertion), `External` ("this
  net continues into something the read did not cover"), and `Rank` (naming priority,
  below).
- **Pin**: a component pin at a point: `(Comp, Pin)`, plus `NoConnect` (a no-connect
  marker sat on the pin) and `Dir` (a virtual power-symbol pin's direction, WS1-014).
- **terminals**: points where a bare wire end is legitimate (junction dots, no-connect
  flags), used only by dangling-endpoint detection.

Solving is two unions and a naming pass:

1. **Union by point.** Every wire unions its two endpoints; anchors and pins join
   whatever node owns their point. After this pass, each union-find root is a
   geometrically-connected node.

2. **Union by label.** Every non-empty label (wire labels and anchor labels alike)
   unions all the points that carry it. This is "connect by name": every `GND` tap on a
   sheet is one net regardless of geometry. Since WS1-018 it is also *aliasing*: a node
   carrying two different labels folds both label groups into one net (two labels on one
   wire are two names for one net), and it is the mechanism the hierarchy walk uses to
   stitch sheets, two points in different coordinate bands that share a label become
   one net.

3. **Naming.** Each merged root takes its lowest-ranked label: wire labels first
   (rank −1, preserving the old wire-label-beats-anchor behavior), then anchors by
   `Rank`, ties broken by input order. Unnamed roots with pins get synthetic `N$<n>`
   names by first appearance, except a lone pin marked `NoConnect`, which takes the
   tool-marker name `unconnected-(REF-PadN)` so no-connect-aware consumers key on it
   (WS1-019). Unnamed, pinless roots are drawing noise and are dropped.

Dangling detection is separate and purely positional: a wire endpoint whose grid point
holds nothing else (no pin, anchor, terminal, or second wire endpoint) is reported with
its wire id.

`Build`'s third return is a **wire → net-name map** keyed by `Wire.Id` (WS1-022): the same
solve that names nets also tells the geometry sidecar which net each drawn wire belongs to,
so the viewer can tint or highlight a KiCad wire by net (KiCad wires carry no inline net
name, unlike xschem `lab=` / gEDA, which fill `WireGeometry.net` from the wire's own label
and pass empty ids here). Only identified wires populate it; a wire on drawing-noise is
omitted. In a hierarchy the geometry reader runs the *combined* solve (`hierWireNets`) once,
not a per-sheet solve, so the names are byte-identical to the netlist read's (same `N$`
numbering, same `/sheet/LOCAL` qualification). A reused sub-sheet's wires share uuids across
its instances, so the geometry pass namespaces each wire id by its instance
(`sheetScope.wirePfx`, the sheet path) before the solve; otherwise `/ampli_ht_horizontal`
and `/ampli_ht_vertical` would collapse onto one name.

The solver is format-agnostic. Everything KiCad-specific (what counts as a connection
point, name scoping, escapes) happens in the reader before `Build` is called.

## KiCad connection-point semantics

Pinned against `kicad-cli sch export netlist` (the reference implementation), because
two of these rules are not what the endpoint-only solver would guess:

- **A pin connects where a wire ENDS on its connect point.** The connect point is
  `placement origin + (local_x, −local_y)` through the shared placement transform
  (`internal/geomath`): the same math the renderer draws with, which is what guarantees
  a pin lands exactly on the wire endpoint it connects to.
- **A label or a junction dot connects anywhere ALONG a wire.** eeschema does not
  rewrite a wire when a label lands mid-span; the real corpus has an `S_OUT+` label in
  the middle of a 70mm segment. The reader reconciles this with the point-identity
  solver by SPLITTING each wire at every label/junction point lying strictly inside it
  (`splitWiresAt`; sheet-frame coordinates only, the collinearity cross-product would
  overflow int64 in the walk's offset bands).
- **Pins do NOT connect mid-span.** The first split implementation included pins and
  merged the showcase board's `USB_D-` into GND: a GND power symbol's pin sits mid-span
  on the D- wire, and kicad-cli keeps them separate. Wire-wire crossings and
  endpoint-on-body touches without a junction dot likewise stay unconnected, so other
  wires' endpoints are not split candidates either.
- **Net-name text is brace-escaped in the file.** KiCad stores `/` in a label as
  `{slash}` (it collides with the hierarchy separator) and unescapes on load, so
  `VPP{slash}MCLR` and `VPP/MCLR` are one net (pic_programmer has exactly this pair).
  `unescapeName` undoes the escape table at every label/port read.
- **A power symbol (`#PWR`, `#FLG`) is an anchor plus a virtual pin.** Its Value names
  the net (a PWR_FLAG names nothing but asserts Driven); its power-typed pin also lands
  as a typed virtual connection (WS1-014) so rules see driver evidence. The symbol never
  enters `Components`.

## Name scoping: the fully-qualified-name model

KiCad has four name kinds, and they map onto familiar software scoping:

| kind | scope | net name | analogy |
|---|---|---|---|
| local label | one sheet instance | `/amp1/SIG` (root: bare `SIG`) | instance-qualified member: same text on two sheets is two nets |
| global label / power symbol | design-wide | bare (`VCC`) | root-namespace global |
| hierarchical label + sheet pin | one instance edge | the child-qualified port name | function parameter: child declares the port, each call site binds it |
| ref-des in a reused sheet | per instance | flat (`RV201` vs `RV301`) | linker gives each inlined copy its own symbol, looked up, not qualified |

The qualifier is the INSTANCE path (built from Sheetnames), not the file: a sheet file
is a class, its placements are objects. Root locals stay bare because that is what
`.kicad_pcb` files store, matching the board's spelling is what lets schematic-vs-board
diffs and net joins agree. (kicad-cli's netlist export adds a leading `/` to root locals
that the board does not carry; we match the board.)

Naming priority across kinds is encoded in `Anchor.Rank` and was pinned empirically: a
net carrying both a local label and a `VCC` power symbol exports from kicad-cli as
`VCC`, so global/power is rank 0 and local/port is rank 1. Rank ties keep input order,
and the walk emits pre-order, so a root-sheet name beats an equal-kind deeper name, 
also KiCad's rule.

## The hierarchy walk

`ReadSchematicHierarchyNets(rootName, rootContent, open)` reads the whole tree into one
design. `open` fetches a child by its relative `Sheetfile` path (the caller, the
`formats` registry, resolves against the root's directory; the reader does no
file I/O, C1). The traversal, cycle guard, and hierarchical sheet ids (`/`, `/<name>`,
`/<name>/<name>`) are the geometry walk's exactly, so netlist and geometry agree on
sheet identity, the join surface for sheet badges and cross-sheet navigation.

Per sheet INSTANCE (a reused file is walked once per placement), pre-order:

1. **Coordinate band.** Instance k's geometry is offset by `k · 2^41` nm on X. Sheets
   are ~10^9 nm wide, so bands never touch: point-union stays within an instance, and
   only label-union crosses bands. One `Build` call solves the whole design.
2. **Scoped collection.** The same collector the single-sheet read uses
   (`collectSheetNets`) runs with a `sheetScope`: the band offset, the name prefix
   (empty for the root), and the instance path. Local and hierarchical labels emit
   prefix-qualified anchors; globals and power symbols emit bare ones.
3. **Components.** `symbolRefAt(ps, instPath)` resolves each placement's ref-des from
   the `instances` entry whose `path` matches this instance's uuid chain
   (`/<root file uuid>/<sheet block uuid>/...`); fallback is the first non-placeholder
   entry, then the Reference property (the single-file behavior). Components merge
   design-wide by resolved ref; part libraries dedup by lib_id across files.
4. **Ports.** For each `(sheet ...)` block, the parent emits an anchor at every sheet
   pin's position labeled `<childID>/<pin name>`: the SAME string the child's
   same-named `hierarchical_label` produces under its prefix. Label-union joins the
   parent net to the child net; rank 1 makes the port name the net's name when nothing
   better exists. This works per instance edge: the two `amp` instances bind their
   `CTRL` ports to different parent nets.
5. **Recursion + completeness.** A child that fails to open (or a `Sheetfile` cycling
   back to an ancestor) is skipped and flips `complete` to false. The rest still reads.

After the walk: one `Build`, then pinless named nets are filtered (KiCad omits a label
on a dangling wire), and dangling endpoints are translated back out of their bands
(`X − k·2^41`, source file looked up by band index) so diagnostics carry sheet-frame
coordinates the viewer can draw.

**Completeness is the WS1-017 witness.** `external` on a net means "the read may not
cover this net". A complete PROJECT walk (`.kicad_pro` read, every Sheetfile opened)
makes that marking stale, and `ReadProject` downgrades external→global
(`netgraph.ResolveExternal`); a sheetless root is trivially complete, so flat projects
behave as before. A bare `.kicad_sch` read also walks (the registry passes the same
opener) but NEVER downgrades: that file may itself be someone's sub-sheet. The root
sheet's own hierarchical labels stay External for the same reason; they continue
upward into a parent nobody read.

`ReadProject` always walks the schematic side even when a board is present: the board
still supplies the nets, but sub-sheet components now arrive with their part types and
typed pins instead of as bare footprint pads (the read gap that forced WS1-014's
`PinDeclared` guard).

## Worked example

The `twosheet`/`hier_root` fixture: a root placing `hier_child.kicad_sch` twice (`amp1`,
`amp2`). The child has two resistors (instances paths give `R101/R102` in amp1,
`R201/R202` in amp2), a local `SIG` label, a hierarchical `CTRL` label, and a `VCC`
power symbol. The root has `R1`, its own local `SIG`, a `VCC` symbol + PWR_FLAG, wires
amp1's `CTRL` pin to R1, and leaves amp2's `CTRL` pin unwired.

| net | members | why |
|---|---|---|
| `SIG` | R1.2 | root local, bare, scoped apart from the children |
| `/amp1/SIG` | R101.2 R102.2 | child local, instance-qualified |
| `/amp2/SIG` | R201.2 R202.2 | same file, second instance, second net |
| `/amp1/CTRL` | R1.1 R101.1 | port: parent pin anchor + child hier label share the string |
| `/amp2/CTRL` | R201.1 | port with no parent wire: child side only |
| `VCC` | #FLG01.1 #PWR01.1 #PWR02.1 R102.1 R202.1 | bare rank-0 rail unifies all three instances; virtual pins dedup |

The child's stray wire dangles at both ends in both instances: four diagnostics, each
with sheet-frame coordinates and `hier_child.kicad_sch` as its source.

## Verification method

Unit tests pin each mechanism, but the acceptance is INTEROP: flatten the corpus
hierarchies with `kicad-cli sch export netlist` and compare components and every labeled
net by name and membership. Both `complex_hierarchy` (68 components, 52 nets, a reused
sheet) and `pic_programmer` (111 nets, escaped names) match exactly; the only deltas are
stub conventions (our `N$k` and `unconnected-(REF-PadN)` vs KiCad's synthesized
`Net-(REF-Pad)` forms), which we deliberately keep. The walked component set also equals
the board-derived set, closing the loop with the `.kicad_pcb` reader. This method, not
the unit tests, is what caught the mid-span label rule, the pins-are-endpoint-only
rule, and the `{slash}` escapes.
