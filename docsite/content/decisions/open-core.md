---
title: "Open core: engine and overlay"
description: "Why Agni ships a public Apache-2.0 engine and keeps private work in a separate overlay that depends on it."
---

Agni is open-core. `github.com/panyam/agni` is the public engine, licensed Apache-2.0. A
separate, private **overlay** depends on it and adds what a company will not release. This
page records the split: who each side is for, what lives where, why the license is Apache and
not GPL, and the two seams an overlay uses to extend the engine without forking it.

## Two personas

Developers build the engine, which is public: the IR and geometry contracts, the general
format readers and writers, the datasheet extractors, render/web/CLI, and the general EE rule
catalog.

Users bring an overlay, which is private: proprietary-format readers, house-style and private
rules, and extracted datasheet and design data. These are the things a company keeps closed.

The principle behind the split is that the mechanism ships and the private inputs do not. The
datasheet layer already works this way: the extractor is shareable, the extracted data is
private. Open-core generalizes that posture to the whole product.

## What lives where

The same public-versus-private line that decides what ships also decides where code lives.

| Public engine (`github.com/panyam/agni`) | Private overlay (a separate repo) |
|---|---|
| IR / geom / param protos and generated Go | proprietary-format readers |
| general format readers (EDIF, KiCad, IPC-2581, xschem, gEDA) | house-style / private rules |
| the general EE rule catalog | extracted datasheet and design data |
| render, web, CLI, the check engine | the composing binary (`cmd/agni-<house>`) |

The direction is one-way. The overlay depends on the engine, and the engine never imports the
overlay. An engine that imported an overlay would drag private code back into the open repo and
defeat the split, so it is a hard architectural rule that the engine has no overlay dependency.

## Why Apache-2.0 and not GPL

A Go source import is a static link. Under a copyleft license like the GPL, a distributed
overlay that imports the engine would inherit the copyleft obligation and be forced open. That
is exactly the outcome the overlay exists to avoid, since the overlay holds a company's private
readers, rules, and data. Apache-2.0 permits the static link without that obligation, so an
overlay can stay closed while still building on the public engine. The permissive license is
what makes the two-persona model work.

## The two extension seams

An overlay contributes only through two public registries. It never edits the engine.

- **Readers, via `formats.Register`.** Register a `formats.Format` (the extension, a UI label,
  and the `Design`/`Geometry`/`Board` reader funcs). The extension then resolves through the CLI
  reader dispatch, the file-tree label, and the supported-extension list. One table, one code
  path, and the built-ins register the same way.
- **Rules, via `check.RegisterSource`.** Register a named `check.RuleSource` and its rules
  appear in `ListRules` and run in `CheckDesign`, namespaced `<source>/<rule>` so a private rule
  can never shadow a built-in. The engine's `DefaultCatalog()` composes the built-ins plus every
  registered source.

Both are process-global registries an overlay writes into from package `init` (import for side
effect, the way the standard library's image readers register) or explicitly from the composing
binary's `main`. Either way the engine is composed by the overlay and is never coupled to a
specific one.

## How an overlay requires the engine

An overlay is its own Go module:

```
module github.com/acme/agni-overlay
require github.com/panyam/agni v<release>
```

It requires a published engine version. The in-repo reference overlay (`examples/overlay`,
below) instead uses `replace github.com/panyam/agni => ../..` so it builds against the working
tree with no release tag. That replace directive is the only difference from a real overlay.

## Reference: `examples/overlay`

`examples/overlay` is the runnable skeleton. It is a separate module that adds a toy `.acme`
reader (`acmeformat/`, via `formats.Register`) and a house-style rule (`acmerules/`, via
`check.RegisterSource`), composed by two blank imports in `main.go`:

```
$ cd examples/overlay && go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

`overlay_test.go` is the acceptance test. From a separate module, the `.acme` extension
resolves through the engine Loader and the `acme/` rule runs in the composed catalog.

## Authoring your own

The step-by-step guide is [Authoring an overlay](../../build/overlay/): create the module,
register a reader (`formats.Register`) and rules (`check.RegisterSource`), the `init`-versus-
explicit choice, and how to host it outside this repo. The `examples/overlay-template` directory
is the starting scaffold to copy.

## Not here yet

- Reusing the engine's whole CLI (`agni-overlay serve`/`check`/…) needs the engine to export a
  reusable command root. Today the skeleton drives the engine library directly.
- Authoring a rule in a DSL rather than Go, and loading extensions dynamically with no rebuild,
  ride the DSL and dynamic-loader work.
