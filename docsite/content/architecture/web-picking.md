---
title: "Picking and querying"
description: "How a reader names an entity in the viewer, walks from one answer to the next, searches by name, and sees what is already known about a selection."
---

A reader names an entity in the viewer in three ways: by clicking the drawing, by clicking a cell in
a query result, or by searching for a name. All three produce the same `Selection` value. A click
SELECTS, and the Properties panel describes the selection; a question about it is a separate act
that leaves an editable datalog query in the box. This page covers those paths, and what the viewer
can then say about what is selected. The render and highlight contracts underneath them are on
[Web app and presenter](../web-app/).

## Picking from the drawing

A reader asks about an entity by clicking it, and the rule is that **each renderer owns its own
selection data**. The SVG backend keys every element it draws (`data-kind`, `data-ref`, `data-pin`,
`data-net`, `data-net-id`, `data-bus`); the WebGL backend has `PrimitiveKey`, whose proto comment has
always said it exists for picking. Neither reads the other's representation, so either can be retired
without stranding the other, and a keyed SVG is self-describing wherever it is embedded.

The renderers share the contract rather than the index. The contract is a `Selection`, which has the
same shape as a finding's subject, plus the priority order and the intent. So a canvas click, a
search result and a finding row produce one value.

- Picking goes by **priority, not topmost**. A symbol is drawn over its own pins, so topmost-wins
  would make a pin unclickable. Resolution goes pin, component, {{ explainable "bus" }}, net, most
  specific first.
- **A click is not a pan.** The press/release pair counts as a click only if the cursor moved less
  than a few pixels. Otherwise a pan that ends over a wire would select it.
- **Two pick aids are opt-in**, through `render.WithPickTargets`, and only the served viewer asks for
  them. A pin has no drawn element in a faithful render, so it gets an invisible circle. A wire is a
  0.8px stroke, and `fill="none"` hit-tests only ON the stroke. We measured a probe in a browser at
  a wire's own midpoint, rounded to whole pixels, and it hit the page rect. So a wire gets an
  invisible wide companion with `pointer-events="stroke"`. A render for a file or a report has
  neither aid, because it should not carry the viewer's interaction model.
- **Entity keys are design data, so they are escaped** (`svg.AEsc`). Attribute values are written
  verbatim otherwise, and the viewer mounts this document with `innerHTML`.

### Picking the board

The board view picks with the same `Selection` and the same priority (agni issue 1324). Under
`render.WithPickTargets`, `BoardSVG` keys each element as the schematic does.

- A pad is a pin of its footprint. Its pad number is the pin designator, and `HighlightBoardSVG`
  matches a pin on that same pair. The pad also carries `data-net`. A pad with no number, such as a
  mechanical hole, keys as its footprint, since a pin needs both halves.
- A track, a via and a zone each pick their net. A via's drill hole draws over its ring, so the
  hole carries the key too.
- A footprint's body is an invisible rect over its pads' bounds, drawn above the zones and under the
  copper. A click between two pads then picks the part, and a pad or a track drawn over the body
  still wins where it is drawn.
- **A pour yields to the part under it.** A pour is filled copper drawn with its layer (#1357), so an
  Allegro board's ground plane lies over every footprint body on its side, and an inner plane over
  every body on the board. `pickAt` looks under a pour for a footprint body and picks the part when
  one is there, so a click between a part's pads on Clay R6 picks the part rather than GND (agni
  issue 1382). A pour where no part lies still picks its net.
- **The silkscreen and the designators pass clicks through** (`pointer-events="none"`), and carry no
  key. On a dense board a designator lies over its neighbour's pads. On Jetson, C123's designator
  covers C133's pads, and while board text was keyed a click on C133's pad picked C123 (agni issue
  1347). The schematic leaves its labels unkeyed for the same reason.
- **A net's tracks share one keyed group per layer.** Tracks are most of a board's elements. A key on
  each line grew Jetson's board SVG from 8.3 MB to 12.1 MB, and one per group grows it to 9.5 MB. So
  `selectionFromElement` reads the nearest keyed element, walking up from the hit and stopping at the
  drawing's `<svg>`. An area counts a grouped net when any one of its lines lies inside, as a
  schematic net counts by its wires.
- **A hidden layer picks nothing.** The Board entry in the Sheets panel lists the board's layers
  (agni 1351). Hiding a row writes a `display: none` rule for that row's `svg_selector`, and
  `elementFromPoint` skips a `display: none` element, so a click there reaches what lies beneath or
  nothing. The body rects split into `picks-front` and `picks-back` by the side the part sits on, and
  the F.Cu and B.Cu rows name them, so a part hides with its side's copper.

A via or a short track at fit zoom is a few pixels across. A mouse event carries whole-pixel
coordinates, so a click can miss a via that a fractional probe hits. The reader zooms in to pick one,
and `web/browser/board-pick.spec.ts` searches whole-pixel points for that reason.

A pick on the board in a sheet view moves the canvas to the schematic sheet that draws the part,
and a pick on the schematic moves a board view to the footprint (agni issue 1326). Each view decides
for itself which sheet to show, from `LocateSubjects`.
`architecture/web-client.md` has the sheet views.

### Selecting is not asking

A click selects the entity under it, and the Properties panel describes it (agni issue 902). A click
used to write the served preset into the query box and run it. That mixed looking around with asking
a question, and a stray click overwrote whatever the reader had typed. Asking is now one of the
panel's Ask rows. The server words and writes each row's question (several per kind,
`EntityQuery.label`), and each row opens a query tab of its own. So the query is still left in a
box, editable, and using the viewer still teaches the language.

**Something is always selected.** The design is the root, a click on empty paper selects the sheet on
screen (`SvgView.onPickEmpty`), Escape selects the design, and an entity sits under the sheet it was
picked on. The breadcrumb at the head of the panel moves between those levels. Every reference in it,
such as a pin's net or a net's components, moves the selection, so a reader can walk the design
inside the panel.
A located query cell selects too, and so does a finding clicked in the Checks or Review panel, so the
panel follows a walk through an answer or a run.

**Where a part or net is drawn is asked once per selection.** Properties lists those sheets, each a
link to it, from `DesignService.LocateSubjects` (agni issue 1325). That rpc and a check run's
findings read one index, so a subject locates where its finding badges. A pin answers the sheets
that draw its unit, so U1's pin 5 answers U1B's sheet. The board sheet joins when the request asks
for it, by ref-des and net name, since an EDIF schematic and an ODB++ job share no net id. The
viewer asks for it while the board is on screen, because the board is read then, and asking from a
schematic would read a large board for one panel row (agni issue 943). A check run that read the
board badges its findings with the board sheet too (#142's WS7-034). Before 1325 the viewer asked
the find-by-name query and read the sheets off the answer's entity, and a query never reads the
board. Properties lists a bus by its members instead, and reads them off the IR's record of the
bus (`InputDiagnostics.unmodeled_buses`). The `bus` relation records only the construct the format
reader saw, and nothing about its members.

**A selected net, part or pin is checked on its own** (agni issue 897). Properties asks
`CheckDesign` for the rules ticked in the Rules panel, with `subjects` naming the selection. It lists
each result that fails or could not decide, and clicking one shows its proof on the drawing. A count
of every outcome sits below the list. Properties names each rule the answer skipped, with the
engine's reason, so a rule that concludes about the whole design (`symbol-unresolved`) reads as not
asked rather than as passing. The viewer holds the answer apart from the Checks panel's run
(`Document.scoped`), because a rule's answer for one net is not its answer for the board. The viewer
drops the answer once the reader leaves that selection or changes the rules. A scope covers no bus,
so a selected bus keeps the run's findings. The design is read before the viewer opens (agni issue
1072). On Jetson in the browser the first selection then costs about 1.6 s, which goes to the
restore and the model build, as it does on a first Run checks. Each later selection costs about
300 ms.

**The panel reads the design once per open** (`PropertiesPresenter`, `INVENTORY_PATHS`). It asks
`GetDesign` for the netlist IR under a field mask naming only what it shows, about 1.8 MB of JSON on
the largest seed, and indexes it. A per-click read was the alternative, but a component's nets cannot
be filtered on the server by component, so it would have fetched every net per click anyway. The
panel reads a sheet's contents off its rendered document's entity keys (`drawnOnSheet`), since
membership is a property of the drawing.

**The datasheet section keeps its outcomes apart.** Four cases each read differently. No corpus is
connected, the part has no MPN, the corpus has no spec for this MPN, or the spec is there. An unasked
question and an absent answer must not look alike. `GetComponentParamsResponse.corpus_connected`
tells the first case from the third.

### Selecting an area

**A Shift-drag selects every part, pin and wire lying wholly inside a rectangle on the sheet on screen,
and a Shift-click adds an entity or takes it out** (agni issue 904). A plain drag still pans and a
plain click still picks one entity. The viewer reads the set off the rendered document's entity keys
(`membersIn` in `web/src/area.ts`) and measures each element's client rectangle, so forming the set
asks the server nothing. A part drawn as several shapes is inside only when every shape is. If a
part is only partly inside, its pins inside the rectangle are members on their own. A wire is
identified by its net, since the renderer keys a wire with nothing finer.

**An area is a `Selection` of kind `area`**, carrying its parts (`refs`), its loose pins (`pins`), the
nets with a wire inside it (`wires`) and every net it touches (`nets`). So it goes wherever a
selection goes: Properties describes it, the drawing lights its members, the selection bar names it,
and the breadcrumb reads `Design › Sheet › Area (N)`. When the reader follows a member from the
area, the member keeps the area as its parent crumb (`Focus.within`), so the Area crumb steps back
out.

**A net is classified, never contained**, because it may continue off the sheet. It is internal when
every one of its pins is in the area, through a member part or a member pin, and on the boundary
otherwise. The boundary nets are the block's interface. A net with a wire through the area and none
of its pins inside is on the boundary too.

**An area's Ask rows bind the whole set at once**, `?ref` to its parts and `?net` to its nets, so a
question is evaluated once and an aggregate reduces across the area. The questions live in
`core/query/entity.go` under kind `area`, like every other kind's. Two engine behaviours shaped them.
A variable bound to a single value is substituted as a constant, so a question never aggregates a
bound variable (the class question counts a `?part` filtered to `?ref`). And a question projects a
bound variable only for a set, because there the question is which value answered.

On a drawing agni laid out itself, nearness says nothing about the design, so Properties says so when
an area is selected on one.

### Walking from an answer to the next question

A result cell is the second way a reader names an entity. The server types each answer column with
an entity kind (`RunQueryResponse.column_kinds`, derived from the relation catalog's arg labels). So
a component or net cell reads back as a `Selection` (`selectionFromCell`), and a canvas click
produces the same value. Clicking a cell locates it, as it always did, and now also selects it. The
query panel then names what is selected and offers the kind's first served question. Taking that
question opens a tab and runs it, and the reader gets a fresh set of cells to walk from. The
preset's text is the same for every pick. It names the variables to bind (`EntityQuery.binds`), and
`entityBindings` fills each from the selection field of the same name. So the pick reaches the query
as a value shown in a chip rather than spliced text (agni issue 793). The list is explicit because
one preset's input is another's output. The pin preset answers `?net`, and the net preset asks
about one.

Two things are deliberate here. A cell click does not itself re-run the query. The locate affordance
exists so a reader can scan a result set and highlight each row in turn, and a click that replaced
the table would take that away. And the question's wording comes from the server with its query
(`EntityQuery.label`). A kind now has several questions, and a client copy of their wording would
drift from the server's list.

### Each question is a tab

The query panel hosts any number of tabs (`querytabs.ts`). An Ask row, the selection bar's question
and "+" each open one, so a question never overwrites another, and each tab keeps its own query,
bindings and answer. The tabs are the panel's own strip rather than dockview tabs, so a saved dock
layout holds none of them, and the last one has no close button.

The presenter answers a run in the tab that asked (`runQuery`'s `tab`, `QueryView.setState(result,
tab)`). An answer that arrives after the reader moved to another tab still goes to the tab that
asked, and Save CSV saves the answer of the tab it was pressed in. **The drawing follows the tab in
front.** A tab remembers the cell it last located, and bringing it forward locates that cell again.
A tab that located nothing releases what the previous tab had located (`releaseFocus`), back to the
findings alone, so the canvas never shows a cell from an answer no longer on screen.

**A pin cell takes its other half from its row.** A pin's identity is two fields, so the cell holding
`5` names nothing until you know it means U7's pin 5. For a long time that left a pin column typed
as a scalar. `QueryRow.cell_refs` carries the component per row, and `selectionFromCell(kind, cell,
ref)` reads both.

<details>
<summary>Where the ref comes from, what counts as a pin column, and why a pin cannot be searched by name</summary>

`cell_refs` is a sibling of `cell_kinds` rather than the same mechanism. A pin column's KIND is
fixed, since every row of it is a pin; what varies per row is the REF. The ref may come from a
sibling column (`pin.net(?ref, ?pin, ?net)`), from a constant (`pin.net("U1", ?pin, ?net)`), or from
a variable left out of the projection, so it resolves against the row's bindings rather than against
the visible columns.

The pairing also decides what counts as a pin column, since a `pin` position is a DESIGN pin only
when the same atom also carries a `ref_des`. `param.pin(mpn, pin, name, function)` and
`param.pin_range` pair `pin` with an `mpn` instead, and theirs is a pin of a part TYPE off a
datasheet, with nothing on any canvas to highlight.

Downstream, a pin takes its sheets and its locate reason from its component's placements, so a pin
of a virtual `#PWR` symbol reports VIRTUAL_SYMBOL rather than a missing render. Asking either
question about the designator would be asking about a thing called `5`. A pin whose component does
not resolve is shown as plain text, since half a pin is not a less precise pin.

Searching for a pin by name is still not possible. `entity(name, kind)` deliberately does not
enumerate pins. A pin cannot be one `name` unless we invent a composite string, and nothing else in
the fact base joins against such a string. `component.pin(?ref, ?pin)` already enumerates pins for
anyone writing the query by hand.

</details>

**The table marks where the reader is standing**, because a click sends the canvas somewhere and a
forty-row result otherwise says nothing about which answer it came from. A cell is marked when
`sameSelection` matches it against the current selection, and a sheet badge when its sheet is also
the one on screen (`QueryView.setCurrentSheet`, pushed from `showSheet` on EVERY navigation).

Both marks are DERIVED rather than remembered. A remembered mark is wrong the moment the reader
picks something on the drawing or opens a finding, and wrong silently. Two consequences follow and
both are deliberate. An entity answering in several rows marks all of them, which is true, since
they all name the thing being shown. And when a marked badge falls past the strip's three-chip cap,
the strip shows more chips to include it. A mark nobody can see is worse than no mark, because the
reader would read the unmarked strip as being somewhere else.

### Searching by name

A click says where to look and gets back what is there. A search says what a thing is called and
gets back where it is. Both write datalog into the box and run it. So search is a MODE on the query
panel rather than a widget of its own, and the reader keeps an editable query either way.

The panel's "Find by name" mode takes a term, binds it into the served query
(`ListRelationsResponse.search_query`, from `query.Search()`), runs it, and hands the panel back to
query mode so the reader ends up looking at the sentence that answered them, with the term as a
binding chip beside it. The query text never changes. The client binds `?pattern` to the served
`pattern` with `{term}` replaced (agni issue 793), so the `(?i)` lives on the server once. The template ranges over
`entity(?name, ?kind)` because every other relation ranges over an ASSOCIATION, so a search built on
one silently cannot find a part with no connections or a net with nothing on it. It matches with
`str.match` and `(?i)` rather than `str.contains`, so case does not have to be guessed at and a reader who
wants `^U` can write it. The client regex-escapes the typed term (`searchPattern`, mirroring Go's
`regexp.QuoteMeta`), because `VDD+` and `DATA[7:0]` are ordinary names here. Escaping is still
needed because the bound value is a regex. A double quote is no longer special, since nothing is
spliced into a string literal.

**A search result's rows are not all the same shape**, and no other answer set is like that. Kind is
normally a COLUMN property, since a variable binds at the same relation position in every row. Under
`entity(?name, ?kind)` one answer set holds a component, a net and a bus, so `column_kinds` has
nothing true to say about the name column and types it scalar, which is to say unclickable.
`QueryRow.cell_kinds` is the per-row override. It is empty for every ordinary query, and the client
reads it through `cellKind(result, row, i)`, so the fallback to `column_kinds` sits in one place. It
types the sheet-badge lookup and the locate reason as well as the click, since a hit nobody can
navigate from is only a third of an answer.

A bus would have broken quietly. It is neither a placement nor a named wire, so neither drawn-entity
set can speak for it. The test is whether it resolved to a sheet at all, and `AnnotateSheets`
applies the same rule to a bus finding.

### What is already known about a selection

A selection now also answers "what has been checked about this" (agni issue 259). The query panel's
selection bar carries a findings count for the picked entity, and the results footer carries one for
every entity the query returned.

**It is a projection of one evaluation and never a scoped re-run.** Every `Finding` carries exactly
one `Subject`, so grouping by subject partitions the findings, and the client already holds the whole
list. `findingsFor(findings, selections)` in `findings.ts` is the filter. The presenter fans ONE
`FindingsState` to both the checks panel and the query panel from a single `pushFindings`, so the two
cannot disagree.

<details>
<summary>Why a scoped re-run was ruled out, and why the filter takes a set</summary>

The alternative fails in three ways at once. A scoped run resolves config independently and can
contradict the report beside it, and C25 exists to prevent that disagreement. It redoes net solving
and reach walks once per click. And it makes "the union of what I clicked equals the full pass" a
hope rather than a property.

`findingsFor` takes a SET rather than one subject, because a set is the primitive and one click is
its degenerate case. The results footer is the set case in the UI. It passes every locatable cell,
deduped by `sameSelection`. That dedupe also counts a finding once when it matches several of the
given subjects. `selectionFromFinding` is the third producer of a `Selection`, after a keyed element
and a result cell, so the canvas, the query table and the checks panel share one identity rule.

</details>

#### What a count means

**A count needs its state or it lies.** A zero has four meanings and only one of them is "nothing is
wrong here": no rules are selected, nobody has run them, the ruleset is half-evaluated, or it ran
clean. `checkedState` names which, and the panel prints the name rather than a bare number, because
a reviewer acts on the reassuring reading. A half-run ruleset reports its count as a floor.

#### Inconclusive results

**An unresolved result is not a defect and not a pass.** `Finding.inconclusive` marks a result the
rule could not DECIDE, meaning it ran, had what it needed, examined this subject, and could not
conclude (agni issue 74). The client dropped that field for its whole life, so every surface counted
"could not decide" as "found a problem". `FindingItem` now carries it, and `tallySeverities` counts
it apart and excludes it from the defect total, including from its own severity bucket, since an
inconclusive error is not an error.

Every surface says so the same way (agni issue 350). In the checks panel the severity dot gives way
to a violet `?`. A finding's severity names what the rule WOULD have reported, and painting it red
would state a conclusion. The rule declined to reach that conclusion. A group badge shows the defects
and then the unresolved count beside it, never one number over both. A sheet tile carries the same
two numbers, each drawn only when it is above zero, and neither before a run, since a count over
rules that never ran claims a clean sheet (agni issue 1132). On the severity axis an inconclusive
finding groups under `unresolved` rather than under its severity, and it sorts after every defect,
so the defect rows a reader works through sit together.

**A sheet tile counts a finding on every sheet its subject is drawn on** (agni issue 1144). A defect
on a net that runs through twelve sheets counts on all twelve, so on a large design many tiles carry
the same total. We kept that rather than counting each finding once, because a tile is a way to a
sheet, and opening the finding from any sheet it counts on shows it there. Counting only the
findings drawn on one sheet alone would answer "what do I fix on this page" better. But a defect on
a ground net would then count on no tile at all, which reads as a clean design. So the tile keeps
the number it had, and its red pill's hover splits it, as in `12 defects on this sheet, 3 drawn here
alone and 9 also on other sheets` (`SheetTile.local`). The panel's legend says the same in a line.
Rolling counts up to parent sheets was the third option, and on Jetson a parent's total mostly
repeated its children's.

**A sheet's id is unique within a design, and a repeated name is labelled by its path.** An EDIF
export holding many block cells often names every block's page `(rename &1 "1")`, so the EDIF reader
prefixes a repeated page id with its owning cell view and a repeated name with the cell's name
(`BlockA / 1`), and leaves a page whose id and name are already unique as the file spells them
(agni issue 1140). A KiCad sheet's id is already its path from the root, so the dock shows that path
where two sheets share a name (`Power/1`, `IO/1`) and nests each sheet under its `parent_id`, as the
file tree does through the shared `sheetForest` (agni issue 1141).
`readers/kicad/testdata/hier_numeric_root.kicad_sch` is the public fixture for that shape. `make
serve` mounts it as `kicad`, so it opens at `/designs/kicad/hier_numeric_root.kicad_sch/view`
(agni issue 1143).

`severitySections` is the deliberate exception. It is the parity oracle for the server report
(`GetCheckReport`), so it mirrors what that report emits rather than what this client believes. If
the client quietly disagreed with the report, it would hide drift, and the oracle exists to catch
that drift.

An inconclusive result is also per-SUBJECT, unlike the skip vocabulary. The needs-* outcomes are
PRECONDITIONS, decided around the rule and always design-wide; this sits on the other side of the
rule and is about one subject. So an entity view already answers "what could not be decided here"
without inventing anything, and the remedy lives in the message the way it does for a defect.

#### What a count leaves out

**A gated rule reports nothing anywhere**, so a per-entity count leaves it out, and nothing it leaves
out is larger. `FindingsState.skipped` is design-wide. So the count states how many rules could not
run at all, and gives the engine's own reasons on hover. Those rules are the ones the catalog lists
as unavailable for this design, plus any selected rule a run skipped. A clean entity under a
half-gated ruleset is a much weaker statement than a clean entity under a full one, and nothing on
the entity itself can say so.

**And the count says what it is not.** An entity view enumerates attention; a review pass enumerates
the design. A design-global rule has no subject and can never appear beside an entity. The caveat is
rendered beside every count rather than parked in a hover, where nobody would see it.

#### Context entities

**A rule about two entities used to name one of them and lose the other.** `Finding` carries exactly
one `Subject`, so a rule whose sentence involves two entities could highlight only one. The sentence
did not always name that one. `crystal-load-caps` reads "crystal terminal net XOUT1 has no
load capacitor" with the crystal as its subject, so clicking it sent the reader to a part while the
message talked about a net. No client-side filter reached that, because the net was bound in the
rule's query and destroyed when the `Finding` was built, so the panel had nothing to render.

`Finding.context` now carries those entities as structured data, each with a `role` naming the part it
plays (agni issue 349). The panel renders them as their own clickable chips beside the message, so the
net in the sentence is reachable. They sit after the message rather than in the subject column,
because the subject column answers "what failed" and these answer "what else the sentence mentions".

A context entity is deliberately NOT counted as a finding about itself. Grouping by subject partitions
the findings. The partition makes "the union of what I clicked equals the full pass" a fact rather
than a hope. The per-entity view depends on that property, and counting one finding under two
entities would break it. Context makes a finding REACHABLE from another entity without making it
ABOUT that entity.

### Table cells wrap, then scroll, and never bleed

The results table is fixed-layout, which keeps the columns equal by default and makes a dragged
width stick. Fixed layout does not clip, so a cell whose content will not wrap draws straight over
the columns to its right, and the column never widens to fit it. That happened on a 21-sheet design,
where the sheet-badge strip on a {{ explainable "ground" }} net was one nowrap line and painted over
the two columns beside it, its last badge's right edge sitting 1927px past its own cell.

So a cell has three behaviours in order. `overflow-wrap: anywhere` breaks a long unbroken value, which
handles nearly everything (a 66-character net name in a 120px column wraps to four lines and remains
inside). `overflow-x: auto` catches whatever still cannot wrap, so the cell scrolls within its own
column. Nothing is allowed to paint across a neighbour. A `td` honours `overflow-x` directly, checked
in Chromium, so no inner wrapper is needed.

The badge strip itself is `SheetBadges` in `web/src/sheetbadges.tsx`, shared by the query, findings
and diff-changes panels, and it shows the first three with a `+N` chip for the rest. Past a handful,
"which sheet" is a menu to open rather than a label to read. The cap keeps the common row one line
tall, and it hides nothing, since a reader can expand the strip.
