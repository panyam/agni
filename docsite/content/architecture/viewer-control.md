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
| a design's relay room, `viewerroom.ts` | an agent on `/mcp`, through the room bridge | a tab whose reader turned on "Let agents drive this tab", under `agni serve --mcp --viewer-rooms` (agni issue 1154) |

The report's case is narrower on purpose. A report opened from disk has an opaque origin that no
server can name, and every link in it targets the window named `agni-viewer`, so clicking it with
scripts off loads the link in that window anyway. The page therefore takes `ApplyLocation` carrying a
link and nothing else from any origin, since that grants what loading the link grants, and its reply
says only whether it worked. Every other call, and every reply carrying the page's location, stays
with the origins the server named. The report loads the link itself when no viewer speaks within 2
seconds, and a page posts an accepted note on an envelope of its own (`agni-viewer/v1/accepted`) the
moment it takes a request, so a verdict link whose rule takes longer to resolve is waited for rather
than loaded over. A report never depends on its script, and one with no links
carries none, since a report has to read the same emailed, committed or opened with scripts off
(`core/report/html.go`).

A page call lands as a click and a link do. `sheetToOpen` in `viewer.ts` picks the sheet for a
finding, a review finding and a verdict, a verdict carries the sheets it is drawn on
(`Verdict.sheets`), and `web/src/navparity.test.ts` runs a link, a click and a call to the same verdict
and names the one that lands elsewhere. [Working in the web client](../web-client/) has the
presenter side of this.

## An agent moving an open tab

MCP lets a client call a server and nothing else, so an agent cannot call a page. Under
`agni serve --mcp --viewer-rooms` something both sides connect to stands between them (agni issue
1154). The server embeds a massrelay relay at `/relay/` and mcpkit's room bridge beside `/mcp`
(`internal/viewerrooms`). Each tab joins the room of the design it shows, `design:<mount>/<entry>`,
and offers every `ViewerPageService` method there as a tool. The bridge serves those tools to agents
as `viewer_get_location`, `viewer_show_sheet`, `viewer_locate_verdict` and the rest, and sends each
call to one tab, where `pageservice.ts` answers it as it answers postMessage.

A tab learns its room and a relay token from `ViewerRoomService.JoinViewerRoom`. The room is resolved
the way a report's links are (`linkMountPath`, C32), so a tab opened on a design's folder and one
opened on its entry share a room, and an agent naming the design either way reaches both. The token
admits its holder to that one room and is signed with a key the server makes at start, so a restart
turns away a page that has not joined yet. When the page opens another design in place, `syncUrl`
moves the tab to that design's room.

Every call names its design, which the bridge maps to the room (`rooms.WithRoomOf`), so an agent
switches between designs from one question to the next without attaching to anything. Within a room
a call goes to the tab named by `page` (`list_pages` lists them), else to the tab of that design the
reader looked at last, and it only ever reaches a tab whose reader turned on "Let agents drive this
tab". That switch is per tab, off by default, and kept in the tab's session storage, so a reload
keeps it and a new tab starts with it off.

A tab answers the bridge alone. `JoinViewerRoom` names the relay subject the bridge joins as, and the
tab passes it to the rooms client's `trustBridges`, so a message from any other peer in the room goes
unanswered (agni issue 1173). Every token the server signs is for the local subject, so no page can
join as the bridge, and a page holding a genuine token for the room still cannot call a tab directly,
past the bridge's check of the switch.

The bridge declares the six tools up front (`rooms.WithPageTools`), so `tools/list` answers the same
for every caller and the bridge serves the stateless 2026-07-28 MCP wire as well as the session one.
It also refuses a tab's tool whose schema differs from the declaration by a byte. The declaration is
built in Go from the service's descriptor (`internal/viewerrooms/tools.go`) and the tab builds its
own from protobuf-es descriptors (`web/src/toolschema.ts`), so both walks are held to one fixture,
`internal/viewerrooms/testdata/viewer_tools.json`, following the mapping mcpkit#1609 sets out for
proto services as tools. `GetLocation` is marked `NO_SIDE_EFFECTS` in the proto, and that is where
both walks read its read-only hint. A page and a server from different builds can still disagree
about a tool, and the bridge then refuses it. The tab hears `notifications/rooms/tool_refused` and
names the tool beside its switch (`onToolRefused`, since mcpkit v0.8.13).

## A tab asking an agent

The same room carries the other direction (agni issue 1173). With its switch on, a tab tells the
room what the reader selected (`page.selection_changed`, from the Properties panel's focus), and the
Ask box beside the switch sends the reader's question as `room.ask`, its data carrying the text, the
tab's `GetLocation` answer and the selection. These are page events, which the bridge buffers per
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
`resource`. Attaching by design names no room, so the front end watches the room in `attach`'s
answer, which mcpkit only started doing in v0.8.13 (mcpkit#1640). Before that it watched nothing, and
a question asked by design never arrived.

`WithRoomOf` declares `design` as its room argument, so `next_request`, `list_pages`, `attach` and
`pair` take the design the way the page tools do, resolved through the same `ViewerRoomService.Room`
(mcpkit#1633). Before that they took only the resolved room name, which no answer told the agent.

## What is not here yet

The server has no identity provider, so every relay token and every MCP session acts for one local
subject, and the switch is the only agreement there is. Several people on one server, each with an
agent, needs OAuth on `/mcp` and a relay token per user, with mcpkit's pairing codes for an agent
driving someone else's tab.

## Running it

`make exercise-mcp-ask` asks every question tool over MCP and holds each reply's link to a working
page. `make exercise-viewer-page` drives a framed viewer from a parent on another origin, moves a
viewer from a saved report beside it, and has an agent on each MCP wire drive tabs on two designs
under `--viewer-rooms`, where a tab that kept agents out stays where its reader left it. [Driving an embedded viewer](../../build/embedding-the-viewer/)
is the guide for a page that wants to do either.
