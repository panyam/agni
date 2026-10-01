---
title: "Calling agni from another language"
description: "Use the typed Python client, over the CLI or a running server, and what the same contract offers any other language."
---

A program outside Go reaches agni in one of two ways. It runs the `agni` binary and reads what the
command prints, or it posts to a running `agni serve`. Both speak one contract. Every command's
`--format json` is the protojson of the message its rpc returns (C31), so a script reading the CLI and
a client calling the server parse the same shape.

The contract is typed, since it is generated from the protos under `protos/agni/v1`. A caller that
parses it as plain JSON throws the types away. The Python client under `clients/python` keeps them,
and the last section says what the same approach needs in any other language.

## The Python client

The package is `agni`, and its only runtime dependency is `protobuf`. The message classes under
`agni.v1` are generated from the same protos as the Go engine and the web viewer. `make proto-py`
regenerates them and `make proto-check` fails the gate when they are stale.

```sh
pip install -e 'clients/python[xlsx]'
```

A `Client` wraps a transport, and each method takes and returns the rpc's own messages:

```python
from agni import Client, CliTransport, ConnectTransport, rows_as_dicts

cli = Client(CliTransport("agni", mounts={"tut": "examples/tutorial-project"}))
srv = Client(ConnectTransport("http://127.0.0.1:8080"))

answer = cli.run_query(uri="mount://tut/designs/gateway",
                       query='component.class(?c, "resistor") => ?c')
for row in rows_as_dicts(answer):
    print(row["c"])
```

A design is named by its `mount://` URI on both transports, so one request object works on either.
`CliTransport` turns its `mounts` into `--mount` flags. A server has to be started with the same
mounts:

```sh
agni serve --mount tut=examples/tutorial-project
```

`Client.call(service, method, ...)` reaches every rpc by name, with the request and response types
read off the generated descriptors. The named methods (`check_design`, `run_query`, `diff_designs`,
`trace_design` and others) are the same calls with type hints.

## Choosing a transport

**The CLI transport** needs no server and ships as one binary. Every call starts a process and reads
the design again, and on a large board that read is the slow part. `run_queries` asks many queries
in one call, so over either transport a whole audit costs one read.

**The Connect transport** posts JSON to `/agni.v1.webapi.<Service>/<Method>`. The server reads a
design once and answers many questions about it. `agni serve` with no web dir serves the API alone,
so an installed binary needs no viewer assets for this.

The CLI covers the rpcs a command maps to. `CLI_COMMANDS` in `agni/transport.py` is the table:

| rpc | command |
|---|---|
| `CheckService/CheckDesign` | `check --format json` |
| `CheckService/GetCheckReport` | `check --format report` |
| `QueryService/RunQuery` | `query --format json` |
| `QueryService/RunQueries` | `query --set - --format json`, the set sent on stdin |
| `DiffService/DiffDesigns` | `diff --format json` |
| `DesignService/TraceDesign` | `trace --format json` |
| `DesignService/GetLayoutReport` | `render --report --report-format json` |

Any other rpc raises `CliUnsupported`. So does a request field the command has no flag for, such as
an `overlay`, because dropping it would answer a different question from the one asked. `validate`
and `params` print a wire message with no rpc behind it, and `CliTransport.run` reads them. `intake`
is C31's declared exception and has no wire message. `review` is Connect only until
`review --format json` emits the `Review` proto (agni issue 734).

## Where the two transports differ today

`clients/python/tests/test_cross_transport.py` sends the same request both ways and asserts the two
messages are equal. That test is C31 checked from outside Go. Three known differences are declared in
it field by field, and each declaration fails once the difference goes away:

- `CheckDesign` over the CLI carries no `verdicts`. `check --format json` strips the considered set on
  purpose and `--verdicts` prints it as a bare list, which is not a wire message.
- `DiffDesigns` over the CLI carries no sheet or placement maps (agni issue 737).
- `GetLayoutReport` on a design FOLDER answers an empty report from the server (agni issue 736).
  Name the entry file with `as_named` until that lands.

## Tables and workbooks

A query answer is `repeated QueryRow { repeated string cells }` plus `column_kinds`. Its cells are
strings, and the client leaves them that way. A column kind says what a column names, such as a net
or a component, and not what type its values are, so converting `"10k"` or `"007"` on a guess would be
wrong.

`tables_to_xlsx(path, [(sheet, table), ...])` writes one sheet per table with a bold header row, a
frozen top row and an autofilter. A table is a `RunQueryResponse`, a `CheckDesignResponse` (one
finding per row), or a `(header, rows)` pair. It needs the `xlsx` extra. Tab names, colours and
highlighted rows stay in the caller, which can reopen the file with openpyxl. `set_sheets(response)`
turns a `RunQueriesResponse` into those pairs, one per query in the set's order, and refuses a set
with an unanswered query unless asked to drop it, because a workbook missing a tab reads as a table
that matched nothing. `clients/python/examples/audit_workbook.py` writes a five-sheet audit of the
tutorial board this way.

The engine has no xlsx writer, on purpose. A workbook is a zip of cross-referencing XML parts whose
layout belongs to whoever reads it, so it stays in the client.

## Another language

Everything the Python client does is available to any language with a protobuf runtime:

1. Generate message classes from `protos/agni/v1` with that language's protoc plugin. Pin the plugin
   by version, the way `clients/python/buf.gen.py.yaml` does.
2. Read a response with the runtime's protojson parser. The CLI's stdout and the server's response
   body are both protojson of the same message.
3. Post to a server with `Content-Type: application/json`. The Connect protocol's unary form needs no
   Connect library.

A generated client committed to this repo belongs in `proto-check`, as the Go, TypeScript and Python
halves are, so that a proto change which forgets it fails the gate.
