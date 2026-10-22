---
title: "Questions, links and the live page"
description: "How agni answers a question, how an answer points at the viewer, and how something outside the viewer moves a page that is already open."
---

Three things in agni look alike from outside and work quite differently. A question about a design
is answered by the engine, over Connect or MCP. An answer that wants to show something in the viewer
carries a link, which is an address and nothing more. And a page that is already open can be moved
by something beside it, a parent frame or a saved report, without loading it again. This page says
which layer a feature belongs in, since one that lands in the wrong layer works on the machine it
was built on and nowhere else.

## The three layers

| Layer | What it is | Who answers | Where its state lives |
|---|---|---|---|
| Questions | `CheckService`, `QueryService`, `DesignService` and the rest, and the MCP tools over them | `agni serve`, `agni mcp`, or the browser engine | snapshots on the host that ingested them, nothing per caller |
| Addresses | a viewer URL, which `router.ts` reads as a `ViewerLocation` | nobody, it is text | in the link itself |
| The live page | `ViewerPageService` (`protos/agni/v1/viewer/page.proto`) | the open viewer page | in that page, for as long as it is open |

A question never moves a page, and a page call never computes an answer the engine owns. The MCP
server in particular holds no viewer state. It answers about snapshots and hands back links, and the
page those links open is the reader's own (agni issue 1153).

## From an answer to an address

Every surface that wants to show something mints its link through the same builders in
`core/report`, so a link from `agni check --server`, from an MCP answer and from a saved report opens
the same view. Each one sets fields of the location, and each has a page call that does the same to a
page already open.

| Builder | Location fields it sets | The page call that does the same |
|---|---|---|
| `VerdictURL` | `verdict`, `hash`, `rule`, `subjects` | `LocateVerdict {id, rule, subjects}` |
| `TraceURL` | `trace`, `traceHops` | `ApplyLocation {url}` |
| `QueryURL` | `query`, `title`, `binds` | `ApplyLocation {url}` |
| `QuerySetURL` | `set`, `q` | `ApplyLocation {url}` |
| `DiffURL` | `against`, `renameApprox` (on the older revision's address) | `ApplyLocation {url}` |

`ApplyLocation` with a `url` reads the link with the parser the page reads its own address with, so
any link is a page call without being spelled a second way. The MCP tools put the call beside each
link as `page` (agni issue 1155), with `LocateVerdict` for a verdict, since it carries the hints that
let a page that has not run the rule ask about one subject.

## Moving a page that is open

The page answers `ViewerPageService` itself, through `pageservice.ts`, which dispatches a method name
and a protojson body. A carrier only brings that pair to the page, so a call behaves the same whichever
one brought it.

| Carrier | Who is calling | Taken from |
|---|---|---|
| postMessage, `embed.ts` | a parent page framing the viewer | the viewer's own origin, and each `agni serve --embed-origin` |
| postMessage, `core/report/viewerlink.js` | a saved check or checklist report | any origin, for `ApplyLocation {url}` alone |
| a server-held session | an MCP client, through the server | not built yet, agni issue 1154 |

The report's case is narrower on purpose. A report opened from disk has an opaque origin that no
server can name, and every link in it targets the window named `agni-viewer`, so clicking it with
scripts off loads the link in that window anyway. The page therefore takes `ApplyLocation` carrying a
link and nothing else from any origin, since that grants what loading the link grants, and its reply
says only whether it worked. Every other call, and every reply carrying the page's location, stays
with the origins the server named.

A page call lands as a click and a link do. `sheetToOpen` in `viewer.ts` picks the sheet for a
finding, a review finding and a verdict, a verdict carries the sheets it is drawn on
(`Verdict.sheets`), and `web/src/navparity.test.ts` runs a link, a click and a call to the same verdict
and names the one that lands elsewhere. [Working in the web client](../web-client/) has the
presenter side of this.

## What is not here yet

An MCP client cannot move an open tab today. It gets a link and a `page` call, and a host that frames
the viewer can send the call over postMessage, but the server itself has no way to reach a tab. agni
issue 1154 is that design: a page that attaches to `agni serve` over a Connect stream, one MCP tool per
page method, and a flag that has to be passed before any of it is on, since it lets a client move a
reader's tabs.

## Running it

`make exercise-mcp-ask` asks every question tool over MCP and holds each reply's link to a working
page. `make exercise-viewer-page` drives a framed viewer from a parent on another origin, and the same
spec moves a viewer from a saved report beside it. [Driving an embedded viewer](../../build/embedding-the-viewer/)
is the guide for a page that wants to do either.
