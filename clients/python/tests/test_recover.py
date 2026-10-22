"""A Client recovers a snapshot the server lost or saw change, as the viewer does (agni issue 1067)."""

from __future__ import annotations

import base64
import json
import urllib.error
from io import BytesIO
from typing import List, Optional, Tuple

import pytest

from agni import AgniError, Client
from agni.services import lookup
from agni.transport import _connect_error
from agni.v1.webapi import checks_pb2, snapshot_pb2

from conftest import DESIGN

MISSING = snapshot_pb2.SnapshotUnavailable.REASON_MISSING
STALE = snapshot_pb2.SnapshotUnavailable.REASON_STALE


class Scripted:
    """A transport answering each call from a script, recording what it was asked."""

    def __init__(self, failures: List[Tuple[str, int]]):
        # failures are (method, reason) to raise, in order, the first time each method is called
        self.failures = list(failures)
        self.calls: List[Tuple[str, object]] = []

    def call(self, rpc, request):
        self.calls.append((rpc.method, request))
        if self.failures and self.failures[0][0] == rpc.method:
            _, reason = self.failures.pop(0)
            name = request.snapshot
            code = "failed_precondition" if reason == STALE else "not_found"
            raise AgniError("gone", code=code, snapshot_unavailable=snapshot_pb2.SnapshotUnavailable(name=name, reason=reason))
        if rpc.method in ("Ingest", "UpdateSnapshot"):
            name = request.snapshot.name or "snapshots/" + (request.snapshot_id or "minted")
            return snapshot_pb2.Snapshot(name=name, uri=request.snapshot.uri)
        return rpc.response()

    def methods(self) -> List[str]:
        return [m for m, _ in self.calls]


def test_a_lost_snapshot_is_ingested_again_under_its_name_and_the_question_asked_once_more():
    t = Scripted([("CheckDesign", MISSING)])
    c = Client(t)
    snap = c.ingest(DESIGN, snapshot_id="s1", flatten_hierarchy=True)
    c.check_design(snapshot=snap)
    assert t.methods() == ["Ingest", "CheckDesign", "Ingest", "CheckDesign"]
    again = t.calls[2][1]
    assert again.snapshot_id == "s1"
    assert again.snapshot.uri == DESIGN and again.snapshot.flatten_hierarchy


def test_a_stale_snapshot_is_updated_with_its_definition():
    t = Scripted([("CheckDesign", STALE)])
    c = Client(t)
    snap = c.ingest(DESIGN, snapshot_id="s1")
    c.check_design(snapshot=snap)
    assert t.methods() == ["Ingest", "CheckDesign", "UpdateSnapshot", "CheckDesign"]
    update = t.calls[2][1]
    assert update.snapshot.name == "snapshots/s1" and update.snapshot.uri == DESIGN


def test_it_asks_once_more_and_no_more():
    t = Scripted([("CheckDesign", MISSING), ("CheckDesign", MISSING)])
    c = Client(t)
    snap = c.ingest(DESIGN, snapshot_id="s1")
    with pytest.raises(AgniError) as e:
        c.check_design(snapshot=snap)
    assert e.value.snapshot_unavailable is not None
    assert t.methods() == ["Ingest", "CheckDesign", "Ingest", "CheckDesign"]


def test_a_snapshot_it_did_not_ingest_or_has_deleted_stays_an_error():
    t = Scripted([("CheckDesign", MISSING)])
    c = Client(t)
    with pytest.raises(AgniError):
        c.check_design(snapshot="snapshots/someone-elses")
    assert t.methods() == ["CheckDesign"]

    t = Scripted([("CheckDesign", MISSING)])
    c = Client(t)
    snap = c.ingest(DESIGN, snapshot_id="s1")
    c.delete_snapshot(snap)
    with pytest.raises(AgniError):
        c.check_design(snapshot=snap)
    assert t.methods() == ["Ingest", "DeleteSnapshot", "CheckDesign"]


def _http_error(body: dict, status: int = 404) -> urllib.error.HTTPError:
    raw = json.dumps(body).encode()
    return urllib.error.HTTPError("http://x", status, "err", {}, BytesIO(raw))  # type: ignore[arg-type]


def test_the_transport_reads_the_detail_off_a_connect_error():
    detail = snapshot_pb2.SnapshotUnavailable(name="snapshots/s1", reason=STALE)
    value = base64.b64encode(detail.SerializeToString()).decode().rstrip("=")
    rpc = lookup("CheckService", "CheckDesign")
    e = _connect_error(rpc, _http_error({"code": "failed_precondition", "message": "stale", "details": [
        {"type": "google.rpc.Other", "value": "AA"},
        {"type": "agni.v1.webapi.SnapshotUnavailable", "value": value},
    ]}, 400))
    assert e.code == "failed_precondition"
    assert e.snapshot_unavailable == detail

    plain = _connect_error(rpc, _http_error({"code": "not_found", "message": "no such design"}))
    assert plain.snapshot_unavailable is None


def test_over_connect_a_snapshot_the_server_forgot_is_recovered(connect: Client):
    snap = connect.ingest(DESIGN)
    # Delete it behind the client's back, through the transport, as an eviction or restart would.
    connect.transport.call(lookup("SnapshotService", "DeleteSnapshot"), snapshot_pb2.DeleteSnapshotRequest(name=snap))
    resp = connect.check_design(snapshot=snap, rules=["single-pin-net"])
    assert isinstance(resp, checks_pb2.CheckDesignResponse)
