---
title: "Questions, links and the live page"
description: "How agni answers a question, how an answer points at the viewer, and how something outside the viewer moves a page that is already open."
---

Three things in agni look alike from outside and work quite differently. A question about a design
is answered by the engine, over Connect or MCP. An answer that wants to show something in the viewer
carries a link, which is an address and nothing more. And a page that is already open can be moved
by something beside it, a parent frame or a saved report, without loading it again. This page says
which layer a feature belongs in. A feature built in the wrong layer works on the machine it was
built on and nowhere else.

## The three layers

| Layer | What it is | Who answers | Where its state lives |
|---|---|---|---|
| Questions | `CheckService`, `QueryService`, `DesignService` and the rest, and the MCP tools over them | `agni serve`, `agni mcp`, or the browser engine | snapshots on the host that ingested them, nothing per caller |
| Addresses | a viewer URL, which `router.ts` reads as a `ViewerLocation` | nobody, it is text | in the link itself |
| The live page | `ViewerPageService` (`protos/agni/v1/viewer/page.proto`) | the open viewer page | in that page, for as long as it is open |

A question never moves a page, and a page call never computes an answer the engine owns. The MCP
server in particular holds no viewer state. It answers about snapshots and hands back links, and the
reader owns the page each link opens (agni issue 1153).

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

`ApplyLocation` with a `url` reads the link with the same parser the page uses for its own address,
so any link is a page call without being spelled a second way. The MCP tools put the call beside each
link as `page` (agni issue 1155), with `LocateVerdict` for a verdict. `LocateVerdict` carries hints
for a page that has not run the rule, so that page can ask about one subject.

## Moving a page that is open

The page answers `ViewerPageService` itself, through `pageservice.ts`, which dispatches a method name
and a protojson body. A carrier only brings that pair to the page, so a call behaves the same whichever
one brought it.

| Carrier | Who is calling | Taken from |
|---|---|---|
| postMessage, `embed.ts` | a parent page framing the viewer | the viewer's own origin, and each `agni serve --embed-origin` |
| postMessage, `core/report/viewerlink.js` | a saved check or checklist report | any origin, for `ApplyLocation {url}` alone |
| a design's relay room, `viewerroom.ts` | an agent on `/mcp`, through the room bridge | a tab whose reader turned on "Let agents drive this tab", under `agni serve --mcp --viewer-rooms` (agni issue 1154) |

The report's case is narrower on purpose. A report opened from disk has an opaque origin that no
server can name, and every link in it targets the window named `agni-viewer`, so clicking it with
scripts off loads the link in that window anyway. The page therefore takes `ApplyLocation` carrying a
link and nothing else from any origin, since that grants what loading the link grants, and its reply
says only whether it worked. Every other call is limited to the origins the server named, and so is
every reply carrying the page's location. The report loads the link itself when no viewer speaks
within 2 seconds. A page posts an accepted note on an envelope of its own (`agni-viewer/v1/accepted`)
the moment it takes a request. So when a verdict link's rule takes longer to resolve, the report
waits for it rather than loading over it. A report never depends on its script, and a report with no
links carries no script. A report has to read the same whether it is emailed, committed or opened
with scripts off (`core/report/html.go`).

A page call opens the same place as a click and a link. `sheetToOpen` in `viewer.ts` picks the sheet
for a finding, a review finding and a verdict. A verdict carries the sheets it is drawn on
(`Verdict.sheets`). `web/src/navparity.test.ts` runs a link, a click and a call to the same verdict,
and names any of the three that opens somewhere else. [Working in the web client](../web-client/)
has the presenter side of this.

## An agent moving an open tab

MCP lets a client call a server and nothing else, so an agent cannot call a page. Under
`agni serve --mcp --viewer-rooms`, both sides connect to something that stands between them (agni
issue 1154). The server embeds a massrelay relay at `/relay/` and mcpkit's room bridge beside `/mcp`
(`internal/viewerrooms`). Each tab joins the room of the design it shows, `design:<mount>/<entry>`,
and offers every `ViewerPageService` method there as a tool. The bridge serves those tools to agents
as `viewer_get_location`, `viewer_show_sheet`, `viewer_locate_verdict` and the rest, and sends each
call to one tab, where `pageservice.ts` answers it as it answers postMessage.

A tab learns its room and a relay token from `ViewerRoomService.JoinViewerRoom`. The room is
resolved the way a report's links are (`linkMountPath`, C32), so a tab opened on a design's folder
and one opened on its entry share a room, and an agent naming the design either way reaches both.
The token admits its holder to that one room. The server makes a signing key at start and signs each
token with it, so a restart turns away a page that has not joined yet. When the page opens another
design in place, `syncUrl` moves the tab to that design's room.

Every call names its design, which the bridge maps to the room (`rooms.WithRoomOf`), so an agent
switches between designs from one question to the next without attaching to anything. Within a room
a call goes to the tab that `page` names (`list_pages` lists them). Without `page`, it goes to the
reader's most recently viewed tab of that design. A call only ever reaches a tab whose reader turned
on "Let agents drive this tab". That switch is per tab, off by default, and kept in the tab's
session storage, so a reload keeps it and a new tab starts with it off.

A tab answers the bridge alone. The bridge joins the relay under a subject, and `JoinViewerRoom`
names that subject to the tab. The tab passes it to the rooms client's `trustBridges`, so a message
from any other peer in the room goes unanswered (agni issue 1173). Every token the server signs is
for the local subject, so no page can join as the bridge. A page that holds a genuine token for the
room still cannot call a tab directly and skip the bridge's check of the switch.

The bridge declares the eight tools up front (`rooms.WithPageTools`), so `tools/list` answers the same
for every caller and the bridge serves the stateless 2026-07-28 MCP wire as well as the session one.
It also refuses a tab's tool whose schema differs from the declaration by a byte. The declaration is
built in Go from the service's descriptor (`internal/viewerrooms/tools.go`) and the tab builds its
own from protobuf-es descriptors (`web/src/toolschema.ts`), so both walks are held to one fixture,
`internal/viewerrooms/testdata/viewer_tools.json`. mcpkit#1609 sets out the mapping for proto
services as tools, and the fixture follows it. `GetLocation` is marked `NO_SIDE_EFFECTS` in the
proto, and both walks read its read-only hint from that mark. A page and a server from different
builds can still disagree about a tool, and the bridge then refuses it. The tab hears
`notifications/rooms/tool_refused` and names the tool beside its switch (`onToolRefused`, since
mcpkit v0.8.13).

## Sheet views beside the canvas

A reader opens a sheet view from the Sheets panel, such as the board beside the schematic (agni
issue 1326). The page contract opens one too, so a parent page or an agent can put the
board beside the canvas without a reader's click (agni issue 1370). `OpenSheetView` opens a view on
a sheet and answers its id, `ShowSheet` with a `view` moves that view to another sheet, and
`CloseSheetView` closes it. The canvas keeps its own sheet throughout. `GetLocation` lists the open
views beside the location. `OpenSheetView` takes a `renderer`, `svg` or `webgl`, and with none the
view opens in the canvas's renderer (agni issue 1368). Each listed view says which renderer drew it.
A browser with no WebGL2 draws a WebGL view in SVG, and the view then says `svg`.

A view is not part of `ViewerLocation`, because a link cannot open one. A link names one sheet, and
a view lives only as long as the page holding it. The id is `sheetview-<n>`, numbered from 1 per
page and never reused, so a caller holding the id of a closed view is refused rather than reaching
some other view. `SheetViews` in `sheetviews.ts` holds the views, and `pageApi` reaches them
through the `SheetViewsHost` port, so a page with no dock lists none and refuses to open one.

## A tab asking an agent

The same room carries the other direction (agni issue 1173). With its switch on, a tab tells the
room what the reader selected (`page.selection_changed`, from the Properties panel's focus). The Ask
box beside the switch sends the reader's question as `room.ask`. Its data carries the text, the
tab's `GetLocation` answer and the selection. A selection carries a `view` (agni issue 1370). It is
the id of the sheet view the reader picked in, or `canvas` for every other selection. So an agent
can tell a pick on the board view from a pick on the schematic. These are page events, which the bridge buffers per
room (`rooms.WithEventFallbacks`) and hands only to agents the tab agreed to. An agent waits for the
next one with `next_request`, on either wire, or subscribes to `rooms://events/{room}` on the session
wire.

`viewerrooms.Rooms.Register` also serves them as SEP-3415 events through mcpkit's `roomsevents`, as
`room.ask`, `page.selection_changed` and `page.event`, each taking the room as its `resource`
argument. A session may subscribe only while it is attached to that room, and it hears only the
tabs that agreed to it, the same rule as `next_request`. Attachment lasts as long as an MCP session,
so the events need the session wire, and with no identity provider they act for the local subject
(`events.Config.UnsafeAnonymousPrincipal`). `rooms-channel` builds on these events to put a reader's
question in front of a Claude Code session as a turn. `TestARoomsChannelHearsATabsQuestion` runs
that front end over a pipe against agni's `/mcp`, attaching by `design` as an agent does and by
`resource`. Attaching by design names no room, so the front end watches the room named in
`attach`'s answer. mcpkit's front end only started doing that in v0.8.13 (mcpkit#1640). Before that
it watched nothing, and a question asked by design never arrived.

`WithRoomOf` declares `design` as its room argument, so `next_request`, `list_pages`, `attach` and
`pair` take the design the way the page tools do, resolved through the same `ViewerRoomService.Room`
(mcpkit#1633). Before that they took only the resolved room name, and no answer told the agent that
name.

## What is not here yet

The server has no identity provider, so every relay token and every MCP session acts for one local
subject, and the switch is the only agreement there is. Before several people can share one server,
each with an agent, the server needs OAuth on `/mcp` and a relay token per user. An agent driving
someone else's tab also needs mcpkit's pairing codes.

## Running it

`make exercise-mcp-ask` asks every question tool over MCP and holds each reply's link to a working
page. `make exercise-viewer-page` drives a framed viewer from a parent on another origin, moves a
viewer from a saved report beside it, and has an agent on each MCP wire drive tabs on two designs
under `--viewer-rooms`. A tab that kept agents out is still where its reader left it.
[Driving an embedded viewer](../../build/embedding-the-viewer/) is the guide for a page that wants to
do either.
