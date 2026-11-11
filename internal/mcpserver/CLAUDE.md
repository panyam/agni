# internal/mcpserver

This file covers agni's MCP server, its tools, its wires and the exercise that checks it.

- **`agni serve --mcp` and `agni mcp` serve the analysis over MCP** (agni 910). The first serves at `/mcp` beside the Connect API. The second serves over stdio. The same `runViewer` composes both.
- **jaala's `frontends/mcp` supplies the query language's tools** (`query`, `explain`, `schema`, `look`, `values`), and agni adds a `design` argument. agni registers `check`, `trace`, `diff`, `design`, `checklists`, `review`, `query_set` and `designs`.
- **jaala evaluates a query and agni presents it.** Presenting goes through `QueryService.ReadQuery`, `QueryRead.Present` and `Tabulate` (C35). `QueryRead.Present` is `RunQuery`'s own projection, so a new query field reaches MCP with no change here.
- **`Run.BaseFor` reads the board only for a program that reads `board.*`**, as `RunQuery` does.
- **`TestMCPAnswersAsConnectDoes` asks every tool and its rpc the same question over one server.** A new tool belongs in it.
- **The HTTP endpoint accepts both MCP wires** (agni 1172). `mcpserver.Wire` pins mcpkit's `ModeDual`, so a change of mcpkit's default cannot drop a wire. `MCPKIT_STATELESS_MODE` overrides it. The MCP tests run on both wires through `eachWire`.
- **agni's own clients speak `adaptive`.** They use the stateless wire when the server offers it and a session otherwise. `agni mcp` on stdio serves both wires (panyam/mcpkit#1637).
- **`--viewer-rooms` also serves a tab's page events as SEP-3415 events** through mcpkit's `roomsevents` (agni 1187). `rooms-channel` then puts a reader's Ask in front of a Claude Code session as a turn. The events need the session wire. `TestARoomsChannelHearsATabsQuestion` runs that front end over a pipe.
- **A Go test joining as a tab sets an `Origin` header** (`viewerRoomsServer`). massrelay's Go dialer sends none, and the relay refuses a connection without one.
- **The lists carry an hour's `ttlMs`** (`listTTLMs`, passed through jaala's `Options.Server`).
- **`make exercise-mcp-ask` is mission #909's exercise** (agni 1121). `examples/ask-over-mcp exercise <spec>` asks a spec's questions over `serve --mcp` on a free port. It writes each reply's table through `service.WriteTableCSV` and holds it to the CLI's csv for the same request. It holds each reply's links to a 200 and its rows to what the spec pins.
- **A new tool's reply opens with `design:` or `diff:`.** It carries `in the viewer: <url>` or `no viewer link: <why>`.
