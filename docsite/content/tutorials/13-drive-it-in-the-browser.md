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
agni serve --addr :8090 \
  --mount gateway=designs/gateway \
  --symbol-path designs/gateway/symbols \
  --conventions conventions.yaml \
  --profile-path profiles \
  --params params \
  --intent-path designs/gateway/intent.yaml
```

```
note: profile-overlay supersedes 5 rule(s): profile/can-signal-missing, profile/can-host-incomplete, ...
serving web at http://localhost:8090/ with 1 mount(s) (Ctrl-C to stop)
  on this network: http://192.168.1.23:8090/ (all interfaces, no auth)
```

That second line is the one to read twice. `--addr :8090` binds every interface, so anyone who can
reach your machine can reach the server, and it has no authentication: whatever you mounted is
readable by them. That is usually what you want on a workbench and rarely what you want on shared
Wi-Fi. `--addr 127.0.0.1:8090` binds this machine only, and the line disappears when it applies to
nobody.

Those flags are the same tiers from rungs 4 through 7, in the same order, doing the same jobs. The
supersession note from rung 5 prints here too, because the server composes its catalog exactly the
way the CLI does.

`--mount name=path` is the only new one. It exposes a folder in the file browser, and it is
repeatable, so a real deployment mounts several project folders at once.

Open `http://localhost:8090/`, pick the mount, choose a design, and open it.

## Serve it wrong first

Leave `--symbol-path` off and run the checks. You get 88 findings, and every one of them says:

```
cannot decide: pins are unknown while gateway:CONN4, gateway:TVS, gateway:BUCK, ...
are unresolved (see symbol-unresolved)
```

This is [rung 1's](../01-read-a-design/) unresolved-symbol failure and [rung 9's](../09-read-the-verdicts/)
`inconclusive` verdict, met together in the panel. The rules ran. They had the design. They could not
reach a verdict because the pins were not there, and rather than pass, fail, or stay silent, each one
says so and names the cause.

Compare that to what the same board did in rung 1 through `agni check`, where the bad read produced
94 confident, wrong findings. The difference comes from the review layer having a word for "I looked
and I cannot tell" and the plain catalog not having one, rather than from the tool getting cleverer.

## Serve it right

Add `--symbol-path` back and re-run:

**12 findings**, and they are the ones you already know:

| subject | rule | message |
|---|---|---|
| I2C_SCL | `i2c-pull-up` | I2C net has no pull-up resistor |
| I2C_SDA | `i2c-pull-up` | I2C net has no pull-up resistor |
| CAN1_CANH | `profile-overlay/can-signal-missing` | CAN interface is missing required signal STB |
| PMIC_MAIN_12V0 | `decoupling-present` | power rail has no decoupling capacitor |
| XTAL_IN | `gateway/signal-net-naming` | net name matches no allowed naming pattern |
| PMIC_IO_1V8 | `intent/voltage-domain-mismatch` | rail is declared in domain "core" (3.3V) but its name declares 1.8V |
| CAN1_CANH | `esd-protection` | externally-exposed signal net has no ESD protection |
| GND | `test-point-coverage` | rail carries no test point |

Read the rule column. `gateway/` is your conventions file. `profile-overlay/` is your CAN profile
superseding the built-in. `intent/` is your architecture declaration. Every tier you added is
present, namespaced exactly as it is at the command line, because it is the same catalog.

## What the panels are for

The **sheet badge** carries the finding count, so a multi-sheet design shows you where the problems
are before you open anything.

**Findings** is the table above. Selecting a row highlights its subject on the canvas, doing the one
thing a terminal cannot: going from "net `CAN1_CANH` has no ESD protection" to seeing where that
net actually runs.

**Canvas** renders faithfully when the design carries geometry and computes a layout when it does
not, exactly as [rung 3](../03-see-it/) described. The WebGL and SVG toggle matters on large boards.

**Rules** lists the composed catalog, which is how you confirm a tier actually loaded rather than
inferring it from findings that did or did not appear.

**Compare** is [rung 10's](../10-compare-revisions/) diff with a revision picker.

**Review** is your checklist from [rung 8](../08-write-your-checklist/), scored in the browser. It
needs a server started with a place to keep runs:

```
agni serve web --mount proj=. --review-store ./reviews
```

Pick your `review.yaml` and press Run review. What comes back is the same verdict
[rung 9](../09-read-the-verdicts/) read in the terminal, item by item, with the same vocabulary: an
item that could not be evaluated is styled differently from one that passed, because the two mean
opposite things. The headline leads with coverage rather than pass/fail, for the reason rung 9 gave
about what a bare pass count hides.

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
