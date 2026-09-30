# netlist-audit

A netlist audit written as a **query set**: the tables a review workbook holds, as named queries in
one file, answered over one read of the design.

## What it shows

`audit.yaml` holds nine queries and the derived relations they share:

| table | what it lists |
|---|---|
| Parts on nets | every part and the nets it lands on |
| Part numbers | every part's MPN |
| Net count | how many nets the design has |
| Test points per net | nets carrying a test point, and which |
| Nets with no test point | the nets a probe cannot reach |
| Ground test points | test points on ground, for a house ratio |
| Passives probed on both nets | parts an in-circuit tester can measure |
| Passives probed on one net | the probed net and the one missing a test point |
| MPNs never probed on both nets | part numbers with no measurable instance |

The same file runs from the command line, where it writes one document with a section per query:

```
agni query <design> --set audit.yaml --format markdown
```

and from the Python client, where `clients/python/examples/audit_workbook.py` sends its tables as one
set and writes a spreadsheet sheet per query.

## Run it

```
make run         # plain text, prompts for the design
make runquiet    # non-interactive defaults (CI-safe)
make demo        # TUI boxes
```

`AGNI_EXAMPLE_DESIGN=/path/to/board.tel make run` points it at your own board.

## How it is built

`main.go` parses the embedded `audit.yaml`, builds ONE fact base over the design, and answers each
query against it. The bundled fixture, `../common/designs/netlist-audit.tel`, is synthetic and gives
every table at least one row.
