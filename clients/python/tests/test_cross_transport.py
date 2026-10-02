"""The same request through the CLI and through a server gives the same message.

This is C31 checked from outside Go: a command's json is its rpc's message, so a client that switches
transport must see no difference. Where the two genuinely differ today the difference is DECLARED
below, field by field, with the issue that will remove it, and the test asserts the difference is
exactly that. A declaration that stops being true fails, so none outlives its bug.
"""

from __future__ import annotations

from typing import Callable, Dict, List, NamedTuple, Optional

import pytest
from google.protobuf.message import Message

from agni import CLI_COMMANDS, Client, bindings
from agni.errors import AgniError
from agni.v1.checks import checks_pb2
from agni.v1.webapi import query_pb2
from agni.v1.webapi import design_pb2

from conftest import DESIGN


def _endpoint(ref: str) -> design_pb2.TraceEndpoint:
    ref_des, pin = ref.split(".")
    return design_pb2.TraceEndpoint(ref_des=ref_des, pin=pin)


def _trace(frm: str, to: str) -> Callable[[Client], Message]:
    def ask(c: Client) -> Message:
        req = design_pb2.TraceDesignRequest(uri=DESIGN, to=_endpoint(to))
        getattr(req, "from").CopyFrom(_endpoint(frm))
        return c.trace_design(req)

    return ask


class Case(NamedTuple):
    rpc: str
    ask: Callable[[Client], Message]
    # Top-level response fields the CLI leaves EMPTY where the server fills them, and why.
    cli_leaves_empty: Dict[str, str] = {}
    # Clears, on both responses, what two runs of one request legitimately differ in, such as when
    # each was made.
    normalize: Optional[Callable[[Message], None]] = None


VERDICTS = (
    "`check --format json` strips the considered set on purpose (writeCheckDesignJSON); "
    "`--verdicts` asks for it as a bare list, which has no wire message"
)
UNSTORED = "the CLI stores nothing, so its review has no resource name (agni issue 734)"


def _run_time(rv: Message) -> None:
    rv.results.meta.created_at = ""


# A checklist exercising each binding the CLI has to carry: a catalog rule, an inline query, an item
# no mechanism answers, and a query calling a library sent with the request (as --lib).
_MANIFEST = checks_pb2.ReviewManifest(
    name="cross-transport",
    areas=[
        checks_pb2.ManifestArea(
            name="A",
            items=[
                checks_pb2.ManifestItem(id="r", title="i2c pull-ups", binding=checks_pb2.ItemBinding(rule="i2c-pull-up")),
                checks_pb2.ManifestItem(
                    id="q",
                    title="resistors",
                    binding=checks_pb2.ItemBinding(
                        query=checks_pb2.ManifestQuery(match='component.class(?r, "resistor") => ?r', subject="r", message="{r} is a resistor")
                    ),
                ),
                checks_pb2.ManifestItem(id="m", title="checked by hand", note="no mechanism"),
                checks_pb2.ManifestItem(
                    id="l",
                    title="nets with a resistor",
                    binding=checks_pb2.ItemBinding(
                        query=checks_pb2.ManifestQuery(match="probe.resistor_net(?n) => ?n", subject="n", kind="net", message="{n} has a resistor")
                    ),
                ),
            ],
        )
    ],
)

DIFF_GEOMETRY = "agni issue 737: the CLI skips the service's sheet and placement annotation"
LAYOUT_TIERS = "agni issue 736: GetLayoutReport does not resolve tiers, so the folder reads as nothing"

# A set whose preamble every query reads, and the same set with one query the design cannot answer:
# over the CLI that exits 1 with the full answer printed, and the two transports must still agree.
_SET = query_pb2.QuerySet(
    title="audit",
    preamble='res(?r) :- component.class(?r, "resistor");',
    queries=[
        query_pb2.NamedQuery(name="resistors", query="res(?r) => ?r"),
        query_pb2.NamedQuery(name="nets", query="pin.net(?c, ?p, ?n) => count(distinct ?n)"),
    ],
)
_SET_WITH_TYPO = query_pb2.QuerySet(
    title="audit",
    queries=list(_SET.queries) + [query_pb2.NamedQuery(name="typo", query="compnent.class(?c, ?k)")],
    preamble=_SET.preamble,
)

# A library sent with the request (agni issue 788): a plain member and one taking a parameter. Over the
# CLI it travels as files under --lib, over Connect as the field itself, and both must answer alike.
_LIB = {
    "config": {
        "library_modules": [
            {
                "path": "probe",
                "text": (
                    "# A net a resistor sits on.\n"
                    "resistor_net(?n: net) :- component.net(?r, ?n), component.class(?r, \"resistor\");\n"
                    "# A net a part of the given class sits on.\n"
                    "class_net(?n: net, ?k: string) :- component.net(?r, ?n), component.class(?r, ?k);\n"
                ),
            }
        ]
    }
}

CASES: List[Case] = [
    Case("CheckService/CheckDesign", lambda c: c.check_design(uri=DESIGN), {"verdicts": VERDICTS}),
    Case(
        "CheckService/CheckDesign",
        lambda c: c.check_design(uri=DESIGN, rules=["i2c-pull-up", "decoupling-present"]),
        {"verdicts": VERDICTS},
    ),
    Case("CheckService/GetCheckReport", lambda c: c.get_check_report(uri=DESIGN)),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query='component.class(?c, "resistor") => ?c')),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query="pin.net(?c, ?p, ?n) => ?n, count(distinct ?c)")),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query='pin.net(?c, ?p, "NO_SUCH_NET") => ?c')),
    Case("QueryService/RunQueries", lambda c: c.run_queries(uri=DESIGN, set=_SET)),
    # A budget the query fits in answers alike, work included (agni issue 792).
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query='component.class(?c, "resistor") => ?c', work_budget=10_000_000)),
    # Bound variables answer alike, text and number, quoting included (agni issue 793).
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query="component.net(?r, ?n) => ?n", bindings=bindings({"r": "U1"}))),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query="net.pin_count(?n, ?c), ?c >= ?min => ?n", bindings=bindings({"min": 3.0}))),
    # The TEXT "3", in a position the schema types as a number, is read as 3 on both transports
    # (panyam/jaala#65), so this answers as the case above.
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query="net.pin_count(?n, ?c), ?c >= ?min => ?n", bindings=bindings({"min": "3"}))),
    Case(
        "QueryService/RunQueries",
        lambda c: c.run_queries(
            uri=DESIGN,
            set=query_pb2.QuerySet(
                title="bound",
                queries=[
                    query_pb2.NamedQuery(name="u1", query="component.net(?r, ?n) => ?n", bindings=bindings({"r": "U1"})),
                    query_pb2.NamedQuery(name="many", query="net.pin_count(?n, ?c), ?c >= ?min => ?n", bindings=bindings({"min": 3})),
                ],
            ),
        ),
    ),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query="probe.resistor_net(?n) => ?n", overlay=_LIB)),
    Case("QueryService/RunQuery", lambda c: c.run_query(uri=DESIGN, query='probe.class_net(?n, "capacitor") => ?n', overlay=_LIB)),
    Case(
        "QueryService/RunQueries",
        lambda c: c.run_queries(
            uri=DESIGN,
            set=query_pb2.QuerySet(title="lib", queries=[query_pb2.NamedQuery(name="r", query="probe.resistor_net(?n) => ?n")]),
            overlay=_LIB,
        ),
    ),
    Case("QueryService/RunQueries", lambda c: c.run_queries(uri=DESIGN, set=_SET_WITH_TYPO)),
    Case(
        "DiffService/DiffDesigns",
        lambda c: c.diff_designs(a_uri=DESIGN + "/gateway.edn", b_uri=DESIGN + "/gateway-rev-b.edn"),
        {
            "component_sheets_b": DIFF_GEOMETRY,
            "net_sheets_a": DIFF_GEOMETRY,
            "net_sheets_b": DIFF_GEOMETRY,
            "shared_placements_a": DIFF_GEOMETRY,
            "shared_placements_b": DIFF_GEOMETRY,
        },
    ),
    Case(
        "ReviewService/CreateReview",
        lambda c: c.create_review(design_uri=DESIGN, manifest=_MANIFEST, overlay=_LIB),
        {"name": UNSTORED},
        _run_time,
    ),
    Case("DesignService/TraceDesign", _trace("J1.3", "J1.4")),
    Case("DesignService/TraceDesign", _trace("J1.3", "ZZ9.1")),
    Case("DesignService/GetLayoutReport", lambda c: c.get_layout_report(uri=DESIGN + "/gateway.edn", as_named=True)),
]

# The server fills nothing for the folder, which is the bug, so the CLI is the side with content.
LAYOUT_FOLDER = Case("DesignService/GetLayoutReport", lambda c: c.get_layout_report(uri=DESIGN))


def _nonempty(msg: Message) -> set:
    return {f.name for f, _ in msg.ListFields()}


@pytest.mark.parametrize("case", CASES, ids=lambda c: c.rpc)
def test_same_request_same_message(case: Case, cli: Client, connect: Client):
    got_cli, got_srv = case.ask(cli), case.ask(connect)
    assert type(got_cli) is type(got_srv)
    declared = set(case.cli_leaves_empty)
    if declared:
        leaked = declared & _nonempty(got_cli)
        assert not leaked, f"the CLI now fills {sorted(leaked)}; remove the declaration ({case.cli_leaves_empty})"
        missing = declared - _nonempty(got_srv)
        assert not missing, f"the server no longer fills {sorted(missing)}; the declaration is stale"
        for name in declared:
            got_srv.ClearField(name)
    if case.normalize:
        case.normalize(got_cli)
        case.normalize(got_srv)
    assert got_cli == got_srv


def test_layout_report_for_the_folder_diverges_as_declared(cli: Client, connect: Client):
    """agni issue 736. When it is fixed this fails, and LAYOUT_FOLDER moves into CASES."""
    assert len(LAYOUT_FOLDER.ask(cli).report.components) > 0
    assert len(LAYOUT_FOLDER.ask(connect).report.components) == 0, LAYOUT_TIERS


def test_every_cli_command_is_compared():
    """A CLI mapping nobody compares against the server is a promise nobody checks."""
    compared = {c.rpc for c in CASES}
    assert compared == set(CLI_COMMANDS)


def test_a_trace_to_nothing_still_answers_over_the_cli(cli: Client):
    """`agni trace` prints a complete answer and exits non-zero for an endpoint naming nothing."""
    got = _trace("J1.3", "ZZ9.1")(cli)
    assert got.trace.outcome == design_pb2.TRACE_OUTCOME_UNRESOLVED


def test_a_query_past_its_budget_fails_on_both_transports(cli: Client, connect: Client):
    """agni issue 792. Over Connect the code is resource_exhausted; over the CLI the binary exits
    non-zero. Both name the budget, so a client can tell an expensive question from a wrong one."""
    for c in (cli, connect):
        with pytest.raises(AgniError) as e:
            c.run_query(uri=DESIGN, query="component.net(?r, ?n), component.net(?r2, ?n) => ?r, ?r2", work_budget=5)
        assert "budget of 5" in (str(e.value) + getattr(e.value, "detail", "")), e.value


def test_a_binding_the_query_does_not_use_fails_on_both_transports(cli: Client, connect: Client):
    """agni issue 793. A misspelled binding is refused rather than leaving the variable free."""
    for c in (cli, connect):
        with pytest.raises(AgniError) as e:
            c.run_query(uri=DESIGN, query="component.net(?r, ?n) => ?n", bindings=bindings({"zz": "U1"}))
        assert "?zz" in (str(e.value) + getattr(e.value, "detail", "")), e.value


def test_bindings_types_each_value():
    b = bindings({"t": "3", "n": 3, "f": 3.3})
    assert b["t"].WhichOneof("kind") == "text" and b["t"].text == "3"
    assert b["n"].WhichOneof("kind") == "number" and b["n"].number == 3.0
    assert b["f"].number == 3.3
    with pytest.raises(TypeError):
        bindings({"x": True})
