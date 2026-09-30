"""The client: one method per rpc, the same types on either transport."""

from __future__ import annotations

from typing import Optional, Protocol

from google.protobuf.message import Message

from agni import services
from agni.v1.webapi import checks_pb2, design_pb2, diff_pb2, query_pb2, review_pb2, workspace_pb2


class Transport(Protocol):
    def call(self, rpc: services.Rpc, request: Message) -> Message: ...


class Client:
    """Asks agni questions through a transport.

    Switching between the CLI and a server changes the constructor argument and nothing else, since
    both return the rpc's own response message::

        c = Client(CliTransport(mounts={"tut": "examples/tutorial-project"}))
        c = Client(ConnectTransport("http://127.0.0.1:8080"))
        rows = c.run_query(uri="mount://tut/designs/gateway", query='component.class(?c,"resistor") => ?c')

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

    def check_design(self, request: Optional[checks_pb2.CheckDesignRequest] = None, **fields) -> checks_pb2.CheckDesignResponse:
        """Run the rule catalog. Over the CLI the response carries no ``verdicts``; see the README."""
        return self.call("CheckService", "CheckDesign", request, **fields)  # type: ignore[return-value]

    def get_check_report(self, request: Optional[checks_pb2.GetCheckReportRequest] = None, **fields) -> checks_pb2.GetCheckReportResponse:
        return self.call("CheckService", "GetCheckReport", request, **fields)  # type: ignore[return-value]

    def list_rules(self, request: Optional[checks_pb2.ListRulesRequest] = None, **fields) -> checks_pb2.ListRulesResponse:
        return self.call("CheckService", "ListRules", request, **fields)  # type: ignore[return-value]

    def run_query(self, request: Optional[query_pb2.RunQueryRequest] = None, **fields) -> query_pb2.RunQueryResponse:
        return self.call("QueryService", "RunQuery", request, **fields)  # type: ignore[return-value]

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

    def get_review_manifest(self, request: Optional[review_pb2.GetReviewManifestRequest] = None, **fields) -> review_pb2.GetReviewManifestResponse:
        return self.call("ReviewService", "GetReviewManifest", request, **fields)  # type: ignore[return-value]

    def list_mounts(self, request: Optional[workspace_pb2.ListMountsRequest] = None, **fields) -> workspace_pb2.ListMountsResponse:
        return self.call("WorkspaceService", "ListMounts", request, **fields)  # type: ignore[return-value]
