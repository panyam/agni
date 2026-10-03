"""The rpc table, read off the generated descriptors rather than written down.

Every service in ``agni.v1.webapi`` and every method on it is looked up here by name, so a new rpc
is reachable through ``Client.call`` the moment ``make proto-py`` regenerates, with no edit to this
package.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Tuple, Type

from google.protobuf import message_factory
from google.protobuf.message import Message

# Imported for their side effect of registering descriptors in the default pool. The list is every
# webapi module that declares a service; test_services.py fails when a new one is missing here. The
# datasheet workbench's DatasheetService is the producer's API, in agni.v1.dsapi, and not part of the
# engine's (C34).
from agni.v1.webapi import (  # noqa: F401
    checks_pb2,
    design_pb2,
    diff_pb2,
    project_pb2,
    query_pb2,
    review_pb2,
    tables_pb2,
    workspace_pb2,
)

PACKAGE = "agni.v1.webapi"

_MODULES = (
    checks_pb2,
    design_pb2,
    diff_pb2,
    project_pb2,
    query_pb2,
    review_pb2,
    tables_pb2,
    workspace_pb2,
)


@dataclass(frozen=True)
class Rpc:
    """One method of one service, with the message classes it takes and returns."""

    service: str
    method: str
    request: Type[Message]
    response: Type[Message]

    @property
    def path(self) -> str:
        """The Connect path, ``/agni.v1.webapi.<Service>/<Method>``."""
        return f"/{PACKAGE}.{self.service}/{self.method}"


def _build() -> Dict[Tuple[str, str], Rpc]:
    table: Dict[Tuple[str, str], Rpc] = {}
    for mod in _MODULES:
        for svc in mod.DESCRIPTOR.services_by_name.values():
            for m in svc.methods:
                table[(svc.name, m.name)] = Rpc(
                    service=svc.name,
                    method=m.name,
                    request=message_factory.GetMessageClass(m.input_type),
                    response=message_factory.GetMessageClass(m.output_type),
                )
    return table


RPCS: Dict[Tuple[str, str], Rpc] = _build()


def lookup(service: str, method: str) -> Rpc:
    """The rpc named ``service``/``method``, where ``service`` is the bare name (``"QueryService"``).

    Raises ``KeyError`` naming the pair when agni declares no such rpc.
    """
    try:
        return RPCS[(service, method)]
    except KeyError:
        raise KeyError(f"agni declares no rpc {service}/{method}") from None
