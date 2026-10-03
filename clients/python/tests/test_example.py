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
    built = [name for name, _ in mod.TABLES] + ["Findings"]
    assert wb.sheetnames == built
    for ws in wb.worksheets:
        assert ws.max_row > 1, f"{ws.title} is empty"

    summary = out.with_suffix(".md").read_text()
    for name in built:
        assert f"## {name}\n" in summary, f"summary has no section for {name}"
    # Until #818 and #822 land, these tab groups are reported missing, each naming the issue that fills it.
    for group in ("Diff", "Review", "Verdicts"):
        assert re.search(rf"## {group}\n\nmissing: .*#\d+", summary), f"{group} is not reported missing with its issue"
