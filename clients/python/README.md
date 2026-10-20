# agni (Python client)

A typed Python client for agni. It calls the `agni` CLI or a running `agni serve` and returns the
wire contract's own message classes, generated from `protos/agni/v1`.

```sh
pip install -e 'clients/python[xlsx]'
```

```python
from agni import Client, CliTransport, ConnectTransport

c = Client(CliTransport("agni", mounts={"tut": "examples/tutorial-project"}))
# or: c = Client(ConnectTransport("http://127.0.0.1:8080"))
snap = c.ingest("mount://tut/designs/gateway")
resp = c.run_query(snapshot=snap, query='component.class(?c, "resistor") => ?c')
c.delete_snapshot(snap)
```

Every question names a snapshot, a design loaded once under one config (agni issue 1061). `ingest`
returns its name, and takes the `board_uri`, `as_named` and `overlay` a request used to carry, and
`flatten_hierarchy`, which reads a hierarchical EDIF netlist whole as `--flatten-hierarchy` does.
A server holds a snapshot until it is deleted or evicted, and a question naming one it no longer holds
raises `AgniError` with code `not_found`, so a long-running script ingests again. The CLI transport has
no server, so it keeps each definition itself under a name it mints (`snapshots/cli-1`) and turns it
back into flags for every command it runs.

The guide, `docsite/content/build/other-languages.md`, covers choosing a transport, which rpcs the
CLI covers, where the two differ today, and writing tables to xlsx.

- `examples/audit_workbook.py` writes a five-sheet audit workbook of the tutorial board, asking its
  four tables as one query set (`run_queries`) so the design is read once.
- `examples/revision_audit.py` compares two revisions of the gateway and audits the newer one, writing
  a workbook and a markdown summary that lists each tab still to be built. `make
  exercise-revision-audit` runs it with the checkout's binary (agni issue 843).
- `make proto-py` regenerates `src/agni/v1`. Never edit it by hand; `make proto-check` fails on drift.
- `make python-test` runs the suite against a built binary and a real server.
