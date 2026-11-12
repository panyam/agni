---
title: "Open core, engine and overlay"
description: "Why Agni ships a public Apache-2.0 engine and keeps private work in a separate overlay that depends on it."
---

Agni is open-core. `github.com/panyam/agni` is the public engine, licensed Apache-2.0. A
separate, private **overlay** depends on it and adds what a company will not release. This
page records the split: who each side is for, what lives where, why the license is Apache and
not GPL, and the two registries an overlay uses to extend the engine without forking it.

## Two personas

Developers build the public engine. It holds the IR and geometry contracts, the general format
readers and writers, the datasheet extractors, render/web/CLI, and the general EE rule catalog.

Users bring a private overlay. It holds proprietary-format readers, house-style and private
rules, and extracted datasheet and design data. A company keeps these things closed.

The split follows one principle. The mechanism ships and the private inputs do not. The
datasheet layer already works this way, since its extractor is shareable and the extracted data
is private. Open-core generalizes that posture to the whole product.

## What lives where

One line between public and private decides what ships, and it also decides where code lives.

| Public engine (`github.com/panyam/agni`) | Private overlay (a separate repo) |
|---|---|
| IR / geom / param protos and generated Go | proprietary-format readers |
| general format readers (EDIF, KiCad, IPC-2581, xschem, gEDA) | house-style / private rules |
| the general EE rule catalog | extracted datasheet and design data |
| render, web, CLI, the check engine | the composing binary (`cmd/agni-<house>`) |

The direction is one-way. The overlay depends on the engine, and the engine never imports the
overlay. If the engine imported an overlay, it would drag private code back into the open repo
and defeat the split. So the engine has no overlay dependency, and that is a hard architectural
rule.

## Why Apache-2.0 and not GPL

A Go source import is a static link. Imagine the engine under a copyleft license like the GPL,
and a company distributing an overlay that imports it. The overlay would inherit the copyleft
obligation, and the company would have to open its source. The overlay exists to avoid exactly
that, because it holds a company's private readers, rules, and data. Apache-2.0 permits the
static link without that obligation, so an overlay can build on the public engine and remain
closed. The two-persona model depends on that permissive license.

## The two extension registries

An overlay contributes only through two public registries. It never edits the engine.

- **Readers** register through `formats.Register`, which takes a `formats.Format` (the extension, a
  UI label, and the `Design`/`Geometry`/`Board` reader funcs). The extension then resolves through the CLI
  reader dispatch, the file-tree label, and the supported-extension list. One table, one code
  path, and the built-ins register the same way.
- **Rules** register through `check.RegisterSource`, which takes a named `check.RuleSource`. The
  source's rules appear in `ListRules` and run in `CheckDesign`. Each is namespaced
  `<source>/<rule>`, so a private rule can never shadow a built-in. The engine's
  `DefaultCatalog()` composes the built-ins plus every registered source.

Both are process-global registries. An overlay writes into them from package `init`, importing
for side effect the way the standard library's image readers register. Or it writes into them
explicitly from the composing binary's `main`. Either way the overlay composes the engine, and
the engine is never coupled to a specific overlay.

## How an overlay requires the engine

An overlay is its own Go module:

```
module github.com/acme/agni-overlay
require github.com/panyam/agni v<release>
```

It requires a published engine version. The in-repo reference overlay is `examples/overlay`,
described below. It uses `replace github.com/panyam/agni => ../..` instead, so it builds against
the working tree with no release tag. That replace directive is the only difference from a real
overlay.

## Reference: `examples/overlay`

`examples/overlay` is the runnable skeleton. It is a separate module. It adds a toy `.acme`
reader (`acmeformat/`, via `formats.Register`) and a house-style rule (`acmerules/`, via
`check.RegisterSource`), and two blank imports in `main.go` compose them.

```
$ cd examples/overlay && go run . testdata/example.acme
loaded testdata/example.acme: 4 components, 3 nets (via the overlay's .acme reader)

1 finding(s):
  [warning] acme/no-experimental-refdes: X1 (experimental (X-prefixed) part in a production design)
```

`overlay_test.go` is the acceptance test. From a separate module, the `.acme` extension
resolves through the engine Loader and the `acme/` rule runs in the composed catalog.

## Authoring your own

[Authoring an overlay](../../build/overlay/) is the step-by-step guide. It covers creating the
module, registering a reader (`formats.Register`) and rules (`check.RegisterSource`), choosing
between `init` and explicit registration, and hosting the overlay outside this repo. Copy the
`examples/overlay-template` directory as the starting scaffold.

## Not here yet

- Reusing the engine's whole CLI (`agni-overlay serve`/`check`/…) needs the engine to export a
  reusable command root. Today the skeleton drives the engine library directly.
- Authoring a rule in a DSL rather than Go waits on the DSL work. Loading extensions
  dynamically with no rebuild waits on the dynamic-loader work.
