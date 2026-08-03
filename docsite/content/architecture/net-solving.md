---
title: "Net solving and the hierarchy walk"
description: "How a schematic's implicit connectivity becomes nets, and how a multi-sheet design is read as one graph."
---

A schematic does not store nets. It stores wires, symbols, and labels, and leaves connectivity implicit in the geometry. Turning that into an explicit connectivity graph is a solve, and every schematic reader (KiCad, xschem, gEDA) feeds the same solver. This page covers that solver, the KiCad connection-point rules layered on it, and the walk that reads a multi-sheet project as one design.

## The problem

A `.kicad_sch` file stores wires (line segments), symbol placements (whose pins sit at symbol-local coordinates), and labels (text at a point). Connectivity is implied by geometry. Things that touch are connected, and things that share a name are connected. xschem and gEDA are the same. So nets have to be *computed* from the geometry.

KiCad adds a second problem. A multi-sheet design is one file per sheet. The root file references its children by a `Sheetfile` property, and one child file can be instantiated several times. Reading a single file therefore reads one sheet of the design, not the design.

For a programmer, the solve is a union-find over points and names, and the hierarchy walk is inlining a call tree where each sheet file is a class and each placement is an instance of it.

## The solver

The shared solver takes four flat inputs on an integer grid. The caller quantizes its native units first, so points that are meant to coincide compare equal.

- **Wire**: a segment between two grid points, optionally carrying an inline net name (xschem `lab=`) and the source's wire id (used for diagnostics).
- **Anchor**: a named point (a label, a power tap, a port). It contributes a name, not a pin. It carries `Driver` (a "this net is fed" assertion, from a power-flag symbol), `External` ("this net continues into something the read did not cover"), and `Rank` (naming priority, below).
- **Pin**: a component pin at a point, `(Comp, Pin)`, plus `NoConnect` (a no-connect marker sitting on the pin) and `Dir` (a virtual power-symbol pin's direction).
- **terminals**: points where a bare wire end is legitimate (junction dots, no-connect flags), used only by dangling-endpoint detection.

Solving is two unions and a naming pass.

1. **Union by point.** Every wire unions its two endpoints, and anchors and pins join whatever node owns their point. After this pass, each union-find root is a geometrically-connected node.
2. **Union by label.** Every non-empty label (wire labels and anchor labels alike) unions all the points that carry it. This is "connect by name": every `GND` tap on a sheet is one net regardless of geometry. It is also aliasing. A node carrying two different labels folds both label groups into one net, so two labels on one wire are two names for one net. This same mechanism stitches sheets together in the hierarchy walk: two points in different coordinate bands that share a label become one net.
3. **Naming.** Each merged root takes its lowest-ranked label. Wire labels come first (rank −1, preserving wire-label-beats-anchor behavior), then anchors by `Rank`, with ties broken by input order. Unnamed roots that have pins get synthetic `N$<n>` names by first appearance, except a lone pin marked `NoConnect`, which takes the tool-marker name `unconnected-(REF-PadN)` so no-connect-aware consumers can key on it. Unnamed, pinless roots are drawing noise and are dropped.

Dangling detection is separate and purely positional. A wire endpoint whose grid point holds nothing else (no pin, anchor, terminal, or second wire endpoint) is reported with its wire id.

The solve also returns a **wire-to-net-name map** keyed by wire id. The same solve that names nets tells the geometry sidecar which net each drawn wire belongs to, so the viewer can tint or highlight a KiCad wire by net. KiCad wires carry no inline net name, unlike xschem `lab=` and gEDA, which fill the wire's net from its own label and pass empty ids here. Only identified wires populate the map, and a wire on drawing-noise is omitted. In a hierarchy, the geometry reader runs the *combined* solve once rather than a per-sheet solve, so the names are byte-identical to the netlist read's (same `N$` numbering, same `/sheet/LOCAL` qualification). A reused sub-sheet's wires share uuids across its instances, so the geometry pass namespaces each wire id by its instance (the sheet path) before the solve. Otherwise two instances of the same sub-sheet would collapse onto one name.

The solver itself is format-agnostic. Everything KiCad-specific (what counts as a connection point, how names are scoped, how escapes work) happens in the reader before the solve is called.

## KiCad connection-point semantics

These rules are pinned against `kicad-cli sch export netlist`, the reference implementation, because two of them are not what an endpoint-only solver would guess.

- **A pin connects where a wire ends on its connect point.** The connect point is `placement origin + (local_x, −local_y)` through the shared placement transform, the same math the renderer draws with. That shared transform is what guarantees a pin lands exactly on the wire endpoint it connects to.
- **A label or a junction dot connects anywhere along a wire.** eeschema does not rewrite a wire when a label lands mid-span, and the real corpus has a label in the middle of a 70mm segment. The reader reconciles this with the point-identity solver by splitting each wire at every label or junction point lying strictly inside it. The split is done in sheet-frame coordinates only, because the collinearity check would overflow a 64-bit integer in the hierarchy walk's offset bands.
- **Pins do not connect mid-span.** An early split that included pins merged the showcase board's `USB_D-` net into GND, because a GND power symbol's pin sits mid-span on the D- wire and `kicad-cli` keeps them separate. Wire-to-wire crossings and endpoint-on-body touches without a junction dot likewise stay unconnected, so other wires' endpoints are not split candidates either.
- **Net-name text is brace-escaped in the file.** KiCad stores a `/` in a label as `{slash}` (it collides with the hierarchy separator) and unescapes on load, so `VPP{slash}MCLR` and `VPP/MCLR` are one net. The reader undoes the escape table at every label and port read.
- **A power symbol is an anchor plus a virtual pin.** Its value names the net (a power-flag names nothing but asserts the net is driven), and its power-typed pin also lands as a typed virtual connection so rules can see driver evidence. The symbol never enters the component list.

## Name scoping: the fully-qualified-name model

KiCad has four name kinds, and they map onto familiar software scoping:

| kind | scope | net name | analogy |
|---|---|---|---|
| local label | one sheet instance | `/amp1/SIG` (root: bare `SIG`) | instance-qualified member: same text on two sheets is two nets |
| global label / power symbol | design-wide | bare (`VCC`) | root-namespace global |
| hierarchical label + sheet pin | one instance edge | the child-qualified port name | function parameter: child declares the port, each call site binds it |
| ref-des in a reused sheet | per instance | flat (`RV201` vs `RV301`) | the linker gives each inlined copy its own symbol, looked up rather than qualified |

The qualifier is the instance path (built from sheet names), not the file. A sheet file is a class and its placements are objects. Root locals stay bare because that is what `.kicad_pcb` files store, and matching the board's spelling is what lets schematic-versus-board diffs and net joins agree. (`kicad-cli`'s netlist export adds a leading `/` to root locals that the board does not carry, and Agni matches the board.)

Naming priority across kinds is encoded in the anchor's rank and was pinned empirically. A net carrying both a local label and a `VCC` power symbol exports from `kicad-cli` as `VCC`, so global and power are rank 0 and local and port are rank 1. Rank ties keep input order, and the walk emits pre-order, so a root-sheet name beats an equal-kind deeper name, which is also KiCad's rule.

## The hierarchy walk

The walk reads the whole sheet tree into one design. It is given the root file's content and an `open` function that fetches a child by its relative `Sheetfile` path. The registry resolves that path against the root's directory, so the reader itself does no file I/O. The traversal, the cycle guard, and the hierarchical sheet ids (`/`, `/<name>`, `/<name>/<name>`) are exactly the geometry walk's, so the netlist and the geometry agree on sheet identity. That shared identity is the join surface for sheet badges and cross-sheet navigation.

Per sheet instance (a reused file is walked once per placement), pre-order:

1. **Coordinate band.** Instance `k`'s geometry is offset by `k · 2^41` nm on X. Sheets are about 10^9 nm wide, so the bands never touch. Point-union stays within an instance, and only label-union crosses bands. One solve handles the whole design.
2. **Scoped collection.** The same collector the single-sheet read uses runs with a sheet scope: the band offset, the name prefix (empty for the root), and the instance path. Local and hierarchical labels emit prefix-qualified anchors. Globals and power symbols emit bare ones.
3. **Components.** Each placement's ref-des is resolved from the `instances` entry whose path matches this instance's uuid chain. The fallback is the first non-placeholder entry, then the Reference property (the single-file behavior). Components merge design-wide by resolved ref, and part libraries dedup by lib_id across files.
4. **Ports.** For each `(sheet ...)` block, the parent emits an anchor at every sheet pin's position, labeled `<childID>/<pin name>`: the same string the child's same-named hierarchical label produces under its prefix. Label-union joins the parent net to the child net, and rank 1 makes the port name the net's name when nothing better exists. This works per instance edge, so two `amp` instances bind their `CTRL` ports to different parent nets.
5. **Recursion and completeness.** A child that fails to open (or a `Sheetfile` cycling back to an ancestor) is skipped and flips `complete` to false. The rest still reads.

After the walk there is one solve, then pinless named nets are filtered (KiCad omits a label on a dangling wire), and dangling endpoints are translated back out of their bands (subtract `k · 2^41`, with the source file looked up by band index) so diagnostics carry sheet-frame coordinates the viewer can draw.

Completeness is a witness on the read. `external` on a net means "the read may not cover this net". A complete project walk (reading a `.kicad_pro`, opening every `Sheetfile`) makes that marking stale, so the project reader downgrades external to global. A sheetless root is trivially complete, so flat projects behave as before. A bare `.kicad_sch` read also walks (the registry passes the same opener) but never downgrades, because that file may itself be someone else's sub-sheet. The root sheet's own hierarchical labels stay external for the same reason: they continue upward into a parent nobody read.

The project reader always walks the schematic side even when a board is present. The board still supplies the nets, but sub-sheet components then arrive with their part types and typed pins instead of as bare footprint pads.

## Worked example

A root places `hier_child.kicad_sch` twice, as `amp1` and `amp2`. The child has two resistors (the instance paths give `R101`/`R102` in amp1 and `R201`/`R202` in amp2), a local `SIG` label, a hierarchical `CTRL` label, and a `VCC` power symbol. The root has `R1`, its own local `SIG`, a `VCC` symbol with a power-flag, wires amp1's `CTRL` pin to R1, and leaves amp2's `CTRL` pin unwired.

| net | members | why |
|---|---|---|
| `SIG` | R1.2 | root local, bare, scoped apart from the children |
| `/amp1/SIG` | R101.2 R102.2 | child local, instance-qualified |
| `/amp2/SIG` | R201.2 R202.2 | same file, second instance, second net |
| `/amp1/CTRL` | R1.1 R101.1 | port: parent pin anchor and child hier label share the string |
| `/amp2/CTRL` | R201.1 | port with no parent wire: child side only |
| `VCC` | #FLG01.1 #PWR01.1 #PWR02.1 R102.1 R202.1 | bare rank-0 rail unifies all three instances, virtual pins dedup |

The child's stray wire dangles at both ends in both instances, producing four diagnostics, each with sheet-frame coordinates and `hier_child.kicad_sch` as its source.

## Verification method

Unit tests pin each mechanism, but the acceptance test is interoperability. Flatten the corpus hierarchies with `kicad-cli sch export netlist` and compare components and every labeled net by name and membership. Both a 68-component reused-sheet hierarchy and an escaped-name design match exactly. The only deltas are stub conventions (the `N$k` and `unconnected-(REF-PadN)` forms versus KiCad's synthesized `Net-(REF-Pad)` forms), which are kept deliberately. The walked component set also equals the board-derived set, closing the loop with the `.kicad_pcb` reader. This method, not the unit tests, is what caught the mid-span label rule, the pins-are-endpoint-only rule, and the `{slash}` escapes.
