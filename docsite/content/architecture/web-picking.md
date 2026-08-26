---
title: "Picking and querying"
description: "How a reader names an entity in the viewer, walks from one answer to the next, searches by name, and sees what is already known about a selection."
---

A reader names an entity in the viewer in three ways: by clicking the drawing, by clicking a cell in
a query result, or by searching for a name. All three produce the same `Selection` value and all
three leave an editable datalog query in the box. This page covers those paths, and what the viewer
can then say about what is selected. The render and highlight contracts underneath them are on
[Web app and presenter](../web-app/).

## Picking: the drawing is the entry point

Clicking an entity is how a reader asks about it, and the rule is that **each renderer owns its own
selection data**. The SVG backend keys every element it draws (`data-kind`, `data-ref`, `data-pin`,
`data-net`, `data-net-id`, `data-bus`); the WebGL backend has `PrimitiveKey`, whose proto comment has
always said it exists for picking. Neither reads the other's representation, so either can be retired
without stranding the other, and a keyed SVG stays self-describing wherever it is embedded.

What IS shared is the contract, not the index: a `Selection` (the same shape as a finding's subject),
the priority order, and the intent. That is what lets a canvas click, a search result and a finding
row produce one value.

- **Priority, not topmost.** A symbol is drawn over its own pins, so topmost-wins would make a pin
  unclickable. Resolution goes pin, component, {{ explainable "bus" }}, net, most specific first.
- **A click is not a pan.** The press/release pair counts as a click only if the cursor moved less
  than a few pixels, or a pan that ends over a wire selects it.
- **Two pick aids, both opt-in** (`render.WithPickTargets`, which only the served viewer asks for).
  A pin has no drawn element in a faithful render, so it gets an invisible circle. A wire is a 0.8px
  stroke and `fill="none"` hit-tests only ON the stroke. Measured in a browser, a probe at a wire's
  own midpoint rounded to whole pixels hits the page rect, so it gets an invisible wide companion
  with `pointer-events="stroke"`. Both are absent from a render destined for a file or a report,
  which should not carry the viewer's interaction model.
- **Entity keys are design data, so they are escaped** (`svg.AEsc`). Attribute values are written
  verbatim otherwise, and the viewer mounts this document with `innerHTML`.

A click generates a query and runs it, rather than opening a bespoke panel. The query is left in the
box, editable, so using the viewer teaches the language instead of routing around it.

### Walking: an answer becomes the next question

A result cell is the second way a reader names an entity. The server types each answer column with an
entity kind (`RunQueryResponse.column_kinds`, derived from the relation catalog's arg labels), so a
component or net cell reads back as the same `Selection` a canvas click produces
(`selectionFromCell`). Clicking one locates it, as it always did, and now also selects it: the query
panel names what is selected and offers the served preset for that kind as a plain-language question.
Taking it fills the box and runs, which lands the reader on a fresh set of cells to walk from.

Two things are deliberate here. A cell click does not itself re-run the query, because scanning a
result set and highlighting each row in turn is what the locate affordance was for, and a click that
replaced the table would take that away. And the question's wording lives on the client (`askLabel`,
beside the locate-reason copy) while the query it runs comes from the server, the same split every
other served preset uses.

**A pin cell takes its other half from its row.** A pin's identity is two fields, so the cell holding
`5` names nothing until you know it means U7's pin 5, which is why a pin column typed as a scalar for
so long. `QueryRow.cell_refs` carries the component per row, and `selectionFromCell(kind, cell, ref)`
reads both.

<details>
<summary>Where the ref comes from, what counts as a pin column, and why a pin cannot be searched by name</summary>

`cell_refs` is a sibling of `cell_kinds` rather than the same mechanism, and the distinction is worth
keeping straight. A pin column's KIND is fixed, since every row of it is a pin; what varies per row
is the REF. The ref may come from a sibling column (`pin.net(?ref, ?pin, ?net)`), from a constant
(`pin.net("U1", ?pin, ?net)`), or from a variable the projection dropped, so it resolves against the
row's bindings rather than against the visible columns.

The rule for what counts as a pin column is the pairing again: a `pin` position is a DESIGN pin only
when the same atom also carries a `ref_des`. `param.pin(mpn, pin, name, function)` and
`param.pin_range` pair `pin` with an `mpn` instead, and theirs is a pin of a part TYPE off a
datasheet, with nothing on any canvas to highlight.

Downstream, a pin resolves through its component: its sheets are the component's placements, and so
is its locate reason, so a pin of a virtual `#PWR` symbol reports VIRTUAL_SYMBOL rather than a
missing render. Asking either question about the designator would be asking about a thing called
`5`. A pin whose component does not resolve stays plain text, since half a pin is not a less precise
pin.

Searching for a pin by name is still not possible. `entity(name, kind)` deliberately does not
enumerate pins, because a pin cannot be one `name` without inventing a composite string nothing else
in the fact base joins against, and `pin(?ref, ?pin)` already enumerates them for anyone writing the
query by hand.

</details>

**The table marks where the reader is standing**, because a click sends the canvas somewhere and a
forty-row result otherwise says nothing about which answer it came from. A cell is marked when
`sameSelection` matches it against the current selection, and a sheet badge when its sheet is also
the one on screen (`QueryView.setCurrentSheet`, pushed from `showSheet` on EVERY navigation).

Both marks are DERIVED rather than remembered. A remembered mark is wrong the moment the reader
picks something on the drawing or opens a finding, and wrong silently, which is the worst way for a
you-are-here marker to fail. Two consequences follow and both are deliberate. An entity answering in
several rows marks all of them, which is true, since they all name the thing being shown. And a
marked badge past the strip's three-chip cap forces the cut to grow, because a mark nobody can see
is worse than no mark: the reader would read the unmarked strip as being somewhere else.

### Searching: the same lesson from the other end

A click says where to look and gets back what is there. A search says what a thing is called and gets
back where it is. Both write datalog into the box and run it, which is why search is a MODE on the
query panel rather than a widget of its own: what the reader keeps is an editable query either way.

The panel's "Find by name" mode takes a term, fills the served template
(`ListRelationsResponse.search_query`, from `query.Search()`), runs it, and hands the panel back to
query mode so the reader ends up looking at the sentence that answered them. The template ranges over
`entity(?name, ?kind)` because every other relation ranges over an ASSOCIATION, so a search built on
one silently cannot find a part with no connections or a net with nothing on it. It matches with
`match` and `(?i)` rather than `contains`, so case does not have to be guessed at and a reader who
wants `^U` can write it. The client regex-escapes the typed term (`searchPattern`, mirroring Go's
`regexp.QuoteMeta`), because `VDD+` and `DATA[7:0]` are ordinary names here.

**A search result is the one answer set whose rows are not all the same shape**, and that is what
made this more than a text box. Kind is normally a COLUMN property, since a variable binds at the
same relation position in every row. Under `entity(?name, ?kind)` one answer set holds a component, a
net and a bus, so `column_kinds` has nothing true to say about the name column and types it scalar,
which is to say unclickable. `QueryRow.cell_kinds` is the per-row override: empty for every ordinary
query, and read through `cellKind(result, row, i)` so the fallback to `column_kinds` lives in one
place. It types the sheet-badge lookup and the locate reason as well as the click, since a hit
nobody can navigate from is only a third of an answer.

A bus is the case that would have broken quietly. It is neither a placement nor a named wire, so
neither drawn-entity set can speak for it, and the honest test is whether it resolved to a sheet at
all, the same rule `AnnotateSheets` applies to a bus finding.

### Inspecting: what is already known about the selection

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
contradict the report beside it, the seam C25 exists to protect. It redoes net solving and reach
walks once per click. And it makes "the union of what I clicked equals the full pass" a hope rather
than a property.

`findingsFor` takes a SET rather than one subject, because a set is the primitive and one click is
its degenerate case. The results footer is the set case in the UI: every locatable cell, deduped by
`sameSelection`, which is also how a finding matching several of the given subjects is still counted
once. `selectionFromFinding` is the third producer of a `Selection`, after a keyed element and a
result cell, so the canvas, the query table and the checks panel share one identity rule.

</details>

**A count needs its state or it lies.** A zero has four meanings and only one of them is "nothing is
wrong here": no rules are selected, nobody has run them, the ruleset is half-evaluated, or it ran
clean. `checkedState` names which, and the panel prints the name rather than a bare number, because
the reassuring reading is the one a reviewer acts on. A half-run ruleset reports its count as a floor.

**An unresolved result is not a defect and not a pass.** `Finding.inconclusive` marks a result the
rule could not DECIDE: it ran, it had what it needed, it examined this subject, and it could not
conclude (agni issue 74). The client dropped that field for its whole life, so every surface counted
"could not decide" as "found a problem". It now rides on `FindingItem`, and `tallySeverities` counts
it apart and excludes it from the defect total, including from its own severity bucket, since an
inconclusive error is not an error.

It is also the per-SUBJECT axis, which is worth holding against the skip vocabulary. The needs-*
outcomes are PRECONDITIONS, decided around the rule and always design-wide; this sits on the other
side of the rule and is about one subject. So an entity view already answers "what could not be
decided here" without inventing anything, and the remedy lives in the message the way it does for a
defect.

**A gated rule reports nothing anywhere**, which is the largest thing a per-entity count leaves out.
`FindingsState.skipped` is design-wide, so the count states how many selected rules could not run at
all and gives the engine's own reasons on hover. A clean entity under a half-gated ruleset is a much
weaker statement than a clean entity under a full one, and nothing on the entity itself can say so.

**And the count says what it is not.** An entity view enumerates attention; a review pass enumerates
the design. A design-global rule has no subject and can never appear beside an entity. The caveat is
rendered beside every count rather than parked in a hover, since a caveat nobody sees is a caveat
nobody has.

**A rule about two entities used to name one of them and lose the other.** `Finding` carries exactly
one `Subject`, so a rule whose sentence involves two entities could highlight only one, and it was
not always the one the sentence named. `crystal-load-caps` reads "crystal terminal net XOUT1 has no
load capacitor" with the crystal as its subject, so clicking it sent the reader to a part while the
message talked about a net. No client-side filter reached that: the net was bound in the rule's query
and destroyed when the `Finding` was built, so the panel had nothing to render.

`Finding.context` now carries those entities as structured data, each with a `role` naming the part it
plays (agni issue 349). The panel renders them as their own clickable chips beside the message, so the
net in the sentence is reachable. They sit after the message rather than in the subject column,
because the subject column answers "what failed" and these answer "what else the sentence mentions".

A context entity is deliberately NOT counted as a finding about itself. Grouping by subject partitions
the findings, which is what makes "the union of what I clicked equals the full pass" a fact rather
than a hope, so counting one finding under two entities would break the property the per-entity view
rests on. Context makes a finding REACHABLE from another entity without making it ABOUT that entity.

### Table cells wrap, then scroll, and never bleed

The results table is fixed-layout, which is what keeps the columns equal by default and makes a
dragged width stick. Fixed layout does not clip: a cell whose content will not wrap draws straight
over the columns to its right, and the column never widens to fit it. That happened on a 21-sheet
design, where the sheet-badge strip on a {{ explainable "ground" }} net was one nowrap line and
painted over the two columns beside it, its last badge's right edge sitting 1927px past its own cell.

So a cell has three behaviours in order. `overflow-wrap: anywhere` breaks a long unbroken value, which
handles nearly everything (a 66-character net name in a 120px column wraps to four lines and stays
inside). `overflow-x: auto` catches whatever still cannot wrap, so the cell scrolls within its own
column. Nothing is allowed to paint across a neighbour. A `td` honours `overflow-x` directly, checked
in Chromium, so no inner wrapper is needed.

The badge strip itself is `SheetBadges` in `web/src/sheetbadges.tsx`, shared by the query, findings and
diff-changes panels, and it shows the first three with a `+N` chip for the rest. Past a handful,
"which sheet" is a menu to open rather than a label to read. The cap is what makes the common row one
line tall; it is not what keeps the table honest, since a reader can expand the strip.
