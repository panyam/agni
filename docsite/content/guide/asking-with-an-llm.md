---
title: "Asking through an LLM"
description: "Point Claude Code, or any MCP client, at agni, so a model asks a design questions and every answer comes back as rows you can check."
---

agni serves its analysis over the [Model Context Protocol](https://modelcontextprotocol.io), so a
coding agent such as Claude Code can ask a design questions in your place. The model never states a
fact about the board on its own authority. It writes a question in agni's
[query language](../querying/), or calls a check, a trace or a diff, and agni answers from the design
with rows, the file and place each row was read from, and a link that opens the answer in the
viewer. A wrong question then shows up as a visibly wrong table rather than a confident wrong
sentence, which is pretty much the point.

## Connecting a client

There are two ways in, and both serve the same tools.

`agni serve --mcp` serves MCP over streamable HTTP at `/mcp`, beside the viewer and the Connect API,
on the same mounts:

```sh
agni serve --mcp --mount tut=examples/tutorial-project
```

and Claude Code connects to it with

```sh
claude mcp add --transport http agni http://localhost:8080/mcp
```

`agni mcp` serves the same tools on stdin and stdout, for a client that starts the server itself. It
reads the mounts the way `serve` does, from `--mount` or the nearest `agni.yaml`:

```sh
claude mcp add agni -- agni mcp --mount tut=/path/to/examples/tutorial-project
```

The HTTP endpoint accepts requests from a localhost origin only, so a page on another site can't
drive a server running on your machine. `--query-budget` caps the work of every question a client
asks, as it does for the viewer's.

The HTTP endpoint speaks both MCP wires on one URL, and a client uses whichever it prefers: the
stateless 2026-07-28 wire, where every request stands alone, or the older session wire that starts
with `initialize`. Every tool answers the same rows and links on both. Nothing the server keeps
depends on a session, since a design's snapshot is kept per server and per design. agni sets this
itself rather than taking its MCP library's default, and `MCPKIT_STATELESS_MODE=legacy` or
`=stateless` forces one wire, for debugging a client. `agni mcp` on stdio speaks the session wire
alone for now (panyam/mcpkit#1637).

## What the model can call

The query language's tools come from [jaala](https://panyam.github.io/jaala/guide/mcp-server/), the
Datalog engine underneath agni, and each takes a `design` beside its own arguments:

- `schema` lists every relation a question can call, with its signature and what it holds, and with a
  design it includes that design's project library too. `look` describes one relation, and `values`
  shows what one of its arguments holds, which is how a model finds that a net is spelled `I2C_SDA`
  rather than `SDA`.
- `query` runs a question and answers its rows. A value the user gave goes in `bind`, by its
  variable's name, rather than being pasted into the question, so a net named with a quote in it
  can't change what is asked. A list there asks about each value at once, so "which of these three
  nets has no pull-up" is one call. Under its table the text says how each row holds, the
  [witness tree](../querying/#see-how-a-row-was-derived-witnesses) cut at eight lines a row, and the
  structured content carries every tree whole.
- `explain` says where a question's work went and, when it answers nothing, which part matched
  nothing.

agni adds the rest:

- `designs` lists the designs a server answers about, by the name the other tools take
  (`tut/designs/gateway`), with each one's declared revisions.
- `check` runs the [rule catalog](../checks-and-reports/), or the rules you name, and answers the
  findings, a count per rule and outcome, and a link per failing verdict. A rule name that doesn't
  exist is refused with the closest ones it does have, rather than answered as an empty, clean run.
- `trace` follows a signal from one pin to another through the series parts between them, and says
  where each net on the way can be probed.
- `diff` compares two revisions, as [comparing revisions](../comparing-revisions/) describes. Its
  link opens the earlier revision with the viewer's Compare started against the later one.
- `design` reads the IR itself, narrowed to the nets or parts asked about.
- `checklists` and `review` list a design's review checklists and score one. Nothing is saved.
- `query_set` runs one of the project's saved query sets.

The relation catalog, the shipped library, the rule catalog and the designs are also resources
(`agni://relations`, `agni://library`, `agni://rules`, `agni://designs`), for a client that reads
those rather than calling a tool.

## What an answer carries

Every answer opens with the design and the question that produced it, so you can see what was
actually asked before reading what came back. The rows follow as a table with a provenance column
naming the file each row rests on, which is the same table `--format csv` writes for that question,
and then a link that opens the answer in the viewer. The structured content carries the same
tables and the full wire message, so a client can render them itself.

A server started with the viewer links to itself. `agni mcp` has no viewer of its own, so it links to
one only when `--server` names a running `agni serve` that mounts the same names, and otherwise each
answer says why it carries no link.

Beside each link the structured content carries `page`, the call that does the same to a viewer a
host already has open (agni issue 1155). A host that frames the viewer, an MCP app host say, sends it
over postMessage instead of opening a new tab. A failing verdict in a `check` answer looks like this.

```json
{
  "verdict": "demo-rule:(net:NET_A)",
  "rule": "demo-rule",
  "subjects": ["net:NET_A"],
  "link": "http://127.0.0.1:8080/designs/demo/demo-board.kicad_sch/view?verdict=demo-rule%3A%28net%3ANET_A%29&rule=demo-rule&subject=net%3ANET_A",
  "page": { "method": "LocateVerdict", "body": { "id": "demo-rule:(net:NET_A)", "rule": "demo-rule", "subjects": ["net:NET_A"] } }
}
```

Every other link (a trace, a query, a diff) comes with `ApplyLocation` carrying the link itself. How a
host sends either is in [Driving an embedded viewer](../../build/embedding-the-viewer/).

A question agni can't answer says so as a tool error, with what went wrong and often how to fix it:
a relation that doesn't exist, a pin the design lacks, a question past the budget. Asking about a
pin that isn't there is a different answer from two real pins with no route between them, and the
two stay apart here as they do in `agni trace`.

## Moving a viewer tab you have open

Every answer links to the viewer, and following a link opens a new tab. Started as
`agni serve --mcp --viewer-rooms`, the server also lets the model move a tab you already have open.
Each viewer tab shows a "Let agents drive this tab" switch beside the sheet tabs. Turn it on in the
tabs you want driven, and the model can call `viewer_show_sheet`, `viewer_locate_verdict`,
`viewer_select_finding`, `viewer_locate_entity`, `viewer_apply_location` and `viewer_get_location`
on them.

Each call names the design, as every other tool does, so with tabs open on several designs the model
moves the one you are asking about. With two tabs on one design it moves the one you looked at last,
or one it names from `list_pages`. A tab with the switch off is never moved, and the switch is off in
every new tab. [Questions, links and the live page](../../architecture/viewer-control/#an-agent-moving-an-open-tab)
has how it works.

The other direction works too. With the switch on, a tab shows an Ask box beside it, and what you
type there goes to the model along with the sheet you are on and what you have selected. The model
hears it by calling `next_request` with the design, which waits up to two minutes for the next
question or selection from a tab that let it in. So "wait for my question in the viewer" is a
reasonable thing to tell it. A tab with its switch off sends nothing, whatever you type.

In Claude Code a question can arrive as a turn of its own, so the model needn't sit waiting.
mcpkit's `rooms-channel` is a channel server that Claude Code starts beside the session, and it
pushes each question from a tab into the conversation as a message. Install it and register it
next to agni in `.mcp.json`:

```bash
go install github.com/panyam/mcpkit/experimental/integrations/roomsevents/cmd/rooms-channel@latest
```

```json
{
  "mcpServers": {
    "rooms": { "command": "rooms-channel", "args": ["-bridge", "http://localhost:8080/mcp"] }
  }
}
```

Channels are a Claude Code research preview, so start the session with
`claude --dangerously-load-development-channels server:rooms`, interactively. Then ask the model to
attach to the design you have open. For now it has to attach with `resource` set to the room, such
as `design:tut/designs/gateway/gateway.edn`, which a first `attach` with the design answers, because
`rooms-channel` listens only to a room named that way (mcpkit issue 1640). From then on each question
you type in a tab that let agents in shows up in the session, fenced as text from the page, and the
model answers through the tab's tools.

## What it doesn't do

agni hosts no model, so the client brings its own, and nothing about a design leaves the machine
except through the client you pointed at it. And a model that never calls the tools can still make
things up, so the useful habit is to look for the question above each answer and the link below it.

## Checking that it works

`make exercise-mcp-ask` asks five questions a reviewer usually starts with (missing pull-ups, what a
bus line connects to and whether it is protected, rails with no test point, what changed between two
revisions, and a trace from a pin to a connector) through an MCP client, as the calls a model would
make. Each reply has to state its question, carry links that open in the viewer, hold the same rows
the CLI answers to the same request, and contain the rows the design is known to have. The questions
live in `examples/ask-over-mcp/exercise/`, one file per board, and `WALK_DESIGN=gateway` runs the
tutorial board instead of the Jetson baseboard. It asks everything twice, once on the stateless
wire and once on the session wire, and writes a report for each.
