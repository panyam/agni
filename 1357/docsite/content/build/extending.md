---
title: "Extending and embedding the engine"
description: "Add your own readers and rules to the engine, and run it inside your own program, without forking it."
---

An extension is a private Go module that builds on the public Agni engine without forking it. There
are two things you can do from one, and this page covers both.

**Extending** adds capability to the engine: your own format reader, your own rules, your own fact
relations, registered through the public registration points. **Embedding** runs the engine inside
your own program, composing it with `agni.New` and serving your own catalog through the service
tier.

This page walks from an empty directory to a working extension that does both.

Both are Go. A program in another language calls agni rather than embedding it, through the CLI or
a running server; [Calling agni from another language](../other-languages/) covers that.

Two example modules back this guide, both in the engine repo under `examples/`:

- `extension-template` is a bare scaffold to copy.
- `extension` is a fuller worked example, with a `.acme` reader and a rule that fires. Read it when
  you want to see a real one.

## Prerequisites

Go 1.26.9 or later and the public engine module `github.com/panyam/agni`. An extension depends on
the engine. The engine never depends on the extension. That one-way arrow keeps your private code
out of the open-source repo.

## Create the module

```
mkdir my-extension && cd my-extension
go mod init github.com/yourorg/my-extension
go get github.com/panyam/agni@latest
```

Your `go.mod` requires a published engine version. The in-repo template uses a
`replace => ../..` so it builds against the working tree. A real extension deletes that and pins a
release, as the template's `go.mod` TODOs describe.

## Register a custom reader with `formats.Register`

A reader turns your file format into the engine's IR. It takes an `io.Reader` and returns an
`*ir.Design`. The engine's `Loader` owns file I/O, so the reader never opens a file itself.
Register one `formats.Format` per extension:

```go
package myfmt

import (
    "github.com/panyam/agni/readers/formats"
    ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func init() {
    formats.Register(&formats.Format{
        Ext:  ".myfmt",       // lowercase, with the dot
        Name: "myfmt",        // the file-tree / UI label
        Design: func(l *formats.Loader, path string) (*ir.Design, error) {
            f, err := l.Open(path) // the Loader owns file I/O
            if err != nil { return nil, err }
            defer f.Close()
            return Read(f, path) // Read is your io.Reader-pure parser
        },
        // Set Geometry and/or Board too if your format carries a faithful schematic or a board.
    })
}
```

Once registered, the extension resolves through every engine surface: the CLI reader dispatch,
the file-tree label, the supported-extensions list. No fork. The built-in readers register the
same way, so there is one table and one code path.

## Register private rules with `check.RegisterSource`

A rule is a typed `check.Rule` value with an `Eval` over the `check.Model`. Group your rules as a
named source and register it:

```go
package myrules

import (
    "context"

    "github.com/panyam/agni/core/check"
)

func init() {
    check.RegisterSource(check.NewSource("myco", []*check.Rule{noExperimentalRefDes}))
}

var noExperimentalRefDes = &check.Rule{
    Name:     "no-experimental-refdes",
    Severity: "warning",
    Summary:  "house rule: an X-prefixed ref-des is experimental, not for production",
    Reads:    []string{"component.ref_des"},
    Tags:     map[string]string{check.KeyCategory: "house-style"},
    Eval: check.FailuresOnly(func(ctx context.Context, m check.Model) []check.Finding {
        var out []check.Finding
        for _, c := range m.Components() {
            if len(c.RefDes) > 0 && c.RefDes[0] == 'X' {
                out = append(out, check.Finding{Kind: check.KindComponent, Subject: c.RefDes,
                    Message: "experimental part in a production design"})
            }
        }
        return out
    }),
}
```

Your rules appear in the catalog namespaced `myco/<rule>`, so they can never shadow a built-in.
`check.DefaultCatalog()` composes the built-ins plus every registered source, so any binary that
imports your package runs your rules alongside the built-ins.

A rule registered this way does not join the built-in Spec-twin regression suite, which is the engine
catalog's own concern.

## Go is one of four ways to write a rule

Go is the shape with the fewest limits, so this page starts with it. It is often not the shape you
want. `check.Rule` is the primitive, and three other shapes compile to it. Your extension
can register any of them, because all four reach the catalog as a `check.RuleSource` and nothing
downstream can tell them apart.

```go
check.RegisterSource(check.NewSource("myco", myGoRules))          // Go
check.RegisterSource(profiles.Source("myco-buses", myProfiles))   // interface profiles
check.RegisterSource(intent.Source("intent", myDeclaration))      // a design-intent declaration
```

Datalog is the fourth. `query.RuleFromQuery` compiles a parsed query into a rule, and it returns an
error rather than a rule that reports nothing, so a query you got wrong fails at composition instead
of reading as a clean board.

Reach for a profile when the thing you are describing is a bus and the knowledge belongs to an
architect rather than a programmer, since adding an interface is then a data value and needs no
rebuild. Reach for an intent declaration when the fact lives outside the design entirely, like a
rail's current budget, which no netlist carries. Reach for Go when the question needs a traversal or
a pairwise join, and the query vocabulary does not express it.

[How a rule gets written](../../architecture/rules-and-checks/#how-a-rule-gets-written) has the axis
in full, including why no shape is allowed to own the primitive.

## Replace built-in rules instead of adding to them

A source normally ADDS to the catalog. A source that implements `check.SupersedingSource` instead
REPLACES the rules it names:

```go
check.NewSupersedingSource("myco", rules,
    check.Facets{Names: []string{"decoupling-missing"}})
```

Each `Facets` selects what to drop, with the same grammar that `Filter` uses for selection. `Names`
replaces individual rules, and `Tags` replaces a family. A source's declaration never applies to its
own rules, so a replacement cannot delete itself.

Interface profiles do this for you. A profile that carries a built-in's name supersedes that
built-in's rules. Say a naming map re-binds `SPI_NOR` to your own net-name suffixes. That map then
replaces the engine's reading of the interface rather than running beside it. [Interface
profiles](../../guide/interface-profiles/) covers the YAML these are written in.

Running both invents failures. Imagine a naming map that re-binds some roles and leaves the others
at the engine's naming. The built-in profile still anchors and still clears its in-use gate. It then
reports each re-bound role as a missing signal, while your profile reads the same board clean. When
you re-bind the anchor role, the built-in profile has nothing to anchor on, and the effect is
invisible. So a convention CLOSER to the engine's produced more spurious failures than one further
from it.

Because supersession works by removing rules, the CLI prints a `note:` to stderr naming what was
dropped and which source dropped it. A rule that was taken away produces no output. Without the
note, a report with removed rules looks exactly like a report where they ran and found nothing.

If you need to drop rules without owning a source, `Catalog.Without(Facets)` is the same exclusion as
a standalone operation.

## Compose in main

An extension reaches the engine through the public registration points that the standard library
uses. [Stack and platform](../../architecture/stack/) draws the shape, and your module is the
fourth source:

{{ includeFile "figures/engine-layers.svg" }}

Blank-import the reader and rule packages so their `init` runs, blank-import the engine's own four,
then compose with `agni.New`:

```go
package main

import (
    "context"
    "log"

    "github.com/panyam/agni"
    "github.com/panyam/agni/core/check"
    "github.com/panyam/agni/readers/formats"

    _ "github.com/yourorg/my-extension/myfmt"
    _ "github.com/yourorg/my-extension/myrules"

    _ "github.com/panyam/agni/stdlib/relations"     // the fact base every datalog rule reads
    _ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a manifest's inline queries
    _ "github.com/panyam/agni/stdlib/rules/builtin" // the shipped EE rule catalog
    _ "github.com/panyam/agni/stdlib/rules/datalog" // the datalog-authored rule suite
)

func main() {
    engine, err := agni.New()
    if err != nil {
        log.Fatal(err)
    }
    for _, w := range engine.Warnings() {
        log.Println("note:", w)
    }
    d, _ := (&formats.Loader{}).ReadDesign("design.myfmt")
    findings, err := check.Run(context.Background(), check.NewModel(d), engine.Catalog().Rules())
    if err != nil {
        log.Fatal(err)
    }
    // ... report findings
}
```

**Compose through `agni.New` rather than reaching for `check.DefaultCatalog` directly.** Those four
blank imports are four independent registration points, and three of them fail SILENTLY when a
binary misses one. The binary gets no built-in rules, or an empty fact base, and every design reports
clean with nothing saying why. `New` refuses both rather than running.

That failure is not hypothetical. The extension example in this repo imported `stdlib/relations` and
never `stdlib/rules/builtin`, so it ran with zero built-in rules and reported only its own two
findings while two real defects on its own fixture went unreported. Nobody noticed until `New`
started refusing it.

A binary may legitimately ship without the datalog rule suite or without an inline-query compiler,
so those absences are `Warnings()` rather than errors. `agni.WithoutDatalogRules()` says the first is
deliberate.

Config reaches `New` as a VALUE, never as a path, because reading files is the caller's business:

```go
ps, _ := profiles.LoadDir("profiles")        // you read it
engine, err := agni.New(
    agni.WithProfiles(ps),
    agni.WithSources(check.NewSource("myco", myRules)),
    agni.WithFSProjectStore(agni.Tree{Mount: "boards", FS: os.DirFS("/srv/boards")}),
)
```

A design's intent is not an engine option, because it belongs to one board. It reaches the catalog,
and the model also reads part of it to learn which connectors are internal (agni issue 831). A
design declares its intent in the `intent:` section of its `design.yaml`, and the project store
reads that section. A caller holding a declaration of its own sends it on the request as
`AnalysisConfig.intent`, and it replaces the design's intent for that run. `intent.LoadFileProto`
reads one from a file.

`WithFSProjectStore` hands you the shipped directory-walking project store. The package that
implements the store does not become public API. A deployment that outgrows the directory shape
implements `service.ProjectStore` and passes `WithProjectStore` instead.

## Registering from init or from main

Two styles both work:

- From `init`, as an import side effect, the way the standard library's image readers register.
  One blank import wires an extension in, and the template does it this way.
- Explicit from `main`. Drop the `init` and call `formats.Register` / `check.RegisterSource`
  yourself. The wiring is then visible and has no hidden ordering, so prefer this when a binary
  composes several extensions and you want the wiring in one place.

## Verify

```
go build ./... && go test ./...
```

Add a smoke test that your reader loads a fixture and your rule fires. The template's
`template_test.go` and `examples/extension/extension_test.go` show the shape.

## Serve your own catalog with the service tier

Registering a reader and a rule suite gets your extensions into a catalog. The `service` package
runs the engine's application layer over that catalog, and it is public for this reason (C13). The
service impls are transport-neutral, so they carry plain protobuf signatures and take every I/O
concern as an injected port:

```go
checkSvc, reviewSvc := engine.RuleServices(agni.RuleServiceDeps{
    Loader:      myLoader,
    ReviewStore: myStore,
    Specs:       mySpecs,
})
resp, err := checkSvc.ListRules(ctx, &webapi.ListRulesRequest{})
```

The two come back TOGETHER and the catalog is not a parameter, so you cannot hand one surface the
composed catalog and the other something else. The shape exists because an extension profile flag
once reached the check surface and the review surface differently. A rule missing from a catalog is
indistinguishable from a rule that ran and found nothing.

Two ports are worth knowing by name. `service.ProjectStore` answers what projects and designs exist
and which design an artifact belongs to, so a deployment backed by a PLM system or an index
implements it instead of walking directories. `service.ConfigResolver` resolves what a project's
analysis config points at, returning a `service.ResolvedConfig` carrying rule sources, a parameter
provider, and symbol paths. `ProjectStore` speaks `artifact.URI`, the `mount://` name for a file,
so the `artifact` package is public too.

Your rules reach the web console through the same path with no extra work. `CheckService.ListRules`
maps whatever catalog it was built over to the wire, and the client resolves its filter bundles
against that response, so there is no static rule table anywhere to also update.

## The CLI is not yet reusable

The service tier above is reusable; the command line over it is not yet. You may want
`my-extension serve` and `my-extension check` to inherit your reader and rules with their flags
intact. That needs the engine to export a reusable command root, and it does not export one yet.
`cmd/agni` is `package main`. Until then, compose the library and the services as above, or run the
stock `agni` and register your extensions into a binary you build.
