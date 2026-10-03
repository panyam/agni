"""Answers as rows, and rows as a workbook.

A query answer is ``repeated QueryRow { repeated string cells }`` plus ``column_kinds``, so its cells
are strings and stay strings here. The kinds say what a column NAMES (a net, a component), not what
type its values are, and converting ``"10k"`` or ``"R12"`` on a guess would be wrong half the time.
"""

from __future__ import annotations

import re
from typing import Dict, Iterable, List, Mapping, Optional, Sequence, Tuple, Union

from agni.v1.webapi import query_pb2, tables_pb2

Rows = Tuple[List[str], List[List[str]]]
Table = Union[tables_pb2.Table, Rows]


def rows_as_dicts(resp: query_pb2.RunQueryResponse) -> List[Dict[str, str]]:
    """Each row as ``{column: cell}``. Cells are the strings agni sent."""
    return [dict(zip(resp.columns, row.cells)) for row in resp.rows]


def to_rows(table: Table) -> Rows:
    """A header and its rows, from a table agni projected or a ``(header, rows)`` pair already.

    Rows are agni's (``Client.tabulate``), so this reads them and never decides them: a workbook of
    an answer carries the cells ``agni check --format csv`` prints for it (agni issue 862).
    """
    if isinstance(table, tables_pb2.Table):
        return [c.name for c in table.columns], [list(r.cells) for r in table.rows]
    if isinstance(table, tuple) and len(table) == 2:
        header, rows = table
        return [str(h) for h in header], [[str(c) for c in r] for r in rows]
    raise TypeError(f"cannot tabulate a {type(table).__name__}; ask agni for its tables with Client.tabulate")


def table_sheets(resp: tables_pb2.TabulateResponse, names: Optional[Mapping[str, str]] = None) -> List[Tuple[str, Table]]:
    """One ``(sheet name, table)`` per table agni returned, in its order, for ``tables_to_xlsx``.

    ``names`` renames a table's sheet (``{"findings": "Findings"}``); a table it does not name keeps
    agni's name. Keep names within Excel's 31 characters.
    """
    names = names or {}
    return [(names.get(t.name, t.name), t) for t in resp.tables]


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
            # openpyxl reads a string starting with "=" as a formula, and a cell here is a name off a
            # design file, so every cell is kept as the text it is.
            for cell in ws[ws.max_row]:
                if cell.data_type == "f":
                    cell.data_type = "s"
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
