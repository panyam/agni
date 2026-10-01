from __future__ import annotations

import openpyxl
import pytest

from agni import rows_as_dicts, set_sheets, tables_to_xlsx, to_rows
from agni.tables import FINDING_COLUMNS
from agni.v1.checks import checks_pb2 as checks
from agni.v1.webapi import checks_pb2, query_pb2


def _answer() -> query_pb2.RunQueryResponse:
    return query_pb2.RunQueryResponse(
        columns=["c", "n"],
        rows=[query_pb2.QueryRow(cells=["R1", "10"]), query_pb2.QueryRow(cells=["R2", "007"])],
    )


def test_rows_as_dicts_keeps_cells_as_sent():
    assert rows_as_dicts(_answer()) == [{"c": "R1", "n": "10"}, {"c": "R2", "n": "007"}]


def test_findings_flatten_one_per_row():
    resp = checks_pb2.CheckDesignResponse(
        findings=[
            checks.Finding(rule="r", severity="error", subject=checks.Subject(kind="net", ref="N1"), message="m", sheets=["/", "/a"])
        ]
    )
    header, rows = to_rows(resp)
    assert header == FINDING_COLUMNS
    assert rows == [["r", "error", "net", "N1", "m", "", "/ /a"]]


def test_xlsx_has_header_freeze_and_filter(tmp_path):
    out = tmp_path / "audit.xlsx"
    tables_to_xlsx(str(out), [("Resistors", _answer()), ("Plain", (["a"], [["1"], ["2"], ["3"]]))])
    wb = openpyxl.load_workbook(out)
    assert wb.sheetnames == ["Resistors", "Plain"]
    ws = wb["Resistors"]
    assert [c.value for c in ws[1]] == ["c", "n"]
    assert ws["A1"].font.bold
    assert ws.freeze_panes == "A2"
    assert ws.auto_filter.ref == "A1:B3"
    # "007" stays a string: a cell is what agni sent, not a guess at its type.
    assert ws["B3"].value == "007"
    assert wb["Plain"].auto_filter.ref == "A1:A4"


@pytest.mark.parametrize("name", ["", "a" * 32, "a/b", "x[1]"])
def test_xlsx_refuses_a_sheet_name_excel_rejects(tmp_path, name):
    with pytest.raises(ValueError):
        tables_to_xlsx(str(tmp_path / "x.xlsx"), [(name, _answer())])


def test_xlsx_refuses_a_name_used_twice(tmp_path):
    with pytest.raises(ValueError, match="twice"):
        tables_to_xlsx(str(tmp_path / "x.xlsx"), [("Nets", _answer()), ("nets", _answer())])


def _set_answer(with_error: bool) -> query_pb2.RunQueriesResponse:
    res = [
        query_pb2.NamedQueryResult(name="first", result=_answer()),
        query_pb2.NamedQueryResult(name="second", result=_answer()),
    ]
    if with_error:
        res.insert(1, query_pb2.NamedQueryResult(name="broken", error="unknown relation"))
    return query_pb2.RunQueriesResponse(title="t", results=res)


def test_set_sheets_keeps_the_set_order():
    sheets = set_sheets(_set_answer(False))
    assert [n for n, _ in sheets] == ["first", "second"]
    assert to_rows(sheets[0][1]) == to_rows(_answer())


def test_set_sheets_refuses_to_drop_an_unanswered_query():
    with pytest.raises(ValueError, match="broken: unknown relation"):
        set_sheets(_set_answer(True))
    assert [n for n, _ in set_sheets(_set_answer(True), allow_missing=True)] == ["first", "second"]
