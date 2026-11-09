---
title: "13. Drive it in the browser"
description: "The same catalog, the same verdicts, against the drawing instead of a terminal."
---

Every rung so far ran at the command line. That suits CI. It does not suit the part of review where
somebody points at a net and asks what is wrong with it. `agni serve` puts the same engine
behind a browser, with the same rules, tiers and verdicts rendered against the drawing.

Nothing new is computed here. If the panel disagreed with the CLI, one of them
would be lying.

## Serve the project

```
agni serve --addr :8090 --mount proj=. --review-store ./reviews --web-dir ../../web
```

Run it from the project root, `examples/jetson-project`. The flag list is short on purpose, because
the server discovers the rest. `--web-dir` points at the viewer's built assets, which sit four levels
up in a checkout, as in [rung 3](../03-see-it/).

```
serving web at http://localhost:8090/ with 1 mount(s) (Ctrl-C to stop)
  on this network: http://192.168.1.23:8090/ (all interfaces, no auth)
```

Read the second line of the startup output twice. `--addr :8090` binds every
interface, so anyone who can reach your machine can reach the server, and it has no authentication,
so they can read whatever you mounted. You usually want that on a workbench, and rarely on shared
Wi-Fi. `--addr 127.0.0.1:8090` binds this machine only, and the line
disappears when it applies to nobody.

Outside a checkout, `--web-dir` (or `web_dir` in an `agni.yaml`) names wherever the built viewer
lives; [Running the server](../../guide/running-the-server/) covers it.

`--mount name=path` exposes a folder in the file browser, and it is repeatable, so a real deployment
mounts several project folders at once. Point it at the **project root**, not at
`designs/jetson/`. `project.yaml` carries the checklists. A mount rooted inside the design leaves
`project.yaml` above the mount, where the server cannot reach it, and the Review panel then has no
checklist to offer.

`--review-store` is the only other flag. It names a place to keep review runs, and it is the one
genuinely new thing here. The server discovers everything else this rung needs.

## The tiers arrive on their own

Rungs 4 through 7 each added a tier, and none of them is a flag here. A project descriptor names its
own layout, so `FSStore` reads the `conventions:` and `checklists:` sections of `project.yaml`,
composes `profiles/` and `params/` from the project root, and takes each design's `intent:` section
and `symbols/` from beside the design.

Passing a flag for one of them does not switch it on, because it is already on. For the conventions
that is harmless. A `--conventions` file replaces the project's section rather than adding to it, so
a file saying the same thing leaves the output unchanged. A directory tier loads a second time
instead. For profiles, `agni check` now refuses the second load outright rather than reporting every
profile finding twice.

Use the Rules panel below to confirm that the tier reaches the design, rather than assuming it.

Open `http://localhost:8090/`, pick the mount, choose a design, and open it.

## What the checks say

The ladder has been building up findings, and running the checks in the panel shows them. Most of
them come from rung 2's built-in rules. The capture below keeps the findings whose rule carries a
tier's namespace, with the run's total. The panel and the command line report them identically:

{{ agniRun "content/tutorials/runs/13-check-tiers.yaml" }}

Only the naming rule has findings. The other tiers are present and passing. A findings list cannot
show a pass, so ask for their verdicts:

{{ agniRun "content/tutorials/runs/13-verdicts-tiers.yaml" }}

Read the rule column. `jetson/` is your conventions section. `jetson-profiles/` is your PCIe profile
superseding the built-in. `intent/` is your architecture declaration. Every tier you added is
present, namespaced exactly as it is at the command line, because it is the same catalog.

The namespace also records **how** the tier arrived. A project composes its profiles under the
project's own name, so these read `jetson-profiles/`. An overlay passed with `--profile-path` composes
under the fixed name `profile-overlay/` instead. The file and the rules are the same, and the label
tells you which route a run took.

## The failure this read cannot have

[Rung 1](../01-read-a-design/) spent its length on a bad read. Its close cousin is the
unresolved-symbol failure. The parts load and the pins do not, because the pins live in the symbols,
and the read never found the library holding them. It is the most expensive mistake on the ladder,
so it is fair to ask what it looks like here.

On this board it cannot happen in the usual way, because every KiCad sheet embeds a copy of each
symbol it places, so the pins travel inside the file. It nearly happened anyway. When an embedded
copy differs from its library, KiCad files it under a second name. Until agni issue 1011 the reader
looked only at the first name, so one stray sheet in this folder resolved 97 of its 138 symbols.
Rung 1's `validate` would have shown it. A KiCad project that keeps its symbols in a separate
library, and xschem and gEDA schematics generally, can still fail the classic way.

## Serve it wrong, on a design that can go wrong

The read that can go wrong here is rung 1's, one sheet named on its own. Score it with the review
layer:

{{ agniRun "content/tutorials/runs/13-review-broken-read.yaml" }}

Seven passes, and they are the dangerous part. I3 and I4 pass because their rules found nothing on
this sheet to judge, and an item whose rule fires on nothing reads as a pass.
[Rung 9](../09-read-the-verdicts/) warned about this hollow pass, and `--verdicts` on the same read
shows both rules with no subjects at all. Only A2 says something is wrong. Its intent names
connectors that are not in the design. The review layer is not a substitute for
rung 1. Confirm what was read, then score it.

## What the panels are for

{{ includeFile "figures/viewer-panel-map.svg" }}

The **sheet badge** carries the finding count, so a multi-sheet design shows you where the problems
are before you open anything.

**Checks** holds the findings table above. Selecting a row highlights its subject on the canvas. A
terminal cannot do that. You go from "net `/SFP/SFI_CONN_RX_P` has no ESD protection" to seeing
where that net actually runs. Nothing is lit until you select one, since a sheet with a few dozen
findings would otherwise be mostly highlight. **Highlight › Show every finding** lights the whole
run at once, and the viewer remembers the choice. Grouped by rule, each rule's header links to its
page in the [rule reference](../../reference/rules/), which says what the rule checks and why.

**Canvas** renders faithfully when the design carries geometry and computes a layout when it does
not, exactly as [rung 3](../03-see-it/) described. It draws in SVG. The **WebGL** button draws the
same sheet on the GPU, which pans a large board more smoothly, and a few features still draw only
in SVG.

**Rules** lists the composed catalog. Use it to confirm a tier actually loaded, rather than inferring
that from findings that did or did not appear. Select the design first. With no design chosen, the
panel lists the server's own catalog. A project's tiers compose per design, so `jetson/`,
`jetson-profiles/` and `intent/` appear only once the panel knows which design it is listing for.
A rule agni documents links to its reference page, and a project's own rule has none.

The Rules panel also shows rung 5's supersession, as arithmetic. Selecting the design moves
the built-in `profile/` count down by four and adds three `jetson-profiles/` rules, because the
project's PCIe profile replaces the built-in one wholesale and declares one requirement fewer. The
CLI says the same thing in its `supersedes` note.

**Properties** follows the selection. Click something on the canvas and it shows what that thing is,
what it connects to, and for a part whether the corpus holds its datasheet spec. Its **ask**
row offers questions about the selection, and each opens in the Query panel with the query visible
before it runs. Shift-drag a rectangle to select a block of the sheet as one area, and Shift-click to
add or remove a part. Properties then lists the nets crossing the area's edge, which are the block's
interface, and its Ask rows ask about every part at once.

**Query** is the [querying guide](../../guide/querying/) with a drawing attached. Each question gets
a tab of its own, so an answer does not change while you ask the next one, and clicking a cell
locates its net or part on the canvas. The **Sets** menu opens the query sets the project keeps in
`queries/`, one tab per query. A query that does not parse shows its line with a caret under the
place it stopped, as `agni query` prints it.

**Trace** asks for two pins and follows the signal through the series parts between them, as
`agni trace` does, then lights the route on the drawing. A pin name that matches nothing in the
design says so. That is a different answer from two pins with no route between them. Each end
lists the design's parts and pins as you type, matching letters in order, so `u13` finds both `U13`
and `U1.3`, and a pin's datasheet name finds it too. Choosing a part lists its pins with their nets.
Typing a full pin and pressing Enter traces it as typed. A part or pin selected in **Properties**
offers **trace from here** and **to here**.

**Tables** shows the run as rows. It lists the findings, the verdicts and the skipped rules, and a
comparison's changes while one is open. A header click sorts it the way
`--order-by` does, and Save CSV writes the csv the CLI would write for the same answer.

**Coverage** shows each interface profile the design matched, with a chip per required signal, so
the PCIe profile from rung 5 reads as signals found and signals missing. A found signal's chip
locates its net. Run checks matches the profiles, so the panel is empty until then.

**Parts** lists every part whose MPN has a published spec, with its parameters, limits and the
datasheet page each came from. Rung 6 seeded `params/`, and the panel reads it, so a part with no
spec there does not appear.

**Compare** in the top bar opens [rung 10's](../10-compare-revisions/) diff with a revision picker,
in the **Diff** and **Changes** panels.

**Review** is your checklist from [rung 8](../08-write-your-checklist/), scored in the browser. It
needs no second server, because `--review-store` was on the command at the top, and `project.yaml`
sits at the project root, which is inside the mount.

The checklist picker lists the project's checklists by name, in the order `project.yaml` declares
them, so its first entry is the project's default. This project declares one, `review`. Pick
`review` and press Run review. [Rung 9](../09-read-the-verdicts/) read this verdict in the
terminal, and the panel returns the same one, item by item, with the same vocabulary. An item that
could not be evaluated is styled differently from one that passed, because the two mean opposite
things. The headline leads with coverage rather than pass/fail. Rung 9 gave the reason, in what it
said about a bare pass count.

Look at `B1`, the board's minimum track width. It is a board question, and the entry is a
schematic, which carries no copper. It reads `pass` in the panel exactly as it does in the
terminal. The descriptor declares the `.kicad_pcb` as a companion, and both surfaces attach it.
Until [agni issue 646](https://github.com/panyam/agni/issues/646) the panel scored the entry alone
and read a board item as `not-applicable`. An older run in the history may disagree for that
reason.

A failing item lists the findings that failed it, and clicking one highlights it on the canvas.
Checks offers the same move one level down.

Runs are kept, so the panel opens on the latest one and the picker holds the history. The history
is the browser half of [rung 11](../11-archive-and-gate/). It compares this week's verdict against
last month's, and nobody had to remember to save either one as a file. Each stored
run also carries the checklist it actually scored, so a run from before you edited the checklist still
shows the questions it really asked.

## Where this fits

The CLI is for the gate. It runs in CI, returns an exit code, and writes the archive.

The browser is for the conversation. You open it in a review meeting when somebody asks "where is
that net", and you hand it to an engineer who has a finding and needs to see the circuit around it.

They read the same catalog and produce the same verdicts, so neither is a second opinion on the
other. Choosing between them is about who is looking and why.

## That is the ladder

Thirteen rungs, one board, from confirming a file was read correctly through to a house checklist
that gates a merge, an archive that outlives the design, and a browser view of the same result.

Once you are running this for real, revisit two things, coverage and the parameter corpus. Coverage
tells you how much of your checklist is genuinely being decided. Seeding parts is usually the
cheapest way to move it.