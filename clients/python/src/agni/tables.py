"""Answers as rows, and rows as a workbook.

A query answer is ``repeated QueryRow { repeated string cells }`` plus ``column_kinds``, so its cells
are strings and stay strings here. The kinds say what a column NAMES (a net, a component), not what
type its values are, and converting ``"10k"`` or ``"R12"`` on a guess would be wrong half the time.
"""

from __future__ import annotations

import re
from typing import Dict, Iterable, List, Sequence, Tuple, Union

from agni.v1.webapi import checks_pb2, query_pb2

Rows = Tuple[List[str], List[List[str]]]
Table = Union[query_pb2.RunQueryResponse, checks_pb2.CheckDesignResponse, Rows]

FINDING_COLUMNS = ["rule", "severity", "subject_kind", "subject", "message", "inconclusive", "sheets"]


def rows_as_dicts(resp: query_pb2.RunQueryResponse) -> List[Dict[str, str]]:
    """Each row as ``{column: cell}``. Cells are the strings agni sent."""
    return [dict(zip(resp.columns, row.cells)) for row in resp.rows]


def to_rows(table: Table) -> Rows:
    """A header and its rows, from a query answer, a check response's findings, or a pair already.

    A check response flattens one finding per row over ``FINDING_COLUMNS``. Verdicts and skipped
    rules are not rows of that table and are left out; flatten them yourself if you want them.
    """
    if isinstance(table, query_pb2.RunQueryResponse):
        return list(table.columns), [list(r.cells) for r in table.rows]
    if isinstance(table, checks_pb2.CheckDesignResponse):
        rows = [
            [
                f.rule,
                f.severity,
                f.subject.kind,
                f.subject.ref,
                f.message,
                "yes" if f.inconclusive else "",
                " ".join(f.sheets),
            ]
            for f in table.findings
        ]
        return list(FINDING_COLUMNS), rows
    if isinstance(table, tuple) and len(table) == 2:
        header, rows = table
        return [str(h) for h in header], [[str(c) for c in r] for r in rows]
    raise TypeError(f"cannot tabulate a {type(table).__name__}")


# Excel refuses these in a sheet name and caps the name at 31 characters.
_BAD_SHEET = re.compile(r"[\[\]:*?/\\]")


def tables_to_xlsx(path: str, sheets: Sequence[Tuple[str, Table]]) -> None:
    """Write one sheet per ``(name, table)``: a bold header row, frozen, with an autofilter.

    Needs the ``xlsx`` extra (``pip install agni[xlsx]``). Styling beyond that belongs to the caller,
    which can reopen the file with openpyxl. A sheet name Excel would reject raises ``ValueError``
    rather than being quietly rewritten, since a caller referring to a tab by name would then miss it.
    """
    try:
        from openpyxl import Workbook
        from openpyxl.styles import Font
        from openpyxl.utils import get_column_letter
    except ImportError as e:  # pragma: no cover - exercised only without the extra
        raise ImportError("tables_to_xlsx needs openpyxl: pip install 'agni[xlsx]'") from e

    _check_names([name for name, _ in sheets])
    wb = Workbook()
    wb.remove(wb.active)
    bold = Font(bold=True)
    for name, table in sheets:
        header, rows = to_rows(table)
        ws = wb.create_sheet(title=name)
        ws.append(header)
        for cell in ws[1]:
            cell.font = bold
        for r in rows:
            ws.append(r)
        ws.freeze_panes = "A2"
        if header:
            ws.auto_filter.ref = f"A1:{get_column_letter(len(header))}{len(rows) + 1}"
    wb.save(path)


def _check_names(names: Iterable[str]) -> None:
    seen = set()
    for n in names:
        if not n or len(n) > 31 or _BAD_SHEET.search(n):
            raise ValueError(f"sheet name {n!r}: Excel needs 1-31 characters and none of []:*?/\\")
        if n.lower() in seen:
            raise ValueError(f"sheet name {n!r} is used twice (Excel compares names case-insensitively)")
        seen.add(n.lower())
