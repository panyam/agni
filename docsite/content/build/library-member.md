---
title: "Adding a library member"
description: "Derived relations agni ships in Datalog: when a question belongs in the library, how to write a member, and the edits that ship it."
---

Most relations a query calls are projected from the design in Go: `component.net`, `net.rail`,
`pin.type`. A few are better written as a question over those relations, and agni ships those as a
small library of derived relations in Datalog under `stdlib/lib`. A query calls a library member
exactly as it calls any other relation, so `component.probed_both(?r)` reads no differently from
`component.class(?r, ?c)`. Running `agni query --relations` shows which kind each one is.

This page covers when a question belongs in the library, how to write a member and its page, and the
edits that ship it. The members themselves are listed under "derived" in the
[relations catalog](../../reference/relations/), each with its own page.

## Where a question belongs

There are three places a derived question can live, and the library is the middle one.

- **Inline, in the one query that asks it.** A rule ahead of the goal, or a query set's preamble,
  when only one report needs the question. The netlist audit's "MPNs never probed on both nets" keeps
  its `mpn_probed` rule inline for this reason.
- **In the library**, when the question is general hardware practice that many queries ask, such as
  whether a net has a test point. A library member has a path, a typed signature, a doc page, and a
  place in the viewer's picker.
- **As a Go base relation in `stdlib/relations`**, when a Go rule or a `check.Spec` needs the answer,
  since those read base relations and never a derived one (DECISIONS.md, "The Datalog engine lives in
  jaala"). `component.net_count` is a base relation for this reason, though Datalog could count it.

A question built from a profile's values, such as a bus's signal suffixes or a clamp class, cannot
be a library member at all, because a module takes no parameters. That is why `stdlib/profiles`
generates its rules per profile rather than calling the library.

## Writing a member

Each file in `stdlib/lib` is one module, registered at the path its name gives, so rules in
`component.dl` define members of `component`. A member lives beside the base relations of the same
module, and nothing in its path says it is derived.

```
# A net at least one test point sits on, so a probe can land on it during bring-up or in-circuit test.
has_test_point(?n: net) :- component.net(?tp, ?n), component.class(?tp, "test_point");
```

Four conventions hold for every member.

- **The head names the member bare.** `has_test_point` in `net.dl` defines `net.has_test_point`, and
  a qualified head is refused.
- **The head declares every argument's type**, `?n: net` or `?r: component`, so the viewer can make
  a cell clickable and drill-down can show the signature. The engine would infer most of them, and a
  test refuses an inferred one anyway, because a reader looking at the signature should see what the
  author meant.
- **The `#` comment directly above the first rule is the one-line summary** that the catalog, the
  picker and `--relations` show.
- **Names resolve the way a module reads them.** In a clause a bare name is another member of the
  same module, and a dotted name is a full path, so `component.dl` writes `two_terminal(...)` and
  `net.has_test_point(...)`. A member whose name starts with `_` is private to its module and gets no
  path.

## Its page

Every member has a reference page, written in `stdlib/lib/docs/<path>.md` from
`stdlib/lib/docs/_TEMPLATE.md`. The page explains the member: what a row says and the edge cases
its definition includes, why the question matters on a real board, its shape as a join, and two
example queries.

The page never restates the rules. `make catalog-docs` appends a "How it is defined" section with
the typed signature, every clause as the engine registered it, and a link to the module file, so the
page and the definition cannot drift apart. The same text reaches the viewer's detail panel through
`Registry.Doc`.

## The edits

1. The rule in `stdlib/lib/<module>.dl`, with its `#` summary and a typed head.
2. Its page in `stdlib/lib/docs/<path>.md`.
3. `service/testdata/columnkinds.golden`, regenerated with
   `UPDATE_GOLDEN=1 go test ./service/ -run TestColumnKindsMatchGolden`. A new member only adds
   lines, so anything else in the diff means an existing column retyped.
4. `make catalog-docs`, then commit what it wrote, since `catalog-docs-check` reads `git status`.

Three tests in `stdlib/lib/lib_test.go` hold the library to this. `TestEveryMemberIsDocumentedAndTyped`
refuses a member with no summary or an inferred type, `TestEveryMemberHasItsDoc` refuses a member
without a page and a page without a member, and `TestDocQueriesValidate` checks every query a page
shows. A new member also wants a row in `TestLibraryMembersAnswerOnTheProbeDesign`, over a design
built so each case it separates has a part in it, red-checked by breaking the rule it tests.

## Why the modules register together

The fact layer checks the whole vocabulary at every registration, so a broken module panics at load
rather than failing some later query. That makes order matter between modules that read each other.
`component.dl` reads `net.has_test_point` and sorts first, so registered one file at a time it
would be refused before `net.dl` arrived. `stdlib/lib` registers every module in one call,
`facts.RegisterModules`, and its docs after them with `facts.RegisterDocs`.

## A project's own library

A team's question that no other team asks belongs in its project rather than in agni. A `lib/`
directory beside `project.yaml` is laid out the same way as `stdlib/lib`: one module per
`<module.path>.dl` file, and optional pages in `lib/docs/<member.path>.md`. A project member can join
a shipped module (`lib/net.dl` adds to `net`) or start its own (`lib/house.dl` starts `house`), and
it reads the shipped relations and library like any query does.

The tutorial project carries one, `examples/tutorial-project/lib/house.dl`, which defines
`house.pmic_rail` from that team's `PMIC_` naming convention. A query on any design in the project
calls it, and `agni query --relations house --design <design>` lists it. Four rules hold.

- **A project member cannot replace a shipped one.** Defining a path agni already defines fails the
  read and names the file and the path, since a member that quietly meant something else in one
  project would make the same query answer two ways.
- **A module that does not compose fails the read**, naming the library directory and the module.
  Answering without the library would read as a design with none of what the query asks for.
- **Outside the project the members do not exist**, so one team's vocabulary never reaches another
  team's design.
- **A checklist's inline queries read it too** (agni issue 779). A `query:` item in `review.yaml`
  compiles against the vocabulary the review runs with, so the tutorial project's
  `house-review.yaml` calls `house.pmic_rail`. Interface profiles do not, because their queries are generated from the
  profile's own fields and never name a library member.

The tier is discovered like `profiles/`, so a project declares nothing to get it, and `lib: ""` in
`project.yaml` turns it off with the directory in place.

## Sending a library with a request

A library can also travel with a request rather than sit in a project folder, which is the shape for
a script or CI job calling a shared server, a generated client over the CLI, or a viewer trying a
module before committing it (agni issue 788). `AnalysisConfig.library_modules` carries each module
as a value, its path, its text, and a `source` that errors name, with optional pages in
`library_docs`. Modules sent this way compose exactly as a `lib/` directory's do and join the
design's project library when it has one. A member two modules define is refused naming both, and
the same module arriving twice, such as a project's own `lib/` also named by `--lib`, counts once.

On the command line `--lib <dir>` reads a directory laid out as `lib/` and sends it this way:

```
agni query designs/board/board.edn 'house.pmic_rail(?n) => ?n' --lib ../house-lib
```

From Python, the request carries the modules, and both transports send them:

```python
client.run_query(
    uri="mount://designs/board.edn",
    query="house.pmic_rail(?n) => ?n",
    overlay={"config": {"library_modules": [{"path": "house", "text": open("lib/house.dl").read()}]}},
)
```

## Calling the library from a checklist

A review manifest's `query:` item compiles against the vocabulary the review runs with, the shipped
library, the design's project library and any library sent with the request together. The tutorial
project carries a second checklist for its own questions, `house-review.yaml`, whose item P6 does
this:

```yaml
- id: "P6"
  title: every rail the PMIC drives has a test point
  query:
    match: 'house.pmic_rail(?n), not house.pmic_probe_point(?n) => ?n'
    subject: n
    kind: net
    message: 'PMIC rail {n} has no test point'
```

`agni review designs/gateway --checklist house-review.yaml` and a served `CreateReview` agree on it,
failing on `PMIC_MAIN_12V0`, and
`agni review --lib <dir>` sends a library for a design that belongs to no project. A manifest's
queries are checked when a review is created rather than when the file is read, since that is the
first moment the library is known. A misspelled member is still refused before anything runs.

## Reading the library from a shipped rule

No shipped rule reads the library yet, and the first one has a trap waiting. Rules in
`stdlib/rules/datalog` and `stdlib/profiles` are validated against the process-default vocabulary
while the program starts, and nothing orders their package's init after `stdlib/lib`'s. The rule's
package has to import `stdlib/lib` itself, the way `stdlib/lib` imports `stdlib/relations` for the
relations it reads, or it is refused at load in whichever binary happens to initialize it first.
