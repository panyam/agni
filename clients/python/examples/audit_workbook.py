"""Write a small netlist-audit workbook: a few query tables and the check findings, one sheet each.

    python examples/audit_workbook.py -o audit.xlsx
    python examples/audit_workbook.py -o audit.xlsx --server http://127.0.0.1:8080
    python examples/audit_workbook.py -o audit.xlsx --folder ~/boards/foo --design designs/foo

The tables go to agni as ONE query set, so the design is read once for all of them, whether the
transport is the CLI (no --server) or a server started with the same --mount. The layout of each
sheet (tab names, which tables) is this script's business; agni answers the questions, and
`set_sheets` and `tables_to_xlsx` lay each answer out as a sheet.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
from typing import List, Tuple

from agni import Client, CliTransport, ConnectTransport, set_sheets, tables_to_xlsx
from agni.v1.webapi import query_pb2

TUTORIAL = Path(__file__).resolve().parents[3] / "examples" / "tutorial-project"

TABLES: List[Tuple[str, str]] = [
    ("Pin to net", "pin.net(?c, ?p, ?n) => ?c, ?p, ?n"),
    ("Part numbers", "component.mpn(?c, ?mpn) => ?c, ?mpn"),
    ("Test points per net", "net.test_point_count(?n, ?c) => ?n, ?c order by ?c, ?n"),
    ("Nets per component", "component.net_count(?c, ?k) => ?c, ?k"),
]


def build(client: Client, design: str, out: str) -> List[str]:
    """Ask every table and the check, write the workbook, and return the sheet names written."""
    audit = query_pb2.QuerySet(
        title="Netlist audit",
        queries=[query_pb2.NamedQuery(name=name, query=q) for name, q in TABLES],
    )
    sheets = set_sheets(client.run_queries(uri=design, set=audit))
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
