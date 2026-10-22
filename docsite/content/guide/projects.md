---
title: "Projects and designs"
description: "Lay out a project.yaml and a design.yaml so every command finds your config without flags."
---

A **project** is the configuration a team shares across its boards. A **design** is one board, with a
statement of which file is which and the project it belongs to. Two small YAML files declare them,
with one schema between them, and once they exist `agni check designs/gateway` picks up your naming
conventions, profiles, parameters and checklists with no flags at all.

This page is the layout. [Projects and designs](../../architecture/projects-and-designs/) in the
architecture section explains why it works this way.

## The layout

The tutorial project, `examples/tutorial-project`, uses every part of it:

```
tutorial-project/
├── project.yaml              name, conventions:, checklists:, and the files below
├── profiles/can.yaml         interface profiles
├── params/                   published part specs, one per MPN
├── lib/house.dl              the project's own derived relations
├── queries/power-review.yaml query sets
└── designs/gateway/
    ├── design.yaml           extends:, entry, companions, revisions, intent:
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
profiles: [profiles]
params: [params]
lib: [lib]
queries: [queries]
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

`extends: ../house/project.yaml` layers this project over another one's config, which is how several
projects share one set of conventions or checklists. The shared one is an ordinary project with no
designs. The architecture page covers how the layers compose.

Every key is checked, and an unknown one is a load error naming its line.

## The files a project names

Five keys name the files a project's runs read, each a list of paths relative to the descriptor or
`mount://` URIs, and each path a file or a directory of them:

- `profiles` names [interface profiles](../interface-profiles/), which supersede a built-in profile
  of the same name.
- `params` names published part specs, layered over a shared corpus part by part (see
  [Datasheets](../datasheets/)).
- `symbols` names symbol libraries for schematics that reference external symbols.
- `lib` names derived relations, one `.dl` module per file, which every query can call (see
  [Querying](../querying/)).
- `queries` names query sets, which `agni query --set <name>` runs and the viewer's Sets menu opens.

Nothing is read for its folder's name, so a folder called `profiles/` that no descriptor names is not
in the run, and to run without a tier you delete its line. A path that names nothing is a load error,
since reading on without it would report as though the team declared no such tier. A flag naming a
directory the project already names adds nothing, and for `profiles` it is refused, since the
profiles would load twice.

## design.yaml

```yaml
name: gateway
title: Sample Board
extends: ../../project.yaml
symbols: [symbols]
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

`extends` names the design's project, and a design takes its project's config only by naming it. A
design that names none while a `project.yaml` sits in a folder above it is a load error, since that is
the shape of a forgotten `extends`. A design that names none in a tree with no project reads on its
own, with whatever its descriptor declares.

`entry` is the file analysis reads, normally the netlist the team produces. `companions` are other
views of the same design, a drawn schematic for sheets and a board for copper. They are drawn and
checked against, never read as a second source of components. Naming the design folder, the entry
or a companion reads the same design with the same tiers, and `--as-named` reads exactly the file
you named instead.

`revisions` declares later revisions, each with its own entry and companions, so each reads with its
own board. [Comparing revisions](../comparing-revisions/#declared-revisions) covers what happens to
one you leave undeclared.

`intent:` declares what the board is supposed to contain, its modules, rails and pin maps.
[Design intent](../design-intent/) covers its forms.

A design can declare any section a project can, and its own layer goes on top of its project's. Most
designs declare only their intent and their own symbol library, but a board kept under an older
naming scheme can carry its own `conventions:`, and a board with a peculiarity can add a checklist.
A design's convention or intent replaces its project's, a checklist of the same name replaces the
project's in place, and the files it names are read beside the project's. Only a design declares
`entry`, `companions`, `revisions` and `flatten_hierarchy`, and any of those in a `project.yaml` is a
load error.

`flatten_hierarchy: true` reads a hierarchical netlist whole, walking into every block its top level
places, for every surface that opens the design: the CLI with no flag, a server, and the browser.
Without it a hierarchical EDIF reads as its top cell, and `--flatten-hierarchy` or the viewer's
**Read every block** asks for the whole design one read at a time. The naming rules for what a walk
reads are in [Ingestion and the IR](../../architecture/ingestion-and-ir/). A release older than the one
that added the key refuses a `design.yaml` carrying it, as it refuses any key it does not know.

Companions and revisions are listed file by file and never inferred from what sits in the folder,
because a later revision beside the entry is a design of its own, and inferring would diff it against
itself.

## Where to go next

- [Running the server](../running-the-server/) serves a project's designs, with every tier above
  applied, in the browser viewer.
- The [tutorials](../../tutorials/) build the tutorial project up one tier at a time.
