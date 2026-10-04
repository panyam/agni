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

A query's variables are bound with `agni.bindings`, which types each value, so a `str` binds text and
an `int` or `float` a number:

```python
client.run_query(uri="mount://tut/designs/gateway", query="component.net(?ref, ?net) => ?net",
                 bindings=agni.bindings({"ref": "U1"}))
```

The query text stays the same for every part, and the value needs no escaping. Over the CLI transport
the bindings travel as `--bind`, and a set's per-query bindings as the set file's `bind:` maps.

### Reading the design itself

When a question is not yet a query, a check or a diff, a script can read the design's IR and work on
it directly, prototyping an analysis before it moves into agni as a library relation or a rule.
`get_design` carries it when a `read_mask` asks, and only the parts asked for:

```python
d = client.get_design(uri="mount://tut/designs/gateway",
                      read_mask={"paths": ["design.nets", "design.components.mpn"]},
                      nets=["PMIC_EN"]).design
for net in d.nets:
    print(net.name, [f"{c.component_ref}.{c.pin_ref}" for c in net.connections])
```

The messages are `agni.v1.ir` types. Unmasked, `get_design` is the summary it always was, because a
large board's IR runs to megabytes. `nets` and `ref_des` narrow which entities come back. Over the
CLI transport the same request runs `agni stats --format json --mask ...`.

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
| `CheckService/CheckDesign` | `check --verdicts --format json`, which prints the whole response, verdicts included |
| `CheckService/GetCheckReport` | `check --format report` |
| `QueryService/RunQuery` | `query --format json` |
| `QueryService/RunQueries` | `query --set - --format json`, the set sent on stdin |
| `DiffService/DiffDesigns` | `diff --format json` |
| `TableService/Tabulate` | `tabulate -`, the request sent on stdin |
| `ReviewService/ListChecklists` | `checklists --format json` |
| `DesignService/TraceDesign` | `trace --format json` |
| `DesignService/GetLayoutReport` | `render --report --report-format json` |

Any other rpc raises `CliUnsupported`. So does a request field the command has no flag for, because
dropping it would answer a different question from the one asked. The one part of an `overlay` the
CLI sends is a library: a query or a set whose overlay carries only `library_modules` and
`library_docs` is written to a temporary directory and passed as `--lib`, so it answers as the same
request over Connect does (agni issue 788). `validate`
and `params` print a wire message with no rpc behind it, and `CliTransport.run` reads them. `intake`
is C31's declared exception and has no wire message. `create_review` runs `agni review`, sending the
manifest on stdin, and answers the same `Review` as the server except that it is unnamed, because the
CLI stores nothing (agni issue 734). Getting, listing and deleting stored reviews need a server. To
run a project's own checklist, ask `list_checklists(design_uri=…)` for the checklists its project
declares, inherited ones included, and send one's `manifest` to `create_review`; the first is the
project's default (agni issue 859).

## Where the two transports differ today

`clients/python/tests/test_cross_transport.py` sends the same request both ways and asserts the two
messages are equal. That test is C31 checked from outside Go. Two known differences are declared in
it field by field, and each declaration fails once the difference goes away:

- `CreateReview` over the CLI has an empty `name`, because the CLI stores nothing. The two runs also
  differ in `results.meta.created_at`, which the test clears on both sides before comparing.
- `GetLayoutReport` on a design FOLDER answers an empty report from the server (agni issue 736).
  Name the entry file with `as_named` until that lands.

## Tables and workbooks

A table has two halves, and agni owns the first. Projecting an answer into rows (which columns, how
a subject is spelled, how rows order) is the engine's call, made once and served by
`TableService.Tabulate`. Writing the rows as a workbook is the client's. So a client never lays an
answer out itself: `client.tabulate(check=…)`, `query=…`, `query_set=…`, `diff=…` or `review=…`
returns the engine's tables, which are the rows the matching `agni … --format csv` prints for the
same answer, and C35 holds the two together. A check run gives `findings`, and when the response
carries verdicts, `verdicts` and a per-rule `verdicts_by_rule`. A diff gives one `diff` table, a row
per change with `change_class` naming its kind, and a review gives `review`, one row per item in the
checklist's order, and a one-row `review_summary`. `order_by` sorts them, as
`["rule", "-subject"]`, by each column's type, so a net or part column puts `R2` before `R10`, and
`column_types` overrides a column the projection could not type, such as a query's count.

`table_sheets(response, {"findings": "Findings"})` names each table as a sheet, and
`tables_to_xlsx(path, sheets)` writes one sheet per table with a bold header row, a frozen top row
and an autofilter. It keeps a cell that starts with `=` as text, since a cell is a name off a design
file and must not run as a formula. It needs the `xlsx` extra, and tab names, colours and highlighted
rows stay in the caller, which can reopen the file with openpyxl.
`clients/python/examples/audit_workbook.py` writes a five-sheet audit of the tutorial board this way. `clients/python/examples/revision_audit.py` is the full form: two revisions in one workbook, with
the audit questions in `revision_audit.yaml` rather than the script, and a committed summary of every
tab the client's tests compare on each run (agni issue 823).

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
