# readers/kicad

This file covers the KiCad schematic and board readers, and kicad-cli as their oracle.

- **A KiCad design reads its same-stem `.kicad_pro` whichever of its files names it** (agni 933). It reads net classes, class definitions and the board's own minimums. The copper rules check `board_rules` before their fixed floors. A sub-sheet has a stem of its own, so it reads alone.
- **KiCad names a local label by its sheet path.** A root-sheet net is `/SCL`, and every finding, query answer and link names it that way. `render --highlight net=SCL` matches nothing and still reports a highlight (#1225), so spell a hand-typed subject as `check` prints it.
- **A label's text is also its net's name, so KiCad markup is drawn and never stripped** (agni 1344). `~{RESET}`, `V_{OUT}` and `{slash}` stay in the text. `SchematicGeometry.text_markup` marks the text as KiCad's, and `core/render/markupRuns` splits it. The packer sends the runs to the overlay in `PackedLabel.runs`, so the viewer never parses markup.
- **A placed pin's alternate rides on `SymbolPlacement.pin_names`.**
- **A new s-expression a fixture uses goes in `census/manifests.go`**, or `TestFixtureCensus` fails.
- **A symbol is rotated and then mirrored**, while the shared transform composes mirror first (agni 577). The reader passes the inverse angle, so the renderer needs no change.
- **A bus vector is spelled `[first..last]` and only that.** A KiCad label written `DATA[1:0]` is a plain scalar name, though xschem and gEDA use that form. The reader records such a label in `range_labels`, and `bus-spelling-not-a-bus` reports it (agni 605).
- **Two buses wired together share the members whose names match.** A bus crossing a sheet pin pairs off by bit position, in ascending index. `architecture/net-solving.md` has all of the bus rules.
- **A net count never shows a placement or bus error.** A pin swap moves one connection out of a net and another in, so the count matches while nets are wrong.
- **A drawn bus takes its name the way the solver names a branch** (agni 1435, `readers/kicad/sch_bus_geom.go`). A piece of bus takes the nearest bus label along the bus, vector or group, on any kind of label. An entry takes the name at its bus end. That name is the bus's join key (`WireGeometry.net`) in both renderers.
- **KiCad 10 writes a field's `(hide yes)` in the `(property …)`**, where KiCad 9 and earlier write it inside `(effects …)`. `fieldHidden` reads both, or every footprint name and datasheet URL draws (agni 1560).
- **KiCad 10 keeps bus aliases in the `.kicad_pro`** (`schematic.bus_aliases`), not in the schematic. `projectBusAliases` reads both, and a test opener must return an error for a missing file rather than fail the test, since every walk asks for the project file (agni 1560).
- **A group bus's members go in `InputDiagnostics.group_buses`, never `unmodeled_buses`**, because `bus-not-modeled` must not judge a group bus (agni 939). Each member takes the name of the net that carries it (`Net.Aliases`).
- **A footprint's angle carries over unchanged** (agni 1261, `placementAngle` and `footprintLocal` in `readers/kicad/pcb_geom.go`). KiCad's angle is counter-clockwise as drawn, and the Y flip preserves the picture. Negating it swaps pad 1 and pad 2.
- **A back footprint's items are un-mirrored.** KiCad stores a flipped footprint's items already mirrored, so the reader negates their x and the angle, and the contract's composer mirrors once.
- **No rule reads a pad's position.** Only the board view and `TestODBReadsAsItsKiCadSource` see a pad placement error. That test holds pad positions against kicad-cli's own export.
- **A pad's angle is in its footprint's frame**, as the proto says (agni 1407). KiCad and ODB++ write the board angle, so their readers subtract the part's angle (`geomath.PadLocalAngle`). Every consumer draws through `geomath.PadAngle` and places a padstack land's offset and outline through `geomath.PlaceLand`.
- **Test a pad angle at 30°.** A rectangle at 90° looks the same turned either way.
- **The netlist keeps a do-not-populate part, and a rule crediting a part asks `Model.IsFitted`** (agni 938). The reach walk crosses no unfitted part, and a query says the same with `not component.dnp(?r)`.
- **A test point keeps its credit when unfitted.** A pad-only test point is marked DNP because nothing is placed, and a probe touches the copper.
- **A rule whose credit changed also leaves out a net with no fitted part** (`check.Unpopulated`, spec fact `net.unpopulated`). Crediting alone would fail a circuit that is unfitted as a whole.
- **A rule that blames a part judges no unfitted part.** A rating or orientation rule is wrapped in `check.OnFittedParts`. The wrapper is opt-in, since two unfitted parts sharing a ref-des are still a defect. `floating-input` counts fitted pins, a connector's included (agni 1082). `architecture/rules-and-checks.md` states the policy.
- **kicad-cli is the oracle for a render.** A rendering PR shows kicad-cli's frame beside ours (CONTRIBUTING.md).
- **kicad-cli's `sch export svg` is the oracle for where KiCad draws text** (agni 1344). It writes every run as an invisible `<text>` at its baseline, half a glyph below its centre, with its anchor. The rotation sits on the enclosing `<g transform="rotate(a x y)">`, about a pivot that need not be the run's own point.
- **kicad-cli draws only the library symbols a sheet embeds.** A hand-written fixture often has an empty `(lib_symbols)` cache, and `design.yaml` names a `.kicad_sym` for agni. kicad-cli then draws labels and wires with no symbol bodies. Copy the library's symbols into a scratch copy as `<lib>:<name>` first, as the tutorial gateway needs.
- **Export from a scratch copy in any case.** kicad-cli writes a `.kicad_prl` beside the board.
- **Gateway's Edge.Cuts outline is a filled `gr_rect`**, and it plots over the copper. Export the copper and silk layers without Edge.Cuts.
- **A text is mirrored exactly when its effects say `(justify mirror)`.** kicad-cli confirmed it on a back footprint that names none.
- **A board frame from `pcb export svg --mode-single` names F.Cu and B.Cu and no inner layer** (agni 1486). With an inner layer named, the plot paints inner pours over F.Cu. F.Mask plots as a solid sheet too. `--fit-page-to-board` crops any text past the outline.
- **`make oracle` is a separate suite and is not in the gate.** It compares the reader's pin-to-net partition against each real board's own `.kicad_pcb`, never net names or counts. It asserts `readers/kicad/oracle_corpus.baseline`, a committed list of the nets the reader still gets wrong. `AGNI_ORACLE_UPDATE=1 make oracle` rewrites it. It also runs `odb-oracle`.
