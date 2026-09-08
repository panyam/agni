---
title: "13. Drive it in the browser"
description: "The same catalog, the same verdicts, against the drawing instead of a terminal."
---

Every rung so far ran at the command line, which is right for CI and wrong for the part of review
where somebody points at a net and asks what is wrong with it. `agni serve` puts the same engine
behind a browser: same rules, same tiers, same verdicts, rendered against the drawing.

Nothing new is computed here. That is the point. If the panel disagreed with the CLI, one of them
would be lying.

## Serve the project

```
agni serve --addr :8090 --mount proj=. --review-store ./reviews
```

Run it from the project root. That is the whole command, and the short flag list is the lesson
rather than an omission.

```
serving web at http://localhost:8090/ with 1 mount(s) (Ctrl-C to stop)
  on this network: http://192.168.1.23:8090/ (all interfaces, no auth)
```

That second line of the startup output is the one to read twice. `--addr :8090` binds every
interface, so anyone who can reach your machine can reach the server, and it has no authentication:
whatever you mounted is readable by them. That is usually what you want on a workbench and rarely
what you want on shared Wi-Fi. `--addr 127.0.0.1:8090` binds this machine only, and the line
disappears when it applies to nobody.

Outside a checkout, add `--web-dir` (or set `web_dir` in an `agni.yaml`) so the server can find the
viewer's own assets; [Running the server](../../guide/running-the-server/) covers it.

`--mount name=path` exposes a folder in the file browser, and it is repeatable, so a real deployment
mounts several project folders at once. Point it at the **project root**, not at
`designs/gateway/`. A mount rooted inside the design puts `project.yaml` and `review.yaml` above the
mount, where the server cannot reach them, and the Review panel then has no checklist to offer.

`--review-store` is the only other flag, and it is the one genuinely new thing here: somewhere to
keep review runs. Everything else this rung needs, the server discovers.

## The tiers arrive on their own

Rungs 4 through 7 each added a tier, and none of them is a flag here. A project descriptor names its
own layout, so `FSStore` composes `conventions.yaml`, `profiles/`, `params/` and `review.yaml` from
the project root, and each design's `intent.yaml` and `symbols/` from beside the design.

Passing a flag for one of them does not switch it on, because it is already on. It loads the tier a
second time. Rung 4's run shows the harmless version of that, where `--conventions` names the file
the project already composed and the output does not change; `agni check` now refuses the profile
case outright rather than reporting every profile finding twice.

That the tier reaches the design is worth confirming rather than assuming, which is what the Rules
panel below is for.

Open `http://localhost:8090/`, pick the mount, choose a design, and open it.

## What the checks say

Run the checks in the panel and you get the findings the ladder has been building up, most of them
built-in rules from rung 2. These are the six that carry a tier's namespace, and the panel and the
command line report them identically:

{{ agniRun "content/tutorials/runs/13-check-tiers.yaml" }}

Read the rule column. `gateway/` is your conventions file. `gateway-profiles/` is your CAN profile
superseding the built-in. `intent/` is your architecture declaration. Every tier you added is
present, namespaced exactly as it is at the command line, because it is the same catalog.

The namespace is worth a second look, because it records **how** the tier arrived. A project
composes its profiles under the project's own name, so these read `gateway-profiles/`. An overlay
passed with `--profile-path` composes under the fixed name `profile-overlay/` instead. Same file,
same rules, different label, and the label is how you tell which route a run took.

## The failure this read cannot have

[Rung 1](../01-read-a-design/) spent its length on the unresolved-symbol failure, where the parts
load, the pins do not, and the checks report a board in ruins that is really a bad read. It is the
most expensive mistake on the ladder, so it is fair to ask what it looks like here.

It does not look like anything here, and the reason is worth knowing. This rung serves the EDIF
netlist, and EDIF declares its own pins:

```
$ agni query designs/gateway/gateway.edn 'pin.net(?r,?p,?n) => count(?p)'
count(p)  provenance
56        designs/gateway/gateway.edn
```

Fifty-six pins, with no symbol path anywhere. `--symbol-path` points at a directory of `.sym` files,
which an `.edn` never references, so there is nothing for the flag to do and nothing that removing
it can break. A netlist format that carries its own pins cannot suffer the failure at all.

The formats that can are the ones whose symbols live in a separate library: `gateway.kicad_sch`
here, and xschem and gEDA schematics generally. This project ships both views of the board, so the
failure is one file away.

## Serve it wrong, on a design that can go wrong

Point the same project at the schematic, with its symbol library moved aside, and it is available
again. That is rung 1's recipe, scored this time by the review layer:

{{ agniRun "content/tutorials/runs/13-review-broken-read.yaml" }}

Eight of the fifteen items are `inconclusive`, and that is the outcome worth knowing. The rules ran.
They had the design. They could not reach a verdict, because the pins they needed were never
resolved, and rather than pass, fail, or stay silent, each one says so and names the parts it could
not resolve.

Compare that to the plain catalog on the same broken read, where rung 1 counted a hundred and
fourteen confident and entirely wrong findings. The difference is not that the tool got cleverer
between the two rungs. It is that [rung 9's](../09-read-the-verdicts/) vocabulary has a word for "I
looked and I cannot tell" and a bare finding list does not.

Serve that design and the panel shows the same eight, styled apart from the passes, which is the
whole reason the review layer is worth the extra tier.

## What the panels are for

{{ includeFile "figures/viewer-panel-map.svg" }}

The **sheet badge** carries the finding count, so a multi-sheet design shows you where the problems
are before you open anything.

**Findings** is the table above. Selecting a row highlights its subject on the canvas, doing the one
thing a terminal cannot: going from "net `CAN1_CANH` has no ESD protection" to seeing where that
net actually runs.

**Canvas** renders faithfully when the design carries geometry and computes a layout when it does
not, exactly as [rung 3](../03-see-it/) described. The WebGL and SVG toggle matters on large boards.

**Rules** lists the composed catalog, which is how you confirm a tier actually loaded rather than
inferring it from findings that did or did not appear. Select the design first: with no design
chosen the panel lists the server's own catalog, and a project's tiers compose per design, so
`gateway/`, `gateway-profiles/` and `intent/` appear only once the panel knows which design it is
listing for.

This is also where supersession from rung 5 is visible, and it shows up as arithmetic rather than as
a message. Selecting the design moves the built-in `profile/` count down by five and adds five
`gateway-profiles/` rules, because the house CAN profile replaces the built-in one wholesale. The
CLI prints a `supersedes` note when an overlay is composed at the command line; a project that
composes its own profiles prints nothing, so the count is the thing to read.

**Compare** is [rung 10's](../10-compare-revisions/) diff with a revision picker.

**Review** is your checklist from [rung 8](../08-write-your-checklist/), scored in the browser. It
needs no second server: `--review-store` was on the command at the top, and `review.yaml` sits at
the project root, which is inside the mount.

Pick your `review.yaml` and press Run review. What comes back is the same verdict
[rung 9](../09-read-the-verdicts/) read in the terminal, item by item, with the same vocabulary: an
item that could not be evaluated is styled differently from one that passed, because the two mean
opposite things. The headline leads with coverage rather than pass/fail, for the reason rung 9 gave
about what a bare pass count hides.

One item does disagree with the command line today, and it is the exception to this page's opening
claim rather than a refinement of it. `B1`, the fab's minimum track width, is a board question.
`agni review` reads it `fail`, because naming the design attaches the `gateway.kicad_pcb` the
descriptor declares as a companion. The panel reads it `not-applicable`, because the viewer scores
the entry netlist alone and never attaches that board. Same design, same checklist, two outcomes.

Read a `not-applicable` board item in the panel as "not measured here", and confirm it at the
command line. The rule is not in dispute, and any board-tier item reads the same way. It is tracked
in [agni issue 646](https://github.com/panyam/agni/issues/646).

A failing item lists the findings that failed it, and clicking one highlights it on the canvas, which
is the same move Findings offers one level down.

Runs are kept, so the panel opens on the latest one and the picker holds the history. That is the
browser half of [rung 11](../11-archive-and-gate/): comparing this week's verdict against last
month's, without either of them being a file somebody had to remember to save. Each stored run also
carries the checklist it actually scored, so a run from before you edited `review.yaml` still shows
the questions it really asked.

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
