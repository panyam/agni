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
  nets has no pull-up" is one call.
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

A question agni can't answer says so as a tool error, with what went wrong and often how to fix it:
a relation that doesn't exist, a pin the design lacks, a question past the budget. Asking about a
pin that isn't there is a different answer from two real pins with no route between them, and the
two stay apart here as they do in `agni trace`.

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
tutorial board instead of the Jetson baseboard.
