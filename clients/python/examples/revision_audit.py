"""Audit a revision: compare two revisions of a design and audit the newer one, in one workbook.

    python examples/revision_audit.py -o /tmp/revision-audit.xlsx
    python examples/revision_audit.py -o /tmp/revision-audit.xlsx --server http://127.0.0.1:8080

The workbook holds the diff between the two revisions, the audit tables revision_audit.yaml asks of
the newer one, the project's own review checklist, and the check run's findings and verdicts. Every
judgement in it lives in that YAML, in the project's checklist and library, or in the engine; this
script only asks for the answers and lays out the tables agni projects (agni issues 823, 862).

Beside the xlsx it writes a markdown summary of every tab: its name, row count and first rows.
`make exercise-revision-audit` runs it, and the summary committed beside this script is regenerated
and compared by the client's tests.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
from typing import List, Tuple

import yaml
from google.protobuf import json_format

from agni import Client, CliTransport, ConnectTransport, table_sheets, tables_to_xlsx
from agni.tables import Rows, to_rows
from agni.v1.webapi import query_pb2, tables_pb2

TUTORIAL = Path(__file__).resolve().parents[3] / "examples" / "tutorial-project"
AUDIT_SET = Path(__file__).with_name("revision_audit.yaml")
PREVIEW = 5

# How the audit tables sort, by the columns revision_audit.yaml names. agni types a part, pin or net
# column as a name, so R2 comes before R10 and U3's pin 2 before its pin 10.
ORDER_BY = ["part", "pin", "net", "mpn"]


def load_set(path: Path) -> query_pb2.QuerySet:
    """The query set in the YAML file, the shape `agni query --set` reads."""
    return json_format.ParseDict(yaml.safe_load(path.read_text()), query_pb2.QuerySet())


def tabs(client: Client, base: str, head: str, audit: query_pb2.QuerySet) -> List[Tuple[str, Rows]]:
    """Every tab of the workbook, in order, each from a table agni projected."""
    out: List[Tuple[str, Rows]] = []

    def add(resp: tables_pb2.TabulateResponse, names=None) -> None:
        out.extend((name, to_rows(t)) for name, t in table_sheets(resp, names))

    diff = client.diff_designs(a_uri=base, b_uri=head, include_equal=True)
    add(client.tabulate(diff=diff), {"diff": "Diff"})

    answers = client.run_queries(uri=head, set=audit)
    add(client.tabulate(query_set=answers, order_by=ORDER_BY))

    # The project's own default checklist, the first one it declares, run on the newer revision.
    lists = client.list_checklists(design_uri=head)
    if not lists.checklists:
        raise SystemExit(f"{head} belongs to no project with a checklist, so there is nothing to review")
    review = client.create_review(design_uri=head, manifest=lists.checklists[0].manifest)
    add(client.tabulate(review=review), {"review": "Review", "review_summary": "Review summary"})

    # Skipped rules sit beside the findings, so a reader sees which rules the run was silent on.
    checked = {t.name: t for t in client.tabulate(check=client.check_design(uri=head)).tables}
    for table, tab in [("findings", "Findings"), ("skipped", "Skipped rules"), ("verdicts", "Verdicts"), ("verdicts_by_rule", "Verdicts by rule")]:
        out.append((tab, to_rows(checked[table])))
    return out


def summary(base: str, head: str, built: List[Tuple[str, Rows]]) -> str:
    """Each tab's name, row count and first rows, as markdown."""
    lines = ["# Revision audit", "", f"- base: `{base}`", f"- head: `{head}`", ""]
    for name, (header, rows) in built:
        lines += [f"## {name}", "", f"{len(rows)} rows", ""]
        if rows:
            lines.append("| " + " | ".join(header) + " |")
            lines.append("|" + "---|" * len(header))
            lines += ["| " + " | ".join(c.replace("|", "\\|") for c in r) + " |" for r in rows[:PREVIEW]]
            lines.append("")
    return "\n".join(lines)


def build(client: Client, base: str, head: str, out: str, audit_set: Path = AUDIT_SET) -> List[str]:
    """Write the workbook and its summary, and return the tabs written."""
    built = tabs(client, base, head, load_set(audit_set))
    Path(out).parent.mkdir(parents=True, exist_ok=True)
    tables_to_xlsx(out, built)
    Path(out).with_suffix(".md").write_text(summary(base, head, built))
    return [name for name, _ in built]


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("-o", "--out", required=True, help="the .xlsx to write; the summary goes beside it as .md")
    ap.add_argument("--folder", default=str(TUTORIAL), help="the folder to mount (default: the tutorial project)")
    ap.add_argument("--base", default="designs/gateway/gateway.edn", help="the earlier revision, relative to --folder")
    ap.add_argument("--head", default="designs/gateway/gateway-rev-b.edn", help="the revision to audit, relative to --folder")
    ap.add_argument("--set", default=str(AUDIT_SET), help="the audit query set (default: revision_audit.yaml)")
    ap.add_argument("--server", help="a running `agni serve` to ask instead of the CLI")
    ap.add_argument("--agni", default=os.environ.get("AGNI_BIN", "agni"), help="the agni binary for the CLI transport")
    args = ap.parse_args(argv)

    transport = ConnectTransport(args.server) if args.server else CliTransport(args.agni, mounts={"audit": args.folder})
    tabs_written = build(Client(transport), f"mount://audit/{args.base}", f"mount://audit/{args.head}", args.out, Path(args.set))
    print(f"wrote {args.out}: {', '.join(tabs_written)}")
    print(f"wrote {Path(args.out).with_suffix('.md')}")


if __name__ == "__main__":
    main()
