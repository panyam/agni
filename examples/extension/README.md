# extension

This module is the reference **extension**, a Go module in its own right that depends on the
public agni engine and adds a private format reader and a private rule suite through the
engine's public extension points, without forking the engine. It is the runnable proof of the
open-core split (see `docsite/content/build/extending.md`).

Two personas drive the split. **Developers** build the engine and the general readers/rules in
the public repo. **Users** bring what they will not release (proprietary-format readers,
house-style rules, private design data) in an extension like this one.

## What it shows

- `acmeformat/` is a toy `.acme` netlist reader that registers itself with `formats.Register`
  (WS12-003). One blank import and `.acme` resolves through the engine's Loader and CLI.
- `acmerules/` holds two house-style rules registered with `check.RegisterSource` (WS12-004), one of
  each authoring style, namespaced `acme/...` so neither can shadow a built-in:
  - `acmerules.go` is a **Go** rule with an `Eval` closure (`X`-prefixed ref-des = experimental).
  - `acmedatalog.go` is a **datalog** rule declared as a query and turned into a catalog rule by
    `query.RuleFromQuery` (WS3-038). It joins two pin relations with a net-level one, over the
    engine's public relations, with no engine change.
- `main.go` composes them with blank imports and `agni.New`, loads a `.acme` design, and runs the catalog
  (built-ins **plus** the registered rules).

```
$ go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the extension's .acme reader)

4 finding(s):
  [warning] acme/experimental-on-power-net: VCC (net VCC carries a production power pin and an experimental (X-prefixed) part)
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
  [error] power-input-not-driven: GND (net has a power-input pin but no power source)
  [error] power-input-not-driven: VCC (net has a power-input pin but no power source)
```

## Three things that bite when authoring a rule in datalog

**A datalog rule needs the fact base imported.** `stdlib/relations` installs the engine's relations
in its `init`, so a composing binary must blank-import it:

```go
_ "github.com/panyam/agni/stdlib/relations"
```

Leave it out and `agni.New` refuses to compose, returning `MissingRelationsError`, so the run stops
before it reads the design. Before `main.go` composed through `agni.New`, the build and the run both
succeeded without it, and the datalog rule simply matched nothing and reported clean. This capture
is from that earlier behaviour:

```
$ go run . testdata/example.acme      # with the import removed
loaded testdata/example.acme: 4 components, 3 nets (via the extension's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

The Go rule still fired and the datalog one was gone without a word, on a design that violates
the rule. `main.go` spells the import out, `agni.New` now refuses the composition, and
`extension_test.go` asserts the rule actually produces findings.

**Clause order no longer decides the cost.** agni evaluates with the engine's semi-naive evaluator,
which plans each rule body, so a literal runs once its inputs are bound whatever order the body is
written in. Before the planner, the evaluator ran literals left to right, and a shipped profile rule
that opened on every power pin rather than on the few parts it was about never finished on a real
board. The rule here still leads with the handful of `X`-prefixed parts, because that is the order
a reader follows. A toy fixture would not have shown the difference then, and the planner test in
`core/query` is what holds it now.

**Pin relations need the reader to declare pins.** `component.pin`, `pin.role`, `pin.type` and `pin.net`
project from PART-TYPE pins, not from net connections. A connection says a pin is wired somewhere; a
pin declaration says the pin exists, what it is called, and what type it is. A format that emits only
connections leaves every pin relation empty, so a pin-level rule silently finds nothing. That is why `.acme` has a `pin` line and why the reader synthesizes a `PartType` per
component for its sections to reference.

## How it depends on the engine

`go.mod` requires `github.com/panyam/agni` and, because this extension lives inside the engine
repo for demonstration, uses `replace github.com/panyam/agni => ../..`, so it builds against
the working tree with no release tag. A real, separately-hosted extension drops the `replace` and
requires a published engine version instead. Dependencies point **extension → engine only**; the
engine never imports the extension (CONSTRAINTS C18).

## Not shown here (follow-ups)

This skeleton drives the engine *library* so the composition is visible in one file. Reusing the
engine's whole CLI (`agni-extension serve`/`check`/…) needs the engine to export a reusable
command root, which is a separate change.

The datalog rule here is still a Go *value* compiled into the binary. Loading rule text from a file
at runtime, with no Go build, rides the WS3-004/007 + WS12-002 dynamic-loader path.
