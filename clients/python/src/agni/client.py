"""The client: one method per rpc, the same types on either transport."""

from __future__ import annotations

from typing import Any, Mapping, Optional, Protocol, Union

from google.protobuf import empty_pb2
from google.protobuf.message import Message

from agni import services
from agni.v1.webapi import checks_pb2, design_pb2, diff_pb2, query_pb2, review_pb2, snapshot_pb2, tables_pb2, workspace_pb2


class Transport(Protocol):
    def call(self, rpc: services.Rpc, request: Message) -> Message: ...


class Client:
    """Asks agni questions through a transport.

    Switching between the CLI and a server changes the constructor argument and nothing else, since
    both return the rpc's own response message::

        c = Client(CliTransport(mounts={"tut": "examples/tutorial-project"}))
        c = Client(ConnectTransport("http://127.0.0.1:8080"))
        snap = c.ingest("mount://tut/designs/gateway")
        rows = c.run_query(snapshot=snap, query='component.class(?c,"resistor") => ?c')
        c.delete_snapshot(snap)

    Every question names a snapshot, the design loaded once under one config (agni issue 1061), so
    a script ingests each design it asks about and passes the name ``ingest`` returns as
    ``snapshot=``, or ``a_snapshot=`` and ``b_snapshot=`` to ``diff_designs``.

    The named methods cover the questions an analysis script asks. Every other rpc is reachable
    through ``call``, which resolves the request and response types from the generated descriptors.
    """

    def __init__(self, transport: Transport) -> None:
        self.transport = transport

    def call(self, service: str, method: str, request: Optional[Message] = None, **fields) -> Message:
        """Call any rpc by name, e.g. ``call("ProjectService", "ListProjects")``.

        Pass a request message, or its fields as keywords, not both. Raises ``KeyError`` for an rpc
        agni does not declare, and ``TypeError`` for a request of the wrong message type.
        """
        rpc = services.lookup(service, method)
        if request is None:
            request = rpc.request(**fields)
        elif fields:
            raise TypeError("pass a request message or keyword fields, not both")
        elif not isinstance(request, rpc.request):
            raise TypeError(
                f"{service}/{method} takes {rpc.request.DESCRIPTOR.full_name}, "
                f"got {request.DESCRIPTOR.full_name}"
            )
        return self.transport.call(rpc, request)

    def ingest(
        self,
        uri: str,
        *,
        board_uri: str = "",
        geometry_uri: str = "",
        as_named: bool = False,
        overlay: Optional[Union[checks_pb2.OverlayConfig, Mapping[str, Any]]] = None,
        read_board: bool = False,
        snapshot_id: str = "",
    ) -> str:
        """Load the design ``uri`` names and return the snapshot's name, ``snapshots/<id>``, which every
        question takes as ``snapshot=``.

        ``board_uri`` and ``geometry_uri`` attach a board or schematic other than the ones the design
        declares, ``as_named`` reads exactly the artifact named, and ``overlay`` is the config every
        question is answered under. ``ingest_snapshot`` returns the whole ``Snapshot`` instead.
        """
        snap = snapshot_pb2.Snapshot(uri=uri, board_uri=board_uri, geometry_uri=geometry_uri, as_named=as_named, overlay=overlay)
        resp = self.call("SnapshotService", "Ingest", snapshot=snap, read_board=read_board, snapshot_id=snapshot_id)
        return resp.name  # type: ignore[attr-defined]

    def ingest_snapshot(self, request: Optional[snapshot_pb2.IngestRequest] = None, **fields) -> snapshot_pb2.Snapshot:
        """Ingest a definition, ``snapshot={"uri": ...}``, and return the snapshot with what its tiers
        resolved to. Over the CLI the snapshot carries its name and definition alone."""
        return self.call("SnapshotService", "Ingest", request, **fields)  # type: ignore[return-value]

    def get_snapshot(self, request: Optional[Union[snapshot_pb2.GetSnapshotRequest, str]] = None, **fields) -> snapshot_pb2.Snapshot:
        """The snapshot a name names, as ``get_snapshot("snapshots/x")`` or ``name=``."""
        if isinstance(request, str):
            request, fields = None, {"name": request, **fields}
        return self.call("SnapshotService", "GetSnapshot", request, **fields)  # type: ignore[return-value]

    def list_snapshots(self, request: Optional[snapshot_pb2.ListSnapshotsRequest] = None, **fields) -> snapshot_pb2.ListSnapshotsResponse:
        return self.call("SnapshotService", "ListSnapshots", request, **fields)  # type: ignore[return-value]

    def update_snapshot(self, request: Optional[snapshot_pb2.UpdateSnapshotRequest] = None, **fields) -> snapshot_pb2.Snapshot:
        """Replace a snapshot's definition, whole or by ``update_mask``, and read it again under the same
        name. The same definition again is how a stale snapshot is refreshed."""
        return self.call("SnapshotService", "UpdateSnapshot", request, **fields)  # type: ignore[return-value]

    def delete_snapshot(self, request: Optional[Union[snapshot_pb2.DeleteSnapshotRequest, str]] = None, **fields) -> empty_pb2.Empty:
        """Forget a snapshot, as ``delete_snapshot("snapshots/x")`` or ``name=``."""
        if isinstance(request, str):
            request, fields = None, {"name": request, **fields}
        return self.call("SnapshotService", "DeleteSnapshot", request, **fields)  # type: ignore[return-value]

    def check_design(self, request: Optional[checks_pb2.CheckDesignRequest] = None, **fields) -> checks_pb2.CheckDesignResponse:
        """Run the rule catalog. Over the CLI the response carries no ``verdicts``; see the README."""
        return self.call("CheckService", "CheckDesign", request, **fields)  # type: ignore[return-value]

    def get_check_report(self, request: Optional[checks_pb2.GetCheckReportRequest] = None, **fields) -> checks_pb2.GetCheckReportResponse:
        return self.call("CheckService", "GetCheckReport", request, **fields)  # type: ignore[return-value]

    def list_rules(self, request: Optional[checks_pb2.ListRulesRequest] = None, **fields) -> checks_pb2.ListRulesResponse:
        return self.call("CheckService", "ListRules", request, **fields)  # type: ignore[return-value]

    def run_query(self, request: Optional[query_pb2.RunQueryRequest] = None, **fields) -> query_pb2.RunQueryResponse:
        return self.call("QueryService", "RunQuery", request, **fields)  # type: ignore[return-value]

    def run_queries(self, request: Optional[query_pb2.RunQueriesRequest] = None, **fields) -> query_pb2.RunQueriesResponse:
        """Answer a named list of queries over one read of the design.

        A query that could not be answered carries ``error`` rather than ``result``; the call itself
        succeeds, so check each result. ``tabulate(query_set=...)`` turns it into tables.
        """
        return self.call("QueryService", "RunQueries", request, **fields)  # type: ignore[return-value]

    def list_query_sets(self, request: Optional[query_pb2.ListQuerySetsRequest] = None, **fields) -> query_pb2.ListQuerySetsResponse:
        """The query sets the design's project keeps in its ``queries/`` directory, inherited ones
        first, each parsed. ``run_queries(set=...)`` runs one. An empty ``project`` means the design
        belongs to none. Connect only."""
        return self.call("QueryService", "ListQuerySets", request, **fields)  # type: ignore[return-value]

    def parse_query_set(self, request: Optional[query_pb2.ParseQuerySetRequest] = None, **fields) -> query_pb2.ParseQuerySetResponse:
        """A query set file's ``text`` read into the ``QuerySet`` ``run_queries`` takes, checked as
        ``agni query --set`` checks it. Connect only."""
        return self.call("QueryService", "ParseQuerySet", request, **fields)  # type: ignore[return-value]

    def tabulate(self, request: Optional[tables_pb2.TabulateRequest] = None, **fields) -> tables_pb2.TabulateResponse:
        """The engine's tables for an answer this client already holds: ``check=`` a CheckDesign
        response, ``query=`` a query answer or ``query_set=`` a set's, with ``order_by`` such as
        ``["rule", "-subject"]``. These are the rows ``agni check --format csv`` prints, so a workbook
        built from them matches the csv of the same run. ``agni.tables_to_xlsx`` writes them.
        """
        return self.call("TableService", "Tabulate", request, **fields)  # type: ignore[return-value]

    def list_relations(self, request: Optional[query_pb2.ListRelationsRequest] = None, **fields) -> query_pb2.ListRelationsResponse:
        return self.call("QueryService", "ListRelations", request, **fields)  # type: ignore[return-value]

    def diff_designs(self, request: Optional[diff_pb2.DiffDesignsRequest] = None, **fields) -> diff_pb2.DiffDesignsResponse:
        return self.call("DiffService", "DiffDesigns", request, **fields)  # type: ignore[return-value]

    def get_design(self, request: Optional[design_pb2.GetDesignRequest] = None, **fields) -> design_pb2.GetDesignResponse:
        return self.call("DesignService", "GetDesign", request, **fields)  # type: ignore[return-value]

    def trace_design(self, request: Optional[design_pb2.TraceDesignRequest] = None, **fields) -> design_pb2.TraceDesignResponse:
        """Follow a signal between two pins. ``from`` is a Python keyword, so pass the request
        message, or ``**{"from": ...}``."""
        return self.call("DesignService", "TraceDesign", request, **fields)  # type: ignore[return-value]

    def get_layout_report(self, request: Optional[design_pb2.GetLayoutReportRequest] = None, **fields) -> design_pb2.GetLayoutReportResponse:
        return self.call("DesignService", "GetLayoutReport", request, **fields)  # type: ignore[return-value]

    def create_review(self, request: Optional[review_pb2.CreateReviewRequest] = None, **fields) -> review_pb2.Review:
        """Score a checklist. Connect only until agni issue 734 lands."""
        return self.call("ReviewService", "CreateReview", request, **fields)  # type: ignore[return-value]

    def list_checklists(self, request: Optional[review_pb2.ListChecklistsRequest] = None, **fields) -> review_pb2.ListChecklistsResponse:
        """The checklists the design's project declares, inherited ones included, the project's
        default first. Each carries its manifest, which ``create_review(manifest=...)`` runs. An
        empty ``project`` means the design belongs to none."""
        return self.call("ReviewService", "ListChecklists", request, **fields)  # type: ignore[return-value]

    def render_review_report(self, request: Optional[review_pb2.RenderReviewReportRequest] = None, **fields) -> review_pb2.RenderReviewReportResponse:
        """A review this client holds, ``review=`` as ``create_review`` returned it, written as the
        checklist page ``agni review --format html`` writes. Nothing is re-run. Connect only."""
        return self.call("ReviewService", "RenderReviewReport", request, **fields)  # type: ignore[return-value]

    def get_review_manifest(self, request: Optional[review_pb2.GetReviewManifestRequest] = None, **fields) -> review_pb2.GetReviewManifestResponse:
        return self.call("ReviewService", "GetReviewManifest", request, **fields)  # type: ignore[return-value]

    def list_mounts(self, request: Optional[workspace_pb2.ListMountsRequest] = None, **fields) -> workspace_pb2.ListMountsResponse:
        return self.call("WorkspaceService", "ListMounts", request, **fields)  # type: ignore[return-value]
