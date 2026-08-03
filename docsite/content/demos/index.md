---
title: "Demos"
description: "Interactive playgrounds that run in your browser."
---

The goal for this section is a set of playgrounds you can drive in the browser, right inside
the docs:

- a **datalog query** playground over a seeded design,
- a **schematic and board viewer** you can pan, zoom, and highlight,
- a **diff** view between two revisions,
- and eventually the full **web app** with an uploaded or seeded corpus.

## How these will run

There is no backend. The docs are hosted as static files (GitHub Pages), so the playgrounds
run entirely in your browser. Agni's engine compiles to WebAssembly, and a small set of
seeded designs is preloaded into an in-browser virtual filesystem. The same service calls the
web app makes over the network resolve against the wasm engine locally instead.

This page is a placeholder while that wiring lands. The site scaffolding already leaves the
seam for it: a front-matter flag on a page selects a playground bundle, built by the same
esbuild toolchain the web app uses.
