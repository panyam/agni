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


# ---- natural order, and the diff, review and verdict sheets (agni issue 822) ----

from agni.tables import diff_sheets, natural_key, natural_sort, review_sheet, verdict_sheets  # noqa: E402
from agni.v1.webapi import diff_pb2, review_pb2  # noqa: E402

from conftest import DESIGN  # noqa: E402


def test_natural_key_puts_r2_before_r10():
    assert sorted(["R10", "R2", "C1", "R1", "U10.3", "U10.12"], key=natural_key) == ["C1", "R1", "R2", "R10", "U10.3", "U10.12"]


def test_natural_sort_orders_by_the_named_columns():
    header, rows = natural_sort((["net", "c"], [["N10", "1"], ["N2", "0"], ["N2", "1"]]), "net", "c")
    assert header == ["net", "c"]
    assert rows == [["N2", "0"], ["N2", "1"], ["N10", "1"]]


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


def test_verdict_sheets_refuse_a_response_stripped_of_its_verdicts():
    resp = checks_pb2.CheckDesignResponse(findings=[checks.Finding(rule="r", subject=checks.Subject(kind="net", ref="N"))])
    with pytest.raises(ValueError, match="no verdicts"):
        verdict_sheets(resp)


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


def test_verdict_sheets_over_the_tutorial(client, tmp_path):
    sheets = dict(verdict_sheets(client.check_design(uri=DESIGN)))
    header, rows = sheets["Verdicts"]
    vs = [dict(zip(header, r)) for r in rows]
    passes = [v for v in vs if v["outcome"] == "pass"]
    assert passes and all(v["subject"] for v in vs)
    assert any(v["witness"] or v["context"] for v in passes)
    by_rule = [dict(zip(sheets["Verdicts by rule"][0], r)) for r in sheets["Verdicts by rule"][1]]
    assert sum(int(r["total"]) for r in by_rule) == len(rows)
    tables_to_xlsx(str(tmp_path / "verdicts.xlsx"), list(sheets.items()))
