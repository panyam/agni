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
- `acmerules/` — a house-style rule (`X`-prefixed ref-des = experimental, not for production)
  registered with `check.RegisterSource` (WS12-004). It appears in the catalog namespaced
  `acme/no-experimental-refdes`, so it can never shadow a built-in.
- `main.go` — composes the two with two blank imports, loads a `.acme` design, and runs the
  catalog (built-ins **plus** the registered rule).

```
$ go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

## How it depends on the engine

`go.mod` requires `github.com/panyam/agni` and, because this overlay lives inside the engine
repo for demonstration, uses `replace github.com/panyam/agni => ../..` — so it builds against
the working tree with no release tag. A real, separately-hosted overlay drops the `replace` and
requires a published engine version instead. Dependencies point **overlay → engine only**; the
engine never imports the overlay (CONSTRAINTS C18).

## Not shown here (follow-ups)

This skeleton drives the engine *library* so the composition is visible in one file. Reusing the
engine's whole CLI (`agni-overlay serve`/`check`/…) needs the engine to export a reusable
command root — a separate change. Authoring a rule in a DSL rather than Go rides the WS3-004/007
+ WS12-002 dynamic-loader path.
