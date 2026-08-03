# Authoring an overlay

This guide walks from zero to a working **overlay**: a private Go module that extends the public
agni engine with your own format reader and rules, without forking it. It is the practical
companion to [25 — Open core](25-open-core.md), which explains why the split exists.

Two artifacts back this guide:

- **[`examples/overlay-template`](../examples/overlay-template/)** — a bare scaffold to copy.
- **[`examples/overlay`](../examples/overlay/)** — a fuller worked example (a `.acme` reader and a
  rule that fires) to read when you want to see a real one.

## Prerequisites

Go 1.26+, and the public engine module `github.com/panyam/agni`. An overlay depends on the
engine; **the engine never depends on the overlay** (CONSTRAINTS C18) — the arrow points one way,
which is what keeps your private code out of the open-source repo.

## 1. Create the module

```
mkdir my-overlay && cd my-overlay
go mod init github.com/yourorg/my-overlay
go get github.com/panyam/agni@latest
```

Your `go.mod` requires a *published* engine version. (The in-repo template uses a
`replace => ../..` so it builds against the working tree; a real overlay deletes that and pins a
release — see the template's `go.mod` TODOs.)

## 2. Register a custom reader — `formats.Register`

A reader turns your file format into the engine's IR. It takes an `io.Reader` and returns an
`*ir.Design`; the engine's `Loader` owns file I/O (CONSTRAINTS C1), so the reader never opens a
file itself. Register one `formats.Format` per extension:

```go
package myfmt

import (
    "os"
    "github.com/panyam/agni/formats"
    ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func init() {
    formats.Register(&formats.Format{
        Ext:  ".myfmt",       // lowercase, with the dot
        Name: "myfmt",        // the file-tree / UI label
        Design: func(_ *formats.Loader, path string) (*ir.Design, error) {
            f, err := os.Open(path)
            if err != nil { return nil, err }
            defer f.Close()
            return Read(f, path) // Read is your io.Reader-pure parser
        },
        // Set Geometry and/or Board too if your format carries a faithful schematic or a board.
    })
}
```

Once registered, the extension resolves through every engine surface — the CLI reader dispatch,
the file-tree label, the supported-extensions list — with no fork. The built-in readers register
the same way, so there is one table and one code path.

## 3. Register private rules — `check.RegisterSource`

A rule is a typed value (`check.Rule`) with an `Eval` over the `check.Model`. Group your rules as a
named `RuleSource` and register it:

```go
package myrules

import "github.com/panyam/agni/check"

func init() {
    check.RegisterSource(check.NewSource("myco", []*check.Rule{noExperimentalRefDes}))
}

var noExperimentalRefDes = &check.Rule{
    Name:     "no-experimental-refdes",
    Severity: "warning",
    Summary:  "house rule: an X-prefixed ref-des is experimental, not for production",
    Reads:    []string{"component.ref_des"},
    Tags:     map[string]string{check.KeyCategory: "house-style"},
    Eval: func(m check.Model) []check.Finding {
        var out []check.Finding
        for _, c := range m.Components() {
            if len(c.RefDes) > 0 && c.RefDes[0] == 'X' {
                out = append(out, check.Finding{Kind: check.KindComponent, Subject: c.RefDes,
                    Message: "experimental part in a production design"})
            }
        }
        return out
    },
}
```

Your rules appear in the catalog namespaced `myco/<rule>`, so they can never shadow a built-in.
`check.DefaultCatalog()` composes the built-ins plus every registered source, so the engine's own
CLI and serve run your rules alongside its own.

A registered rule is a Go rule; it does not join the built-in Spec-twin regression suite (that is
the engine catalog's own concern). Authoring a rule in a DSL instead of Go is future work.

## 4. Compose in main

Blank-import the reader and rule packages so their `init` runs, then use the engine library:

```go
package main

import (
    "github.com/panyam/agni/check"
    "github.com/panyam/agni/formats"
    _ "github.com/yourorg/my-overlay/myfmt"
    _ "github.com/yourorg/my-overlay/myrules"
)

func main() {
    d, _ := (&formats.Loader{}).ReadDesign("design.myfmt")
    findings := check.Run(check.NewModel(d), check.DefaultCatalog().Rules())
    // ... report findings
}
```

## Registration timing: `init` vs explicit `main`

Two styles, both supported:

- **`init` (import side effect)** — like the standard library's image readers. Wire an extension
  in with one blank import. Idiomatic; what the template uses.
- **Explicit from `main`** — drop the `init` and call `formats.Register` / `check.RegisterSource`
  yourself. More visible, no hidden ordering. Prefer this when a binary composes several overlays
  and you want the wiring in one place.

## 5. Verify

```
go build ./... && go test ./...
```

Add a smoke test that your reader loads a fixture and your rule fires (see the template's
`template_test.go` and `examples/overlay/overlay_test.go`).

## Current limitation: the CLI is not yet reusable

This path drives the engine **library**. Reusing the engine's whole CLI (so `my-overlay serve` /
`check` inherit your reader and rules) needs the engine to export a reusable command root, which
it does not yet — `cmd/agni` is `package main`. Until then, compose the library as above, or run
the stock `agni` and register your extensions into a binary you build. Tracked as a follow-up.
