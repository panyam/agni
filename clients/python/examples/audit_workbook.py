"""Write a small netlist-audit workbook: a few query tables and the check findings, one sheet each.

    python examples/audit_workbook.py -o audit.xlsx
    python examples/audit_workbook.py -o audit.xlsx --server http://127.0.0.1:8080
    python examples/audit_workbook.py -o audit.xlsx --folder ~/boards/foo --design designs/foo

With no --server it runs the agni CLI once per table. With one, it asks that server, which reads the
design once for every table, and the server must have been started with the same --mount. The layout
of each sheet (tab names, which tables) is this script's business; agni answers the questions and
`tables_to_xlsx` lays each answer out as a sheet.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
from typing import List, Tuple

from agni import Client, CliTransport, ConnectTransport, tables_to_xlsx

TUTORIAL = Path(__file__).resolve().parents[3] / "examples" / "tutorial-project"

TABLES: List[Tuple[str, str]] = [
    ("Pin to net", "pin.net(?c, ?p, ?n) => ?c, ?p, ?n"),
    ("Part numbers", "component.mpn(?c, ?mpn) => ?c, ?mpn"),
    (
        "Test points per net",
        'entity(?n, "net"), component-on-net(?tp, ?n), component.class(?tp, "test_point")'
        " => ?n, count(distinct ?tp), list(distinct ?tp)",
    ),
    ("Nets per component", "component.net_count(?c, ?k) => ?c, ?k"),
]


def build(client: Client, design: str, out: str) -> List[str]:
    """Ask every table and the check, write the workbook, and return the sheet names written."""
    sheets = [(name, client.run_query(uri=design, query=q)) for name, q in TABLES]
    sheets.append(("Findings", client.check_design(uri=design)))
    tables_to_xlsx(out, sheets)
    return [name for name, _ in sheets]


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("-o", "--out", required=True, help="the .xlsx to write")
    ap.add_argument("--folder", default=str(TUTORIAL), help="the folder to mount (default: the tutorial project)")
    ap.add_argument("--design", default="designs/gateway", help="the design, relative to --folder")
    ap.add_argument("--server", help="a running `agni serve` to ask instead of the CLI")
    ap.add_argument("--agni", default=os.environ.get("AGNI_BIN", "agni"), help="the agni binary for the CLI transport")
    args = ap.parse_args(argv)

    transport = (
        ConnectTransport(args.server)
        if args.server
        else CliTransport(args.agni, mounts={"audit": args.folder})
    )
    names = build(Client(transport), f"mount://audit/{args.design}", args.out)
    print(f"wrote {args.out}: {', '.join(names)}")


if __name__ == "__main__":
    main()
