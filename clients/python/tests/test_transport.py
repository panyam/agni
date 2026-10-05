from __future__ import annotations

import pytest

from agni import CLI_ONLY, AgniError, CliUnsupported, Client, parse
from agni.transport import _unknown_field_paths
from agni.v1.param import param_pb2
from agni.v1.webapi import checks_pb2, query_pb2, validate_pb2

from conftest import DESIGN, FIXTURE

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
        connect.run_query(uri=DESIGN, query=BAD_QUERY)
    assert e.value.code == "invalid_argument"
    assert "neither an atom nor a comparison" in str(e.value)


def test_cli_surfaces_the_exit_status_and_stderr(cli: Client):
    with pytest.raises(AgniError) as e:
        cli.run_query(uri=DESIGN, query=BAD_QUERY)
    assert e.value.code.startswith("exit_") and e.value.code != "exit_0"
    assert "neither an atom nor a comparison" in e.value.detail


def test_cli_refuses_a_field_it_has_no_flag_for(cli: Client):
    req = query_pb2.RunQueryRequest(uri=DESIGN, query="entity(?x, ?k) => ?x")
    req.overlay.SetInParent()
    with pytest.raises(CliUnsupported, match="overlay"):
        cli.run_query(req)


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
