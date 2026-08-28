---
title: "Projects and designs"
description: "Why the engine names a design and a set of designs, and what resolves through those names."
---

Everything else in the engine addresses a **file**. Almost everything the review layer needs is
scoped to something larger, and for a long time the engine had no name for it.

Two symptoms, both reachable without contriving anything.

**A design folder holds several files and nothing says which is the design.** Open one and you find
a {{ explainable "netlist" }}, a schematic export, and a board, all openable, all giving different
answers. The netlist is what the design team produces and what component and connectivity analysis
has to read; the
others are views of that same design ([C21](#the-netlist-is-the-source-the-rest-are-views)). Nothing
on disk said so, so a tool had to guess, and the guess was invisible: one observed board read 565
components off its schematic view against the netlist's 1385, with no error to explain it. The CLI
could only print a warning telling the operator to go consult their own descriptor.

**Per-design config could not be resolved from a design.** Naming conventions, interface profiles,
design intent, seeded parameters, and a review checklist all apply to *some* designs and not others.
With no model of the larger scope, `agni serve` could only take each as a process-wide flag, so a
deployment mounting a mixed set applied one team's config to every design it read.

## The two nouns

```
projects/gateway                      a set of designs that share configuration
└── designs/gateway                   one design: an entry, and companion views of it
```

A **project** owns what is shared across designs: the team's naming vocabulary, its interface
profiles, its seeded parameters, its review checklist. A **design** owns what is specific to it:
which file is the analysis source, which files are views of it, and its design intent.

Both ids are **declared** by an operator, in a `project.yaml` and a `design.yaml`, rather than
derived from a folder path. That is what makes a resource name worth having: the id survives the
folder being renamed or moved between mounts, and it gives a review run a parent to hang off.

```yaml
# project.yaml
name: gateway
title: Sample Board review project
```

```yaml
# designs/gateway/design.yaml
name: gateway
title: Sample Board
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - gateway.kicad_pcb
```

## The netlist is the source; the rest are views

`entry` names the file analysis reads. `companions` name files that are **views** of that same
design: a schematic export to draw it, a board to locate findings on and to check copper against.
This is [C21](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md) written down per design
instead of assumed.

What the entry is authoritative for is **which components exist and how they are connected**. That
is the claim C21 makes and the one the annotation argument supports. It is not a claim about what a
part *is*. A netlist does not carry do-not-populate status, an approved alternate, or the quantity
ordered, and where it carries a part number at all it is rarely the copy the team maintains. Those
attributes come from a bill of materials, joined onto components the entry already established by
{{ explainable "reference-designator" }}. A BOM can therefore enrich a component and can be
reconciled against the netlist so a disagreement is reported, and it never adds or removes one. The
component set is the entry's.

{{ includeFile "figures/design-entry-companions.svg" }}

Companions are declared **file by file**, never inferred from "everything beside the entry". A later
revision of the netlist sits in the same folder and is a legitimate analysis source in its own
right; an inferred rule would turn a diff of two revisions into a diff of one against itself.

What that buys at the CLI:

```
agni check designs/gateway                    # names the DESIGN: the entry's netlist, the companion's copper
agni check designs/gateway/gateway.kicad_pcb  # names a companion: reads the entry, and says so
agni check designs/gateway/gateway.edn        # names the ENTRY: the same design, so the same tiers
agni check designs/gateway/gateway-rev-b.edn  # names an undeclared sibling: exactly that file
```

The first form is new capability, and it is the one worth reaching for: a netlist carries no copper
and a board carries no reliable component identity, so a design that declares both gets its
connectivity rules and its board rules from one argument. On the tutorial project that moves the
board checklist item from **not applicable** to a real verdict, because nobody had told the tool the
board file beside the netlist was part of the same design.

The third form is the same design as the first. Naming the entry is naming the design, because the
descriptor is what says so, and a run therefore draws on the companion's sheets whether the caller
typed the folder or the filename inside it. Until agni issue 528 it did not, so a design whose
faithful geometry lived in a companion rendered its auto-layout under one spelling and its real
schematic under the other. The file-by-file rule above is what the fourth form protects, and the
entry is by definition not the file it protects.

The second form is the old warning turned into behaviour. It fires for any format rather than only
the one stem-matched `.eds`/`.edn` pair the heuristic knew, it names the file the operator actually
declared rather than one it guessed at, and it never fires on a file nobody declared. It also always
says what it did, on stderr, because which file was read is not recoverable from a component count.

`--as-named` opts out, and it exists for a real case rather than as an escape valve: checking that a
schematic export and the netlist still describe the same design means reading **both** as netlists
and diffing them. The tutorial project's `check-views` target does exactly that.

## Three tiers of configuration

Config reaching a run splits three ways, and the split is worth stating because two of the tiers look
alike from a command line where they are both "flags".

| Tier | What it decides | Where it lives |
|---|---|---|
| **Environment** | where bytes are, what tools exist | `agni.yaml`, and `serve`'s own startup flags |
| **Analysis** | what a design is checked against | a project's and a design's `AnalysisConfig` |
| **Invocation** | this run's question and output | flags: `--rule`, `--format`, `--fail-on`, `--coverage` |

**The test that assigns a knob to a tier: does it change WHAT is checked, or only WHERE bytes are
found?** Naming conventions, interface profiles, seeded parameters, design intent and a review
checklist all change the answer, so they are analysis config, scoped to the designs that declared
them. Mounts only locate input, so they are environment config and a plain machine-wide file is safe.

Symbol search paths are the interesting case, and they landed in **analysis** config despite only
locating bytes. The deciding question is not what a knob does but how it fails: a schematic naming a
library nothing resolves reads SHORT, the components it could not resolve are simply absent, every
rule then evaluates cleanly over the shortened read, and the run reports fewer findings with no error.
A tier whose absence changes the answer while still looking like an answer belongs with the config
that changes the answer.

**The descriptor is what binds config, and its absence fails exactly that way.** A team folder can
hold `conventions.yaml`, `profiles/`, `params/` and a checklist at every conventional name and still
bind none of it, because nothing declared a project.

<details>
<summary>What that cost on one folder, and why only the server was affected</summary>

Adding a two-line `project.yaml` moved a run from 316 findings to 369, took `rail-not-classified`
from 40 to 0 (their own lexicon recognises {{ explainable "rail" "rails" }} the built-ins miss), and
surfaced 95 further `test-point-coverage` findings.

The CLI and a `Makefile` passing the flags by hand were unaffected. Only the server, which discovers
config rather than being handed it, ran on the built-in vocabulary. It reported an `invalid_argument`
badge and served an authoritative-looking review anyway, failing in the shape this page warns about
above, where an answer still looks like an answer.

</details>

Two operational notes follow from that. A design's `name` is an **id** (lowercase, digits, `-`, `_`, `.`), not a label; the human-readable string belongs in `title`, and a name with spaces is rejected. And a rejected descriptor is worth treating as a hard failure rather than a badge, because every downstream number is quietly computed against a different configuration.

That boundary is load-bearing rather than tidy. An analysis tier in the machine-wide file would apply
to every design a CLI opened, reproducing exactly the bug per-design config fixed: one team's vocabulary
reaching another team's board, correct in isolation and aimed at the wrong design. `agni.yaml`
rejects unknown keys so that reaching for `conventions:` there is an error rather than a silent
global.

One `AnalysisConfig` carries the analysis tier everywhere it appears (on a `Project`, on a `Design`,
and on a request), so adding a tier is one schema change every surface gets. Which fields each
populates is what keeps the scopes apart: a `Design` sets only its intent, because a board has its own
architecture where conventions and profiles describe the team. The message's own header in
`protos/agni/v1/webapi/config.proto` carries the rest.

Whether a given host can RESOLVE the ref-shaped tiers is a property of the deployment, not of the
schema. A host wired with no resolver still composes a value-shaped config with no file access, and
REFUSES one naming a directory rather than dropping the tier. See the C22 amendment in
`CONSTRAINTS.md`.

## Sharing config between projects

A project can declare that it layers on another's config:

```yaml
# project.yaml
name: gateway
extends: projects/house
```

`projects/house` is an ordinary project that happens to declare config and no designs. That reuse is
the point: shared config is not a second kind of document with its own format, its own loader and its
own id rules, it is a project with nothing under `designs/`.

The chain composes **root-first**, so a project overrides what it inherits, matching every other layer
here (a request overrides a project, a project overrides the deployment default). The ref-shaped tiers
ACCUMULATE, so inheriting a profile set and declaring your own runs both, while the naming convention
REPLACES, because two naming vocabularies cannot both be in effect and the nearest declaration wins.

{{ includeFile "figures/config-composition.svg" }}

**Inheritance is declared, never ambient, and that is the whole safety property.** A deployment-wide
config that applied unless overridden is precisely the bug per-design config fixed: one team's
profiles reaching every board on the server. An `extends` is written in a descriptor, scoped to the
project that wrote it, and reaches no design that did not ask for it.

Cycles are an error naming the loop, and the chain is bounded at four levels. Both refuse rather than
resolving partway, for the reason everything else in this layer refuses: a config that silently
stopped resolving would compose a subset of what the operator declared, and the run would look clean
for a reason nobody could see.

## Resolution is an interface, not a path convention

The service tier holds a port (`service.ProjectStore`), not a directory walk. The implementation
that ships walks a tree for descriptors; another may consult an index, a database, or a PLM system,
and no caller can tell, because none of the port's five methods names a file, a path, or a parent
directory to walk up from.

That layering is load-bearing enough to be worth stating as a rule: **everything that is true only
of storing projects in directories lives behind the port.** Tree walking, the descriptor file names,
design-folder-relative paths, and the upward walk are all facts about one storage shape. They live
in `internal/projects`, and nothing above the port imports it, including the CLI, which reaches
projects through the same `ProjectService` a browser does.

Artifacts are named by a single URI, `mount://<mount>/<path>`, so a design's entry and its companions
are one string each rather than a mount paired with a path (see
[C22](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md)). Resource names are the other system
and stay AIP paths: `projects/gateway` names an identity that survives the folder moving, where a URI
names where the bytes are.

The shipped store is built on `fs.FS`, which makes containment **structural** rather than checked:
an `fs.FS` has no parent to climb into, so an upward resolution walk stops at the tree root and a ref
carrying `..` never opens a file at all. The CLI uses that property rather than a special case. It
roots its tree a bounded number of levels above the path you named and asks the same service, so the
CLI and the server run one code path and differ only in where the client rooted the tree.

**Where the CLI roots that tree is itself a config decision, and getting it wrong is invisible.** The
CLI mints a mount per argument, rooted at the enclosing project when one resolves and at the file's
own folder otherwise. So the answer to "is there a project above this path" decides the mount
BOUNDARY. The lesson generalises past the bug that taught it: a layer that decides what is IN SCOPE
cannot use "absent" as its error value, because everything after it is reasoning about a smaller
world and has no way to know the world was cut down.

<details>
<summary>The bug that taught it</summary>

A descriptor that exists but does not parse used to be answered as "no project here". That rooted
the mount at the design's own folder, which put the broken descriptor outside the tree entirely,
where no later layer could see it. Every downstream check then honestly reported a loose file with
no project, and the run composed against the built-in vocabulary and reported an
authoritative-looking answer at exit 0.

</details>

**A minted mount is real for ONE PROCESS, which is why a link built from one is refused.** The name
comes from the enclosing project or is invented as `local`, and either way no other `agni` has it: a
second process resolving `mount://gateway/...` reports that the mount was not declared and says to pass
`--mount`. That is the distinction `linkablePath` tests before emitting a verdict URL, since a link
built from a name only this run knows resolves on nobody's server. It is also why `agni open` prints
`--mount` in the check command it hands you, and why `agni open` may pass its own minted mount to the
viewer safely: the process doing the minting is the server.

**Absent and malformed have to stay distinct at every layer that touches config, and the CLI now
draws it once** (`cliResolveProject`). Absent is ordinary, since most files on a mounted folder belong
to no project and run against the fallback. Malformed is refused, matching how the served surfaces
behave and how the other config tiers already fail. An unknown mount is a third thing and stays
quiet: nothing to resolve against is not the same as something broken.

There is also no separate Go type for a project anywhere in this stack. The descriptors parse
straight into the wire messages, the port passes those, and the service serves them. A
runtime-neutral twin of a resource whose whole content is the message would be a field-for-field
copy and one more place for two layers to disagree about what a design is.

Discovery is bounded and uncached, both deliberately. Bounded, because a mount is a folder an
operator handed the server and may contain a build directory or a home directory. Uncached, because
a descriptor is a small file an operator edits while the server runs, and an index that answered
with a design's old entry after they fixed it would be exactly the silent-wrong-answer this whole
feature exists to remove.

## Read-only, on purpose

There is no `CreateProject`, and its absence is not a gap waiting to be filled. Scaffolding a
project means authoring design intent from a design, which is a judgment step with a confidentiality
boundary rather than a server operation. Read-side resolution is what unblocks everything else;
mutating a project's files raises ownership, concurrency, and provenance questions that nothing here
needs answered.

That makes `Project` and `Design` the read-only-resource case in
[C23](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md): AIP-shaped, with `Get` and `List`
and no mutators.

## Every surface has to HOLD the resolver, and a missing one is silent

The composition is correct and centralised, which turns out not to be enough. A surface that never
asks gets the defaults, and the defaults are a legitimate answer, so nothing anywhere complains.

`NewQueryService` took a `*ProjectResolver` and never assigned it. Every query therefore ran against
the built-in naming vocabulary, and on a project whose whole point is a different vocabulary that
meant one rail where there are four.

The general shape is this: **a collaborator whose absence is meaningful degrades instead of
erroring.** A nil `ProjectStore`, a nil `ConfigResolver`, a nil datasheet provider all mean something
here, so none of them can announce its own absence. The guard is a test at the surface that
uses them, asserting the OUTCOME a project's config is supposed to produce, rather than one at the
layer that composes it, which was correct throughout.

<details>
<summary>The three things that kept it invisible, and the same bug from the other end</summary>

- it **compiles**, because an unused parameter is legal;
- it **passes the service tests**, because those construct the service and assert on what the REQUEST
  carries;
- a nil resolver is a **supported state**, meaning "this deployment resolves no projects", so the
  code took a real, tested path, just the wrong one for a caller that had handed one over.

The same shape bit the CLI's non-service commands from the other end: `readDesign` built its loader
with no read options at all, so six commands read under the built-in vocabulary. One choke point, one
fix, and the same class of silence.

</details>

## What resolves through it, and what does not yet

A design resolves to a project, and that edge is where per-design config belongs. Today the config
still arrives as `agni serve` startup flags; moving it onto the resolved project is the next step,
and it is what makes one project's rules structurally unable to reach another project's design
rather than merely discouraged from it. Two properties that move has to keep: resolution must be
**visible**, since config that changes which rules run cannot be inferred from a findings list, and
there must be a way to view a project's design under the plain built-in catalog, because "is this
finding the engine's opinion or my project's" is a question a reviewer will ask.

A design that resolves to no project is a normal state, not a failure. It gets the plain viewer and
the built-in catalog. That carries the whole safety property: a design with no project has no project
config to apply, so it cannot be checked against another project's rules.
