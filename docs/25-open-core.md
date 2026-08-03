# 25 — Open core: engine + overlay

Agni is open-core. `github.com/panyam/agni` is the public engine, licensed Apache-2.0. A
separate, private **overlay** depends on it and adds what a user will not release. This note is
the split: who each side is for, what lives where, and the two seams an overlay uses to extend the
engine without forking it.

## Two personas

- **Developers build the engine** (public): the IR and geometry contracts, the general format
  readers and writers, the datasheet extractors, render/web/CLI, and the general EE rule catalog.
- **Users bring an overlay** (private): proprietary-format readers, house-style and private rules,
  and extracted datasheet/design data. These are the things a company keeps closed.

This generalizes the datasheet posture (CONSTRAINTS C16 — the *extractor* is shareable, the
*extracted data* is private) to the whole product: the mechanism ships, the private inputs do not.

## What lives where

The research-vs-decisions doc-placement rule, extended to code:

| Public engine (`github.com/panyam/agni`) | Private overlay (a separate repo) |
|---|---|
| IR / geom / param protos and generated Go | proprietary-format readers |
| general format readers (EDIF, KiCad, IPC-2581, xschem, gEDA) | house-style / private rules |
| the general EE rule catalog | extracted datasheet + design data (C16) |
| render, web, CLI, the check engine | the composing binary (`cmd/agni-<house>`) |

The direction is one-way: the overlay depends on the engine; **the engine never imports the
overlay** (CONSTRAINTS C18). An engine that imported an overlay would drag private code back into
the open repo, defeating the split.

## The two extension seams

An overlay contributes only through two public registries — it never edits the engine.

- **Readers — `formats.Register`**: register a `formats.Format` (extension, UI label,
  and the `Design`/`Geometry`/`Board` reader funcs) and the extension resolves through the CLI
  reader dispatch, the file-tree label, and the supported-extension list. One table, one code path;
  the built-ins register the same way.
- **Rules — `check.RegisterSource`**: register a named `check.RuleSource` and its rules
  appear in `ListRules` and run in `CheckDesign`, namespaced `<source>/<rule>` so a private rule can
  never shadow a built-in. The engine's `DefaultCatalog()` composes the built-ins plus every
  registered source.

Both are process-global registries an overlay writes into from package `init` (import-for-side-
effect, like the standard library's image readers) or explicitly from the composing binary's
`main`. Either way the engine is composed *by* the overlay; it is never coupled to a specific one.

## How an overlay requires the engine

An overlay is its own Go module:

```
module github.com/acme/agni-overlay
require github.com/panyam/agni v<release>
```

It requires a published engine version. The in-repo reference overlay (`examples/overlay/`, below)
instead uses `replace github.com/panyam/agni => ../..` so it builds against the working tree with
no release tag — the only difference from a real overlay.

## Reference: `examples/overlay`

`examples/overlay/` is the runnable skeleton — a separate module that adds a toy `.acme` reader
(`acmeformat/`, via `formats.Register`) and a house-style rule (`acmerules/`, via
`check.RegisterSource`), composed by two blank imports in `main.go`:

```
$ cd examples/overlay && go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

`overlay_test.go` is the acceptance: from a separate module, the `.acme` extension resolves through
the engine Loader and the `acme/` rule runs in the composed catalog.

## Authoring your own

The step-by-step guide is [OVERLAY_AUTHORING.md](OVERLAY_AUTHORING.md): create the module, register
a reader (`formats.Register`) and rules (`check.RegisterSource`), the `init`-vs-explicit choice, and
how to host it outside this repo. Copy [`examples/overlay-template`](../examples/overlay-template/)
as the starting scaffold.

## Not here yet

- **Reusing the engine's whole CLI** (`agni-overlay serve`/`check`/…) needs the engine to export a
  reusable command root; today the skeleton drives the engine library directly. Tracked as a
  follow-up.
- **Authoring a rule in a DSL** rather than Go, and **loading extensions dynamically** (no rebuild),
  ride the DSL and dynamic-loader work.
