---
title: "Querying your design"
description: "Search your whole design as data with ad-hoc datalog queries, each answer traceable to its source."
---

A **check** answers "does my design break this rule?" A **query** answers a question you make up
on the spot: "which nets carry more than 12V?", "which parts have no datasheet?", "how many parts
sit on each net?". `agni query` lets you search your whole design as data, and every answer comes
back with its **source** so you can go check it.

The tool turns your design into a set of simple facts. A net has a voltage, a part sits on a net,
a datasheet lists a limit, a copper track has a width. A query is a question over those facts.

## The facts you can ask about

Each fact is a named relation with a few fields. You query them by name:

| Relation | Reads | From |
|---|---|---|
| `net.max_voltage(net, volts)` | a net's rail voltage | the schematic |
| `component.mpn(ref, mpn)` | a part's manufacturer part number | the schematic |
| `component.class(ref, class)` | a device class the part is in (a family tag too: a TVS is both `tvs` and `diode`) | the schematic |
| `component-on-net(ref, net)` | a part sits on a net | the schematic |
| `param(mpn, symbol, value)` | a datasheet limit | `--params` (see [Datasheets](../datasheets/)) |
| `reaches(from, net)` | nets reachable through passives | the connectivity |
| `board.track_width(net, mm)` | a net's thinnest copper track | the PCB |
| `board.via_drill(net, mm)` | a net's smallest {{ explainable "via" }} drill | the PCB |
| `board.layer(net, layer)` | a layer the net is routed on | the PCB |

The datasheet facts need a parameter set (`--params`), the board facts need a `.kicad_pcb` or an
IPC-2581 board. Ask for a fact your design doesn't carry and you simply get no rows. The tool never
makes one up.

That table is the short form. Every relation also has a full card covering what it means for
hardware, how the projection is built, and the cases where an empty result is *not* a clean answer.
The ones this page leans on are inlined at the [bottom of this page](#reference-the-relations-used-here);
the complete set is the [relation catalog](../../reference/relations/).

## Writing a query

A query is a list of facts separated by commas, an optional filter, and an optional `=>` that picks
which columns to show:

```
component.mpn(?ref, ?mpn), net.max_voltage(?net, ?v), ?v < 30  =>  ?ref, ?net
```

- `?ref`, `?mpn`, `?net`, `?v` are **variables**, blanks the tool fills in.
- Reusing a variable **joins**: `?net` in two facts means "the same net in both".
- `?v < 30` **filters**. Operators: `< <= = != > >=`. Text in double quotes: `"VIN"`. Numbers plain.
- `=> ?ref, ?net` picks the answer columns. Omit it to show every variable.

That is the whole language. The examples below build on it.

## Start here: the five-rung ladder

These are the starter queries the panel offers as click-to-run chips (and `agni query --examples`
prints the same set). Each adds exactly one idea over the one before. Run one, read the result, then
edit it. Every rung names its concept, with a one-line analogy for the SQL-literate.

### 1. Every part on every net (*projection*)

```
component-on-net(?ref, ?net) => ?ref, ?net
```

A single fact is already a question. This lists each part and the net it sits on. `=>` picks the
answer columns, SQL's `SELECT` list, with the relations as the tables.

### 2. Rails above 3V (*filter*)

```
net.max_voltage(?net, ?v), ?v > 3 => ?net, ?v
```

A bare comparison prunes rows, a `WHERE` clause. Operators: `< <= = != > >=`. Numbers plain, text in
`"quotes"`.

### 3. Parts sitting on a rail above 3V (*join*)

```
component-on-net(?ref, ?net), net.max_voltage(?net, ?v), ?v > 3 => ?ref, ?net, ?v
```

Reusing `?net` in two facts means "the same net in both", that is a `JOIN ... ON`. Joins are how you
connect what a part is, where it sits, and what its {{ explainable "rail" }} carries.

{{ includeFile "figures/query-join.svg" }}

The two relations are separate tables of facts until a variable appears in both. That shared `?net`
is what pairs a row on the left with a row on the right, and `?v > 3` then decides which of the
paired rows survive.

### 4. Parts on USB nets (*predicate*)

```
component-on-net(?ref, ?net), contains(?net, "USB") => ?ref, ?net
```

`contains` is a test over an already-bound value, SQL's `LIKE '%USB%'`. `prefix`/`suffix` are the
anchored variants.

### 5. Reachable through series pass elements (*recursion*)

```
reaches(?from, ?net) => ?from, ?net
```

`reaches` walks connectivity transitively through series passives (a resistor, an inductor, a
{{ explainable "ferrite-bead" }}, a fuse), a recursive CTE / transitive closure over the connectivity
graph. It answers "what does this rail actually feed after the filter", which no per-net question can
see across a series element.

![reach-walk diagram]({{.Site.PathPrefix}}/static/images/querying/reach-walk.svg)

## Where these commands run

Every transcript below is generated by running the command shown, so each one works as written, from
the working directory its sample lives in. Three samples appear, all checked into a clone of the
engine repo:

| Sample | Run from | What it is |
|---|---|---|
| `designs/gateway/gateway.edn` | `examples/tutorial-project` | the tutorial's synthetic gateway board, with a team's naming and parameters wrapped around it |
| `regulator.fires.kicad_sch`, `fires.edn` | `cmd/agni/testdata/conformance` | small single-purpose boards, each built so one thing has something true to report |
| `board.kicad_pcb` | `readers/kicad/testdata` | a board export, for the copper facts a netlist cannot answer |

Point any of them at your own design instead and the same queries run. Nothing here depends on the
sample beyond the names it happens to carry.

## Going further

### Find something by name

`entity(name, kind)` is the relation that names what exists. Every other relation ranges over a
relationship, so a search built on one inherits its blind spots: looking through `component-on-net`
cannot find a part that sits on no net, because such a part has no row there.

{{ agniRun "content/guide/runs/query-entity-by-name.yaml" }}

`kind` is `component`, `net` or `bus`, so you can scope the search:

{{ agniRun "content/guide/runs/query-entity-by-kind.yaml" }}

The string predicates (`contains`, `prefix`, `suffix`, `glob`, `match`) decide how the name is
matched. A leading wildcard is the case `prefix` cannot express, which is how you find every net
named for the rail it carries whichever block named it:

{{ agniRun "content/guide/runs/query-entity-by-glob.yaml" }}

The cases only this relation reaches are the ones worth finding during a review. A part connected to
nothing:

{{ agniRun "content/guide/runs/query-entity-unconnected.yaml" }}

A net with nothing on it, here against a board that has none:

{{ agniRun "content/guide/runs/query-entity-empty-net.yaml" }}

That empty answer is the answer. Asking for something a design does not carry returns no rows rather
than an invention, which is the same property the relation cards spend their space on.

Pins are not in `entity`, because a pin is named by two things rather than one. Enumerate those with
`pin(?ref, ?pin)`, which is also what to reach for when you want to find a pin: the viewer's find-by-
name box searches `entity`, so it will not turn one up.

In the web viewer this has a front door. The query panel has a **Find by name** mode: type part of a
name, and it writes the same query into the box, runs it, and hands you back the query. Each hit is
clickable, whatever sort of thing it turns out to be, so a search lands you on the drawing and leaves
a sentence you can edit into a better question.

### Reading a result cell in the viewer

A cell that names a component, a net or a {{ explainable "bus" }} is a link: clicking it highlights
that thing on the drawing. The small chips beside it are the **sheets it appears on**, one per
sheet, and clicking one opens that sheet. They are not a grouping of related names, and they have
nothing to do with the cell being a net in particular. Any locatable cell gets them.

```
AVDD_3V3   [ 04_POWER ] [ 07_SENSOR ] [ 11_MCU ] [ +2 ]
```

That row says the net `AVDD_3V3` is drawn on at least five sheets, three of them named. A
single-sheet design shows no chips at all, because there is nowhere to navigate. The strip caps at
three and counts the rest, since past a handful the interesting fact is how many sheets a thing
spans rather than which; clicking the `+N` expands it.

Whichever entity is on the drawing right now is marked in the table, and so is the chip for the
sheet you are looking at. That is what tells you where you are after a click sends the canvas
somewhere, and it follows the viewer rather than your last click here: pick something on the drawing
instead and the mark moves to that.

### Find parts stressed above their datasheet rating

This joins what the part is (`component.mpn`), what its datasheet says (`param`), where it sits
(`component-on-net`), and the rail's voltage (`net.max_voltage`), then keeps only the ones where the
{{ explainable "absolute-maximum-rating" "rated maximum" }} is below the rail.

{{ agniRun "content/guide/runs/query-datasheet-join.yaml" }}

`U1` (an LM1117) sits on a `+24V` rail, but its datasheet caps VIN at 20V. The **provenance** column
cites both sides, the schematic and the datasheet page, so you can open each and confirm.

### Find undatasheeted parts (negation)

`not` keeps the rows where a fact is **absent**. "Parts on a net that have no MPN"
(the `component.mpn` relation is populated from the datasheet join, so pass `--params`):

{{ agniRun "content/guide/runs/query-negation.yaml" }}

`J1` is placed but carries no part number, so a datasheet can never be matched to it.

### Count parts per net (aggregation)

`count`, `min`, `max`, and `sum` summarize. Group by the plain columns. The aggregate reduces the rest:

{{ agniRun "content/guide/runs/query-aggregation.yaml" }}

### Search the board (any tier, one language)

Board facts query the same way. "Nets routed thinner than 0.3 mm":

{{ agniRun "content/guide/runs/query-board-track-width.yaml" }}

And you can **join across the schematic, the datasheet, and the board in one question**, "a net
routed thin that carries a high-current part":

```
board.track_width(?net,?w), component-on-net(?ref,?net), component.mpn(?ref,?mpn), param(?mpn,"IOUT",?i), ?w < 0.25 => ?net, ?ref, ?i
```

A single query can span copper, connectivity, and datasheets at once, and each answer stays
traceable back to the layer and the datasheet page.

### Follow a rail through passives (reaches)

`reaches(from, net)` walks connectivity through series passives (resistors, ferrites, fuses). It is
how you ask "what does this rail actually feed after the filter". "Everything reachable from GND":

{{ agniRun "content/guide/runs/query-reaches.yaml" }}

## Asking under your own vocabulary

Some relations do not report what is in the file; they report what the engine *believes*. `rail`,
`feedback`, and `pin.type` are resolved from a vocabulary at the moment the design is read.

That vocabulary is the built-in one unless you say otherwise, and it is anchored on the names most
boards use. On a board that names rails function-first, what the built-in vocabulary returns can be
badly wrong for your project.

Isolating that takes moving two things aside rather than one, which is itself worth knowing. The
tutorial project declares its `conventions.yaml`, so naming the design applies it automatically and
the before-state is otherwise unreachable. And a seeded datasheet's pin functions establish the rail
role on their own, so with `params/` in place this board classifies all four rails whatever the
naming vocabulary says. Move both and the question is about NAMING alone:

{{ agniRun "content/guide/runs/query-rail-builtin-vocabulary.yaml" }}

Pass your own vocabulary and ask again, with the corpus still aside so the pair differs in exactly
one thing:

{{ agniRun "content/guide/runs/query-rail-own-vocabulary.yaml" }}

Nothing about the design changed. This is the loop to author a lexicon in: ask, compare against the
rails you know the board has, fix the pattern, ask again. See
[Naming conventions](../naming-conventions/).

The datasheet route is the other way to the same answer, and it needs no lexicon at all: that is what
the corpus was doing before it was moved aside, and [Datasheets](../datasheets/) is where it lives.

Only the config's lexicon half is used here, since a query runs no rules.

## In the viewer

The viewer has a **Query** panel that runs the same queries against the design you have open. Type a
query, press Run (or ⌘/Ctrl+Enter), and the results appear as a table. Each row has a small toggle
that expands its provenance, so the citations stay out of your way until you want them.

The panel runs the query on the server, over the open file, so you get the same answers as
`agni query` without leaving the design. The **vocabulary** control in the top bar applies here too,
and a query and a check in the same session then answer under the same vocabulary.

Datasheet (`param`) facts are not yet wired into the viewer. A query over `param` returns nothing
there, and datasheet joins stay on the CLI for now.

You do not have to memorize the vocabulary. Below the query box the panel lists every relation as a
**click-to-insert chip**, grouped by kind (Netlist, Board, Datasheet, Predicates, and any overlay
relations your deployment adds). Clicking a chip drops its template at your cursor, so
`component-on-net` inserts `component-on-net(?ref_des, ?net)` ready to wire into the rest of the
query. Hover a chip to see its full signature and a one-line description.

## Every answer is checkable

The last column is always **provenance**, the source of the facts that produced the row: a schematic
file, or a datasheet document, page, and table, or a board net. A query never gives you a number you
cannot trace back to its source, so a search you run is one you can verify. In the viewer the
provenance rides behind each row's expand toggle instead of a trailing column.

## What a query is not

A query **reports**, it does not judge. It has no notion of pass/fail, that is what
[checks](../checks-and-reports/) are for. If you find yourself running the same query to catch a
recurring problem, that is a sign it should become a rule.

## Reference: the relations used here

These are the full cards for the relations the examples above use, inlined so you do not have to
leave the page. They are the same text the [relation catalog](../../reference/relations/) serves, and
they are generated from the relation definitions themselves, so they cannot drift from what the engine
actually projects.

**Read the "absence is not a pass" section on any relation you scope a question by.** An empty result
means the fact is absent from your design, which is a different statement from "nothing matched", and
the cards say which is which.

<details>
<summary><strong><code>net.max_voltage</code></strong>, a net's rail voltage</summary>

{{ includeCard "content/reference/relations/net.max_voltage.md" }}

</details>

<details>
<summary><strong><code>component.mpn</code></strong>, a part's manufacturer part number</summary>

{{ includeCard "content/reference/relations/component.mpn.md" }}

</details>

<details>
<summary><strong><code>component.class</code></strong>, a device class the part is in</summary>

{{ includeCard "content/reference/relations/component.class.md" }}

</details>

<details>
<summary><strong><code>component-on-net</code></strong>, a part sits on a net</summary>

{{ includeCard "content/reference/relations/component-on-net.md" }}

</details>

<details>
<summary><strong><code>reaches</code></strong>, nets reachable through passives</summary>

{{ includeCard "content/reference/relations/reaches.md" }}

</details>

<details>
<summary><strong><code>board.track_width</code></strong>, a net's thinnest copper track</summary>

{{ includeCard "content/reference/relations/board.track_width.md" }}

</details>
