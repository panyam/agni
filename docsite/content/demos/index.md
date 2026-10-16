---
title: "About the demo"
description: "What the live demo is, why it runs in your browser, and how it relates to agni serve."
playground: viewer
---

[The demo]({{.Site.PathPrefix}}/demo/) opens a design in the agni viewer with the engine running in
your browser. You install nothing, and nothing you open is uploaded.

## What it holds

The landing page lists a few seed designs. One is the gateway board the
[tutorials](../tutorials/) use, and the others are open-source KiCad boards from the samples corpus,
each shipped under its own licence. Open one and the viewer draws its sheets and its board, runs the
rule catalog, answers queries and traces a signal, the same as it would against a server. Running a
checklist needs a server for now, because the browser engine keeps no review store
([#931](https://github.com/panyam/agni/issues/931)).

You can also drop a folder or a `.zip` of your own design files on the page, or pick them with its
"Open files" button. The page proposes which designs the files make, shows each as the `design.yaml`
that would declare it, and lets you edit that before opening one. Dropped files stay in the tab and
are never sent anywhere.

The browser engine has a size limit, 128 MB of design files by default. A page served by
`agni serve --engine wasm` sends a design over `--wasm-max-bytes` to the server and says so in its
top bar. The demo has no server behind it, so it is for designs under that size.

## Why it runs in the browser

The project does not host a server for its demos. The docs are static files on GitHub Pages, so the
demo is static files too. `agni site` (`make demo-site`) writes the viewer, the engine compiled to
WebAssembly and the seed designs under `/agni/demo/`, and the docs workflow publishes them with every
merge to main.

The analysis needs no server because the same services run in either place. The viewer sends the
same calls whoever answers them. `agni serve` answers them on your machine or a shared host, and the
WebAssembly engine answers them in the visitor's browser. Either one reads the design, runs the
checks and evaluates the queries, and the viewer only draws the answers. On your own server,
`agni serve --engine wasm` or a page's `?engine=wasm` picks the browser engine for one design, and
[Running the server](../guide/running-the-server/) covers both. How the two engines are held to the
same answers is in
[The same contract in the browser](../architecture/web-services/#the-same-contract-in-the-browser).

## Two static renders

These two are pictures rather than the live viewer. `agni render` drew them at build time, and you
can drag to pan and scroll to zoom.

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/demo-board.svg"
             caption="Demo board with front and back copper, zones and vias"></agni-viewer>

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/demo-schematic.svg"
             caption="Faithful schematic render"></agni-viewer>
