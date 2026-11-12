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

Every release after v0.3.11 publishes it to PyPI as `agni-eda`, since PyPI's `agni` is an unrelated
project. The import is still `agni`. Install the version that matches the `agni` binary or server
you talk to, because both are built from one tag's protos.

```sh
pip install 'agni-eda[xlsx]==X.Y.Z'
```

Between releases, pip installs a commit or a branch straight from GitHub, and in a checkout an
editable install follows the tree.

```sh
pip install 'agni-eda[xlsx] @ git+https://github.com/panyam/agni@main#subdirectory=clients/python'
pip install -e 'clients/python[xlsx]'
```

The client does not carry the engine. `ConnectTransport` needs a running `agni serve`, which the
`ghcr.io/panyam/agni` image provides, and `CliTransport` needs an `agni` binary, from `go install
github.com/panyam/agni/cmd/agni@vX.Y.Z` or `make install` in a checkout.

A `Client` wraps a transport, and each method takes and returns the rpc's own messages:

```python
from agni import Client, CliTransport, ConnectTransport, rows_as_dicts

cli = Client(CliTransport("agni", mounts={"tut": "examples/tutorial-project"}))
srv = Client(ConnectTransport("http://127.0.0.1:8080"))

snap = cli.ingest("mount://tut/designs/gateway")
answer = cli.run_query(snapshot=snap, query='component.class(?c, "resistor") => ?c')
for row in rows_as_dicts(answer):
    print(row["c"])
cli.delete_snapshot(snap)
```

A script loads a design once with `ingest`. It takes the design's `mount://` URI and any
`board_uri`, `as_named`, `flatten_hierarchy` or library `overlay`, and it returns a snapshot. Every
later question names that snapshot (agni issue 1061). A server resolves the design's tiers and
config when it ingests and holds the snapshot until it is deleted, evicted or restarted. The client
keeps each snapshot's definition. Suppose a question names a snapshot the server lost. The client
ingests it again under the same name and retries the question once. If the snapshot's files
changed, the client updates it and retries once, as the viewer does (agni issue 1067).
`CliTransport` records the definition and mints a name, since each `agni` command reads the design
itself. It also turns its `mounts` into `--mount` flags. A server has to be started with the same
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
client.run_query(snapshot=snap, query="component.net(?ref, ?net) => ?net",
                 bindings=agni.bindings({"ref": "U1"}))
```

The query text is the same for every part, and the value needs no escaping. Over the CLI transport
the bindings travel as `--bind`, and a set's per-query bindings as the set file's `bind:` maps.

### Reading the design itself

When a question is not yet a query, a check or a diff, a script can read the design's IR and work on
it directly, prototyping an analysis before it moves into agni as a library relation or a rule.
`get_design` returns the IR when a `read_mask` asks for it, and returns only the parts asked for:

```python
d = client.get_design(snapshot=snap,
                      read_mask={"paths": ["design.nets", "design.components.mpn"]},
                      nets=["PMIC_EN"]).design
for net in d.nets:
    print(net.name, [f"{c.component_ref}.{c.pin_ref}" for c in net.connections])
```

The messages are `agni.v1.ir` types. Unmasked, `get_design` still returns only the summary, because a
large board's IR runs to megabytes. `nets` and `ref_des` narrow which entities come back. Over the
CLI transport the same request runs `agni stats --format json --mask ...`.

## Choosing a transport

**The CLI transport** needs no server and ships as one binary. Every call starts a process and reads
the design again, and on a large board that read is the slow part. `run_queries` asks many queries
in one call, so over either transport a whole audit costs one read.

**The Connect transport** posts to `/agni.v1.webapi.<Service>/<Method>`, in binary protobuf by
default and in JSON with `ConnectTransport(url, binary=False)`. The message is the same either way.
Binary is the default because a large answer is about half the bytes and decodes about a hundred
times faster in Python. In binary a 9,323-row query answer is 8.3 MB and 4 ms, against 15.5 MB and
0.43 s as JSON. The server reads a design once and answers many questions about it. `agni serve`
with no web dir serves the API alone, so an installed binary needs no viewer assets for this.

A query's answer lists each entity once, in `entities`, however many cells name it. Each entry
carries the sheets where the entity is drawn and the reason when it cannot be located. The answer
also lists each citation once, in `sources`. Each row points into both (`cell_entity`,
`cite_index`), so a net named on a thousand rows is sent once (agni issue 916). A script that draws
nothing can pass `omit_locations=True` on `run_query` or `run_queries`, or `--omit-locations` on
the CLI. The answer then leaves out the sheets and reasons, and the engine skips loading the
drawing. The entities, cells and citations are unchanged.

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

Any other rpc raises `CliUnsupported`. So does a request field with no flag on the command, because
dropping the field would answer a different question from the one asked. The CLI sends only one part
of an `overlay`, the library. Suppose a query or a set has an overlay that carries only
`library_modules` and `library_docs`. The CLI writes that library to a temporary directory and passes
it as `--lib`, so the request answers as it does over Connect (agni issue 788). `validate`
and `params` print a wire message with no rpc behind it, and `CliTransport.run` reads them. `intake`
is C31's declared exception and has no wire message. `create_review` runs `agni review` and sends the
manifest on stdin. It answers the same `Review` as the server, except that the review is unnamed,
because the CLI stores nothing (agni issue 734). Getting, listing and deleting stored reviews need a
server. To run a project's own checklist, call `list_checklists(snapshot=…)`. It returns the
project's declared checklists, inherited ones included, and the first is the project's default. Send
one checklist's `manifest` to `create_review` (agni issue 859).

### When a request fails

A failed rpc answers with a Connect error, a `code`, a `message` and a list of `details`. A
`RunQuery` whose query does not parse fails with `invalid_argument` and carries one detail, an
`agni.v1.webapi.TextPosition` saying where in the query's text it stopped. Its `offset` counts bytes,
and its `line` and `column` count from 1, the column in characters as an editor counts it. A `RunQueries` answer does not
fail for one bad query. That query's `NamedQueryResult` carries the reason in `error`. When the query
did not parse, `error_at` carries the place. The rest of the set still answers.

The CLI writes the same error. Suppose a command runs with `--format json` and its request fails.
The command writes the error as the Connect protocol encodes it, to stdout or to `-o`'s file. It
still exits non-zero with the text error on stderr, so a script reads one JSON shape whichever
transport it used. A mistake in the command's own flags makes no request, so its error goes to
stderr as text alone. The
[CLI reference](../../guide/cli-reference/) shows an example, and CONSTRAINTS C31 is the rule.

## Where the two transports differ today

`clients/python/tests/test_cross_transport.py` sends the same request both ways and asserts the two
messages are equal. That test is C31 checked from outside Go. Two known differences are declared in
it field by field, and each declaration fails once the difference goes away:

- `CreateReview` over the CLI has an empty `name`, because the CLI stores nothing. The two runs also
  differ in `results.meta.created_at`, which the test clears on both sides before comparing.
- `GetLayoutReport` on a design FOLDER answers an empty report from the server (agni issue 736).
  Ingest the entry file with `as_named` until that is fixed.

## Tables and workbooks

A table has two halves, and agni owns the first. The first half projects an answer into rows. It
decides which columns there are, how a subject is spelled and how rows order. The engine makes that
call once and serves it through `TableService.Tabulate`. The second half writes the rows as a
workbook, and the client owns it. So a client never lays an answer out itself. It calls
`client.tabulate(check=…)`, or passes `query=…`, `query_set=…`, `diff=…` or `review=…`, and gets the
engine's tables back. They hold the same rows the matching `agni … --format csv` prints for that
answer, and C35 holds the two together. A check run gives `findings`. When the response carries
verdicts, it also gives `verdicts` and a per-rule `verdicts_by_rule`. A diff gives one `diff` table,
with a row per change and `change_class` naming its kind. A review gives `review`, one row per item
in the checklist's order, and a one-row `review_summary`. `order_by` sorts the tables by each
column's type, as in `["rule", "-subject"]`, so a net or part column puts `R2` before `R10`.
`column_types` sets a column's type when the projection could not type it, such as a query's count.

`table_sheets(response, {"findings": "Findings"})` names each table as a sheet, and
`tables_to_xlsx(path, sheets)` writes one sheet per table with a bold header row, a frozen top row
and an autofilter. It keeps a cell that starts with `=` as text, since a cell is a name off a design
file and must not run as a formula. It needs the `xlsx` extra. Tab names, colours and highlighted
rows are left to the caller, which can reopen the file with openpyxl.
`clients/python/examples/audit_workbook.py` writes a five-sheet audit of the tutorial board this way.
`clients/python/examples/revision_audit.py` is the full form. It puts two revisions in one workbook
and reads the audit questions from `revision_audit.yaml` rather than the script. A committed summary
records every tab, and the client's tests compare against it on each run (agni issue 823).

The engine has no xlsx writer, on purpose. A workbook is a zip of cross-referencing XML parts. Its
layout belongs to whoever reads it, so the writer belongs in the client.

## Another language

Everything the Python client does is available to any language with a protobuf runtime:

1. Generate message classes from `protos/agni/v1` with that language's protoc plugin. Pin the plugin
   by version, the way `clients/python/buf.gen.py.yaml` does.
2. Read a response with the runtime's protojson parser. The CLI's stdout and the server's response
   body are both protojson of the same message.
3. Post to a server with `Content-Type: application/json`. The Connect protocol's unary form needs no
   Connect library.

A generated client committed to this repo belongs in `proto-check`, as the Go, TypeScript and Python
halves are, so that the gate fails on a proto change that forgets it.
