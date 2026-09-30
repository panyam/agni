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
resp = c.run_query(uri="mount://tut/designs/gateway", query='component.class(?c, "resistor") => ?c')
```

The guide is `docsite/content/build/other-languages.md`: choosing a transport, which rpcs the CLI
covers, where the two differ today, and writing tables to xlsx.

- `examples/audit_workbook.py` writes a five-sheet audit workbook of the tutorial board.
- `make proto-py` regenerates `src/agni/v1`. Never edit it by hand; `make proto-check` fails on drift.
- `make python-test` runs the suite against a built binary and a real server.
