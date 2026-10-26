---
title: "Trying agni on your own boards"
description: "Run a published agni release on your machine with docker or podman compose, walk the guided trial, and send back a report that holds nothing from your design."
---

This page is for trying a released agni on a machine that holds your boards, where nothing about them
should leave. You run the release from one compose file, try it on a board, and send back a report
built only from counts, timings and agni's own rule names. The release is the same one the
[demo](../../demos/) runs in a browser; here it runs on your machine as a server, so a board of any
size works and checklist runs are kept.

## What you need

Docker with Compose, or Podman with `podman compose`, and a folder holding the designs you want to
try. Each subfolder is a design (a KiCad project, an EDIF netlist, a design folder with a
`design.yaml`) or a project of several. Nothing in the folder is written to; it is mounted read-only.

## Start it

Download the release's `compose.yaml`, and `env.example` if you want to change a default, from the
release page on GitHub. The file is pinned to that release.

```bash
curl -LO https://github.com/panyam/agni/releases/latest/download/compose.yaml
AGNI_DESIGNS=./designs docker compose up -d
```

Open <http://localhost:8080>. The landing page lists your folder under `designs`. With Podman, run
`podman compose up -d` instead; on a host with SELinux enforcing, append `:Z` to the designs
volume in `compose.yaml` so the container may read it. `docker compose --profile datasheets up -d`
also starts the datasheet workbench on port 8090, over a `./datasheets` folder.

## Run the guided trial

```bash
docker compose run --rm trial /designs/<board>
```

The trial reads the board, runs every rule, scores the checklists and query sets its project keeps,
traces a signal through a series part and saves the check report, timing each step. It prints the
result as a report and, after it, a link that opens the report as a GitHub issue. It never prints the
board's net names, reference designators, part numbers, its file or folder names, or the names of
rules your project wrote; those are counted, and the report's type has no field that could hold
them.

A board whose `design.yaml` declares an `io_map` also gets its pin map counted. The report says how
many rows the map has, how many each check (pin, net, far end, function) passed, failed or could not
judge, why a declared function went unchecked (no part number, no datasheet entry, no pin-mux table
for the pin, and so on) and how many matches had to be inferred. A pin, a net and a function are all
your board's words, so none of them is kept, only how many.

### Signing off a pin map in a clean room

A team checking its own board's pin map runs three commands where the board lives, and only the last
one's output leaves the room:

```bash
agni check --format html -o pins.html designs/<board>                        # read it yourself
agni check --verdicts --format csv --table pin_map designs/<board> > pins.csv # a row per declared pin
agni trial designs/<board>                                                    # the report you send
```

The first two carry the board's names and stay with the team. Judge each row by eye: is the net the
one the map declares, does the far end land where it should, and is a failed function really one the
part cannot put on that pin. Then send the trial report with what you found in your own words. The
pin map's rows are described in [Design intent](../design-intent/#reading-the-map-one-pin-at-a-time).

## What to look at in the viewer

The trial times the engine. These are the things only a person can judge, in the order a review
goes:

1. **Open the board.** Does the drawing look like your schematic, and does the Properties panel's
   count of parts and nets match what your tools say?
2. **Run checks.** Is each finding something you would want flagged? A finding that is wrong has a
   "looks wrong?" link beside its message, which opens an issue naming only agni's rule.
3. **Click a finding.** Does it light the part or net it is about, on the right sheet?
4. **Run a checklist** in the Review panel, if your project has one, and save the report. Is each
   item's answer the one you would give?
5. **Ask a question** in the Query panel, or open a query set from the Sets menu. Do the answers
   match the board?
6. **Trace** between two pins across a resistor or a ferrite. Is the route the one on the board?

## Send feedback

Open the link the trial printed, read the issue it prefills, and add what you saw in your own words,
without naming nets, parts or files. Submit it when you are content that it says nothing you would
not post publicly. The same goes for a "looks wrong?" link from the viewer.

## Asking an assistant to drive it

An assistant running on your machine (Claude Code, or another agent with a shell) can drive agni
through the same container. These prompts keep everything that names your design on your machine,
and route anything that leaves it through the trial report:

- *"Run `docker compose run --rm -T --entrypoint agni trial check /designs/<board> --format markdown`
  and tell me which rules fired most, and which findings look like false positives and why."*
- *"Run `docker compose run --rm -T --entrypoint agni trial intake /designs/<board>` and summarise the
  design: its interfaces, its power rails, and what agni could not classify."*
- *"Score the project's checklist with `docker compose run --rm -T --entrypoint agni trial review
  /designs/<board>` and list the items that failed, with the finding behind each."*
- *"Run the trial on `/designs/<board>` and, from its report alone, draft the feedback issue. Do not
  add anything the report does not say about the design."*

The last one is the only prompt whose output leaves your machine, and it works from the sanitized
report, never from the board.
