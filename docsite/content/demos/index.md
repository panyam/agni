---
title: "Demos"
description: "Interactive playgrounds that run in your browser."
playground: viewer
---

The goal for this section is a set of playgrounds you can drive in the browser, right inside
the docs:

- a **datalog query** playground over a seeded design,
- a **schematic and board viewer** you can pan, zoom, and highlight,
- a **diff** view between two revisions,
- and eventually the full **web app** with an uploaded or seeded corpus.

## Design viewer (prototype)

Drag to pan, scroll to zoom, or use the toolbar. These render a design Agni produced with
`agni render`, baked into the page at build time. The tag and mount contract are the same
ones the live wasm engine will drive later; only the data source changes.

### A board layout

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/demo-board.svg"
             caption="Demo board — front/back copper, zones, vias"></agni-viewer>

### A faithful schematic

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/demo-schematic.svg"
             caption="Faithful schematic render"></agni-viewer>

## How these will run

There is no backend. The docs are hosted as static files (GitHub Pages), so the playgrounds
run entirely in your browser. The viewer above needs no engine at all: it pans and zooms a
pre-rendered SVG. The richer playgrounds (query, diff, live highlight) need Agni's engine
compiled to WebAssembly, with a small set of seeded designs preloaded into an in-browser
virtual filesystem. The same service calls the web app makes over the network resolve against
the wasm engine locally instead.

The site scaffolding leaves the seam for it: a front-matter flag (`playground: viewer`)
selects a component bundle, built by the same esbuild toolchain the web app uses, which
hydrates the inline `<agni-viewer>` tags on the page.
