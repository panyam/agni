"""Audit a revision: compare two revisions of a design and audit the newer one, in one workbook.

    python examples/revision_audit.py -o /tmp/revision-audit.xlsx
    python examples/revision_audit.py -o /tmp/revision-audit.xlsx --server http://127.0.0.1:8080

This is the exercise of agni issue 843, and `make exercise-revision-audit` runs it. Beside the xlsx it
writes a markdown summary with each tab's row count and first rows. A tab whose helper has not landed
yet is listed as `missing:` with the issue that fills it, so the summary shows how far the mission has
got. agni issue 823 turns this into the finished example.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
from typing import Callable, List, Tuple

from agni import Client, CliTransport, ConnectTransport, diff_sheets, set_sheets, tables_to_xlsx, verdict_sheets
from agni.tables import Rows, to_rows
from agni.v1.webapi import query_pb2

from audit_workbook import TABLES, TUTORIAL

PREVIEW = 5


class Missing(Exception):
    """A tab that cannot be built yet. The message names the issue that fills it."""


def missing(why: str) -> Callable[[], List[Tuple[str, Rows]]]:
    def build() -> List[Tuple[str, Rows]]:
        raise Missing(why)

    return build


def plan(client: Client, base: str, head: str) -> List[Tuple[str, Callable[[], List[Tuple[str, Rows]]]]]:
    """Every tab group the finished workbook holds, in order, each built lazily."""

    def audit():
        audit = query_pb2.QuerySet(
            title="Revision audit",
            queries=[query_pb2.NamedQuery(name=name, query=q) for name, q in TABLES],
        )
        return [(name, to_rows(t)) for name, t in set_sheets(client.run_queries(uri=head, set=audit))]

    # One check answers both the findings and the verdicts tabs.
    check = None

    def checked():
        nonlocal check
        if check is None:
            check = client.check_design(uri=head)
        return check

    return [
        ("Diff", lambda: diff_sheets(client.diff_designs(a_uri=base, b_uri=head, include_equal=True))),
        ("Audit", audit),
        ("Review", missing("#859 (ListChecklists, so the review runs the project's own checklist)")),
        ("Findings", lambda: [("Findings", to_rows(checked()))]),
        ("Verdicts", lambda: verdict_sheets(checked())),
    ]


def summary(base: str, head: str, built: List[Tuple[str, Rows]], absent: List[Tuple[str, str]]) -> str:
    lines = ["# Revision audit", "", f"- base: `{base}`", f"- head: `{head}`", ""]
    for name, (header, rows) in built:
        lines += [f"## {name}", "", f"{len(rows)} rows", ""]
        if rows:
            lines.append("| " + " | ".join(header) + " |")
            lines.append("|" + "---|" * len(header))
            lines += ["| " + " | ".join(c.replace("|", "\\|") for c in r) + " |" for r in rows[:PREVIEW]]
            lines.append("")
    for name, why in absent:
        lines += [f"## {name}", "", f"missing: {why}", ""]
    return "\n".join(lines)


def build(client: Client, base: str, head: str, out: str) -> Tuple[List[str], List[str]]:
    """Write the workbook and its summary. Returns the tabs written and the tab groups still missing."""
    built: List[Tuple[str, Rows]] = []
    absent: List[Tuple[str, str]] = []
    for group, make in plan(client, base, head):
        try:
            built += make()
        except Missing as why:
            absent.append((group, str(why)))
    Path(out).parent.mkdir(parents=True, exist_ok=True)
    tables_to_xlsx(out, built)
    Path(out).with_suffix(".md").write_text(summary(base, head, built, absent))
    return [name for name, _ in built], [name for name, _ in absent]


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("-o", "--out", required=True, help="the .xlsx to write; the summary goes beside it as .md")
    ap.add_argument("--folder", default=str(TUTORIAL), help="the folder to mount (default: the tutorial project)")
    ap.add_argument("--base", default="designs/gateway/gateway.edn", help="the earlier revision, relative to --folder")
    ap.add_argument("--head", default="designs/gateway/gateway-rev-b.edn", help="the revision to audit, relative to --folder")
    ap.add_argument("--server", help="a running `agni serve` to ask instead of the CLI")
    ap.add_argument("--agni", default=os.environ.get("AGNI_BIN", "agni"), help="the agni binary for the CLI transport")
    args = ap.parse_args(argv)

    transport = (
        ConnectTransport(args.server)
        if args.server
        else CliTransport(args.agni, mounts={"audit": args.folder})
    )
    tabs, absent = build(Client(transport), f"mount://audit/{args.base}", f"mount://audit/{args.head}", args.out)
    print(f"wrote {args.out}: {', '.join(tabs)}")
    print(f"wrote {Path(args.out).with_suffix('.md')}")
    if absent:
        print(f"missing: {', '.join(absent)}")


if __name__ == "__main__":
    main()
