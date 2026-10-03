from __future__ import annotations

import openpyxl
import pytest

from agni import rows_as_dicts, table_sheets, tables_to_xlsx, to_rows
from agni.v1.checks import checks_pb2 as checks
from agni.v1.webapi import checks_pb2, query_pb2, tables_pb2


def _answer() -> query_pb2.RunQueryResponse:
    return query_pb2.RunQueryResponse(
        columns=["c", "n"],
        rows=[query_pb2.QueryRow(cells=["R1", "10"]), query_pb2.QueryRow(cells=["R2", "007"])],
    )


def test_rows_as_dicts_keeps_cells_as_sent():
    assert rows_as_dicts(_answer()) == [{"c": "R1", "n": "10"}, {"c": "R2", "n": "007"}]


def _table(name="t") -> tables_pb2.Table:
    return tables_pb2.Table(
        name=name,
        columns=[tables_pb2.TableColumn(name="c"), tables_pb2.TableColumn(name="n")],
        rows=[tables_pb2.TableRow(cells=["R1", "10"]), tables_pb2.TableRow(cells=["R2", "007"])],
    )


def test_to_rows_reads_an_engine_table_and_refuses_an_answer():
    assert to_rows(_table()) == (["c", "n"], [["R1", "10"], ["R2", "007"]])
    with pytest.raises(TypeError, match="tabulate"):
        to_rows(checks_pb2.CheckDesignResponse())


def test_table_sheets_keep_order_and_rename():
    resp = tables_pb2.TabulateResponse(tables=[_table("findings"), _table("verdicts")])
    assert [n for n, _ in table_sheets(resp, {"findings": "Findings"})] == ["Findings", "verdicts"]


def test_xlsx_has_header_freeze_and_filter(tmp_path):
    out = tmp_path / "audit.xlsx"
    tables_to_xlsx(str(out), [("Resistors", _table()), ("Plain", (["a"], [["1"], ["2"], ["3"]]))])
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
        tables_to_xlsx(str(tmp_path / "x.xlsx"), [(name, _table())])


def test_xlsx_refuses_a_name_used_twice(tmp_path):
    with pytest.raises(ValueError, match="twice"):
        tables_to_xlsx(str(tmp_path / "x.xlsx"), [("Nets", _table()), ("nets", _table())])


def test_xlsx_keeps_a_formula_shaped_cell_as_text(tmp_path):
    out = tmp_path / "f.xlsx"
    tables_to_xlsx(str(out), [("Nets", (["net"], [["=HYPERLINK(\"x\")"], ["+5V"]]))])
    ws = openpyxl.load_workbook(out)["Nets"]
    assert ws["A2"].data_type == "s" and ws["A2"].value == '=HYPERLINK("x")'
    assert ws["A3"].value == "+5V"


# ---- natural order, and the diff, review and verdict sheets (agni issue 822) ----

from agni.tables import diff_sheets, review_sheet  # noqa: E402
from agni.v1.webapi import diff_pb2, review_pb2  # noqa: E402

from conftest import DESIGN  # noqa: E402


def test_diff_sheets_keep_every_kind_and_the_near_rename_evidence():
    nc = diff_pb2.DiffReport.NetChange
    resp = diff_pb2.DiffDesignsResponse(
        report=diff_pb2.DiffReport(
            components_added=["R10", "R6"],
            components_changed=[diff_pb2.DiffReport.ComponentChange(ref_des="U1", field="Value", old="a", new="b")],
            nets=[
                nc(kind="hard", name="N1", added=["R6.2"]),
                nc(kind="renamed-approx", name="EN2", old_name="EN", added=["R6.1"],
                   approx=diff_pb2.DiffReport.RenameEvidence(old_coverage=1.0, overlap=2, old_endpoints=2, new_endpoints=3)),
            ],
        )
    )
    sheets = dict(diff_sheets(resp))
    assert list(sheets) == ["New nets", "Deleted nets", "Renamed nets", "Near renames", "Changed nets", "Attribute changes", "Component changes"]
    header, rows = sheets["Near renames"]
    near = dict(zip(header, rows[0]))
    assert (near["net"], near["old_name"], near["added"], near["old_coverage"], near["overlap"]) == ("EN2", "EN", "R6.1", "1", "2")
    assert sheets["Changed nets"][1][0][:3] == ["N1", "", "R6.2"]
    assert [r[:2] for r in sheets["Component changes"][1]] == [["R6", "added"], ["R10", "added"], ["U1", "changed"]]


def test_diff_sheets_refuse_a_kind_they_do_not_know():
    resp = diff_pb2.DiffDesignsResponse(report=diff_pb2.DiffReport(nets=[diff_pb2.DiffReport.NetChange(kind="sideways", name="N")]))
    with pytest.raises(ValueError, match="sideways"):
        diff_sheets(resp)


@pytest.fixture(params=["cli", "connect"])
def client(request):
    return request.getfixturevalue(request.param)


def test_diff_sheets_over_the_tutorial(client, tmp_path):
    resp = client.diff_designs(a_uri=DESIGN + "/gateway-rev-b.edn", b_uri=DESIGN + "/gateway-rev-c.edn", near_renames={})
    sheets = dict(diff_sheets(resp))
    header, rows = sheets["Near renames"]
    assert [dict(zip(header, r))["net"] for r in rows] == ["PMIC_ENABLE"]
    assert dict(zip(header, rows[0]))["old_name"] == "PMIC_EN"
    tables_to_xlsx(str(tmp_path / "diff.xlsx"), list(sheets.items()))


def test_unchanged_nets_appear_when_asked_for(client):
    resp = client.diff_designs(a_uri=DESIGN + "/gateway.edn", b_uri=DESIGN + "/gateway-rev-b.edn", include_equal=True)
    sheets = dict(diff_sheets(resp))
    assert len(sheets["Unchanged nets"][1]) == 10
    assert [r[0] for r in sheets["Renamed nets"][1]] == ["CLK_IN", "CLK_OUT"]


_CHECKLIST = checks.ReviewManifest(
    name="tables",
    areas=[
        checks.ManifestArea(
            name="Power",
            items=[
                checks.ManifestItem(id="P1", title="rails carry bulk capacitance", binding=checks.ItemBinding(rule="bulk-cap")),
                checks.ManifestItem(id="P2", title="i2c pull-ups", binding=checks.ItemBinding(rule="i2c-pull-up")),
                checks.ManifestItem(id="P3", title="reviewed by hand", note="the lead signs this off"),
            ],
        )
    ],
)


def test_review_sheet_over_the_tutorial(client, tmp_path):
    rv = client.create_review(design_uri=DESIGN, manifest=_CHECKLIST)
    sheets = dict(review_sheet(rv))
    header, rows = sheets["Review"]
    assert header[:4] == ["area", "id", "title", "outcome"]
    assert [r[1] for r in rows] == ["P1", "P2", "P3"]
    failed = [dict(zip(header, r)) for r in rows if r[3] == "fail"]
    assert failed and all(f["findings"] for f in failed)
    summary = dict(sheets["Review summary"][1])
    assert int(summary["total"]) == len(rows)
    tables_to_xlsx(str(tmp_path / "review.xlsx"), list(sheets.items()))


def test_tabulate_gives_the_cli_csv_tables(client, tmp_path):
    out = client.tabulate(check=client.check_design(uri=DESIGN), order_by=["rule", "subjects"])
    tables = {t.name: t for t in out.tables}
    assert list(tables) == ["findings", "verdicts", "verdicts_by_rule"]
    header, rows = to_rows(tables["verdicts"])
    # The header `agni check --verdicts --format csv` publishes, and its kind:ref subject spelling.
    assert header == ["verdict_id", "url", "rule", "outcome", "subjects", "statement", "context", "terms", "reason"]
    vs = [dict(zip(header, r)) for r in rows]
    passes = [v for v in vs if v["outcome"] == "pass"]
    assert passes and all(v["statement"] or v["context"] for v in passes)
    assert all(":" in v["subjects"] for v in vs)
    rules = [v["rule"] for v in vs]
    assert rules == sorted(rules) and len(set(rules)) > 1, "order_by rule did not order the verdicts"
    count_header, count_rows = to_rows(tables["verdicts_by_rule"])
    assert sum(int(dict(zip(count_header, r))["total"]) for r in count_rows) == len(rows)
    tables_to_xlsx(str(tmp_path / "check.xlsx"), table_sheets(out))


def test_tabulate_refuses_an_unknown_order_column(client):
    with pytest.raises(Exception, match="nope"):
        client.tabulate(check=client.check_design(uri=DESIGN), order_by=["nope"])


# The table rows an answer becomes are the engine's (agni issue 862). These are the public functions
# that still take an answer message, each for its stated reason, so a new one fails here.
STILL_TAKES_AN_ANSWER = {
    "rows_as_dicts": "reads a query answer as dicts; it lays no table out",
    "diff_sheets": "until the engine projects a diff (agni issue 862, part two)",
    "review_sheet": "until the engine projects a review (agni issue 862, part two)",
}


def test_no_other_function_lays_out_an_answer():
    import inspect

    import agni.tables as tables

    takers = set()
    for name, fn in inspect.getmembers(tables, inspect.isfunction):
        if name.startswith("_") or fn.__module__ != tables.__name__:
            continue
        hints = " ".join(str(p.annotation) for p in inspect.signature(fn).parameters.values())
        if "_pb2." in hints and "tables_pb2." not in hints:
            takers.add(name)
    assert takers, "the sweep matched nothing, so it is not reading the annotations"
    assert takers == set(STILL_TAKES_AN_ANSWER), f"functions taking an answer message: {sorted(takers)}"
