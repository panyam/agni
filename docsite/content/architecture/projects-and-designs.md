---
title: "Projects and designs"
description: "Why the engine names a design and a set of designs, and what resolves through those names."
---

Everything else in the engine addresses a **file**. Almost every input to the review layer is
scoped to something larger, and for a long time the engine had no name for it.

Two symptoms show it, and neither takes contriving to reach.

**A design folder holds several files and nothing says which is the design.** Open one and you find
a {{ explainable "netlist" }}, a schematic export, and a board, all openable, all giving different
answers. The design team produces the netlist, and component and connectivity analysis has to read
it; the others are views of that same design ([C21](#the-netlist-is-the-source-the-rest-are-views)).
Nothing on disk said so, so a tool had to guess, without saying it had. One observed board read 565
components off its schematic view against the netlist's 1385, with no error to explain it. The CLI
could only print a warning telling the operator to go consult their own descriptor.

**Per-design config could not be resolved from a design.** Naming conventions, interface profiles,
design intent, seeded parameters, and a review checklist all apply to *some* designs and not others.
With no model of the larger scope, `agni serve` could only take each as a process-wide flag, so a
deployment mounting a mixed set applied one team's config to every design it read.

## The two nouns

```
projects/gateway                      a set of designs that share configuration
├── designs/gateway                   one design: an entry, and companion views of it
└── designs/gateway-rev-b             a later version of the same board, a design of its own
```

A **project** owns what is shared across designs: the team's naming vocabulary, its interface
profiles, its seeded parameters, its review checklists. A **design** owns what is specific to it:
which file is the analysis source, which files are views of it, its design intent, and which project
it belongs to.

On a server that stores designs, two more nouns join them (agni issue 1704). A **pack** holds what a
`project.yaml` holds, as a resource of its own, `packs/{pack}`. A **report** is one check of a stored
design against packs, kept under the design, `designs/{design}/reports/{report}`. A stored project
then lists designs and names its default packs, and a stored design names no project at all. The
sections from "Mounted is read-only" on cover them.

Both descriptors share ONE schema (agni issue 1136), and the file's name says which role it plays.
Only a design names its own files (`entry`, `companions`, `flatten_hierarchy`) and its `series`,
and any of those in a `project.yaml` is a load error. Every other section is open to both, and a design's
own layer goes on top of its project's, so a board kept under an older naming scheme carries its own
`conventions:` and a board with a peculiarity adds a checklist. Before 1136 the two had separate
schemas, and a design could carry only its intent and symbols. That split said nothing about what
the config was. It only decided where a team was allowed to write it.

Both ids are **declared** by an operator, in a `project.yaml` and a `design.yaml`, rather than
derived from a folder path. So the id does not change when the folder is renamed or moved between
mounts, and a review run has a parent to belong to.

```yaml
# project.yaml
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
  house:
    name: Gateway house checklist
    areas: [...]
```

```yaml
# designs/gateway/design.yaml
name: gateway
title: Sample Board
extends: ../../project.yaml
symbols: [symbols]
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - gateway.kicad_pcb
```

A naming vocabulary, checklists and intent are sections of a descriptor. `conventions` binds to the
`config.NamingConvention` proto, and a standalone `--conventions` file uses the same schema.
`checklists` maps a name to a review manifest. The descriptor NAMES the files for interface
profiles, seeded parameters, symbol libraries, derived relations and query sets, under `profiles`,
`params`, `symbols`, `lib` and `queries`. Each is a list of paths relative to the descriptor or
`mount://` URIs, and each path is a file or a directory of them.

**Nothing is found by its folder's name.** Until agni issue 1136 those five were directories
discovered beside `project.yaml` by their conventional names. That had two costs. A flag naming the
project's own directory loaded it twice (`--profile-path` doubled every profile finding, issue 450),
and the only way to run without a tier was to move its directory aside, so a tutorial capture opened
with `mv profiles profiles-off`. A tier is now in a run because a descriptor wrote its path, and
deleting the line takes it out. A path that names nothing is a load error rather than an absent
tier, since the operator who wrote it asked for that tier.

**The first checklist written is the project's default**. `agni review <design>` runs it when no
`--checklist` is given, and the CLI says which it ran. `--checklist` takes a name or a file. A
value ending in `.yaml` or `.yml`, or `-` for stdin, is a manifest read from outside the project, and
any other value names one of the project's checklists, so the tutorial project's second one runs
with `--checklist house`.

Both used to be files beside the descriptor, `conventions.yaml` and `review.yaml`, until agni issue
828 folded them in. The checks that refused those older layouts by name were removed in 1136. A
section holding a file name now fails as any malformed section does, at its line.

## The netlist is the source; the rest are views

`entry` names the file for analysis to read. `companions` name files that are **views** of that same
design: a schematic export to draw it, a board to locate findings on and to check copper against.
This is [C21](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md) written down per design
instead of assumed.

The entry is authoritative for **which components exist and how they are connected**. C21 makes
that claim, on the grounds that the authoring tool's annotation pass assigns ref-des. It is not a
claim about what a part *is*. A netlist does not carry do-not-populate status, an approved
alternate, or the quantity ordered, and where it carries a part number at all it is rarely the
team's maintained copy. Those attributes come from a bill of materials. The entry has already
established the components, and the BOM joins onto them by
{{ explainable "reference-designator" }}. A BOM can therefore enrich a component and can be
reconciled against the netlist so a disagreement is reported, and it never adds or removes one. The
component set is the entry's.

{{ includeFile "figures/design-entry-companions.svg" }}

Companions are declared **file by file**, never inferred from "everything beside the entry". A later
netlist that sits in the same folder is a legitimate analysis source in its own right; an inferred
rule would turn a diff of two netlists into a diff of one against itself.

**A later version of a board is a design of its own, in its own folder** (agni issue 1692). Rev B of
the gateway has its own netlist and its own board, so it lives in `designs/gateway-rev-b` with its
own `design.yaml`. Both descriptors name the same `series`, and the series is how a listing knows
the two are versions of one board:

```yaml
# designs/gateway-rev-b/design.yaml
name: gateway-rev-b
series: gateway
extends: ../../project.yaml
entry: gateway-rev-b.edn
companions: [gateway-rev-b.kicad_pcb]
```

Rev B's board-tier rules then run against rev B's copper, and a board is never borrowed by a netlist
it was not drawn for. Until 1692 a `design.yaml` listed its later versions under `revisions:` in the
same folder. That gave one folder several entries and a resolver rule to pick between them, and a
server that stores designs has no folder to share. A `design.yaml` still carrying `revisions:` fails
to load at that line. A netlist declared nowhere, such as the gateway's rev C, reads with no board,
and a check of it says on stderr which board-tier rules did not run, since "no copper was read" and
"the copper is clean" otherwise look the same.

At the CLI it looks like this.

```
agni check designs/gateway                    # names the DESIGN: the entry's netlist, the companion's copper
agni check designs/gateway/gateway.kicad_pcb  # names a companion: reads the entry, and says so
agni check designs/gateway/gateway.edn        # names the ENTRY: the same design, so the same tiers
agni check designs/gateway-rev-b              # names ANOTHER design: rev B's netlist, rev B's copper
agni check designs/gateway/gateway-rev-c.edn  # names an undeclared sibling: exactly that file, no board
```

Reach for the first form. A netlist carries no copper and a board carries no reliable component
identity, so a design that declares both gets its connectivity rules and its board rules from one
argument. On the tutorial project the board checklist item used to read **not applicable**, because
nobody had told the tool that the board file beside the netlist belonged to the same design. The
first form moves that item to a real verdict.

The third form is the same design as the first. Naming the entry is naming the design, because the
descriptor says so. So a run draws on the companion's sheets whether the caller typed the folder or
the filename inside it. Until agni issue 528 it did not. Take a design whose faithful geometry
lived in a companion. It rendered its auto-layout under one spelling and its real schematic under
the other. The fifth form depends on the file-by-file rule above. That rule protects an undeclared
sibling, and the entry is by definition not that file.

The second form is the old warning turned into behaviour. It fires for any format, where the
heuristic knew only the one stem-matched `.eds`/`.edn` pair. It names the declared file rather than
a guessed one, and it never fires on a file nobody declared. It also always says what it did, on
stderr, because nobody can recover which file was read from a component count.

`--as-named` opts out, and it exists for a real case. To check that a schematic export and the
netlist still describe the same design, you read **both** as netlists and diff them. The tutorial
project's `check-views` target does exactly that.

## Three tiers of configuration

Config reaching a run splits three ways, and the split is worth stating because two of the tiers look
alike from a command line where they are both "flags".

| Tier | What it decides | Where it lives |
|---|---|---|
| **Environment** | where bytes are, what tools exist | `agni.yaml`, and `serve`'s own startup flags |
| **Analysis** | what a design is checked against | a project's and a design's `AnalysisConfig` |
| **Invocation** | this run's question and output | flags: `--rule`, `--format`, `--fail-on`, `--coverage` |

**A knob's tier depends on whether it changes WHAT is checked or only WHERE bytes are
found.** Naming conventions, interface profiles, seeded parameters, design intent and a review
checklist all change the answer, so they are analysis config, scoped to the designs that declared
them. Mounts only locate input, so they are environment config and a plain machine-wide file is safe.

Symbol search paths are the interesting case. They belong to **analysis** config even though they
only locate bytes. How a knob fails decided it, rather than what the knob does. Picture a schematic
that names a library nothing resolves. The read comes back SHORT, because the components it could
not resolve are simply absent. Every rule then evaluates cleanly over the shortened read, and the
run reports fewer findings with no error. Some tiers change the answer by their absence while still
producing something that looks like an answer. Such a tier belongs with the config that changes the
answer.

**The descriptor binds config, and its absence fails exactly that way.** A team folder can
hold `profiles/`, `params/` and `lib/` and still bind none of them, because nothing named them.

<details>
<summary>What that cost on one folder, and why only the server was affected</summary>

This was before agni issue 828, when the vocabulary was a `conventions.yaml` beside the descriptor.
Adding a two-line `project.yaml` to a folder already holding one moved a run from 316 findings to
369, took `rail-not-classified` from 40 to 0 (the built-ins miss some
{{ explainable "rail" "rails" }}, and their own lexicon recognises them), and surfaced 95 further
`test-point-coverage` findings.

The CLI and a `Makefile` passing the flags by hand were unaffected. Only the server ran on the
built-in vocabulary, since it discovers config rather than being handed it. It reported an
`invalid_argument` badge and served an authoritative-looking review anyway. That is the failure
shape this page warns about above, where an answer still looks like an answer.

</details>

The environment/analysis boundary matters for a concrete reason. Put an analysis tier in the
machine-wide file, and it applies to every design a CLI opens. That brings back an old bug, fixed
by per-design config, where one team's vocabulary reached another team's board, correct in
isolation and aimed at the wrong design. `agni.yaml` rejects unknown keys so that reaching for
`conventions:` there is an error rather than a silent global.

The descriptor binds config, and two operational notes follow. A design's `name` is an
**id** (lowercase, digits, `-`, `_`, `.`), not a label; the human-readable string belongs in
`title`, and a name with spaces is rejected. And a rejected descriptor is worth treating as a hard
failure rather than a badge, because every downstream number is quietly computed against a
different configuration.

### Where the analysis tier is carried

One `AnalysisConfig` carries the analysis tier everywhere it appears (on a `Project`, on a `Design`,
and on a request), so adding a tier is one schema change, and every surface gets it. A design's
config is layered over its project's chain (`mergeConfig`). The file tiers accumulate, checklists
layer by name, and the nearer convention or intent replaces the further one. The conventions, the
checklists and the intent travel as values, read once from the descriptors. The file tiers travel
as URIs, and a config resolver reads them. The message's own header in
`protos/agni/v1/webapi/config.proto` carries the rest.

The deployment decides whether a given host can RESOLVE the ref-shaped tiers, and the schema does
not. A host wired with no resolver still composes a value-shaped config with no file access, and
REFUSES one naming a directory rather than dropping the tier. See the C22 amendment in
`CONSTRAINTS.md`.

## Sharing config between projects

A project can declare that it layers on another's config:

```yaml
# project.yaml
name: gateway
extends: ../house/project.yaml
```

`extends` names the descriptor by where it is, a path relative to this one or a `mount://` URI, and
the store answers with that project's resource name (`projects/house`), so nothing above the store
walks a path. `house` is an ordinary project that happens to declare config and no designs. Shared
config is just a project, one extended by no design directly. It is not a second kind of document
with its own format, its own loader and its own id rules.

A design is the last link of the same chain, naming its project with `extends: ../../project.yaml`.
It is never found under a project by where it sits, so a design and its project can live anywhere a
mount reaches, and a project's designs are the descriptors that name it. Suppose a design names no
project while a `project.yaml` sits in a folder above it. That is a load error, because it is the
shape of a forgotten `extends`. Reading on would check the board against the built-in config with
nothing said.

The chain composes **root-first**, so a project overrides what it inherits, matching every other layer
here (a request overrides a project, a project overrides the deployment default). The ref-shaped tiers
ACCUMULATE, so inheriting a profile set and declaring your own runs both, while the naming convention
REPLACES, because two naming vocabularies cannot both be in effect and the nearest declaration wins.
Checklists layer by NAME. When the extending project declares a checklist under an inherited name,
it replaces the inherited one in place. A new name is appended after the inherited ones, so the
inherited default is still first unless the nearer project redeclares it. A team shares its
conventions or its checklists this way. It writes them in a config-only project, and the other
projects name that project with `extends`.

{{ includeFile "figures/config-composition.svg" }}

**Inheritance is declared, never ambient.** Imagine a deployment-wide config that applied unless
overridden. It would bring back the bug fixed by per-design config, and one team's profiles would
reach every board on the server. An `extends` is written in a descriptor, scoped to the descriptor
that wrote it, and reaches no design that did not ask for it. Since agni issue 1136 that holds for
a design's link to its project too. Before that, the link was found by walking up from the design's
folder.

Cycles are an error naming the loop, and the chain is bounded at four levels. Both refuse rather
than resolving partway, as everything else in this layer does. A config that silently stopped
resolving would compose a subset of what the operator declared, and the run would look clean for a
reason nobody could see.

## Resolution is an interface, not a path convention

The service tier holds a port (`service.ProjectStore`), not a directory walk. The shipped
implementation walks a tree for descriptors; another may consult an index, a database, or a PLM
system, and no caller can tell, because none of the port's five methods names a file, a path, or a
parent directory to walk up from.

One rule keeps that layering in place. **Everything that is true only of storing projects in
directories lives behind the port.** Tree walking, the descriptor file names,
design-folder-relative paths, and the upward walk are all facts about one storage shape. They live
in `internal/projects`, and nothing above the port imports it, including the CLI, which reaches
projects through the same `ProjectService` a browser does.

Artifacts are named by a single URI, `mount://<mount>/<path>`, so a design's entry and its companions
are one string each rather than a mount paired with a path (see
[C22](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md)). Resource names are the other
system and are still AIP paths. `projects/gateway` names an identity that does not change when the
folder moves, where a URI names where the bytes are.

The shipped store is built on `fs.FS`. An `fs.FS` has no parent to climb into, so containment is
**structural** rather than checked. An upward resolution walk stops at the tree root, and a ref
carrying `..` never opens a file at all. The CLI uses that property rather than a special case. It
roots its tree a bounded number of levels above the path you named and asks the same service, so the
CLI and the server run one code path and differ only in where the client rooted the tree.

**The CLI's choice of tree root is itself a config decision, and getting it wrong is invisible.**
The CLI mints a mount per argument, rooted at the enclosing project when one resolves and at the
file's own folder otherwise. So the answer to "is there a project above this path" decides the
mount BOUNDARY. More generally, consider a layer that decides what is IN SCOPE. It cannot use
"absent" as its error value. Everything after it reasons about a smaller world and has no way to
know the world was cut down.

<details>
<summary>The bug that taught it</summary>

Picture a descriptor that exists but does not parse. It used to be answered as "no project here".
That rooted the mount at the design's own folder. The broken descriptor then sat outside the tree
entirely, where no later layer could see it. Every downstream check then correctly reported a loose
file with no project, and the run composed against the built-in vocabulary and reported an
authoritative-looking answer at exit 0.

</details>

**A minted mount is real for ONE PROCESS, so a link built from one is refused unless that process
serves it.** The name comes from the enclosing project or is invented as `files`, never `local`,
which is the browser's mount for dropped files. Either way no other `agni` has it, so a second
process resolving `mount://gateway/...` reports that the mount was not declared and says to pass
`--mount`. `linkTarget` tests that distinction before emitting a verdict URL. A link built from a
name known only to this run resolves on nobody's server. For the same reason, `agni open` prints
`--mount` in the check command it hands you. `agni open` may also pass its own minted mount to the
viewer safely, because the process doing the minting is the server.

**Absent and malformed have to be kept distinct at every layer that touches config, and the CLI
now draws the distinction once** (`cliResolveProject`). Absent is ordinary, since most files on a
mounted folder belong to no project and run against the deployment defaults alone. Malformed is
refused, matching how the served surfaces behave and how the other config tiers already fail. An
unknown mount is a third thing and reports nothing, because having nothing to resolve against is
not the same as having something broken.

There is also no separate Go type for a project anywhere in this stack. The descriptors parse
straight into the wire messages, the port passes those, and the service serves them. The whole
content of the resource is the message. A runtime-neutral twin would be a field-for-field copy and
one more place for two layers to disagree about what a design is.

Discovery is bounded. An operator hands the server a mount as a folder, and that folder may contain
a build directory or a home directory. Discovery is cached, but the cache never answers without
checking. A descriptor is a small file an operator edits while the server runs. So before it
returns, every read revalidates the mtime of each directory the walk visited, plus each
descriptor's mtime and size, and an edit shows on the next request (agni issue 176). Suppose an
operator fixes a design's entry and the index still answers with the old one. That is a silent
wrong answer.

## Mounted is read-only, stored is writable

Where a project or design is declared decides whether the server may change it (agni issue 1692).
One declared in a MOUNTED folder is read-only. The folder belongs to whoever mounted it, and the
server only reads it. One the server STORES, in a writable tree it was given, is a full resource, and
a user creates, updates and deletes it over the wire. Picture an engineer with no folder on the
server who uploads a board's schematic and layout. The server has to keep them somewhere, under a
name, across a restart, and a stored design is that place.

A design is named `designs/{design}` and ties to no project (agni issue 1694). A stored project is a
title and a list of design names, so a design can sit in many projects, and deleting a project
deletes none of them. `DesignService` creates, reads, changes and deletes designs, and
`ProjectService` does the same for stored projects. `agni serve --store` keeps both as records, and
`--uploads` keeps a stored design's files.

`agni serve`'s designs are its records alone (agni issue 1736). A folder in a mount makes no design,
whatever `design.yaml` it holds. So a server with mounts and no `--store` lists no designs. Picture
an engineer who mounts the team's boards folder. They pick the boards to keep, and each becomes a
design. `AddDesigns`, and `agni designs add <mount>/<path>`, propose the folder's designs as
`ProposeDesigns` does, one per board. A request names each board by its folder. One folder can hold
two boards, such as a root schematic and a sheet the root never reaches, and then the request also
names the board's entry. The server keeps the ones named, copies their files by hash,
and records each artifact's `source`, the mount and folder its files came from. It writes nothing
into the mount. A design's title, series and intent come from the proposal and the request, never
from a `design.yaml`. `ListDesigns` with `source = "mount://<mount>/<path>"` lists the designs read
from a folder. Reading a folder by its path, as `agni check <folder>` does, still honours its
`design.yaml`. The browser engine and the static demo still list their seeded boards, since they
have no store.

## A stored design holds artifacts

Picture an engineer who fixes one sheet of a KiCad schematic and wants the server's copy to match.
They replace the schematic, every sheet at once, and leave the board alone. So a stored design holds
artifacts, `designs/{design}/artifacts/{artifact}` (agni issue 1696). An artifact is one source of
the design: a netlist, a schematic with its sheets, a board such as a `.kicad_pcb` or an ODB++ job,
or files no reader opens, such as a BOM. It holds one file or several, each by its path relative to
the artifact, since a KiCad root sheet finds its child sheets by filename. `UpdateArtifact` replaces
all of an artifact's files in one write, and `BatchUpdateArtifacts` replaces several artifacts in
one write.

An artifact also has a source, `Artifact.source`: a mount, a path in it and a ref (agni issue 1723).
An uploaded artifact's mount is the uploads mount. A linked artifact's mount is another, such as a
GitHub repo, and the server reads its files from there and keeps them by hash like an upload's. The
two differ only in the mount, so a check, a diff and a report read both the same way. `SyncDesign`
moves a followed link to its source's newest version. "Linking an artifact to a mount" in
[running the server](../../guide/running-the-server/#linking-an-artifact-to-a-mount) walks through it.

The server reads each artifact to find its type and its entry, with the grouping `ProposeDesigns`
uses on a dropped folder. Files that make two designs are refused, since the artifact would hold two
sources. From the types it picks the file analysis reads: the netlist, else the schematic, else the
board (C21). Every other artifact a reader opens is a companion. Two artifacts of the winning type
are refused, and so are two boards, because the server could not tell which one to read. The viewer
and the checks then read one design built from all the artifacts, and a finding names its file as
`{artifact}/{path}`.

Nothing on the wire names a folder. The folder adapter, `internal/projects.DirStore`, keeps a
design's record apart from its files (agni issues 1732, 1739). The record is `<id>.yaml` in the
store, holding each artifact's type, entry and manifest of paths and hashes. The files sit once each,
by hash, in the uploads. A design reads through a view in the uploads mount,
`<id>/<rev>/<artifact>/<path>`, whose files link to those blobs, beside a `design.yaml` the store
writes from the record. A write builds a new revision's view and never changes an old one. A design
with no artifact a reader opens has no view. A database adapter would keep the same records as rows.

A snapshot names a design by `designs/{design}` in `Snapshot.design`, and the server looks the name
up at every read (agni issue 1695). So a snapshot rebuilt after a restart, or ingested again after an
artifact changed, reads the design's files as they are then. `DesignSnapshotID` derives one snapshot
id from the name, so every client opening the design shares one snapshot.

## A pack carries the rules a design is judged by

Picture an engineer who pushes a board's schematic and layout but not the `project.yaml` beside them.
On the laptop, `agni check` reads the project's convention, profiles and params. On the server the
stored design had none of them, so Jetson rev 1.1.3 gave 476 findings stored and 87 on its folder. A
pack closes that gap (agni issue 1704).

A pack is named `packs/{pack}` and holds an `AnalysisConfig`, the shape a `project.yaml` binds to. Its
conventions, checklists and intent are values. Its profile, param, symbol, library and query tiers
are each a `mount://` URI or a path among the pack's own files. `agni packs push` reads a
`project.yaml` and sends the files its paths name. The folder adapter, `DirPacks`, keeps the record as
`<id>.json` and links the files into a view, `@packs/<id>/<rev>/` in the uploads mount, which never
changes once built. A pack's hash covers its config and its files' hashes, so a changed profile moves
it. The startup sweep removes each view no pack reads now, once it is an hour old (agni issue 1746).

`agni packs push` and the packs page send a project folder through `ImportPackFiles`, and the server
reads its `project.yaml` with `projects.PackFromFiles` (agni issue 1697). The server asks for
`project.yaml`'s contents first, then for the files its tiers name, so a folder's designs are never
uploaded. The CLI and the page read a folder one way, and so give one hash.

`agni designs push` reads the `extends` of the folder's `design.yaml` too. When it names a
`project.yaml`, push keeps that project as a pack under the project's name before it pushes the
design, and leaves a pack the server holds with the same hash alone. `--no-pack` skips it.

A snapshot names its packs on `Snapshot.packs`, and each read looks them up again. They layer between
the design's own config and the request's, in the order named, each the way a request layers. A pack's
convention, intent and params replace what came before, and its profiles, symbols and library
accumulate. A pack's profiles register as `<pack>-profiles`, the name a project of that id gives them.
So a pack pushed under its project's name reports the same rule names as a run on the folder.
`DesignSnapshotID` adds the pack names in order, so one design under two pack sets is two snapshots.

A stored design keeps its own intent, as `config.intent` on `CreateDesign` and `UpdateDesign`. `agni
designs push` sends the `intent:` of the folder's `design.yaml`, and `ImportDesignFiles` keeps it from
a folder dropped on the designs page (agni issue 1697). The store writes it into each view's
`design.yaml`. Every other tier of a stored design's config comes from its packs. Scaffolding
intent is still not a server operation. Writing `intent:` from a design is a judgment step with a
confidentiality boundary, so the server never generates one.

A stored project names its default packs on `Project.default_packs`. `agni projects packs lab b200`
sets them. A report that names the project applies them first, as the next section says.

`agni check designs/<d> --pack <k>` and `agni review designs/<d> --pack <k>` read a stored design
under packs from a terminal. Review takes its checklist from the packs, the first one unless
`--checklist` names another. A later pack's checklist replaces an earlier one's of the same name.

## A report keeps a check of a design against packs

Picture an engineer who checks rev 1.1.3 against the team's pack, then replaces its schematic with a
fixed one. The old result should stay, and it should say it ran on the earlier files. A report does
that (agni issue 1704).

`CreateReport` on `designs/{design}` runs a full check of the catalog its packs compose, as `agni
check` with no `--rule` runs, and keeps it as `designs/{design}/reports/{report}`. The packs apply in
order over the design's own intent. The default packs of the project the request names come first,
then the packs the request names, and a pack in both applies once, where the project put it. The
report records the design's content hash and each pack's hash. The design's hash covers its intent and
every artifact's files by path and hash. Every `GetReport` and `ListReports` compares those with the
hashes now and sets `design_changed` and `packs_changed`. A report is never updated, since it is what
the run found. `DeleteDesign` removes a design's reports with it.

A report keeps its whole run, every verdict included, so one on Jetson runs to 15 MB. `ListReports`
therefore returns each report without its run by default, with `finding_count` set, and `GetReport`
returns it whole (`ReportView`, AIP-157, agni issue 1753). The designs page fetches a report when its
tab opens. Each finding there links to `/designs/{design}/view` under the report's packs, naming the
verdict the finding came from (`Finding.verdict_ids`). A finding of a rule that states no verdicts
has no link, since the viewer has nothing to open it on.

A report needs a stored design, since only a stored design has artifacts to hash. The folder adapter, `DirReports`, keeps each report as
`<design>/<report>.json` in the store. A report the server names gets an id that sorts by creation
time, and `ListReports` returns the newest first.

`agni reports create designs/bb-1.1.3 --pack jetson` runs one from a terminal, and `agni reports list
designs/bb-1.1.3` marks each report "run on earlier design files" or "packs/<k> changed since".

That makes `Project` and `Design` the declared-identity case in
[C23](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md). A project has two homes. A mounted
one has `Get` and `List`, and a stored one has every standard method. A design has only the stored
home on `agni serve`. A pack is a full resource, and a report is
one nested under its design.

## Every surface has to HOLD the resolver, and a missing one is silent

The composition is correct and centralised, and that turns out not to be enough. A surface that never
asks gets the defaults, and the defaults are a legitimate answer, so nothing anywhere complains.

`NewQueryService` took a `*ProjectResolver` and never assigned it. Every query therefore ran against
the built-in naming vocabulary, and on a project whose whole point is a different vocabulary that
meant one rail where there are four.

In general, **a collaborator whose absence is meaningful degrades instead of
erroring.** A nil `ProjectStore`, a nil `ConfigResolver`, a nil datasheet provider all mean something
here, so none of them can announce its own absence. The guard is a test at the surface that uses
them. It asserts the OUTCOME expected from a project's config. A test at the layer that composes
the config would not catch the bug, because that layer was correct throughout.

<details>
<summary>The three things that kept it invisible, and the same bug from the other end</summary>

- it **compiles**, because an unused parameter is legal;
- it **passes the service tests**, because those construct the service and assert on what the REQUEST
  carries;
- a nil resolver is a **supported state**, meaning "this deployment resolves no projects", so the
  code took a real, tested path, just the wrong one for a caller that had handed one over.

The same bug hit the CLI's non-service commands from the other end. `readDesign` built its loader
with no read options at all, so six commands read under the built-in vocabulary. It was fixed at
that one choke point.

</details>

## What resolves through a project

A design resolves to a project, and per-design config arrives through that edge.
`ProjectResolver.Overlay` composes the project's and the design's tiers onto the deployment's. So
one project's rules are structurally unable to reach another project's design, rather than merely
discouraged from it. Resolution is kept **visible**, since config that changes which rules run
cannot be inferred from a findings list. A reviewer will ask "is this finding the engine's opinion
or my project's", so the viewer can show a project's design under the plain built-in catalog.

A design that resolves to no project is a normal state, not a failure. It gets the plain viewer and
the built-in catalog, and since a design with no project has no project
config to apply, it cannot be checked against another project's rules. A `design.yaml` in no project
still brings its own config, its `symbols/` library and its `intent:`, which compose under the
design's own name exactly as they would under a project (agni issue 887). Until that fix a
projectless design applied its companions and silently dropped both, so a dropped folder drew its
schematic with every externally-symboled part missing.

**A project's query sets resolve through the same edge and change no run** (agni issue 745).
`queries/` holds one set per `.yaml` file in the shape read by `agni query --set`, travels as
`query_set_uris`, and is parsed by the config resolver into `ResolvedConfig.QuerySets`.
`QueryService.ListQuerySets` lists them, inherited ones first, by resolving that tier alone, so a
broken profile or library cannot hide a project's sets. The viewer's Sets menu and `agni query --set
<name>` both read that list. A set file that does not parse fails the listing and names the file,
since leaving it out would read as the project keeping no such set.

**A project's own library of derived relations resolves through the same edge** (agni issue 773). A
descriptor's `lib` paths travel as `library_uris`, and the config resolver reads them into module
texts on `ResolvedConfig.Library`. `Overlay.Registry` composes them with the shipped vocabulary once
per distinct library, and `QueryService.read` builds each design's fact base over the result, so
every query surface sees the project's members and no other project's design can. A module that does
not compose fails the read rather than answering without it. Otherwise a member could silently
vanish, and a query naming it would read as a design with none of what it asks for. A review
manifest's inline queries compile against the same vocabulary (agni issue 779). `review.Load` checks
only a manifest's structure, because it runs before any project is known, and `CreateReview`
compiles each query against `Overlay.Registry` before the review runs, so a misspelled member is
still refused before anything is checked.
