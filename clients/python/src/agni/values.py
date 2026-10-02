"""Values a query's variables are bound to."""

from __future__ import annotations

from typing import Dict, Mapping, Union

from agni.v1.webapi import query_pb2


def bindings(values: Mapping[str, Union[str, int, float]]) -> Dict[str, query_pb2.QueryValue]:
    """Turn plain values into a request's ``bindings`` (agni issue 793)::

        c.run_query(uri=..., query="component.net(?r, ?n) => ?n", bindings=agni.bindings({"r": "U1"}))

    A ``str`` binds text and an ``int`` or ``float`` a number. Where the query's relations type the
    variable, the engine reads the value as that type either way, so ``"3"`` against a count is 3
    and ``"abc"`` is refused. Where nothing types it, the Python type decides, as ``"3"`` versus
    ``3`` written into the query would. Naming a variable the query does not use is an error.
    """
    out: Dict[str, query_pb2.QueryValue] = {}
    for name, v in values.items():
        if isinstance(v, bool) or not isinstance(v, (str, int, float)):
            raise TypeError(f"binding {name}: want str, int or float, got {type(v).__name__}")
        out[name] = query_pb2.QueryValue(text=v) if isinstance(v, str) else query_pb2.QueryValue(number=float(v))
    return out
