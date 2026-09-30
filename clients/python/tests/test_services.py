from __future__ import annotations

import inspect
import pkgutil
import typing

import agni.v1.webapi as webapi
from agni import services
from agni.client import Client


def test_every_webapi_service_is_in_the_table():
    """services._MODULES is a hand-kept import list; a new service module must join it."""
    declared = set()
    for info in pkgutil.iter_modules(webapi.__path__):
        if not info.name.endswith("_pb2"):
            continue
        mod = __import__(f"agni.v1.webapi.{info.name}", fromlist=["DESCRIPTOR"])
        for svc in mod.DESCRIPTOR.services_by_name.values():
            declared |= {(svc.name, m.name) for m in svc.methods}
    assert declared, "no services found: the generated package is missing"
    assert declared == set(services.RPCS)


def test_named_methods_carry_their_rpc_types():
    """Each typed method's annotations match the rpc it calls, so the hints cannot lie."""
    src_rpc = {}
    for name, fn in inspect.getmembers(Client, inspect.isfunction):
        if name.startswith("_") or name == "call":
            continue
        body = inspect.getsource(fn)
        call = body[body.index("self.call(") :].split(",")[:2]
        service, method = (s.split('"')[1] for s in call)
        src_rpc[name] = services.lookup(service, method)
        hints = typing.get_type_hints(fn)
        assert hints["return"] is src_rpc[name].response, name
        assert typing.get_args(hints["request"])[0] is src_rpc[name].request, name
    assert len(src_rpc) >= 10
