---
title: "Projects and designs"
description: "Lay out a project.yaml and a design.yaml so every command finds your config without flags."
---

A **project** is a folder holding the configuration a team shares across its boards. A **design** is
one board inside it, with a statement of which file is which. Two small YAML files declare them, and
once they exist `agni check designs/gateway` picks up your naming conventions, profiles, parameters
and checklists with no flags at all.

This page is the layout. [Projects and designs](../../architecture/projects-and-designs/) in the
architecture section explains why it works this way.

## The layout

The tutorial project, `examples/tutorial-project`, uses every part of it:

```
tutorial-project/
├── project.yaml              name, conventions:, checklists:
├── profiles/can.yaml         interface profiles
├── params/                   published part specs, one per MPN
├── lib/house.dl              the project's own derived relations
├── queries/power-review.yaml query sets
└── designs/gateway/
    ├── design.yaml           entry, companions, revisions, intent:
    ├── gateway.edn           the entry, the netlist analysis reads
    ├── gateway.kicad_sch     companion views of the same design
    ├── gateway.kicad_pcb
    ├── gateway-rev-b.edn     a later revision, with its own board
    ├── gateway-rev-b.kicad_pcb
    └── symbols/              this design's own symbol library
```

`agni start <design-file> [dir]` writes a project like this around a design file you already have,
with a checklist seeded from the shipped catalog. The [CLI reference](../cli-reference/) has its
flags.

## project.yaml

```yaml
name: gateway
title: Sample Board review project
conventions:
  name: gateway
  lexicon:
    net:
      rail:
        patterns: ["_[0-9]+V[0-9]$"]
checklists:
  review:
    name: Sample Board design review
    areas: [...]
```

`name` is the project's identity. The server addresses it as `projects/gateway` whatever its folder
is called, so renaming or moving the folder renames nothing.

`conventions:` is the team's naming vocabulary and house rules, in the same shape a `--conventions`
file takes. [Naming conventions](../naming-conventions/) covers what goes in it.

`checklists:` maps a name to a review checklist. The first one written is the default that
`agni review <design>` runs, and `--checklist <name>` picks another. `agni checklists <design>`
lists them.

`extends: projects/<name>` layers this project over another one's config, which is how several
projects share one set of conventions or checklists. The shared one is an ordinary project with no
designs. The architecture page covers how the layers compose.

Every key is checked. An unknown key, or a `conventions.yaml` or `review.yaml` left beside the
descriptor from an older layout, is a load error that says where the content goes.

## The directories a project discovers

Beside `project.yaml`, agni looks for five directories by their conventional names and composes each
one it finds into every run on the project's designs:

- `profiles/` holds [interface profiles](../interface-profiles/), which supersede a built-in profile
  of the same name.
- `params/` holds published part specs, layered over a shared corpus part by part (see
  [Datasheets](../datasheets/)).
- `symbols/` holds a symbol library for schematics that reference external symbols.
- `lib/` holds derived relations, one `.dl` module per file, which every query can call (see
  [Querying](../querying/)).
- `queries/` holds query sets, which `agni query --set <name>` runs and the viewer's Sets menu opens.

`project.yaml` can name a different directory for any of these, or opt out of one with an empty
value. Because they are discovered, a flag naming the same directory adds nothing, and for
`profiles/` it is refused, since the profiles would load twice. To run without a tier, move its
directory aside.

## design.yaml

```yaml
name: gateway
title: Sample Board
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - gateway.kicad_pcb
revisions:
  - entry: gateway-rev-b.edn
    companions: [gateway-rev-b.kicad_pcb]
intent:
  nets:
    PMIC_CORE_3V3: {nominal: 3.3, domain: io}
```

`entry` is the file analysis reads, normally the netlist the team produces. `companions` are other
views of the same design, a drawn schematic for sheets and a board for copper. They are drawn and
checked against, never read as a second source of components. Naming the design folder, the entry
or a companion reads the same design with the same tiers, and `--as-named` reads exactly the file
you named instead.

`revisions` declares later revisions, each with its own entry and companions, so each reads with its
own board. [Comparing revisions](../comparing-revisions/#declared-revisions) covers what happens to
one you leave undeclared.

`intent:` declares what the board is supposed to contain, its modules, rails and pin maps, and is
per design where everything in `project.yaml` is per team. [Design intent](../design-intent/) covers
its forms. A design can also keep its own `symbols/` beside its descriptor.

Companions and revisions are listed file by file and never inferred from what sits in the folder,
because a later revision beside the entry is a design of its own, and inferring would diff it against
itself.

## Where to go next

- [Running the server](../running-the-server/) serves a project's designs, with every tier above
  applied, in the browser viewer.
- The [tutorials](../../tutorials/) build the tutorial project up one tier at a time.
