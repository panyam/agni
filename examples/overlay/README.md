# overlay — the open-core overlay skeleton

This module is the reference **overlay**: a Go module in its own right that depends on the
public agni engine and adds a private format reader and a private rule suite through the
engine's public extension points, without forking the engine. It is the runnable proof of the
open-core split (see `docs/25-open-core.md`).

Two personas drive the split. **Developers** build the engine and the general readers/rules in
the public repo. **Users** bring what they will not release — proprietary-format readers,
house-style rules, private design data — in an overlay like this one.

## What it shows

- `acmeformat/` — a toy `.acme` netlist reader that registers itself with `formats.Register`
  (WS12-003). One blank import and `.acme` resolves through the engine's Loader and CLI.
- `acmerules/` — two house-style rules registered with `check.RegisterSource` (WS12-004), one of
  each authoring style, namespaced `acme/...` so neither can shadow a built-in:
  - `acmerules.go` — a **Go** rule with an `Eval` closure (`X`-prefixed ref-des = experimental).
  - `acmedatalog.go` — a **datalog** rule declared as a query and turned into a catalog rule by
    `query.RuleFromQuery` (WS3-038). It joins two pin relations with a net-level one, over the
    engine's public relations, with no engine change.
- `main.go` — composes them with blank imports, loads a `.acme` design, and runs the catalog
  (built-ins **plus** the registered rules).

```
$ go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

2 finding(s):
  [warning] acme/experimental-on-power-net: VCC (net VCC carries a production power pin and an experimental (X-prefixed) part)
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

## Authoring a rule in datalog: two things that bite

**A datalog rule needs the fact base imported.** `stdlib/relations` installs the engine's relations
in its `init`, so a composing binary must blank-import it:

```go
_ "github.com/panyam/agni/stdlib/relations"
```

Leave it out and nothing fails. The build succeeds, the run succeeds, and the datalog rule simply
matches nothing and reports clean:

```
$ go run . testdata/example.acme      # with the import removed
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

The Go rule still fires; the datalog one is gone without a word. A quiet pass on a design that may
be violating the rule is the worst failure shape there is, which is why `main.go` spells the import
out and `overlay_test.go` asserts the rule actually produces findings.

**Clause order decides the cost.** The evaluator is a naive backtracking join running literals left to
right, so the first atom decides what gets enumerated before any filter applies. Lead with the atom
that binds your head variable and is most selective. The rule here opens on the handful of
`X`-prefixed parts rather than on every power pin in the design; the reverse spelling reads more
naturally and is the shape that made a shipped profile rule non-terminating on a real board. A toy
fixture will never show you the difference.

**Pin relations need the reader to declare pins.** `pin`, `pin.role`, `pin.type` and `pin.net`
project from PART-TYPE pins, not from net connections. A connection says a pin is wired somewhere; a
pin declaration says the pin exists, what it is called, and what type it is. A format that emits only
connections leaves every pin relation empty, so a pin-level rule finds nothing — the same silent
shape as above. That is why `.acme` has a `pin` line and why the reader synthesizes a `PartType` per
component for its sections to reference.

## How it depends on the engine

`go.mod` requires `github.com/panyam/agni` and, because this overlay lives inside the engine
repo for demonstration, uses `replace github.com/panyam/agni => ../..` — so it builds against
the working tree with no release tag. A real, separately-hosted overlay drops the `replace` and
requires a published engine version instead. Dependencies point **overlay → engine only**; the
engine never imports the overlay (CONSTRAINTS C18).

## Not shown here (follow-ups)

This skeleton drives the engine *library* so the composition is visible in one file. Reusing the
engine's whole CLI (`agni-overlay serve`/`check`/…) needs the engine to export a reusable
command root — a separate change.

The datalog rule here is still a Go *value* compiled into the binary. Loading rule text from a file
at runtime, with no Go build, rides the WS3-004/007 + WS12-002 dynamic-loader path.
