---
title: "13. Drive it in the browser"
description: "The same catalog, the same verdicts, against the drawing instead of a terminal."
---

Every rung so far ran at the command line, which is right for CI and wrong for the part of review
where somebody points at a net and asks what is wrong with it. `agni serve` puts the same engine
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
so whatever you mounted is readable by them. That is usually what you want on a workbench and rarely
what you want on shared Wi-Fi. `--addr 127.0.0.1:8090` binds this machine only, and the line
disappears when it applies to nobody.

Outside a checkout, `--web-dir` (or `web_dir` in an `agni.yaml`) names wherever the built viewer
lives; [Running the server](../../guide/running-the-server/) covers it.

`--mount name=path` exposes a folder in the file browser, and it is repeatable, so a real deployment
mounts several project folders at once. Point it at the **project root**, not at
`designs/jetson/`. A mount rooted inside the design puts `project.yaml`, which carries the
checklists, above the mount, where the server cannot reach it, and the Review panel then has no
checklist to offer.

`--review-store` is the only other flag, and it is the one genuinely new thing here, a place to
keep review runs. Everything else this rung needs, the server discovers.

## The tiers arrive on their own

Rungs 4 through 7 each added a tier, and none of them is a flag here. A project descriptor names its
own layout, so `FSStore` reads the `conventions:` and `checklists:` sections of `project.yaml`,
composes `profiles/` and `params/` from the project root, and takes each design's `intent:` section
and `symbols/` from beside the design.

Passing a flag for one of them does not switch it on, because it is already on. For the conventions
that is harmless, since a `--conventions` file replaces the project's section rather than adding to
it, so a file saying the same thing leaves the output unchanged. A directory tier loads a second
time instead, and `agni check` now refuses the profile case outright rather than reporting every
profile finding twice.

The Rules panel below is how you confirm the tier reaches the design rather than assuming
it.

Open `http://localhost:8090/`, pick the mount, choose a design, and open it.

## What the checks say

Run the checks in the panel and you get the findings the ladder has been building up, most of them
built-in rules from rung 2. These are the ones that carry a tier's namespace, with the run's total,
and the panel and the command line report them identically:

{{ agniRun "content/tutorials/runs/13-check-tiers.yaml" }}

Only the naming rule has findings. The other tiers are present and passing, which a findings list
cannot show, so ask for their verdicts:

{{ agniRun "content/tutorials/runs/13-verdicts-tiers.yaml" }}

Read the rule column. `jetson/` is your conventions section. `jetson-profiles/` is your PCIe profile
superseding the built-in. `intent/` is your architecture declaration. Every tier you added is
present, namespaced exactly as it is at the command line, because it is the same catalog.

The namespace also records **how** the tier arrived. A project composes its profiles under the
project's own name, so these read `jetson-profiles/`. An overlay passed with `--profile-path` composes
under the fixed name `profile-overlay/` instead. The file and the rules are the same, and the label is
how you tell which route a run took.

## The failure this read cannot have

[Rung 1](../01-read-a-design/) spent its length on a bad read. Its close cousin is the
unresolved-symbol failure, where the parts load and the pins do not, because the symbols that carry
the pins live in a library the read never found. It is the most expensive mistake on the ladder, so
it is fair to ask what it looks like here.

On this board it cannot happen in the usual way, because every KiCad sheet embeds a copy of each
symbol it places, so the pins travel inside the file. It nearly happened anyway. KiCad files an
embedded copy that differs from its library under a second name, and until agni issue 1011 the reader
looked only at the first, so one stray sheet in this folder resolved 97 of its 138 symbols. `validate`
in rung 1 is how you would have seen it. A KiCad project that keeps its symbols in a separate library,
and xschem and gEDA schematics generally, can still fail the classic way.

## Serve it wrong, on a design that can go wrong

The read that can go wrong here is rung 1's, one sheet named on its own. Score it with the review
layer:

{{ agniRun "content/tutorials/runs/13-review-broken-read.yaml" }}

Seven passes, and they are the dangerous part. I3 and I4 pass because their rules found nothing on
this sheet to judge, and an item whose rule fires on nothing reads as a pass. That is the hollow pass
[rung 9](../09-read-the-verdicts/) warned about, and `--verdicts` on the same read shows both rules
with no subjects at all. The one item that says something is wrong is A2,
whose intent names connectors the design does not have. The review layer is not a substitute for
rung 1. Confirm what was read, then score it.

## What the panels are for

{{ includeFile "figures/viewer-panel-map.svg" }}

The **sheet badge** carries the finding count, so a multi-sheet design shows you where the problems
are before you open anything.

**Checks** holds the findings table above. Selecting a row highlights its subject on the canvas, which a terminal
cannot do, so you go from "net `/SFP/SFI_CONN_RX_P` has no ESD protection" to seeing where that net
actually runs. Nothing is lit until you select one, since a sheet with a few dozen findings would
otherwise be mostly highlight. **Highlight › Show every finding** lights the whole run at once, and the
viewer remembers the choice.

**Canvas** renders faithfully when the design carries geometry and computes a layout when it does
not, exactly as [rung 3](../03-see-it/) described. The WebGL and SVG toggle matters on large boards.

**Rules** lists the composed catalog, which is how you confirm a tier actually loaded rather than
inferring it from findings that did or did not appear. Select the design first, because with no design
chosen the panel lists the server's own catalog, and a project's tiers compose per design, so
`jetson/`, `jetson-profiles/` and `intent/` appear only once the panel knows which design it is
listing for.

This is also where supersession from rung 5 is visible, as arithmetic. Selecting the design moves
the built-in `profile/` count down by four and adds three `jetson-profiles/` rules, because the
project's PCIe profile replaces the built-in one wholesale and declares one requirement fewer. The
CLI says the same thing in its `supersedes` note.

**Properties** follows the selection. Click something on the canvas and it shows what that thing is,
what it connects to, and for a part whether the corpus holds its datasheet spec. Its **ask**
row offers questions about the selection, and each opens in the Query panel with the query visible
before it runs.

**Query** is the [querying guide](../../guide/querying/) with a drawing attached. Each question gets
a tab of its own, so an answer stays put while you ask the next one, and clicking a cell locates its
net or part on the canvas. The **Sets** menu opens the query sets the project keeps in `queries/`,
one tab per query. A query that does not parse shows its line with a caret under the place it
stopped, as `agni query` prints it.

**Trace** asks for two pins and follows the signal through the series parts between them, as
`agni trace` does, then lights the route on the drawing. A pin name that matches nothing in the
design says so, which is a different answer from two pins with no route between them.

**Tables** lays the run on screen out as rows, the findings, the verdicts and the rules that were
skipped, and a comparison's changes while one is open. A header click sorts it the way
`--order-by` does, and Save CSV writes the csv the CLI would write for the same answer.

**Coverage** shows each interface profile the design matched, with a chip per required signal, so
the PCIe profile from rung 5 reads as signals found and signals missing. A found signal's chip
locates its net.

**Parts** lists every part whose MPN has a published spec, with its parameters, limits and the
datasheet page each came from. It reads the `params/` rung 6 seeded, so a part with no spec there
does not appear.

**Compare** in the top bar opens [rung 10's](../10-compare-revisions/) diff with a revision picker,
in the **Diff** and **Changes** panels.

**Review** is your checklist from [rung 8](../08-write-your-checklist/), scored in the browser. It
needs no second server, because `--review-store` was on the command at the top, and `project.yaml`
sits at the project root, which is inside the mount.

The checklist picker lists the project's checklists by name, in the order `project.yaml` declares
them, so its first entry is the project's default. This project declares one, `review`. Pick `review` and
press Run review. What comes back is the same verdict
[rung 9](../09-read-the-verdicts/) read in the terminal, item by item, with the same vocabulary, so an
item that could not be evaluated is styled differently from one that passed, because the two mean
opposite things. The headline leads with coverage rather than pass/fail, for the reason rung 9 gave
about what a bare pass count hides.

`B1`, the board's minimum track width, is worth a look because it is a board question and the entry
is a schematic, which carries no copper. It reads `pass` in the panel exactly as it does in the
terminal, because both attach the `.kicad_pcb` the descriptor declares as a companion. Until
[agni issue 646](https://github.com/panyam/agni/issues/646) the panel scored the entry alone and read
a board item as `not-applicable`, so if you have an older run in the history, that is why it
disagrees.

A failing item lists the findings that failed it, and clicking one highlights it on the canvas, which
is the same move Checks offers one level down.

Runs are kept, so the panel opens on the latest one and the picker holds the history. That is the
browser half of [rung 11](../11-archive-and-gate/), and it compares this week's verdict against
last month's without either of them being a file somebody had to remember to save. Each stored
run also carries the checklist it actually scored, so a run from before you edited the checklist still
shows the questions it really asked.

## Where this fits

The CLI is for the gate. It runs in CI, returns an exit code, and writes the archive.

The browser is for the conversation. It is what you open in a review meeting when somebody asks
"where is that net", and what you hand to an engineer who has a finding and needs to see the
circuit around it.

They read the same catalog and produce the same verdicts, so neither is a second opinion on the
other. Choosing between them is about who is looking and why.

## That is the ladder

Thirteen rungs, one board, from confirming a file was read correctly through to a house checklist
that gates a merge, an archive that outlives the design, and a browser view of the same result.

The two things worth revisiting once you are running this for real are coverage and the parameter
corpus. Coverage tells you how much of your checklist is genuinely being decided. Seeding parts is
usually the cheapest way to move it.
