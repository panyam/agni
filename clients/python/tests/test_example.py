from __future__ import annotations

import importlib.util
import re
from pathlib import Path

import openpyxl

from conftest import AGNI

EXAMPLE = Path(__file__).resolve().parents[1] / "examples" / "audit_workbook.py"


def test_audit_workbook_example_runs(tmp_path, isolated, monkeypatch):
    env, _ = isolated
    for k in ("HOME", "XDG_CONFIG_HOME"):
        monkeypatch.setenv(k, env[k])
    spec = importlib.util.spec_from_file_location("audit_workbook", EXAMPLE)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    out = tmp_path / "audit.xlsx"
    mod.main(["-o", str(out), "--agni", AGNI])
    wb = openpyxl.load_workbook(out)
    assert wb.sheetnames == [name for name, _ in mod.TABLES] + ["Findings"]
    # Every sheet carries rows on the tutorial board, so a query that silently matched nothing fails.
    for ws in wb.worksheets:
        assert ws.max_row > 1, f"{ws.title} is empty"


REVISION_AUDIT = EXAMPLE.with_name("revision_audit.py")


def test_revision_audit_exercise_runs(tmp_path, isolated, monkeypatch):
    env, _ = isolated
    for k in ("HOME", "XDG_CONFIG_HOME"):
        monkeypatch.setenv(k, env[k])
    monkeypatch.syspath_prepend(str(EXAMPLE.parent))
    spec = importlib.util.spec_from_file_location("revision_audit", REVISION_AUDIT)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    out = tmp_path / "audit.xlsx"
    mod.main(["-o", str(out), "--agni", AGNI])
    wb = openpyxl.load_workbook(out)
    diff = ["New nets", "Deleted nets", "Renamed nets", "Near renames", "Changed nets", "Attribute changes", "Unchanged nets", "Component changes"]
    built = diff + [name for name, _ in mod.TABLES] + ["Findings", "Verdicts", "Verdicts by rule"]
    assert wb.sheetnames == built
    # A diff kind that did not happen between the two revisions is an empty tab on purpose.
    for ws in wb.worksheets:
        if ws.title not in ("New nets", "Deleted nets", "Near renames", "Attribute changes"):
            assert ws.max_row > 1, f"{ws.title} is empty"

    summary = out.with_suffix(".md").read_text()
    for name in built:
        assert f"## {name}\n" in summary, f"summary has no section for {name}"
    # The review tab waits on a way to ask for the project's checklist (agni issue 859), and says so.
    assert re.findall(r"## (\w+)\n\nmissing: .*#\d+", summary) == ["Review"]
