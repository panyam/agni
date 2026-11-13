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
| `component.net(ref, net)` | a part sits on a net | the schematic |
| `param.max(mpn, symbol, value)` | a datasheet limit | `--params` (see [Datasheets](../datasheets/)) |
| `net.reaches(from, net)` | nets reachable through passives | the connectivity |
| `net.route(from, net, path)` | the same walk, plus what it crossed to get there | the connectivity |
| `board.track_width(net, mm)` | a net's thinnest copper track | the PCB |
| `board.via_drill(net, mm)` | a net's smallest {{ explainable "via" }} drill | the PCB |
| `board.layer(net, layer)` | a layer the net is routed on | the PCB |

The datasheet facts need a parameter set (`--params`), and the board facts need a `.kicad_pcb` or an
IPC-2581 board. Ask for a fact your design doesn't carry and you simply get no rows. The tool never
makes one up.

That table is the short form. Every relation also has a full card covering what it means for
hardware, how the projection is built, and the cases where an empty result is *not* a clean answer.
This page inlines the cards for the relations it uses at the
[bottom of this page](#the-relations-used-here); the complete set is the
[relation catalog](../../reference/relations/).

A name before a dot is a **module** and the part after it is a **member**: `net.pin_count` is the
`pin_count` member of the `net` module. No name is both, so these relations moved into modules
(agni issue 751). A query saved before the move names them the old way and now gets
"unknown relation":

| Was | Now |
|---|---|
| `component-on-net` | `component.net` |
| `pin` | `component.pin` |
| `param` | `param.max` |
| `reaches`, `route` | `net.reaches`, `net.route` |
| `rail`, `feedback`, `switching` | `net.rail`, `net.feedback`, `net.switching` |
| `external_signal_net` | `net.connector_signal` |
| `has_nc_channel`, `types_power_out`, `has_netclass`, `has_netclass_defs` | `design.` plus the same name |
| `pin_net_conflict`, `ref_des_collision`, `unresolved_symbol` | `reader.` plus the same name |
| `contains`, `prefix`, `suffix`, `glob`, `match` | `str.` plus the same name |

## Writing a query

A query is a list of facts separated by commas, an optional filter, and an optional `=>` that picks
which columns to show:

```
component.mpn(?ref, ?mpn), net.max_voltage(?net, ?v), ?v < 30  =>  ?ref, ?net
```

- `?ref`, `?mpn`, `?net`, `?v` are **variables**, blanks the tool fills in.
- Reusing a variable **joins**: `?net` in two facts means "the same net in both".
- `?v < 30` **filters**. Operators: `< <= = != > >=`. Text in double quotes: `"VIN"`, taken exactly as
  written, since a string has no escapes. Numbers plain.
- `=> ?ref, ?net` picks the answer columns. Omit it to show every variable.

That is the whole language. The examples below build on it.

A query that does not parse says where it stopped. `agni query` prints the line with a caret under
the place, and the viewer's query panel shows the same under its error:

```
error: invalid argument: query: variable ?r ?n: a variable's name is letters, digits and _
  component.net(?r ?n) => ?r
                ^
```

## The five-rung ladder

The panel offers these starter queries as click-to-run chips, and `agni query --examples` prints
the same set. Each adds exactly one idea over the one before. Run one, read the result, then
edit it. Every rung names its concept, with a one-line analogy for the SQL-literate.

### 1. Every part on every net (*projection*)

```
component.net(?ref, ?net) => ?ref, ?net
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
component.net(?ref, ?net), net.max_voltage(?net, ?v), ?v > 3 => ?ref, ?net, ?v
```

Reusing `?net` in two facts means "the same net in both". In SQL that is a `JOIN ... ON`. A join
connects what a part is, where it sits, and what its {{ explainable "rail" }} carries.

{{ includeFile "figures/query-join.svg" }}

The two relations are separate tables of facts until a variable appears in both. That shared `?net`
pairs a row on the left with a row on the right, and `?v > 3` then decides which of the paired rows
survive.

### 4. Parts on USB nets (*predicate*)

```
component.net(?ref, ?net), str.contains(?net, "USB") => ?ref, ?net
```

`str.contains` is a test over an already-bound value, SQL's `LIKE '%USB%'`. `str.prefix`/`str.suffix` are the
anchored variants.

### 5. Reachable through series pass elements (*recursion*)

```
net.reaches(?from, ?net) => ?from, ?net
```

`net.reaches` walks connectivity transitively through series passives (a resistor, an inductor, a
{{ explainable "ferrite-bead" }}, a fuse). In SQL it is a recursive CTE / transitive closure over the
connectivity graph. It answers "what does this rail actually feed after the filter". No per-net
question can see across a series element.

![reach-walk diagram]({{.Site.PathPrefix}}/static/images/querying/reach-walk.svg)

## Where these commands run

Every transcript below is generated by running the command shown, so each one works as written when
you run it from its sample's working directory. Three samples appear, all checked into a clone of the
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

`entity(name, kind)` names everything that exists. Every other relation ranges over a relationship,
so a search built on one inherits its blind spots. A part that sits on no net has no row in
`component.net`, so a search through `component.net` cannot find it.

{{ agniRun "content/guide/runs/query-entity-by-name.yaml" }}

`kind` is `component`, `net` or `bus`, so you can scope the search:

{{ agniRun "content/guide/runs/query-entity-by-kind.yaml" }}

One of the string predicates matches the name: `str.contains`, `str.prefix`, `str.suffix`,
`str.glob` or `str.match`. `str.prefix` cannot express a leading wildcard. A leading wildcard finds
every net named for the rail it carries, whichever block named it:

{{ agniRun "content/guide/runs/query-entity-by-glob.yaml" }}

`str.glob` follows SQLite's GLOB: `*` is any run of characters (including `/`, so a hierarchical
name matches), `?` is one character, and `[...]` is a character class, so `DATA[0-3]` matches
`DATA0` to `DATA3`. That makes a literal bracket special. A bus name such as `DATA[1:0]` is matched
with the bracket inside a class, `DATA[[]1:0]`. Before jaala v0.1.8 a `[` was literal, so a pattern
written for a bracketed name before then now means something else.

The string functions build a name as well as test one. Each puts its result last, so it reads as an
expression (`?l = str.replace(?h, "CANH", "CANL")`). They are `str.concat`, `str.replace`,
`str.lower`, `str.upper`, `str.capitalize` and `str.split`. `str.concat` runs from any two of its
three arguments. Given the whole name and its tail it binds the head, and holds for nothing when the
name does not end that way. That form pairs the two halves of a differential pair by their shared
base name:

{{ agniRun "content/guide/runs/query-entity-pair-by-concat.yaml" }}

`str.split` gives one row per part, or only the part at an index you bind, where a negative index
counts from the end.

Only this relation reaches two cases worth finding during a review. The first is a part connected
to nothing:

{{ agniRun "content/guide/runs/query-entity-unconnected.yaml" }}

A net with nothing on it, here against a board that has none:

{{ agniRun "content/guide/runs/query-entity-empty-net.yaml" }}

That empty answer is the answer. Ask for something the design does not carry and you get no rows
rather than an invention. The relation cards spend their space on the same property.

Pins are not in `entity`, because a pin is named by two things rather than one. Enumerate pins with
`component.pin(?ref, ?pin)`, and use the same relation to find a pin. The viewer's find-by-name box
searches `entity`, so it will not turn one up.

The web viewer runs this search for you. The query panel has a **Find by name** mode. Type part of a
name, and the panel writes the same query into the box with your text bound to its `?pattern`. It
runs the query and hands you back the query and the binding. Each hit is clickable, whatever sort of
thing it turns out to be. So a search takes you to the drawing, and it leaves the query in the box
for you to edit into a better question.

Command-F (Ctrl-F on Linux and Windows) opens **Find by name** from anywhere on the page. It brings
the query panel forward and puts the cursor in the box, with any text already there selected. In
another text field, such as the query box, the key does what the browser does. Command-Shift-F
(Ctrl-Shift-F) brings the query panel forward and puts the cursor in the query box, closing any
drawer that covers it.

### Reading a result cell in the viewer

A cell is a link when it names a component, a net or a {{ explainable "bus" }}, and clicking it
highlights that thing on the drawing. The small chips beside it are the **sheets it appears on**,
one per sheet, and clicking one opens that sheet. They are not a grouping of related names, and they
have nothing to do with the cell being a net in particular. Any locatable cell gets them.

```
AVDD_3V3   [ 04_POWER ] [ 07_SENSOR ] [ 11_MCU ] [ +2 ]
```

That row says the net `AVDD_3V3` is drawn on at least five sheets, three of them named. A
single-sheet design shows no chips at all, because there is nowhere to navigate. The strip caps at
three and counts the rest. Past a handful, the interesting fact is how many sheets a thing spans
rather than which. Clicking the `+N` expands it.

The table marks whichever entity is on the drawing right now, and it marks the chip for the sheet
you are looking at. After a click sends the canvas somewhere, those marks tell you where you are.
They follow the viewer rather than your last click here, so if you pick something on the drawing
instead, the mark moves to that.

### Find parts stressed above their datasheet rating

This joins what the part is (`component.mpn`), what its datasheet says (`param.max`), where it sits
(`component.net`), and the rail's voltage (`net.max_voltage`), then keeps only the ones where the
{{ explainable "absolute-maximum-rating" "rated maximum" }} is below the rail.

{{ agniRun "content/guide/runs/query-datasheet-join.yaml" }}

`U1` (an LM1117) sits on a `+24V` rail, but its datasheet caps VIN at 20V. The **provenance** column
cites both sides, the schematic and the datasheet page, so you can open each and confirm.

### Find undatasheeted parts (negation)

`not` keeps the rows where a fact is **absent**. The datasheet join populates `component.mpn`, so
pass `--params` to ask for "parts on a net that have no MPN":

{{ agniRun "content/guide/runs/query-negation.yaml" }}

`J1` is placed but carries no part number, so a datasheet can never be matched to it.

**A `not` has to be anchored.** At least one variable inside it must also appear in a positive
relation, so the negation is about the row rather than about the design. Above, `?r` is the anchor, so
`not component.mpn(?r,?m)` asks whether THIS part has a part number. The `?m` is free on purpose and
means "for any value".

Drop the anchor and the question changes without looking like it has:

```
entity(?n,"net"), not component.class(?tp,"test_point") => ?n
```

`?tp` appears nowhere else, so that asks whether the design contains no test point AT ALL, and on a
board that has even one it filters away every row. It used to answer "no results", which reads as a
fact about the board. It is now an error naming the unanchored variable.

**To negate a pair of relations, name them first.** `not` takes one relation. That spelling was
reaching for nets with no test point ON THEM, and that question needs both `component.net` and
`component.class` to be true of the same part. Define a relation for that and negate it by name:

```
has_test_point(?n) :- component.net(?tp,?n), component.class(?tp,"test_point");
entity(?n,"net"), not has_test_point(?n) => ?n
```

Clauses are separated by `;`, a clause containing `:-` defines a relation, and the one clause
without one is the question. This is the general form for any "X with no related Y". Most coverage
questions take that form: rails with no decoupling, buses with no pull-up, parts with no
protection.

A defined relation is fully derived before anything negates it, so the answer does not depend on the
order you wrote the clauses in.

### Ask what a net IS (roles)

`net.role` gives you what the engine worked out about a net from its name. A net can carry more than
one, so the relation yields a row per role:

```
net.role(?n, ?r) => ?n, ?r
```

Four of the six roles exist to say **named after a rail, and not one**. A regulator's pins are
conventionally named for the supply they serve, so a 12V converter's nodes are `12V_FB`, `12V_SW`,
`12V_MODE1` and `12V_VDRV`, none of which carries 12V. Listing them is one clause:

```
net.role(?n, ?r), ?r != "rail", ?r != "ground" => ?n, ?r
```

**`net.rail(?n)` and `net.role(?n, "rail")` are
different sets.** `net.rail` is a conclusion, true for a net that is asserted-driven or global or a ground
*or* carries the rail role. `net.role` is what the naming lexicon stamped. On a board with a few
regulators the second is the larger of the two, and a probe-point or decoupling question almost always
wants the first.

Each role's vocabulary is config, and you extend it in
[your conventions](../naming-conventions/#teach-it-your-vocabulary).

### Ask what the design DECLARED about a net (attributes)

`net.role` is what the engine inferred. `net.attr` is what the file said:

```
net.attr(?n, "external", "true") => ?n
```

`external` marks a net whose full connectivity was not read, because it continues onto a sheet the
read did not cover. Most connectivity rules exclude those rather than report a one-pin net that is an
artefact of what was opened. It is the net-side twin of `component.attr`.

### Count parts per net (aggregation)

`count`, `min`, `max`, `sum`, `avg` and `list` summarize. Group by the plain columns. The aggregate
reduces the rest. `list` answers a list value, written as its JSON text (`["R1","R2"]`), so a script
reading a cell parses it rather than splitting on spaces:

{{ agniRun "content/guide/runs/query-aggregation.yaml" }}

**With no plain column, the answer is always one row.** The whole result is then a single group,
and it exists even when nothing matched, so a count of something the design lacks answers `0`
rather than "no results":

```
net.ground(?n), component.net(?tp,?n), component.class(?tp,"test_point") => count(distinct ?tp)
```

`count` and `sum` over nothing are `0` and `list` is the empty list `[]`. `min`, `max` and `avg` have no
number to report, so their cells are absent.
A projection that does group, such as `=> ?n, count(?r)`, still answers nothing when nothing
matched, because there is no net to name a group after.

So a grouped count never lists the nets with nothing to count. Most reviews want that coverage table
for test points, and the library member `net.test_point_count` gives every net a row and a `0` where
it has none, so the uncovered nets sort to the top:

{{ agniRun "content/guide/runs/query-test-point-count.yaml" }}

### Keep only some groups (having)

A comparison in the question filters facts, one at a time, before there is any group. To ask about
the group itself you need `having`, which runs after the reduce. "Nets carrying more than one part":

{{ agniRun "content/guide/runs/query-having.yaml" }}

The aggregate does not have to be a column. Drop `count(?r)` from the projection and keep the
`having`, and the answer is the nets rather than the tally. A coverage question usually wants the
nets:

```
component.net(?r,?n) => ?n having count(?r) > 1
```

### Counting values instead of bindings (distinct)

**An aggregate reduces one entry per ANSWER, not per distinct value.** A question that joins two
things produces one answer per combination, so a part appears once for every partner it was paired
with, and `count` counts all of them. `distinct` reduces the values instead:

{{ agniRun "content/guide/runs/query-distinct.yaml" }}

`+24V` carries two parts. Asking twice about what sits on a net pairs each with each, so there are
four answers and `count(?r)` reports 4. `count(distinct ?r)` reports the 2 you meant, and
`list(distinct ?r)` names them.

`distinct` works the same way on every aggregate, `list` included, so a projection carrying both
`count(?r)` and `list(?r)` always describes the same set. If you are ever unsure which you have,
select both spellings and compare, as above.

A defined relation also gets there. It projects the extra column away before the group forms:

```
on(?r,?n) :- component.net(?r,?n), component.net(?other,?n);
on(?r,?n) => ?n, count(?r)
```

The defined relation keeps `?r` and drops `?other`, so its tuples are already one per part per net
and a plain `count` matches `count(distinct ?r)`. Drop `?r` from the head too and you are counting
nets, not parts. That is the easiest mistake to make with this idiom.

### Count inside a clause (body aggregates)

A grouped count leaves out every group with nothing to count. A body aggregate counts inside the
clause instead. The rest of the clause gives the shared variable its values, and the aggregate
counts once for each, so the empty groups answer `0`. Every net with the number of test points on it:

```
entity(?n, "net"), ?c = count(distinct ?tp) : { component.net(?tp, ?n), component.class(?tp, "test_point") } => ?n, ?c
```

The braces share a variable with the clause (`?n` here). A relation outside the braces has to bind
it, as well as one inside them. `net.test_point_count` in the shipped library is this clause.

### Keep a row when part of it is missing (optional)

`optional { ... }` keeps the row whether or not the braces match. Where they do not match, a
variable bound only inside the braces is absent. Every resistor with its part number, and the
resistors that have none still listed:

```
component.class(?r, "resistor"), optional { component.mpn(?r, ?m) } => ?r, ?m
```

Without `optional`, a resistor with no part number would drop out of the answer. That answer reads
the same as a design with no such resistor.

### Sort and page an answer (order by, limit)

`order by` sorts by projected columns, `desc` reverses one, and `limit` keeps the first rows. With an
`order by`, the evaluation holds only the page while it runs, so asking for the ten busiest nets on
a large board does not build the whole answer first:

```
net.pin_count(?n, ?c) => ?n, ?c order by ?c desc, ?n limit 10
```

`offset` skips rows and `after` continues from the last row of the page before, by value. Both need
an `order by`.

A limited answer says how many rows it left out, as `10 of 1617 result(s), 1607 left out by the
limit` under the table and as `total` in `--format json`. On the command line `--limit 10` caps an
answer without editing the query, and so does `limit` on the `RunQuery` request. Where the query
writes its own `limit`, the smaller applies. Counting the total solves the whole goal, so a limit
saves keeping, locating and sending the rows it leaves out, and does not save the solving.

### Units

A voltage carries volts and a board width carries metres, shown in millimetres. When all of a
relation's numbers share a unit, the relation declares it. Comparing two different units is refused
before the query runs, since volts are never less than metres:

```
net.max_voltage(?n, ?v), board.track_width(?n, ?w), ?v < ?w => ?n
```

A datasheet parameter's unit varies from row to row, so its relations declare none and each value
keeps its own. An aggregate can mix units, such as `sum` over one part's limits in volts and amps.
It then answers absent, and the cell says why (`absent: mixed units: A, V`). `num.value` and
`num.unit` take a number apart when a question really does mean the bare number.

A constant can carry its unit, so `?v >= 3.3V` and `?v > 3000mV` ask the same thing, since a prefix is
folded into the base unit before anything compares. A constant in the wrong unit is refused the same
way a relation's is, and `?v > 3mm` against a voltage fails with `compares number[V] with number[m],
which never holds`.

### Arithmetic

A comparison can compute. `+`, `-`, `*` and `/` work on numbers, with parentheses and unary minus,
and `?x = expression` binds a new variable to the result:

```
net.max_voltage(?n, ?v), ?mv = ?v * 1000 => ?n, ?mv order by ?mv desc
```

An expression can sit on either side of any comparison, so `net.pin_count(?n, ?c), ?c * 2 > 20 => ?n`
needs no variable of its own. Each operator is a function relation underneath (`num.mul` for `*`,
and so on), and those run in whichever direction their bound values allow, so `?v = ?i * ?r` finds
whichever of the three is missing. For the same reason, it doesn't matter in which order an
expression is written.

A one-rule relation whose head ends in its result works as a function, called with that last
argument left off:

```
headroom(?v, ?h) :- ?h = 5V - ?v; net.max_voltage(?n, ?v), ?h = headroom(?v), ?h > 0V => ?n, ?h
```

The same rule still works as a relation, `headroom(?v, ?h)`.

An expression can also stand in the projection, where its column is named by the expression's own
text without the `?`s:

```
net.max_voltage(?n, ?v) => ?n, ?v * 1000
```

answers in columns `n` and `v * 1000`. And it can stand in a rule's head. There it computes the value
as `?y = expression` in the body would:

```
margin(?n, 5V - ?v) :- net.max_voltage(?n, ?v); margin(?n, ?m), ?m > 3V => ?n, ?m
```

A computed number compares as the constant it looks like, so `5V - 3.3V` shows as `1.7` and
`net.max_voltage(?n, ?v), 5V - ?v = 1.7V` finds the 3.3 V nets (fixed in
[panyam/jaala#216](https://github.com/panyam/jaala/issues/216)).

### Use the shipped library (derived relations)

Some questions come up often enough that agni ships them as relations of their own, defined in
Datalog over the base relations rather than projected from the design in Go. They sit in the same
modules as the relations they extend, so a query calls them the same way:

```
component.probed_one(?part, ?probed, ?unprobed), component.mpn(?part, ?mpn) => ?mpn, ?part, ?unprobed
```

| Member | What it answers |
|---|---|
| [`net.has_test_point(net)`](../../reference/relations/net.has_test_point/) | a net at least one test point sits on |
| [`component.two_terminal(part, a, b)`](../../reference/relations/component.two_terminal/) | a part on exactly two nets, the nets in name order so each part answers once |
| [`component.probed_both(part)`](../../reference/relations/component.probed_both/) | a two-terminal part with a test point on both nets, so an in-circuit tester can measure it |
| [`component.probed_one(part, probed, unprobed)`](../../reference/relations/component.probed_one/) | a two-terminal part probed on one net only, naming the net that is missing a test point |

`agni query --relations` prints everything a query can call, module by module, with each member's
argument types. Give it a module to list one (`agni query --relations net`), or a member to read its
definition:

```
agni query --relations component.probed_both
```

```
component.probed_both(r: component)
  derived, defined in module component

  A two-terminal part with a test point on both of its nets, so it can be measured in circuit.

  defined as
    probed_both(?r: component) :- two_terminal(?r, ?a, ?b), net.has_test_point(?a), net.has_test_point(?b)
```

Each member's page says what it counts and leaves out, why the question matters on a board, and how
it is defined. The library lives in `stdlib/lib`, one file per module, and
[Adding a library member](../../build/library-member/) covers when a question belongs there. If only
one report asks a question, the question belongs in that report's query instead, as a rule ahead of
its goal or in a query set's preamble.

A project can carry its own library too, in a `lib/` directory beside `project.yaml`, laid out the
same way. The tutorial project's `lib/house.dl` names the PMIC's rails after that team's `PMIC_`
convention, and a query on its board calls them like any other relation. Of the three, one has
no test point:

{{ agniRun "content/guide/runs/query-project-library.yaml" }}

The members exist only inside that project. `agni query --relations house --design <design>` lists
them with their definitions, and [Adding a library member](../../build/library-member/#a-projects-own-library)
covers the rules a project library follows.

### See how a row was derived (witnesses)

A row's provenance names the facts behind it, and for a query over library members that is not the
whole reason. `PMIC_MAIN_12V0` answers the query above because `house.pmic_rail` holds for it and
`house.pmic_probe_point` does not, and neither is a fact the design states. `--witnesses` asks the
engine to keep how each row was derived and prints it under the row:

{{ agniRun "content/guide/runs/query-witnesses.yaml" }}

Each line is one fact, and the lines under it are what proved it. When a member derived a fact, its
line names the member's rule after `via`, with each of its body facts filled in, in the order the
rule is written. A fact from the design ends in its citation. A `not` that held reads
`not ... (absent)`, because it proves that something is missing and so has nothing to cite. A
predicate such as `str.prefix` gets a line, and a comparison such as `?a < ?b` does not, since the
rule beside it already states it.

Witnesses cost nothing unless asked for. The engine keeps a derivation per derived fact only on a
witnessed query, and a query without the flag runs exactly as it did, at the same `work`. An aggregate
row has none, because a count merges many derivations into one number.

`text` and `html` print the tree under its row, `markdown` after the table under each row's number,
and `json` as each row's `witnesses`, whose citations index the answer's `sources` as a row's do.
`csv` refuses the flag, since a csv's first row has to be its header and every other row a row of the
table. `--set` asks every query in the set for witnesses. Over the API it is
`RunQueryRequest.witnesses` (and `RunQueriesRequest.witnesses`).

A checklist item written as an inline query opens the same way in the viewer's Review panel. The
arrow beside one of its findings asks the item's query again with the finding's subject bound and
witnesses on, and draws the tree under the finding. The run keeps the checklist it scored, and the
query comes from that copy. So an older run asks its own question and not whatever the checklist
file says today. A run older than the open design can name a finding that no longer holds, and the
panel says so rather than drawing nothing. A finding from a rule, a tag or a profile has no arrow yet
(agni issue 1183).

### Ask the same question about different things (bindings)

Say a script asks about one part, then another. It should not build a new query string each time.
Write the query once with a variable where the part goes, and bind the variable to a value with
`--bind name=value`:

{{ agniRun "content/guide/runs/query-bind.yaml" }}

A bound variable answers exactly as the same value written into the query as a constant, so the run
above is `component.net("U1", ?net), ...` without the quoting. Each position declares a type, and a
value is read as that type. A voltage, a count or a datasheet limit is a number, so `--bind min=3`
and `--bind 'min="3"'` both compare as `3`, and `--bind min=abc` is refused as not a number. Where
nothing in the query types the variable, as in `?a < ?b` alone, a value that parses as a number binds
a number and a quoted one (`--bind 'a="3"'`) binds the text.
`--bind` repeats, one per variable. Naming a variable the query does not use is an error, so a typo
fails rather than leaving the variable free and widening the question.

Naming one variable twice binds it to a list, and the query answers for every value in it, as if the
goal were joined with a relation holding exactly those values. So
`--bind n=I2C_SDA --bind n=I2C_SCL` asks about both nets in one answer, an aggregate counts across
both, and the document view states the binding as `?n in ["I2C_SDA", "I2C_SCL"]`.

A program sends bindings rather than building text, so a value needs no escaping and a name holding
a quote is asked about exactly. Over the API they are `RunQueryRequest.bindings`, and
the Python client builds them with `agni.bindings({"ref": "U1"})`, or `agni.bindings({"ref": ["U1", "U2"]})`
for a list, which travels as `QueryValue.list`. A viewer link carries a list as its `bind` value
repeated, once per value, and opens with the list as one chip. The `--format markdown` and `html`
views print them under the query, because the query text alone no longer states the whole question.

### Ask many questions at once (query sets)

An audit is usually a workbook rather than a single question: which nets have no test point, how
many ground test points there are, which passives a tester can measure. A **query set** keeps those
questions in one YAML file, with their shared derived relations written once as a `preamble`,
and `agni query --set` answers all of them over ONE read of the design:

```yaml
title: Test point audit
preamble: |
  has_tp(?n) :- component.net(?tp, ?n), component.class(?tp, "test_point");
queries:
  - name: Net count
    query: entity(?net, "net") => count(?net)
  - name: Nets with no test point
    query: entity(?net, "net"), not has_tp(?net) => ?net
  - name: Ground test points
    query: net.ground(?net), component.net(?tp, ?net), component.class(?tp, "test_point") => count(distinct ?tp)
```

{{ agniRun "content/guide/runs/query-set.yaml" }}

The preamble holds rules only, and each query brings its own goal. A query may also carry a
`description`, which the rendered report shows, and a `bind:` map of values for its own variables
(`bind: {ref: U1}`, or `bind: {ref: [U1, U2]}` for a list). A `bind:` value means what `--bind`
means for one query. YAML types each value, so `3.3` is a number and `"3"` is text.
`--format markdown` and `--format html` write the whole set as one document with a section per
query, and `--format json` writes the `RunQueriesResponse` wire message. csv is refused, because one
csv file holds one table.

A query may be unanswerable, because it names a misspelled relation for instance. It is reported
under its own name, and the rest still answer. One that does not parse also names the line and
column in its own text, counted without the preamble. The command then exits non-zero, after writing
everything, so a script sees both the answers and the gap. `examples/netlist-audit` is a full audit
written this way, and it needs no preamble at all, because the shipped library (above) holds
everything its tables share.

**A project keeps its sets in `queries/`**, one per `.yaml` file beside `project.yaml`, and `--set`
takes a set's name there as well as a file: `agni query designs/gateway --set power-review` runs the
tutorial project's `queries/power-review.yaml`. As with `--checklist`, a value ending in `.yaml` or
`.yml` is a file and any other value is a name the project must keep. In the viewer, the query
panel's **Sets** menu lists the same sets and opens one as a tab per query, in the set's order. Each
tab answers over one read of the design. **From a file…** does the same for a set file on your
disk. A query the engine cannot answer shows its error in its own tab.

A set holds TABLES. A question with a pass or fail answer belongs in a check or a review checklist,
whose verdicts also say what passed and why.

### Search the board (any tier, one language)

Board facts query the same way. A length is held in metres and every answer shows it in
millimetres, the unit the board is drawn in, so a threshold over one writes its unit, `?w < 0.3mm`.
A bare `0.3` would be read as 0.3 m and every net would pass it (agni issue 1035). "Nets routed
thinner than 0.3 mm":

{{ agniRun "content/guide/runs/query-board-track-width.yaml" }}

And you can **join across the schematic, the datasheet, and the board in one question**, "a net
routed thin that carries a high-current part":

```
board.track_width(?net,?w), component.net(?ref,?net), component.mpn(?ref,?mpn), param.max(?mpn,"IOUT",?i), ?w < 0.25mm => ?net, ?ref, ?i
```

### Follow a rail through passives (reaches)

`net.reaches(from, net)` walks connectivity through series passives (resistors, ferrites, fuses). Use
it to ask "what does this rail actually feed after the filter". "Everything reachable from GND":

{{ agniRun "content/guide/runs/query-reaches.yaml" }}

### See what the walk crossed (route)

`net.reaches` tells you a net is reachable. It does not tell you what stands between the two, so you
end up opening the schematic to check an answer the tool already knew. `net.route(from, net, path)` is
the same walk with that half kept. Its `path` binds the nets in crossing order, with the part crossed
between each pair, so an answer carries its own evidence.

{{ agniRun "content/guide/runs/query-route.yaml" }}

Read `GND -> [R1] -> SIG` as: the walk left GND, went through R1, and arrived at SIG. The names outside
the brackets are nets and the one inside is the part. The first row is the reflexive one, since a net
reaches itself at zero crossings and its route is its own name.

It answers with *a* route and not every route. The walk is breadth-first, so where two resistors
bridge the same two nets you get the shorter one and no mention of the other. And a route never ends
on a rail or a plane, because the walk refuses to enter one at all.
[`agni trace`](../cli-reference/#trace-file) is the pin-to-pin form. It does end on a rail, and it
reports the test points sitting on each net along the way.

`net.reaches` and `net.route` are the walk the rules use, so a query and a rule agree on what is
reachable. For a walk over a relation of your own, the engine's
`graph.closure(rel, from, to, min, max, hops)` and `graph.paths` take the step relation by name and
walk no further than the hops you give them. Every net two resistor crossings or fewer from
CAN1_CANH:

```
step(?a, ?b) :- component.class(?r, "resistor"), component.net(?r, ?a), component.net(?r, ?b), ?a != ?b;
graph.closure("step", "CAN1_CANH", ?to, 1, 2, ?h) => ?to, ?h
```

Unlike `net.reaches`, this walk enters rails, so a rail's step ends the question only if your step
relation says so.

## Asking under your own vocabulary

Some relations report what the engine *believes* rather than what is in the file. `net.rail`,
`net.feedback`, and `pin.type` are resolved from a vocabulary at the moment the design is read.

That vocabulary is the built-in one unless you say otherwise, and it is built from the names most
boards use. On a board that names rails function-first, the built-in vocabulary can return badly
wrong answers for your project.

Isolating that takes moving two things aside rather than one. First, the tutorial project declares a
`conventions:` section in its `project.yaml`. Naming the design applies it automatically, so the
before-state is otherwise unreachable, and the run below deletes the section from its copy of the
project. Second, a seeded datasheet's pin functions establish the rail role on their own. With
`params/` in place, this board classifies all four rails whatever the naming vocabulary says. Move
both and the question is about NAMING alone:

{{ agniRun "content/guide/runs/query-rail-builtin-vocabulary.yaml" }}

Leave the project's vocabulary in place and ask again, with the corpus still aside so the pair
differs in exactly one thing:

{{ agniRun "content/guide/runs/query-rail-own-vocabulary.yaml" }}

Nothing about the design changed. Author a lexicon in this loop. Ask, compare against the rails you
know the board has, fix the pattern, and ask again. See
[Naming conventions](../naming-conventions/).

The datasheet route reaches the same answer with no lexicon at all. The corpus was doing exactly
that before these runs left it out. [Datasheets](../datasheets/) covers that route.

Only the config's lexicon half is used here, since a query runs no rules.

## In the viewer

The viewer has a **Query** panel that runs the same queries against the design you have open. Type a
query, press Run (or ⌘/Ctrl+Enter), and the results appear as a table. Each row has a small toggle
that expands how the row holds, so the citations are out of your way until you want them. When you
open the first row, the panel asks the question again with
[witnesses](#see-how-a-row-was-derived-witnesses). The row then shows the tree of rules and facts
behind it. Each net and part in the tree links to the drawing, as a result cell does, and a `not`
that held is marked **absent**. If nobody opens a row, the query never asks for witnesses.

The panel runs the query on the server, over the open file, so you get the same answers as
`agni query` without leaving the design. The **vocabulary** control in the top bar applies here too,
and a query and a check in the same session then answer under the same vocabulary.

A drawer at the panel's edge holds **Examples & relations**. It is `agni query --relations` with a
mouse. It opens on the root of the namespace, and choosing a module (`net ›`) lists its members,
each marked with its tier (Netlist, Board, Library and so on). Choosing a member shows its typed
signature, which argument types the engine inferred, a library member's definition and its reference
page, and an Insert button that puts `net.has_test_point(?n)` into the query at the cursor. The
breadcrumb above walks back up. The drawer lists the open design's project library beside the
shipped one, as `--relations --design` does.

Datasheet relations (`param.max`, `param.typ`) answer in the viewer when the design's project has a
`params/` corpus or the server was started with `--params`, the same corpus `agni query` reads.

A click on the drawing selects what you clicked and describes it in the **Properties** panel, and
leaves the query box alone. Properties lists a few **Ask** questions for what you selected ("What
nets does it touch?", "Which test points reach its nets?"), and each opens in a query tab of its
own, named for the question and the part, so a query you were writing is still in its tab. **+**
opens an empty tab. A search fills the box in the tab you are on. Either way the query's variables
are bound to what you picked or searched for. The values show as chips under the box, one per
variable, so the query text does not change, whatever you picked. Edit a chip's value to ask about
something else, or remove it to leave the variable free and widen the question. Editing a bound
variable out of the query drops its chip.

A question about one thing you picked asks for its first 50 rows, so a click on a ground net with a
thousand pins draws quickly. The count under the answer then reads `50 of 1229 result(s)` with a
**show all** button that asks the same question again without the limit. A query you write and run
yourself is never cut.

You do not have to memorize the vocabulary. Below the query box the panel lists every relation as a
**click-to-insert chip**, grouped by kind (Netlist, Board, Datasheet, Predicates, and any extension
relations your deployment adds). Clicking a chip drops its template at your cursor, so
`component.net` inserts `component.net(?ref_des, ?net)` ready to wire into the rest of the
query. Hover a chip to see its full signature and a one-line description.

## Every answer is checkable

The last column is always **provenance**, the source of the facts that produced the row: a schematic
file, or a datasheet document, page, and table, or a board net. A query never gives you a number you
cannot trace back to its source. In the viewer the
provenance sits behind each row's expand toggle instead of in a trailing column.

## Taking a view out of the tool

A question you keep asking is a **view**, and `--format` takes one out of the tool.

```bash
agni query designs/gateway/gateway.edn \
  'component.net(?r,?n), component.class(?r,"test_point") => ?n, ?r' \
  --format markdown --title "Test point coverage" > tp-coverage.md
```

Five formats. `text` is the aligned terminal table and is the default. `csv` opens in a spreadsheet.
`json` is for tooling. `markdown` and `html` are documents that carry the title, the design, and
**the query itself**, above the answer.

A table pasted into a ticket is a
screenshot, and nobody downstream can re-run it, scope it differently, or disagree with it. A view
carries the question that produced it, so it can be checked the way any other claim can.

**`--server` adds a link that asks the question again in the viewer**, printed as `look at it:` on
stderr, as `check` and `trace` do. The link carries the query, its `--bind` values and its `--title`
rather than an answer, so following it re-asks the question of the design as it is now, and
`--server self` serves the design from the same process until Ctrl-C. When a query comes from a
project's set (`--set power-review`), its link names the set and the query, one link per query. That
keeps the link short however long the query is. A set read from a file is on no server, so its links
carry each query's text with the preamble in front.

The link opens the design on its first sheet and brings the Query panel forward with the answer in
a tab of its own. When the answer has one row, the viewer draws it the way a click on its first
part, net or pin would, on that entity's sheet. A longer answer draws nothing until you click a
cell, because the link cannot say which row it meant. A line above the table says the answer is
there.
A trace link brings the Trace panel forward the same way.

Two of these behaviours are deliberate:

- **`csv` carries no preamble.** No title, no query, no count, because its first row has to be the
  header for a spreadsheet or a script to bind to it. The document formats carry the question; csv
  carries the table.
- **An empty result still says something.** `csv` writes the header alone, so a consumer can tell a
  view that matched nothing from a run that produced no file. `markdown` and `html` write a sentence
  rather than an empty table, because a table with no rows reads as an omission rather than an
  answer.

The design is named by its mount URI, never by the path on the machine that ran the query, so a view
is safe to commit or mail.

```bash
agni query designs/gateway/gateway.edn 'net.rail(?n) => ?n' --format csv > rails.csv
```

## Following a signal across the parts in the way

A query joins facts, and a path is a sequence of unknown length. A relation can hold a path only as
a rendered string. `net.route` does that between two nets, one route per pair and never ending on a
rail, so it cannot answer pin to pin. "Which nets carry a resistor" is a query and "what does this
pin go through to reach that one" is not.

`agni trace` is that second question. It walks from one pin to another through the series parts that
split a net without breaking the path, and prints what it crossed.

{{ agniRun "content/guide/runs/trace-path.yaml" }}

That output shows three things.

**The part in the middle** is `R3`. It sits between the MCU's reset pin and the PMIC's power-good
output, so the two pins are on different nets and no per-net question can see that they are joined.
A series element that splits a net is the ordinary case, not an awkward one. So people often get
"are these connected" wrong by eye.

**The output also lists what else sits on each net**, probe points first, because a
reviewer reading a route is usually working out where to put a probe or which capacitor is in the
way. Filtering the output down to the series elements would remove the most useful thing on the line.

**A no is an answer.** The third trace found nothing, and it says which nets the two pins are actually
on, how far it searched, and what it will and will not cross. A capacitor is a DC block, so the walk
never crosses one. A route can END on a rail or plane but never passes through one, since a supply
joins everything to everything and a route through one would mean nothing.

The command does not guess. If you name a pin the design does not have, it reports an error rather
than an empty result. A name spelled wrong in a declaration and two pins that are genuinely
unconnected are opposite problems. Reporting the first as the second would send you to look at the
board instead of at what you typed.

## What a query is not

A query **reports** and does not judge. It has no notion of pass/fail, and
[checks](../checks-and-reports/) handle that. If you find yourself running the same query to catch a
recurring problem, that is a sign it should become a rule.

## The relations used here

These are the full cards for the relations in the examples above, inlined so you do not have to
leave the page. The [relation catalog](../../reference/relations/) serves the same text. Both are
generated from the relation definitions themselves, so they cannot drift from what the engine
actually projects.

**Read the "absence is not a pass" section on any relation you scope a question by.** An empty result
means the fact is absent from your design. That is a different statement from "nothing matched", and
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
<summary><strong><code>component.net</code></strong>, a part sits on a net</summary>

{{ includeCard "content/reference/relations/component.net.md" }}

</details>

<details>
<summary><strong><code>reaches</code></strong>, nets reachable through passives</summary>

{{ includeCard "content/reference/relations/net.reaches.md" }}

</details>

<details>
<summary><strong><code>route</code></strong>, the same walk, with the path it took</summary>

{{ includeCard "content/reference/relations/net.route.md" }}

</details>

<details>
<summary><strong><code>board.track_width</code></strong>, a net's thinnest copper track</summary>

{{ includeCard "content/reference/relations/board.track_width.md" }}

</details>
