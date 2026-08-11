---
title: "Projects and designs"
description: "Why the engine names a design and a set of designs, and what resolves through those names."
---

Everything else in the engine addresses a **file**. Almost everything the review layer needs is
scoped to something larger, and for a long time the engine had no name for it.

Two symptoms, both reachable without contriving anything.

**A design folder holds several files and nothing says which is the design.** Open one and you find
a netlist, a schematic export, and a board, all openable, all giving different answers. The netlist
is what the design team produces and what component and connectivity analysis has to read; the
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
title: Gateway ECU review project
```

```yaml
# designs/gateway/design.yaml
name: gateway
title: Gateway ECU
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

Companions are declared **file by file**, never inferred from "everything beside the entry". A later
revision of the netlist sits in the same folder and is a legitimate analysis source in its own
right; an inferred rule would turn a diff of two revisions into a diff of one against itself.

What that buys at the CLI:

```
agni check designs/gateway                    # names the DESIGN: the entry's netlist, the companion's copper
agni check designs/gateway/gateway.kicad_pcb  # names a companion: reads the entry, and says so
agni check designs/gateway/gateway.edn        # names the entry: exactly that, as always
```

The first form is new capability, and it is the one worth reaching for: a netlist carries no copper
and a board carries no reliable component identity, so a design that declares both gets its
connectivity rules and its board rules from one argument. On the tutorial project that moves the
board checklist item from **not applicable** to a real verdict, because nobody had told the tool the
board file beside the netlist was part of the same design.

The second form is the old warning turned into behaviour. It fires for any format rather than only
the one stem-matched `.eds`/`.edn` pair the heuristic knew, it names the file the operator actually
declared rather than one it guessed at, and it never fires on a file nobody declared. It also always
says what it did, on stderr, because which file was read is not recoverable from a component count.

`--as-named` opts out, and it exists for a real case rather than as an escape valve: checking that a
schematic export and the netlist still describe the same design means reading **both** as netlists
and diffing them. The tutorial project's `check-views` target does exactly that.

## Resolution is an interface, not a path convention

The service tier holds a port (`service.ProjectStore`), not a directory walk. The implementation
that ships walks a tree for descriptors; another may consult an index, a database, or a PLM system,
and no caller can tell, because none of the port's five methods names a file, a path, or a parent
directory to walk up from.

That layering is load-bearing enough to be worth stating as a rule: **everything that is true only
of storing projects in directories lives behind the port.** Tree walking, the descriptor file names,
design-folder-relative paths, and the upward walk are all facts about one storage shape. They live
in `internal/projects`, and nothing above the port imports it — including the CLI, which reaches
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

## What resolves through it, and what does not yet

A design resolves to a project, and that edge is where per-design config belongs. Today the config
still arrives as `agni serve` startup flags; moving it onto the resolved project is the next step,
and it is what makes one project's rules structurally unable to reach another project's design
rather than merely discouraged from it. Two properties that move has to keep: resolution must be
**visible**, since config that changes which rules run cannot be inferred from a findings list, and
there must be a way to view a project's design under the plain built-in catalog, because "is this
finding the engine's opinion or my project's" is a question a reviewer will ask.

A design that resolves to no project is a normal state, not a failure. It gets the plain viewer and
the built-in catalog, which is the whole safety property: a design with no project has no project
config to apply, so it cannot be checked against another project's rules.
