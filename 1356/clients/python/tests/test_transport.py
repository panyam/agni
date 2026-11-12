from __future__ import annotations

import pytest

from agni import CLI_ONLY, AgniError, CliTransport, CliUnsupported, Client, parse
from agni.transport import _unknown_field_paths
from agni.v1.param import param_pb2
from agni.v1.webapi import checks_pb2, query_pb2, snapshot_pb2, validate_pb2

from conftest import DESIGN, FIXTURE, snap

BAD_QUERY = "x"


def test_strict_parse_rejects_a_field_the_message_lacks():
    with pytest.raises(AgniError) as e:
        parse('{"columns": ["c"], "summary": {"total": 1}}', query_pb2.RunQueryResponse, strict=True)
    assert e.value.code == "bad_response"


def test_lenient_parse_skips_a_field_from_a_newer_server():
    got = parse('{"columns": ["c"], "addedLater": 1}', query_pb2.RunQueryResponse)
    assert list(got.columns) == ["c"]


def test_connect_surfaces_the_error_code_and_message(connect: Client):
    with pytest.raises(AgniError) as e:
        connect.run_query(snapshot=snap(connect), query=BAD_QUERY)
    assert e.value.code == "invalid_argument"
    assert "neither an atom nor a comparison" in str(e.value)


def test_cli_surfaces_the_exit_status_and_stderr(cli: Client):
    with pytest.raises(AgniError) as e:
        cli.run_query(snapshot=snap(cli), query=BAD_QUERY)
    assert e.value.code.startswith("exit_") and e.value.code != "exit_0"
    assert "neither an atom nor a comparison" in e.value.detail


def test_cli_refuses_a_field_it_has_no_flag_for(cli: Client):
    # An overlay carrying nothing the CLI sends, which no command could be given, is refused when the
    # snapshot is defined.
    req = snapshot_pb2.IngestRequest(snapshot=snapshot_pb2.Snapshot(uri=DESIGN))
    req.snapshot.overlay.SetInParent()
    with pytest.raises(CliUnsupported, match="overlay"):
        cli.ingest_snapshot(req)
    # A library overlay is refused by a command with no --lib, at the question.
    lib = snap(cli, overlay={"config": {"library_modules": [{"path": "probe", "text": "x(?n: net) :- net.name(?n);"}]}})
    with pytest.raises(CliUnsupported, match="overlay"):
        cli.check_design(snapshot=lib)
    # And a request field the command has no flag for, as before snapshots.
    with pytest.raises(CliUnsupported, match="work_budget"):
        cli.check_design(snapshot=snap(cli), work_budget=5)
    with pytest.raises(CliUnsupported, match="geometry_uri"):
        cli.ingest(DESIGN, geometry_uri=DESIGN + "/gateway.eds")


class _Recording(CliTransport):
    """A CliTransport that records each argv instead of running it, and answers an empty message."""

    def __init__(self) -> None:
        super().__init__("agni")
        self.argvs: list = []

    def _run_text(self, args, allow_failure=False, stdin=None):
        self.argvs.append(list(args))
        return "{}"


def test_cli_ingest_get_delete_round_trip():
    t = _Recording()
    c = Client(t)
    name = c.ingest(DESIGN, board_uri="mount://tut/boards/x.kicad_pcb")
    assert name == "snapshots/cli-1"
    assert c.ingest(DESIGN) == "snapshots/cli-2", "each ingest is a snapshot of its own"
    got = c.get_snapshot(name)
    assert (got.name, got.uri, got.board_uri, got.as_named) == (name, DESIGN, "mount://tut/boards/x.kicad_pcb", False)
    assert not got.HasField("tiers"), "the CLI resolves no tiers"
    assert [s.name for s in c.list_snapshots().snapshots] == ["snapshots/cli-1", "snapshots/cli-2"]
    assert [s.name for s in c.list_snapshots(filter='uri="nope"').snapshots] == []
    # A named ingest of the same definition is that snapshot again, and of another is refused.
    assert c.ingest(DESIGN, snapshot_id="mine") == "snapshots/mine"
    assert c.ingest(DESIGN, snapshot_id="mine") == "snapshots/mine"
    with pytest.raises(AgniError) as e:
        c.ingest(DESIGN, as_named=True, snapshot_id="mine")
    assert e.value.code == "already_exists"
    up = c.update_snapshot(snapshot={"name": name, "as_named": True}, update_mask={"paths": ["as_named"]})
    assert (up.uri, up.board_uri, up.as_named) == (DESIGN, "mount://tut/boards/x.kicad_pcb", True)
    c.delete_snapshot(name)
    for ask in (lambda: c.get_snapshot(name), lambda: c.delete_snapshot(name), lambda: c.run_query(snapshot=name, query="q")):
        with pytest.raises(AgniError) as e:
            ask()
        assert e.value.code == "not_found"
    assert t.argvs == [], "no snapshot rpc, and no question naming a missing snapshot, runs the binary"


def test_cli_argv_carries_the_snapshot_definition():
    """The board and naming a snapshot defines reach each command as the flags the old request
    fields produced, and its library as --lib."""
    t = _Recording()
    c = Client(t)
    board = c.ingest(DESIGN, board_uri="mount://tut/boards/b.kicad_pcb", as_named=True)
    named = c.ingest(DESIGN + "/gateway.edn", as_named=True)
    lib = c.ingest(DESIGN, overlay={"config": {"library_modules": [{"path": "probe", "text": "x(?n: net) :- net.name(?n);"}]}})
    plain = c.ingest(DESIGN)
    c.check_design(snapshot=board, rules=["r1"])
    c.run_query(snapshot=board, query="q", work_budget=7)
    c.create_review(snapshot=board, manifest={"name": "m"})
    c.get_layout_report(snapshot=named)
    c.get_design(snapshot=named)
    c.diff_designs(a_snapshot=plain, b_snapshot=plain)
    c.run_query(snapshot=lib, query="q")
    assert t.argvs[:6] == [
        ["check", DESIGN, "--format", "json", "--verdicts", "--rule", "r1", "--board-path", "mount://tut/boards/b.kicad_pcb", "--as-named"],
        ["query", DESIGN, "q", "--format", "json", "--budget", "7", "--board-path", "mount://tut/boards/b.kicad_pcb", "--as-named"],
        ["review", DESIGN, "--checklist", "-", "--format", "json", "--board-path", "mount://tut/boards/b.kicad_pcb", "--as-named"],
        ["render", DESIGN + "/gateway.edn", "--report", "--report-format", "json", "--as-named"],
        ["stats", DESIGN + "/gateway.edn", "--format", "json", "--as-named"],
        ["diff", DESIGN, DESIGN, "--format", "json"],
    ]
    assert t.argvs[6][:5] == ["query", DESIGN, "q", "--format", "json"] and t.argvs[6][5] == "--lib", t.argvs[6]
    # A definition field a command has no flag for is refused at the question, never dropped.
    with pytest.raises(CliUnsupported, match="board_uri"):
        c.get_design(snapshot=board)
    with pytest.raises(CliUnsupported, match="as_named"):
        c.diff_designs(a_snapshot=named, b_snapshot=named)
    assert len(t.argvs) == 7


def test_cli_argv_carries_flatten_hierarchy():
    """A snapshot ingested with flatten_hierarchy reaches every command that reads it as
    --flatten-hierarchy, and a definition without it adds nothing (agni issue 1104)."""
    t = _Recording()
    c = Client(t)
    flat = c.ingest(DESIGN, flatten_hierarchy=True)
    plain = c.ingest(DESIGN)
    assert c.get_snapshot(flat).flatten_hierarchy and not c.get_snapshot(plain).flatten_hierarchy
    assert c.get_snapshot(flat) == c.ingest_snapshot(snapshot={"uri": DESIGN, "flatten_hierarchy": True}, snapshot_id=flat.split("/")[1])
    c.check_design(snapshot=flat)
    c.run_query(snapshot=flat, query="q")
    c.create_review(snapshot=flat, manifest={"name": "m"})
    c.get_design(snapshot=flat)
    c.get_layout_report(snapshot=flat)
    c.get_design(snapshot=plain)
    assert [a[-1] for a in t.argvs[:5]] == ["--flatten-hierarchy"] * 5, t.argvs
    assert "--flatten-hierarchy" not in t.argvs[5]


def test_cli_refuses_an_rpc_no_command_serves(cli: Client):
    # GetReview reads a stored run, and the CLI stores none. CreateReview served as this example until
    # agni issue 734 gave it a command.
    with pytest.raises(CliUnsupported, match="ReviewService/GetReview"):
        cli.call("ReviewService", "GetReview", name="reviews/x")


def test_connect_reaches_an_rpc_with_no_named_method(connect: Client):
    got = connect.call("WorkspaceService", "ListMounts")
    assert "tut" in [m.name for m in got.mounts]


def test_a_server_with_no_viewer_says_so(server):
    import urllib.request

    with urllib.request.urlopen(server + "/", timeout=5) as r:
        body = r.read().decode()
    assert "WITHOUT the viewer" in body
    assert "agni.v1.webapi.QueryService" in body


def test_call_rejects_a_request_of_the_wrong_type(connect: Client):
    with pytest.raises(TypeError, match="agni.v1.webapi.RunQueryRequest"):
        connect.call("QueryService", "RunQuery", checks_pb2.CheckDesignRequest())


def test_call_names_an_rpc_agni_does_not_declare(connect: Client):
    with pytest.raises(KeyError, match="QueryService/Nope"):
        connect.call("QueryService", "Nope")


def test_cli_only_commands_parse_as_their_message(cli: Client):
    """`validate` and `params` have a wire form and no rpc; strict parsing is the check."""
    t = cli.transport
    assert CLI_ONLY == {"validate": validate_pb2.ValidateReport, "params": param_pb2.PartSpec}
    rep = t.run(["validate", str(FIXTURE / "designs" / "gateway" / "gateway.edn"), "--format", "json"], validate_pb2.ValidateReport)
    assert len(rep.ListFields()) > 0
    spec = t.run(["params", "ACME-LDO-1V8", "--params", str(FIXTURE / "params"), "--format", "json"], param_pb2.PartSpec)
    assert spec.mpn == "ACME-LDO-1V8"


def test_strict_binary_names_a_field_the_message_lacks():
    """The binary decoder keeps a field it does not know rather than failing, so strict has to look
    for one, nested ones included, which is where a newer server's additions usually land."""
    known = query_pb2.RunQueryResponse(columns=["c"], rows=[query_pb2.QueryRow(cells=["x"])])
    raw = known.SerializeToString()
    unknown_top = raw + b"\xb8\x3e\x01"  # field 999, varint 1
    row = query_pb2.QueryRow(cells=["x"]).SerializeToString() + b"\xb8\x3e\x01"
    unknown_nested = query_pb2.RunQueryResponse(columns=["c"]).SerializeToString() + b"\x12" + bytes([len(row)]) + row
    assert _unknown_field_paths(query_pb2.RunQueryResponse.FromString(raw)) == []
    assert _unknown_field_paths(query_pb2.RunQueryResponse.FromString(unknown_top)) == ["RunQueryResponse (field 999)"]
    assert _unknown_field_paths(query_pb2.RunQueryResponse.FromString(unknown_nested)) == ["RunQueryResponse.rows[0] (field 999)"]
