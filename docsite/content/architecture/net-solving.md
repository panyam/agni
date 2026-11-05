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

- A **Wire** is a segment between two grid points, optionally carrying an inline net name (xschem `lab=`) and the source's wire id (used for diagnostics).
- An **Anchor** is a named point (a label, a power tap, a port). It contributes a name, not a pin. It carries `Driver` (a "this net is fed" assertion, from a power-flag symbol), `External` ("this net continues into something the read did not cover"), and `Rank` (naming priority, below).
- A **Pin** is a component pin at a point, `(Comp, Pin)`, carrying `NoConnect` (a no-connect marker sitting on the pin) and `Dir` (a virtual power-symbol pin's direction).
- The **terminals** are points where a bare wire end is legitimate (junction dots, no-connect flags), used only by dangling-endpoint detection.

Solving is two unions and a naming pass.

{{ includeFile "figures/net-solve-two-unions.svg" }}

1. **Union by point.** Every wire unions its two endpoints, and anchors and pins join whatever node owns their point. After this pass, each union-find root is a geometrically-connected node.
2. **Union by label.** Every non-empty label unions all the points that carry it, wire labels and anchor labels alike. This is "connect by name", so every `GND` tap on a sheet is one net regardless of geometry. It is also aliasing. A node carrying two different labels folds both label groups into one net, so two labels on one wire are two names for one net. The same mechanism stitches sheets together in the hierarchy walk. There, two points in different coordinate bands share a label and become one net.
3. **Naming.** Each merged root takes its lowest-ranked label. Wire labels come first (rank −1, preserving wire-label-beats-anchor behavior), then anchors by `Rank`, with ties broken by input order. Unnamed roots that have pins get synthetic `N$<n>` names by first appearance, except a lone pin marked `NoConnect`, which takes the tool-marker name `unconnected-(REF-PadN)` so no-connect-aware consumers can key on it. Unnamed, pinless roots are drawing noise and are dropped.

Dangling detection is separate and purely positional. If a wire endpoint's grid point holds nothing else (no pin, anchor, terminal, or second wire endpoint), the endpoint is reported with its wire id.

The solve also returns a **wire-to-net-name map** keyed by wire id. The solve names the nets, and it also tells the geometry sidecar which net each drawn wire belongs to, so the viewer can tint or highlight a KiCad wire by net.

<details>
<summary>What populates that map, and the two traps in filling it from a hierarchy</summary>

KiCad wires carry no inline net name. xschem `lab=` and gEDA differ, since they fill the wire's net from its own label and pass empty ids here. Only identified wires populate the map, and a wire on drawing-noise is omitted.

In a hierarchy, the geometry reader runs the *combined* solve once rather than a per-sheet solve, so the names are byte-identical to the {{ explainable "netlist" }} read's (same `N$` numbering, same `/sheet/LOCAL` qualification). A reused sub-sheet's wires share uuids across its instances, so the geometry pass namespaces each wire id by its instance (the sheet path) before the solve. Otherwise two instances of the same sub-sheet would collapse onto one name.

</details>

The solver itself is format-agnostic. Everything KiCad-specific happens in the reader before the solve is called. That covers what counts as a connection point, how names are scoped, and how escapes work.

## KiCad connection-point semantics

These rules are pinned against `kicad-cli sch export netlist`, the reference implementation, because two of them are not what an endpoint-only solver would guess.

- **A pin connects where a wire ends on its connect point.** The connect point is `placement origin + (local_x, −local_y)` through the shared placement transform, the same math the renderer draws with. That shared transform guarantees a pin sits exactly on the wire endpoint it connects to.
- **A label or a junction dot connects anywhere along a wire.** eeschema does not rewrite a wire when a label is placed mid-span, and the real corpus has a label in the middle of a 70mm segment. The reader reconciles this with the point-identity solver by splitting each wire at every label or junction point lying strictly inside it. The split is done in sheet-frame coordinates only, because the collinearity check would overflow a 64-bit integer in the hierarchy walk's offset bands.
- **Pins do not connect mid-span.** An early split that included pins merged the showcase board's `USB_D-` net into {{ explainable "ground" "GND" }}, because a GND power symbol's pin sits mid-span on the D- wire and `kicad-cli` keeps them separate. Wire-to-wire crossings and endpoint-on-body touches without a junction dot are likewise unconnected, so other wires' endpoints are not split candidates either.
- **Net-name text is brace-escaped in the file.** KiCad stores a `/` in a label as `{slash}` (it collides with the hierarchy separator) and unescapes on load, so `VPP{slash}MCLR` and `VPP/MCLR` are one net. The reader undoes the escape table at every label and port read.
- **A bus is a drawing convention, and its members cross a sheet boundary by BIT POSITION.** `AN[0..7]` declares that `AN0`..`AN7` are drawn together, not that anything is connected; the members join through the tap labels. Two buses WIRED together on a sheet share the members whose names match, so a trunk and its slices read as one drawing. A bus crossing a sheet PIN pairs off positionally instead, by ascending index whatever direction either range is written in. So a parent bus `PP_OUT[8..15]` meeting a pin `PP_OUT[0..7]` puts the child's bit 0 on the parent's `PP_OUT8`. Only the `[first..last]` spelling is a KiCad bus, so a label written `DATA[1:0]` is an ordinary scalar net name, though xschem and gEDA do use that form. `bus-spelling-not-a-bus` reports such a label, since whoever wrote it meant a bus.
- **A gEDA part is mirrored and THEN rotated**, the shared transform's own order, so the gEDA reader passes its angle through unchanged. Lepton's netlist of `readers/geda/testdata/orient-matrix.sch`, one asymmetric part at every angle with and without a mirror, confirms it pin for pin (agni issue 1344). A gEDA attribute's angle in a `.sch` is the angle it sits at on the sheet, so the reader stores each field's angle less its part's turn, which the renderer adds back. EDIF's eight orientation codes follow the spec's matrices, and `orient-codes.eds` holds them.
- **A symbol is rotated and THEN mirrored.** The shared placement transform composes the other way round, mirror first. The two orders agree at 0 and 180 and disagree at 90 and 270, because a reflection and a quarter turn do not commute. Feeding the transform the INVERSE angle reconciles the two, since `M∘R` equals `R⁻¹∘M` for a reflection and a rotation. An unmirrored symbol takes KiCad's own angle. Drawing and connectivity read one transform (`kicadTransform`), so a drawn pin sits where it connects. Until agni issue 1322 the drawing negated every angle, so an unmirrored symbol at 90 or 270 drew turned 180 degrees from its wires. Otherwise a symbol placed mirrored at an odd quarter turn resolves its two pins onto each other's nets. No net COUNT can see that, because the swap moves one connection out of a net and another in.
- **A power symbol is an anchor plus a virtual pin.** Its value names the net (a power-flag names nothing but asserts the net is driven), and its power-typed pin also enters the design as a typed virtual connection so rules can see driver evidence. The symbol never enters the component list.

## Name scoping

KiCad has four name kinds, and they map onto familiar software scoping:

| kind | scope | net name | analogy |
|---|---|---|---|
| local label | one sheet instance | `/amp1/SIG` (root: `/SIG`) | instance-qualified member: same text on two sheets is two nets |
| global label / power symbol | design-wide | bare (`VCC`) | root-namespace global |
| hierarchical label + sheet pin | one instance edge | the child-qualified port name | function parameter: child declares the port, each call site binds it |
| {{ explainable "reference-designator" "ref-des" }} in a reused sheet | per instance | flat (`RV201` vs `RV301`) | the linker gives each inlined copy its own symbol, looked up rather than qualified |

Say a local label's text matches a global label or a power symbol drawn on the SAME sheet instance.
The local label joins that design-wide net as well. KiCad 10 does this and warns about the pair
(`same_local_global_label`), and `hier_samelabel_root.kicad_sch` holds the reader to `kicad-cli` on
five cases. A local `X` beside a global `X`, a local `VCC` beside a `VCC` power symbol, and a local
`W` beside a global `W` on a sub-sheet each join. Two cases are kept apart. A local `Y` on the root
and a global `Y` drawn only on the sub-sheet are two nets, and a local `VCC` on a sheet with no power
symbol is still `/sub/VCC`. HackRF's `!VAA_ENABLE` and the MNT Reform's two LPC UART lines each read
as two nets until this was followed (agni issue 1234).

The qualifier is the instance path (built from sheet names), not the file. A sheet file is a class
and its placements are objects. The root's path is `/`, so a root local is `/SIG`. Both
`kicad-cli`'s netlist and the `.kicad_pcb` spell it that way.

Every name follows KiCad 10.0's own rules (agni issue 1019, read from `connection_graph.cpp` and
`sch_pin.cpp`), and `TestFixtureNetNamesAreKiCads` holds the reader to `kicad-cli`'s netlist of every
fixture, regenerated with `hack/kicad_net_names.sh`. On the Jetson baseboard all 1355 names match.

- **Which label names a net.** A design-wide name (a global label or a power symbol) wins, then a local
  label, then a hierarchical label, then a sheet pin, and at equal priority the name nearer the root.
  A tie after that is alphabetical, so `+3.3V` beats `+3V3` and `/DCDC/X` beats `/Power/X`. The ranks
  live on each anchor and netgraph resolves them with `Options.AlphaTies`.
- **A net nobody labelled** takes a pin's name. A pin with a real name gives `Net-(D1-C)`, with the unit
  letter on a multi-unit part (`Net-(J1A-GPIO30)`) and the pad number added when the part has another
  pin of that name. A pin with no name gives `Net-(R6-Pad2)`. Among the pins the name without a pad
  number wins, then the alphabetically first.
- **A stub** is `unconnected-(...)`, with the pad number, when a no-connect flag sits on the net or one
  pin alone is on it, flagged or not. So the name says nothing about intent. A no-connect flag marks
  the net `no_connect` instead, and the rules read that mark.
- **A slash inside a label** is kept escaped, `FTDI_CBUS0{slash}SDA`, so a sheet path's `/` is never
  ambiguous.

Say a bus member enters a sub-sheet through a bus sheet pin. It keeps its own sheet's name, as KiCad
does, and joins the parent's member through an anchor that names nothing (`rankJoin`). If the parent
draws a label for the bus, that label competes as a local name on the parent instead.

## The hierarchy walk

The walk reads the whole sheet tree into one design. It is given the root file's content and an `open` function that fetches a child by its relative `Sheetfile` path. The registry resolves that path against the root's directory, so the reader itself does no file I/O. The walk shares its traversal, its cycle guard and its hierarchical sheet ids (`/`, `/<name>`, `/<name>/<name>`) with the geometry walk, so the netlist and the geometry agree on sheet identity. Sheet badges and cross-sheet navigation join on that shared identity.

Per sheet instance (a reused file is walked once per placement), pre-order:

1. **Coordinate band.** Instance `k`'s geometry is offset by `k · 2^41` nm on X. Sheets are about 10^9 nm wide, so the bands never touch. Point-union works within one instance, and only label-union crosses bands. One solve handles the whole design.
2. **Scoped collection.** The single-sheet read's collector runs here too, with a sheet scope made of the band offset, the name prefix (empty for the root), and the instance path. Local and hierarchical labels emit prefix-qualified anchors. Globals and power symbols emit bare ones.
3. **Components.** Each placement's ref-des is resolved from the `instances` entry whose path matches this instance's uuid chain. The fallback is the first non-placeholder entry, then the Reference property (the single-file behavior). Components merge design-wide by resolved ref, and part libraries dedup by lib_id across files.
4. **Ports.** For each `(sheet ...)` block, the parent emits an anchor at every sheet pin's position, labeled `<childID>/<pin name>`. The child's same-named hierarchical label produces the same string under its prefix. Label-union joins the parent net to the child net, and the sheet-pin rank makes the port name the net's name when nothing better exists. This works per instance edge, so two `amp` instances bind their `CTRL` ports to different parent nets.
5. **Recursion and completeness.** A child that fails to open is skipped and flips `complete` to false, and so is a `Sheetfile` that cycles back to an ancestor. The rest still reads.

### Buses cross by a different rule

A sheet pin joins a scalar signal by POSITION, because the parent drops an anchor at the pin carrying the child-qualified name, the child's hierarchical label emits the same string, and label-union does the rest. A bus pin cannot work that way, because a bus is a drawing convention rather than a wire and its members are not at the pin. Each member is tapped off the bus somewhere else on each sheet, under its own label. So members cross by resolved NAME instead, paired by the rules in the next paragraph. Inside the child, every member of a crossing bus resolves to its name in the parent.

KiCad spells a bus two ways, and they pair their members by opposite rules. A **vector** `AN[0..7]` takes its members from the index range, and two vectors pair by bit position, so a child's `B0` joins a parent's `A0` across a rename and joins `PP2` when the parent's bus is `PP[2..3]`. A **group bus** `CAM0{CSI}` takes its members from a `bus_alias` declaration, or from a list written in the braces (`ANALOG{A[0..5]}`, `SPI{SCK, MOSI, MISO}`), and names them `CAM0.CSI2_CLK+`. Two group buses pair by member name, so a parent of members `XX`/`YY` against a child of members `PP`/`QQ` joins nothing at all even though both are two wide. Both rules were measured against `kicad-cli sch export netlist` rather than reasoned about, and guessing either one wrong joins signals that the design keeps apart.

Two things decide whether a label names a bus at all. A bus label is only read off the BUS wire graph, so a label on an ordinary wire never promotes anything. And a brace pair is a group only when it does not follow `_`, `^` or `~`, because KiCad renders those as a subscript, a superscript and an overbar. So `A_{1}` and `SWD_TRG.~{RESET}` are scalar labels with the exact shape of `I2C0{I2C}`. The Jetson baseboard carries over a hundred distinct subscript labels (`1`, `CC`, `CLR`, `CS0`). Inside a group the braces either name an alias or list members, separated by spaces or commas. A member may be a vector, listed or from an alias, and a vector member expands. So Cynthion's `ULPI` alias, whose member is `DATA[0..7]`, crosses a sheet pin as `DATA0` to `DATA7`. Before agni issue 1234 its 32 PHY and RAM data lines each read as two one-pin nets. A listed member may also carry formatting of its own (`SWD_TRG{~{RESET}, SWDIO, SWDCLK}`). The RoyalBlue54L Feather crosses every sheet that way. Before the reader handled the inline form, every header signal on it came apart into two one-pin nets (agni issue 936). The group is the LAST brace pair, because a prefix can carry a subscript of its own. `I2C_{SYS}{I2C}` has that shape, and it is the most common group bus on that board, 33 of its 224 occurrences. Aliases resolve project-wide rather than per file. The sample board has fourteen sheets that use a group bus, and nine of them declare no alias at all. Resolving them per file recognized 12 of that board's 251 split members.

A bus with no label on it takes its name from the sheet pins it joins. For a group bus that name covers the whole connected bus, and it is the first of its pins' names in sorted order. So two sibling sheets that spell one bus differently still meet by member name. The Feather joins `I2C{SCL, SDA}` on one sheet and `I2C_54{SCL, SDA}` on another over a bare bus wire, and KiCad's netlist makes them one net. A vector pin on an unlabelled bus keeps its own name, since nothing has measured that case.

Where the drawing does not say which branch a pin sits on, nothing is promoted. Two cases resolve to no promotion and leave the members split. In one, two different bus labels tie at the same distance from the pin. In the other, one sheet file is placed several times, so every instance's pin is spelled identically. Splitting is the safe direction. A split net reports as stubs and someone notices, while a wrongly joined net is a short that every later rule believes.

{{ includeFile "figures/hierarchy-bands.svg" }}

After the walk there is one solve. Then pinless named nets are filtered out, since KiCad omits a label on a dangling wire. Dangling endpoints are translated back out of their bands by subtracting `k · 2^41`, with the source file looked up by band index. So diagnostics carry sheet-frame coordinates the viewer can draw.

Completeness is a witness on the read. `external` on a net means "the read may not cover this net". A complete project walk reads a `.kicad_pro` and opens every `Sheetfile`, which makes that marking stale, so the project reader downgrades external to global. A sheetless root is trivially complete, so flat projects behave as before. A `.kicad_sch` read also walks (the registry passes the same opener). It downgrades only when its own same-stem `.kicad_pro` sits beside it, since that file names it the project's root (agni issue 1013). A schematic with no project beside it never downgrades, because it may itself be someone else's sub-sheet. For the same reason the root sheet's own hierarchical labels are still external, since they continue upward into a parent nobody read.

The project reader always walks the schematic side even when a board is present. The board still supplies the nets, but sub-sheet components then arrive with their part types and typed pins instead of as bare {{ explainable "footprint" }} pads.

## Worked example

A root places `hier_child.kicad_sch` twice, as `amp1` and `amp2`. The child has two resistors (the instance paths give `R101`/`R102` in amp1 and `R201`/`R202` in amp2), a local `SIG` label, a hierarchical `CTRL` label, and a `VCC` power symbol. The root has `R1`, its own local `SIG`, a `VCC` symbol with a power-flag, wires amp1's `CTRL` pin to R1, and leaves amp2's `CTRL` pin unwired.

| net | members | why |
|---|---|---|
| `SIG` | R1.2 | root local, bare, scoped apart from the children |
| `/amp1/SIG` | R101.2 R102.2 | child local, instance-qualified |
| `/amp2/SIG` | R201.2 R202.2 | same file, second instance, second net |
| `/amp1/CTRL` | R1.1 R101.1 | port: parent pin anchor and child hier label share the string |
| `/amp2/CTRL` | R201.1 | port with no parent wire: child side only |
| `VCC` | #FLG01.1 #PWR01.1 #PWR02.1 R102.1 R202.1 | bare rank-0 {{ explainable "rail" }} unifies all three instances, virtual pins dedup |

The child's stray wire dangles at both ends in both instances, producing four diagnostics, each with sheet-frame coordinates and `hier_child.kicad_sch` as its source.

## Verification method

Unit tests pin each mechanism, but the acceptance test is interoperability, and the oracle is free. Every KiCad project carries both the schematic and the `.kicad_pcb`, and the board file holds the netlist as KiCad itself resolved it. So any real board is a reference answer with no fixture to build. `make oracle` is that cross-check; `readers/kicad/oracle_corpus.baseline` records what still disagrees, net by net.

**Compare the PARTITION of pins into nets, never the names and never the counts.** Both of those look like they would work and neither does.

Names cannot match. An unnamed net is auto-named by each tool in its own vocabulary, `N$37` against `Net-(C104-Pad1)`, and `kicad-cli`'s netlist export adds a leading `/` to root locals that the board file does not carry. Counts hide compensating errors, and that is the more expensive half. A swapped pin pair moves one connection out of a net and another in, so on one demo board we read 47 nets against KiCad's 47 while disagreeing about 19 of them. A matching total is not agreement.

The oracle, and not the unit tests, caught the mid-span label rule, the pins-are-endpoint-only rule, the `{slash}` escapes, the bus rules above, and the mirrored-placement pin swap.

**Ask `kicad-cli` rather than reasoning about the format.** Sign conventions, a Y-flip, and two composition orders are in play, and a plausible derivation has been wrong more often than not here. Build the smallest schematic that distinguishes the cases, export a netlist from each variant, and read the answer off it. A fixture with labelled wire stubs at the compass points reports where a pin ended up; one with a tap per bus member reports which members crossed.

## How far a walk crosses series parts

Several rules ask whether one net reaches another through series parts. Examples are a clamp near
a connector pin, a regulator feeding a load, and a pull-up returning a bus to its rail. The walk crosses a part
terminal to terminal only when the part is a series pass element (resistor, inductor, ferrite,
fuse). A capacitor is a DC block, a diode has polarity, and anything active is not a wire, so none of
them is crossed. Each crossing is one hop, and the radius a rule asks at is an electrical claim
rather than a search budget.

| Constant | Hops | Asked by | Why that number |
|---|---|---|---|
| `ProtectionReachHops` | 2 | every protection guard (clamp, fuse, power pin near a net) | a discharge arrives at a connector pin as a fast transient, and every series element before the clamp is impedance the surge pushes through before the clamp conducts. A TVS six resistors away protects what is downstream of itself, not the pin |
| `SupplyPathReachHops` | 1 | supply-compatibility rules (WS3-028) | a bead or series resistor between a regulator and its load is ordinary. Voltage does not degrade along the path, so a wider radius makes every part look fed by every regulator |
| `PowerPathReachHops` | 3 | the power-entry walk | a power entry legitimately crosses connector, fuse and bead before the regulator's input node, and asking at 2 stops short of the regulator |
| `PullUpReachHops` | 3, or the net's declared reach | the I2C pull-up requirement, the pull-up count rules, and a profile's `missing-pullup` | a pull-up directly on the bus is 1, behind a series isolation resistor is 2, and past 3 the series resistance is comparable to the pull-up, so the bus no longer returns high in time |

The radii differ in opposite directions for opposite reasons. A protection question tolerates
distance badly because the surge's effect grows with every element. A supply question tolerates it
badly because voltage does NOT degrade, so distance stops meaning anything. **Widening any of them is
the silent direction.** A wider protection radius credits distant clamps and turns an unprotected
pin into a clean pass.

A design can move the pull-up radius, and only one net at a time. Some buses reach their pull-up
through a deeper series network, such as an isolation resistor followed by a level-shifting network,
and the designer has sized it so the bus still returns high. A design says so with
`pull_up_reach_hops` on that net in its intent (agni issue 820, see
[design intent](../../guide/design-intent/)). The walks read the limit through
`Model.PullUpReachLimit`, so the rule, its `check.Spec` twin, the count rules and the profiles all ask
at the declared radius, and a failing verdict states the limit it searched to. The declaration is per
net rather than a project setting, so every other net keeps the default.

A passing pull-up verdict lists every distinct path within that radius, one per terminating resistor,
ordered by hop count, then resistor, then rail (agni issue 821). A bus with a direct pull-up and a
second one behind a series resistor, or with pull-ups to two rails, shows each path as a `path` term
rendered by `model.RenderRoute`. A bus with one path reads exactly as it did before.

The walk also refuses to cross INTO a net that looks like distribution rather than a series path.
Such a net is a ground, a net marked global, or a net with more than 16 connections (`maxWalkFan`). A series node
carries a handful of members (a fuse, a bead, a clamp, a cap or two and the load), where a rail or bus
carries dozens. Take a name-only rail, such as an EDIF `+5V` with no attributes. Without that guard
it turns a pull-up into a doorway to the whole design. The query relation `net.bus_like` reads the same
definition. The start net is never treated as a stop.

Two radii elsewhere are search budgets and are wide on purpose. `agni trace` searches 6 hops by
default and prints the radius with its answer, since a route eight crossings long is still the
route. The `net.reaches` relation searches the whole series neighbourhood (100 hops), because "what is
connected to what through passives" is a topology question and not a claim about protection.

## Walking outwards from a part

A trace needs both ends. On a large board a reader often knows one part and wants to see what is
around it, so `agni walk` and `DesignService.WalkDesign` start from a part or one pin and widen hop
by hop (agni issue 1339). Hop 1 is the start's own nets. Each later hop crosses the parts on the
previous hop's nets into their other nets, and records the part and its pin on each side.

Which parts a hop crosses depends on the question, so the walk has kinds, and `ListWalkKinds` lists
them for a viewer to offer.

| Kind | Crosses | Stops at | Answers |
|---|---|---|---|
| `series` | the series pass elements a trace crosses, when the part touches two nets | grounds, global nets and fan-outs over 16, as trace does | what a signal passes through |
| `small-parts` | any fitted part with up to four connected pins, capacitors and diodes included | ICs, connectors, rails and grounds | the circuit around a part |
| `all` | every fitted part, 64 new nets a hop at most, with the rest counted | rails and grounds | the raw neighbourhood |

A part the kind refuses is listed once with its reason, at the first hop that met it, and naming it
in `through` crosses it on the next ask. The series walk passes through a rail-named node such as
`VBUS_PROT`, as a trace does, so the two agree. The other kinds stop at a rail, because a walk that
widened through a 3V3 rail would list every part the rail feeds. A branch that stops at a rail or a
ground says so in `ended`, rather than leaving the net out. A do-not-populate part is never crossed
unless named. Each net carries the sheets it is drawn on, as a trace's nets do.

A level translator or a buffer carries one signal per channel, so crossing it to every pin would
walk from one I2C line onto the next bus over. The naming lexicon's `pin.through_pair` pairs such
pins by name (`A{n}` with `B{n}`, `{n}A` with `{n}Y`, `SDA1` with `SDA2`), and `Model.ThroughPartner`
answers a pin's partner. A walk that crosses a part from a paired pin goes on through the partner
alone, and marks the crossing `paired`. The `small-parts` walk crosses such a part on its own,
since a translator channel carries the signal as a resistor does, and the `series` walk crosses one
only when named in `through`. On a part with pairs, an unpaired pin such as a supply or an enable
carries no signal across, so the walk crosses nothing from it. Only the crossed channel is marked
crossed, so the walk can come back across the part's other channels. The `all` walk ignores pairs.
On Jetson, the walk from the I2C2 pull-up R352 crosses the translator U80 from A2 to B2 alone.
